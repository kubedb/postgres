package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const durationCheckPostgres = time.Minute * 30

func NewPostgres() *tapi.Postgres {
	postgres := &tapi.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name: rand.WithUniqSuffix("e2e-postgres"),
		},
		Spec: tapi.PostgresSpec{
			Version: "canary",
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
	if _, err := c.Client.CoreV1().Services(postgres.Namespace).Get(postgres.Name, metav1.GetOptions{}); err != nil {
		return err
	}

	// SatatefulSet for Postgres database
	statefulSetName := fmt.Sprintf("%v-%v", postgres.Name, tapi.ResourceCodePostgres)
	if _, err := c.Client.AppsV1beta1().StatefulSets(postgres.Namespace).Get(statefulSetName, metav1.GetOptions{}); err != nil {
		return err
	}

	podName := fmt.Sprintf("%v-%v", statefulSetName, 0)
	pod, err := c.Client.CoreV1().Pods(postgres.Namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// If job is success
	if pod.Status.Phase != apiv1.PodRunning {
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
