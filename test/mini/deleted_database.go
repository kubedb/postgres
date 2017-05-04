package mini

import (
	"time"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/pkg/controller"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
)

const durationCheckDeletedDatabase = time.Minute * 30

func CheckDeletedDatabasePhase(c *controller.Controller, postgres *tapi.Postgres, phase tapi.DeletedDatabasePhase) (bool, error) {
	doneChecking := false
	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckDeletedDatabase {
		deletedDb, err := c.ExtClient.DeletedDatabases(postgres.Namespace).Get(postgres.Name)
		if err != nil {
			if k8serr.IsNotFound(err) {
				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return false, err
			}
		}

		log.Debugf("DeletedDatabase Phase: %v", deletedDb.Status.Phase)

		if deletedDb.Status.Phase == phase {
			doneChecking = true
			break
		}

		time.Sleep(time.Minute)
		now = time.Now()

	}

	if !doneChecking {
		return false, nil
	}

	return true, nil
}

func DeleteDeletedDatabase(c *controller.Controller, postgres *tapi.Postgres) error {
	return c.ExtClient.DeletedDatabases(postgres.Namespace).Delete(postgres.Name)
}
