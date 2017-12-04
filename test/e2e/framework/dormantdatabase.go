package framework

import (
	"time"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1/util"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) GetDormantDatabase(meta metav1.ObjectMeta) (*api.DormantDatabase, error) {
	return f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) TryPatchDormantDatabase(meta metav1.ObjectMeta, transform func(*api.DormantDatabase) *api.DormantDatabase) (*api.DormantDatabase, error) {
	return util.TryPatchDormantDatabase(f.extClient, meta, transform)
}

func (f *Framework) DeleteDormantDatabase(meta metav1.ObjectMeta) error {
	return f.extClient.DormantDatabases(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallyDormantDatabase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDormantDatabaseStatus(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DormantDatabasePhase {
			drmn, err := f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if !kerr.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				return api.DormantDatabasePhase("")
			}
			return drmn.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}
