package framework

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/graymeta/stow"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/k8sdb/apimachinery/pkg/storage"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (f *Invocation) Snapshot() *tapi.Snapshot {
	return &tapi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("snapshot"),
			Namespace: f.namespace,
			Labels: map[string]string{
				"app": f.app,
				tapi.LabelDatabaseKind: tapi.ResourceKindPostgres,
			},
		},
	}
}

func (f *Framework) CreateSnapshot(obj *tapi.Snapshot) error {
	_, err := f.extClient.Snapshots(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetSnapshot(meta metav1.ObjectMeta) (*tapi.Snapshot, error) {
	return f.extClient.Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) DeleteSnapshot(meta metav1.ObjectMeta) error {
	return f.extClient.Snapshots(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallySnapshotPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() tapi.SnapshotPhase {
			snapshot, err := f.extClient.Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(snapshot.Status.Phase).ToNot(Equal(tapi.SnapshotPhaseFailed))
			return snapshot.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotDataFound(snapshot *tapi.Snapshot) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			found, err := f.checkSnapshotData(snapshot)
			Expect(err).NotTo(HaveOccurred())
			return found
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {

	labelMap := map[string]string{
		tapi.LabelDatabaseKind: tapi.ResourceKindPostgres,
		tapi.LabelDatabaseName: meta.Name,
	}

	return Eventually(
		func() int {
			snapshotList, err := f.extClient.Snapshots(meta.Namespace).List(metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			})
			Expect(err).NotTo(HaveOccurred())

			return len(snapshotList.Items)
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) checkSnapshotData(snapshot *tapi.Snapshot) (bool, error) {
	storageSpec := snapshot.Spec.SnapshotStorageSpec
	cfg, err := storage.NewOSMContext(f.kubeClient, storageSpec, snapshot.Namespace)
	if err != nil {
		return false, err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return false, err
	}
	containerID, err := storageSpec.Container()
	if err != nil {
		return false, err
	}
	container, err := loc.Container(containerID)
	if err != nil {
		return false, err
	}

	folderName, _ := snapshot.Location()
	prefix := fmt.Sprintf("%v/%v", folderName, snapshot.Name)
	cursor := stow.CursorStart
	totalItem := 0
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return false, err
		}

		totalItem = totalItem + len(items)

		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return totalItem != 0, nil
}
