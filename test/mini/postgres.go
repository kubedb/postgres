package mini

import (
	"fmt"
	"time"

	"errors"
	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/postgres/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
)

const durationCheckPostgres = time.Minute * 30

func NewPostgres() *tapi.Postgres {
	postgres := &tapi.Postgres{
		ObjectMeta: kapi.ObjectMeta{
			Name:      rand.WithUniqSuffix("e2e-postgres"),
		},
		Spec: tapi.PostgresSpec{
			Version: "9.5-v4-db",
		},
	}
	return postgres
}

func ReCreatePostgres(c *controller.Controller, postgres *tapi.Postgres) (*tapi.Postgres, error) {
	_postgres := &tapi.Postgres{
		ObjectMeta: kapi.ObjectMeta{
			Name:        postgres.Name,
			Namespace:   postgres.Namespace,
			Labels:      postgres.Labels,
			Annotations: postgres.Annotations,
		},
		Spec:   postgres.Spec,
		Status: postgres.Status,
	}

	return c.ExtClient.Postgreses(_postgres.Namespace).Create(_postgres)
}

func CheckPostgresStatus(c *controller.Controller, postgres *tapi.Postgres) (bool, error) {
	postgresReady := false
	then := time.Now()
	now := time.Now()
	for now.Sub(then) < durationCheckPostgres {
		_postgres, err := c.ExtClient.Postgreses(postgres.Namespace).Get(postgres.Name)
		if err != nil {
			return false, err
		}

		log.Debugf("Pod Phase: %v", _postgres.Status.DatabaseStatus)

		if _postgres.Status.DatabaseStatus == tapi.StatusDatabaseRunning {
			postgresReady = true
			break
		}
		time.Sleep(time.Minute)
		now = time.Now()

	}

	if !postgresReady {
		return false, nil
	}

	return true, nil
}

func CheckPostgresWorkload(c *controller.Controller, postgres *tapi.Postgres) error {
	if _, err := c.Client.Core().Services(postgres.Namespace).Get(postgres.Name); err != nil {
		return err
	}

	// SatatefulSet for Postgres database
	statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, postgres.Name)
	if _, err := c.Client.Apps().StatefulSets(postgres.Namespace).Get(statefulSetName); err != nil {
		return err
	}

	podName := fmt.Sprintf("%v-%v", statefulSetName, 0)
	pod, err := c.Client.Core().Pods(postgres.Namespace).Get(podName)
	if err != nil {
		return err
	}

	// If job is success
	if pod.Status.Phase != kapi.PodRunning {
		return errors.New("Pod is not running")
	}

	return nil
}

func DeletePostgres(c *controller.Controller, postgres *tapi.Postgres) error {
	return c.ExtClient.Postgreses(postgres.Namespace).Delete(postgres.Name)
}

func UpdatePostres(c *controller.Controller, postgres *tapi.Postgres) (*tapi.Postgres, error) {
	return c.ExtClient.Postgreses(postgres.Namespace).Update(postgres)
}
