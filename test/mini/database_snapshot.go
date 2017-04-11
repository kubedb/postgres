package mini

import (
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
)

const durationCheckDatabaseSnapshot = time.Minute * 30

func CreateDatabaseSnapshot(c *controller.Controller, namespace string, snapshotSpec tapi.DatabaseSnapshotSpec) (*tapi.DatabaseSnapshot, error) {
	dbSnapshot := &tapi.DatabaseSnapshot{
		ObjectMeta: kapi.ObjectMeta{
			Name:      rand.WithUniqSuffix("e2e-db-snapshot"),
			Namespace: namespace,
			Labels: map[string]string{
				"k8sdb.com/type": "postgres",
			},
		},
		Spec: snapshotSpec,
	}

	return c.ExtClient.DatabaseSnapshots(namespace).Create(dbSnapshot)
}

func CheckDatabaseSnapshot(c *controller.Controller, dbSnapshot *tapi.DatabaseSnapshot) (bool, error) {
	doneChecking := false
	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckDatabaseSnapshot {
		dbSnapshot, err := c.ExtClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name)
		if err != nil {
			if k8serr.IsNotFound(err) {
				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return false, err
			}
		}

		log.Debugf("DatabaseSnapshot Phase: %v", dbSnapshot.Status.Status)

		if dbSnapshot.Status.Status == tapi.StatusSnapshotSuccessed {
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
