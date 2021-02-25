package controller

import (
	"github.com/go-pg/pg"

	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"gomodules.xyz/x/log"
	"sync"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"

	_ "github.com/lib/pq"

	"github.com/go-xorm/xorm"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kmapi "kmodules.xyz/client-go/api/v1"
	core_util "kmodules.xyz/client-go/core/v1"
)
func (c *Controller) RunHealthChecker(stopCh <-chan struct{}) {
	// As CheckPostgresDBHealth() is a blocking function,
	// run it on a go-routine.
	go c.CheckPostgresDBHealth(stopCh)
}

func (c *Controller) CheckPostgresDBHealth(stopCh <-chan struct{}) {
	go wait.Until(func() {
		dbList, err := c.pgLister.Postgreses(core.NamespaceAll).List(labels.Everything())
		if err != nil {
			glog.Errorf("Failed to list PostgreSQL objects with: %s", err.Error())
			return
		}

		var wg sync.WaitGroup
		for idx := range dbList {
			db := dbList[idx]

			if db.DeletionTimestamp != nil {
				continue
			}

			wg.Add(1)
			go func() {
				defer func() {
					wg.Done()
				}()
				podList, err := c.Client.CoreV1().Pods(db.Namespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.Set(db.OffshootSelectors()).String(),
				})
				if err != nil {
					glog.Warning("Failed to list DB pod with ", err.Error())
					return
				}

				for _, pod := range podList.Items {
					if core_util.IsPodConditionTrue(pod.Status.Conditions, core_util.PodConditionTypeReady) {
						continue
					}

					engine, err := c.getPostgreSQLClient(db, HostDNS(db, pod.ObjectMeta), api.PostgresDatabasePort)
					if err != nil {
						glog.Warning("Failed to get db client for host ", pod.Namespace, "/", pod.Name)
						continue
					}

					func(engine *xorm.Engine) {
						defer func() {
							if engine != nil {
								err = engine.Close()
								if err != nil {
									glog.Errorf("Can't close the engine. error: %v", err)
								}
							}
						}()

						isHostOnline, err := c.isHostOnline(db, engine)
						if err != nil {
							glog.Warning("Host is not online ", err.Error())
							return
						}

						if isHostOnline {
							pod.Status.Conditions = core_util.SetPodCondition(pod.Status.Conditions, core.PodCondition{
								Type:               core_util.PodConditionTypeReady,
								Status:             core.ConditionTrue,
								LastTransitionTime: metav1.Now(),
								Reason:             "DBConditionTypeReadyAndServerOnline",
								Message:            "DB is ready because of server getting Online and Running state",
							})
							_, err = c.Client.CoreV1().Pods(pod.Namespace).UpdateStatus(context.TODO(), &pod, metav1.UpdateOptions{})
							if err != nil {
								glog.Warning("Failed to update pod status with: ", err.Error())
								return
							}
						}
					}(engine)
				}

				// verify db is going to accepting connection and in ready state
				port, err := c.GetPrimaryServicePort(db)
				if err != nil {
					glog.Warning("Failed to primary service port with: ", err.Error())
					return
				}
				engine, err := c.getPostgreSQLClient(db, PrimaryServiceDNS(db), port)
				if err != nil {
					// Since the client was unable to connect the database,
					// update "AcceptingConnection" to "false".
					// update "Ready" to "false"
					_, err = util.UpdatePostgresStatus(
						context.TODO(),
						c.DBClient.KubedbV1alpha2(),
						db.ObjectMeta,
						func(in *api.PostgresStatus) (types.UID, *api.PostgresStatus) {
							in.Conditions = kmapi.SetCondition(in.Conditions,
								kmapi.Condition{
									Type:               api.DatabaseAcceptingConnection,
									Status:             core.ConditionFalse,
									Reason:             api.DatabaseNotAcceptingConnectionRequest,
									ObservedGeneration: db.Generation,
									Message:            fmt.Sprintf("The PostgreSQL: %s/%s is not accepting client requests, reason: %s", db.Namespace, db.Name, err.Error()),
								})
							in.Conditions = kmapi.SetCondition(in.Conditions,
								kmapi.Condition{
									Type:               api.DatabaseReady,
									Status:             core.ConditionFalse,
									Reason:             api.ReadinessCheckFailed,
									ObservedGeneration: db.Generation,
									Message:            fmt.Sprintf("The PostgreSQL: %s/%s is not ready.", db.Namespace, db.Name),
								})
							return db.UID, in
						},
						metav1.UpdateOptions{},
					)
					if err != nil {
						glog.Errorf("Failed to update status for PostgreSQL: %s/%s", db.Namespace, db.Name)
					}
					// Since the client isn't created, skip rest operations.
					return
				}
				defer func() {
					if engine != nil {
						err = engine.Close()
						if err != nil {
							glog.Errorf("Can't close the engine. error: %v", err)
						}
					}
				}()
				// While creating the client, we perform a health check along with it.
				// If the client is created without any error,
				// the database is accepting connection.
				// Update "AcceptingConnection" to "true".
				_, err = util.UpdatePostgresStatus(
					context.TODO(),
					c.DBClient.KubedbV1alpha2(),
					db.ObjectMeta,
					func(in *api.PostgresStatus) (types.UID, *api.PostgresStatus) {
						in.Conditions = kmapi.SetCondition(in.Conditions,
							kmapi.Condition{
								Type:               api.DatabaseAcceptingConnection,
								Status:             core.ConditionTrue,
								Reason:             api.DatabaseAcceptingConnectionRequest,
								ObservedGeneration: db.Generation,
								Message:            fmt.Sprintf("The PostgreSQL: %s/%s is accepting client requests.", db.Namespace, db.Name),
							})
						return db.UID, in
					},
					metav1.UpdateOptions{},
				)
				if err != nil {
					glog.Errorf("Failed to update status for PostgreSQL: %s/%s", db.Namespace, db.Name)
					// Since condition update failed, skip remaining operations.
					return
				}

				// check PostgreSQL database health
				var isHealthy bool
				if *db.Spec.Replicas > int32(1)  {
					isHealthy, err = c.checkPostgreSQLClusterHealth(db, engine)
					if err != nil {
						glog.Errorf("PostgreSQL Cluster %s/%s is not healthy, reason: %s", db.Namespace, db.Name, err.Error())
					}
				} else {
					isHealthy, err = c.checkPostgreSQLStandaloneHealth(engine)
					if err != nil {
						glog.Errorf("PostgreSQL standalone %s/%s is not healthy, reason: %s", db.Namespace, db.Name, err.Error())
					}
				}

				if !isHealthy {
					// Since the get status failed, skip remaining operations.
					return
				}
				// database is healthy. So update to "Ready" condition to "true"
				_, err = util.UpdatePostgresStatus(
					context.TODO(),
					c.DBClient.KubedbV1alpha2(),
					db.ObjectMeta,
					func(in *api.PostgresStatus) (types.UID, *api.PostgresStatus) {
						in.Conditions = kmapi.SetCondition(in.Conditions,
							kmapi.Condition{
								Type:               api.DatabaseReady,
								Status:             core.ConditionTrue,
								Reason:             api.ReadinessCheckSucceeded,
								ObservedGeneration: db.Generation,
								Message:            fmt.Sprintf("The PostgreSQL: %s/%s is ready.", db.Namespace, db.Name),
							})
						return db.UID, in
					},
					metav1.UpdateOptions{},
				)
				if err != nil {
					glog.Errorf("Failed to update status for PostgreSQL: %s/%s", db.Namespace, db.Name)
				}

			}()
		}
		wg.Wait()
	}, c.ReadinessProbeInterval, stopCh)

	// will wait here until stopCh is closed.
	<-stopCh
	glog.Info("Shutting down PostgreSQL health checker...")

}
func (c *Controller) checkPostgreSQLClusterHealth(db *api.Postgres, engine *xorm.Engine) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	session := engine.NewSession()
	session.Context(ctx)
	defer cancel()
	defer session.Close()
	// sql queries for checking cluster healthiness
	// 1. ping database
	_, err := session.QueryString("SELECT now();")
	if err != nil {
		return false, err
	}

	// 2. check all nodes are in ONLINE
	podList, err := c.Client.CoreV1().Pods(db.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(db.OffshootSelectors()).String(),
	})
	for _,pod :=  range podList.Items {
		eng, err := c.getPostgreSQLClient(db, HostDNS(db, pod.ObjectMeta), api.PostgresDatabasePort)
		if err != nil {
			glog.Warning("Failed to get db client for host ", pod.Namespace, "/", pod.Name)
			continue
		}

		err = func(eng *xorm.Engine) error {
			defer func() {
				if eng != nil {
					err = eng.Close()
					if err != nil {
						glog.Errorf("Can't close the engine. error: %v", err)
					}
				}
			}()

			isHostOnline, err := c.isHostOnline(db, eng)
			if err != nil || !isHostOnline {
				glog.Warning("Host is not online ", err.Error())
				return fmt.Errorf("Host  is not online" )
			}
			return nil

		}(eng)

		if err != nil {
			return false, err
		}
	}

	// 3. check replicas data sync with master
	//TODO
	return true, nil
}

