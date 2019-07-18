package framework

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	"gomodules.xyz/stow"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	storage "kmodules.xyz/objectstore-api/osm"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

func (i *Invocation) Snapshot() *api.Snapshot {
	return &api.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(api.ResourceSingularSnapshot),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app":                 i.app,
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			},
		},
	}
}

func (f *Framework) CreateSnapshot(obj *api.Snapshot) error {
	_, err := f.dbClient.KubedbV1alpha1().Snapshots(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetSnapshot(meta metav1.ObjectMeta) (*api.Snapshot, error) {
	return f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) DeleteSnapshot(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallySnapshotPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.SnapshotPhase {
			snapshot, err := f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return snapshot.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotDataFound(snapshot *api.Snapshot) GomegaAsyncAssertion {
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

func (f *Framework) EventuallySnapshot(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return true
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {

	labelMap := map[string]string{
		api.LabelDatabaseKind: api.ResourceKindPostgres,
		api.LabelDatabaseName: meta.Name,
	}

	return Eventually(
		func() int {
			snapshotList, err := f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).List(metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			})
			Expect(err).NotTo(HaveOccurred())

			return len(snapshotList.Items)
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) EventuallyJobVolumeEmptyDirSize(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	jobName := fmt.Sprintf("%s-%s", api.DatabaseNamePrefix, meta.Name)
	return Eventually(
		func() string {
			job, err := f.kubeClient.BatchV1().Jobs(meta.Namespace).Get(jobName, metav1.GetOptions{})
			if err != nil {
				return err.Error()
			}

			found := false
			ed := core.EmptyDirVolumeSource{}
			for _, v := range job.Spec.Template.Spec.Volumes {
				if v.Name == "util-volume" && v.EmptyDir != nil {
					ed = *v.EmptyDir
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("empty directory util-volume not found in job spec").Error()
			}

			// match "0" if sizelimit is not given
			if ed.SizeLimit == nil {
				return "0"
			}
			return ed.SizeLimit.String()
		},
		time.Minute*5,
		time.Second*1,
	)
}

func (f *Framework) EventuallyJobPVCSize(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	jobName := fmt.Sprintf("%s-%s", api.DatabaseNamePrefix, meta.Name)
	return Eventually(
		func() string {
			job, err := f.kubeClient.BatchV1().Jobs(meta.Namespace).Get(jobName, metav1.GetOptions{})
			if err != nil {
				return err.Error()
			}

			found := false
			for _, v := range job.Spec.Template.Spec.Volumes {
				if v.Name == "util-volume" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("util-volume not found in job spec").Error()
			}

			pvc, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).Get(jobName, metav1.GetOptions{})
			if err != nil {
				return err.Error()
			}
			if sz, found := pvc.Spec.Resources.Requests[core.ResourceStorage]; found {
				return sz.String()
			}
			return ""
		},
		time.Minute*5,
		time.Second*1,
	)
}

func (f *Framework) checkSnapshotData(snapshot *api.Snapshot) (bool, error) {
	storageSpec := snapshot.Spec.Backend
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

	prefixLocation, _ := snapshot.Location() // error checked by .Container()
	prefix := filepath.Join(prefixLocation, snapshot.Name)
	prefix += "/" // A separator after prefix to prevent multiple snapshot's prefix matching. ref: https://github.com/kubedb/project/issues/377

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

func (f *Framework) CleanSnapshot() {
	snapshotList, err := f.dbClient.KubedbV1alpha1().Snapshots(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, s := range snapshotList.Items {
		if _, _, err := util.PatchSnapshot(f.dbClient.KubedbV1alpha1(), &s, func(in *api.Snapshot) *api.Snapshot {
			in.ObjectMeta.Finalizers = nil
			return in
		}); err != nil {
			fmt.Printf("error Patching Snapshot. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().Snapshots(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Snapshot. Error: %v", err)
	}
}
