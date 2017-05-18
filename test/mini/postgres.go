package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
)

const durationCheckPostgres = time.Minute * 30

func NewPostgres() *tapi.Postgres {
	postgres := &tapi.Postgres{
		ObjectMeta: kapi.ObjectMeta{
			Name: rand.WithUniqSuffix("e2e-postgres"),
		},
		Spec: tapi.PostgresSpec{
			Version: "canary-db",
		},
	}
	return postgres
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

		log.Debugf("Pod Phase: %v", _postgres.Status.Phase)

		if _postgres.Status.Phase == tapi.DatabasePhaseRunning {
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
	statefulSetName := fmt.Sprintf("%v-%v", postgres.Name, tapi.ResourceCodePostgres)
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

func UpdatePostgres(c *controller.Controller, postgres *tapi.Postgres) (*tapi.Postgres, error) {
	return c.ExtClient.Postgreses(postgres.Namespace).Update(postgres)
}
