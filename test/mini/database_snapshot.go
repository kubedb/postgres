package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	"github.com/ghodss/yaml"
	"github.com/graymeta/stow"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
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

		log.Debugf("DatabaseSnapshot Phase: %v", dbSnapshot.Status.Phase)

		if dbSnapshot.Status.Phase == tapi.SnapshotPhaseSuccessed {
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

const (
	keyProvider = "provider"
	keyConfig   = "config"
)

func CheckSnapshotData(c *controller.Controller, dbSnapshot *tapi.DatabaseSnapshot) (int, error) {
	secret, err := c.Client.Core().Secrets(dbSnapshot.Namespace).Get(dbSnapshot.Spec.StorageSecret.SecretName)
	if err != nil {
		return 0, err
	}

	provider := secret.Data[keyProvider]
	if provider == nil {
		return 0, errors.New("Missing provider key")
	}
	configData := secret.Data[keyConfig]
	if configData == nil {
		return 0, errors.New("Missing config key")
	}

	var config stow.ConfigMap
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return 0, err
	}

	loc, err := stow.Dial(string(provider), config)
	if err != nil {
		return 0, err
	}

	container, err := loc.Container(dbSnapshot.Spec.BucketName)
	if err != nil {
		return 0, err
	}

	folderName := dbSnapshot.Labels[amc.LabelDatabaseType] + "-" + dbSnapshot.Spec.DatabaseName
	prefix := fmt.Sprintf("%v/%v", folderName, dbSnapshot.Name)
	cursor := stow.CursorStart
	totalItem := 0
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return 0, err
		}

		totalItem = totalItem + len(items)

		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return totalItem, nil
}
