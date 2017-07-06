package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	"github.com/graymeta/stow"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/pkg/controller"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"github.com/k8sdb/apimachinery/pkg/storage"
)

const durationCheckSnapshot = time.Minute * 30

func CreateSnapshot(c *controller.Controller, namespace string, snapshotSpec tapi.SnapshotSpec) (*tapi.Snapshot, error) {
	snapshot := &tapi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("e2e-db-snapshot"),
			Namespace: namespace,
			Labels: map[string]string{
				tapi.LabelDatabaseKind: tapi.ResourceKindPostgres,
			},
		},
		Spec: snapshotSpec,
	}

	return c.ExtClient.Snapshots(namespace).Create(snapshot)
}

func CheckSnapshot(c *controller.Controller, snapshot *tapi.Snapshot) (bool, error) {
	doneChecking := false
	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckSnapshot {
		snapshot, err := c.ExtClient.Snapshots(snapshot.Namespace).Get(snapshot.Name)
		if err != nil {
			if kerr.IsNotFound(err) {
				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return false, err
			}
		}

		log.Debugf("Snapshot Phase: %v", snapshot.Status.Phase)

		if snapshot.Status.Phase == tapi.SnapshotPhaseSuccessed {
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

func CheckSnapshotData(c *controller.Controller, snapshot *tapi.Snapshot) (int, error) {
	storageSpec := snapshot.Spec.SnapshotStorageSpec
	cfg, err := storage.NewOSMContext(c.Client, storageSpec, snapshot.Namespace)
	if err != nil {
		return 0, err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return 0, err
	}
	containerID, err := storageSpec.Container()
	if err != nil {
		return 0, err
	}
	container, err := loc.Container(containerID)
	if err != nil {
		return 0, err
	}


	folderName, _ := snapshot.Location()
	prefix := fmt.Sprintf("%v/%v", folderName, snapshot.Name)
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

func CheckSnapshotScheduler(c *controller.Controller, postgres *tapi.Postgres) error {
	labelMap := map[string]string{
		tapi.LabelDatabaseKind: tapi.ResourceKindPostgres,
		tapi.LabelDatabaseName: postgres.Name,
	}

	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckSnapshot {
		snapshotList, err := c.ExtClient.Snapshots(postgres.Namespace).List(metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelMap).String(),
		})

		if err != nil {
			return err
		}

		if len(snapshotList.Items) >= 2 {
			return nil
		}

		time.Sleep(time.Second * 10)
		now = time.Now()
	}

	return errors.New("Scheduler is not working")
}