func (c *Controller) checkPostgreSQLStandaloneHealth(engine *xorm.Engine) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	session := engine.NewSession()
	session.Context(ctx)
	defer cancel()
	defer session.Close()
	// sql queries for checking standalone healthiness
	// 1. ping database
	_, err := session.QueryString("SELECT now();")
	if err != nil {
		return false, err
	}
	return true, nil
}


func (c *Controller) GetPostgresAuthCredentials(db *api.Postgres) (string, string, error) {
	if db.Spec.AuthSecret == nil {
		return "", "", errors.New("no database secret")
	}
	secret, err := c.Client.CoreV1().Secrets(db.Namespace).Get(context.TODO(), db.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	return string(secret.Data[core.BasicAuthUsernameKey]), string(secret.Data[core.BasicAuthPasswordKey]), nil
}

func (c *Controller) updateErrorAcceptingConnections(db *api.Postgres, connectionErr error) {
	_, err := util.UpdatePostgresStatus(
		context.TODO(),
		c.DBClient.KubedbV1alpha2(),
		db.ObjectMeta,
		func(in *api.PostgresStatus) (types.UID, *api.PostgresStatus) {
			in.Conditions = kmapi.SetCondition(in.Conditions,
				kmapi.Condition{
					Type:               api.DatabaseAcceptingConnection,
					Status:             core.ConditionFalse,
					Reason:             api.DatabaseNotAcceptingConnectionRequest,
					ObservedGeneration: db.Generation,
					Message:            fmt.Sprintf("The PostgreSQL: %s/%s is not accepting client requests. error: %s", db.Namespace, db.Name, connectionErr),
				})
			in.Conditions = kmapi.SetCondition(in.Conditions,
				kmapi.Condition{
					Type:               api.DatabaseReady,
					Status:             core.ConditionFalse,
					Reason:             api.ReadinessCheckFailed,
					ObservedGeneration: db.Generation,
					Message:            fmt.Sprintf("The PostgreSQL: %s/%s is not ready.", db.Namespace, db.Name),
				})
			return db.UID, in
		},
		metav1.UpdateOptions{},
	)
	if err != nil {
		glog.Errorf("Failed to update status for PostgreSQL: %s/%s", db.Namespace, db.Name)
	}
}

func (c *Controller) updateDatabaseReady(db *api.Postgres) {
	_, err := util.UpdatePostgresStatus(
		context.TODO(),
		c.DBClient.KubedbV1alpha2(),
		db.ObjectMeta,
		func(in *api.PostgresStatus) (types.UID, *api.PostgresStatus) {
			in.Conditions = kmapi.SetCondition(in.Conditions,
				kmapi.Condition{
					Type:               api.DatabaseReady,
					Status:             core.ConditionTrue,
					Reason:             api.ReadinessCheckSucceeded,
					ObservedGeneration: db.Generation,
					Message:            fmt.Sprintf("The PostgreSQL: %s/%s is ready.", db.Namespace, db.Name),
				})
			return db.UID, in
		},
		metav1.UpdateOptions{},
	)
	if err != nil {
		glog.Errorf("Failed to update status for PostgreSQL: %s/%s", db.Namespace, db.Name)
	}
}

// emon
func (c *Controller) isHostOnline(db *api.Postgres, eng *xorm.Engine) (bool, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	session := eng.NewSession()
	session.Context(ctx)
	defer cancel()
	defer session.Close()
	// 1. ping for both standalone and group replication member
	_, err := session.QueryString("SELECT now();")
	if err != nil {
		return false, err
	}


	return true, nil
}
func (c *Controller) getPostgreSQLClient1(db *api.Postgres, dns string, port int32) (*xorm.Engine, error) {
	user, pass, err := c.GetPostgresAuthCredentials(db)
	if err != nil {
		return nil, fmt.Errorf("DB basic auth is not found for PostgreSQL %v/%v", db.Namespace, db.Name)
	}

	opt := &pg.Options{
		Addr:      fmt.Sprintf("%s:%d", dns, port),
		Database:  "postgres",
		User:      user,
		Password: pass,
	}

	if db.Spec.TLS != nil {
		serverSecret, err := c.Client.CoreV1().Secrets(db.Namespace).Get(context.TODO(), db.MustCertSecretName(api.PostgresServerCert), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		cacrt := serverSecret.Data[core.ServiceAccountRootCAKey]
		CACertPool := x509.NewCertPool()
		CACertPool.AppendCertsFromPEM(cacrt)
		// tls custom setup
		if db.Spec.ClientAuthMode == api.ClientAuthModeCert {
			clientCert := serverSecret.Data[core.TLSCertKey]
			clientKey := serverSecret.Data[core.TLSPrivateKeyKey]
			cert, err := tls.X509KeyPair(clientCert, clientKey)
			if err != nil {
				log.Errorf("failed to load client certificate: %v", err)
			}

			tlsConfig := &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            CACertPool,
				InsecureSkipVerify: true,
			}
			opt.TLSConfig = tlsConfig
		} else {
			tlsConfig := &tls.Config{
				RootCAs:            CACertPool,
				InsecureSkipVerify: true,
			}
			opt.TLSConfig = tlsConfig
		}
	}

	DB := pg.Connect(opt)
	var num int
	_, err = DB.QueryOne(pg.Scan(&num), `SELECT 1+1`)
	if err != nil {
		log.Errorf("failed to query db: %v", err)
	}

	log.Infof("DB string: %v", DB.String())
	defer DB.Close()
	fmt.Printf(".......................query resp : %v\n", num)
	return nil,nil
}

func (c *Controller) getPostgreSQLClient(db *api.Postgres, dns string, port int32) (*xorm.Engine, error) {
	user, pass, err := c.GetPostgresAuthCredentials(db)
	if err != nil {
		return nil, fmt.Errorf("DB basic auth is not found for PostgreSQL %v/%v", db.Namespace, db.Name)
	}
	cnnstr := ""
	if db.Spec.TLS != nil {
		serverSecret, err := c.Client.CoreV1().Secrets(db.Namespace).Get(context.TODO(), db.MustCertSecretName(api.PostgresServerCert), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		cacrt := serverSecret.Data[core.ServiceAccountRootCAKey]

		// tls custom setup
		if db.Spec.ClientAuthMode == api.ClientAuthModeCert {
			clientCert := serverSecret.Data[core.TLSCertKey]
			clientKey := serverSecret.Data[core.TLSPrivateKeyKey]

			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=5432 dbname=postgres replication=database sslmode=%s sslrootcert=%s sslcert=%s sslkey=%s", user, pass, dns, db.Spec.SSLMode,cacrt,clientCert, clientKey)

		} else {
			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=5432 dbname=postgres replication=database sslmode=%s sslrootcert=%s", user, pass, dns, db.Spec.SSLMode,cacrt)
		}
	} else {
		cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=5432 dbname=postgres replication=database sslmode=%s", user, pass, dns, db.Spec.SSLMode)
	}

	engine, err := xorm.NewEngine(api.ResourceSingularPostgres, cnnstr)
	if err != nil {
		return engine, err
	}
	return engine, nil
}
func HostDNS(db *api.Postgres, podMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%v.%v.%v.svc", podMeta.Name, db.GoverningServiceName(), podMeta.Namespace)
}
func PrimaryServiceDNS(db *api.Postgres)  string{
	 return fmt.Sprintf("%v.%v.svc",  db.ServiceName(), db.Namespace)
}