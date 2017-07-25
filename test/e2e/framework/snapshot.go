package framework

import (
	"time"

	"github.com/appscode/go/crypto/rand"
	tapi "github.com/k8sdb/apimachinery/api"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return f.extClient.Snapshots(meta.Namespace).Get(meta.Name)
}

func (f *Framework) DeleteSnapshot(meta metav1.ObjectMeta) error {
	return f.extClient.Snapshots(meta.Namespace).Delete(meta.Name)
}

func (f *Framework) EventuallySnapshotSuccessed(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			snapshot, err := f.extClient.Snapshots(meta.Namespace).Get(meta.Name)
			Expect(err).NotTo(HaveOccurred())
			return snapshot.Status.Phase == tapi.SnapshotPhaseSuccessed
		},
		time.Minute*5,
		time.Second*5,
	)
}
