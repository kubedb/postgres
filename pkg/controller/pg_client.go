package controller

import (
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
	"gomodules.xyz/x/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/certholder"
)

func (c *Controller) GetPostgresClient(db *api.Postgres, dnsName string, port int32) (*xorm.Engine, error) {
	fmt.Println("....................i am here")
	user, pass, err := c.GetPostgresAuthCredentials(db)
	if err != nil {
		return nil, fmt.Errorf("DB basic auth is not found for PostgreSQL %v/%v", db.Namespace, db.Name)
	}
	cnnstr := ""
	if db.Spec.TLS != nil {
		secretName := db.MustCertSecretName(api.PostgresClientCert)

		certSecret, err := c.Client.CoreV1().Secrets(db.Namespace).Get(context.TODO(), secretName, metav1.GetOptions{})

		if err != nil {
			log.Error(err, "failed to get certificate secret.", secretName)
			return nil, err
		}

		certs, _ := certholder.DefaultHolder.ForResource(api.SchemeGroupVersion.WithResource(api.ResourcePluralPostgres), db.ObjectMeta)
		paths, err := certs.Save(certSecret)
		if err != nil {
			log.Error(err, "failed to save certificate")
			return nil, err
		}
		if db.Spec.ClientAuthMode == api.ClientAuthModeCert {
			fmt.Println("....................",paths.Key,"...........cert",paths.Cert,"...............ca",paths.CACert)
			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=postgres sslmode=%s sslrootcert=%s sslcert=%s sslkey=%s", user, pass, dnsName, port, db.Spec.SSLMode, paths.CACert, paths.Cert, paths.Key)
		} else {
			fmt.Println("...................................ca:",paths.CACert)
			cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=postgres sslmode=%s sslrootcert=%s", user, pass, dnsName, port, db.Spec.SSLMode, paths.CACert)
		}
	} else {
		cnnstr = fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=postgres sslmode=%s", user, pass, dnsName, port, db.Spec.SSLMode)
	}

	return xorm.NewEngine("postgres", cnnstr)
}

