package framework

import (
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

func (f *Framework) GetDormantDatabase(meta metav1.ObjectMeta) (*api.DormantDatabase, error) {
	return f.dbClient.KubedbV1alpha1().DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchDormantDatabase(meta metav1.ObjectMeta, transform func(*api.DormantDatabase) *api.DormantDatabase) (*api.DormantDatabase, error) {
	dormantDatabase, err := f.dbClient.KubedbV1alpha1().DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	dormantDatabase, _, err = util.PatchDormantDatabase(f.dbClient.KubedbV1alpha1(), dormantDatabase, transform)
	return dormantDatabase, err
}

func (f *Framework) DeleteDormantDatabase(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().DormantDatabases(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) EventuallyDormantDatabase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
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
			drmn, err := f.dbClient.KubedbV1alpha1().DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
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

func (f *Framework) EventuallyWipedOut(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() error {
			labelMap := map[string]string{
				api.LabelDatabaseName: meta.Name,
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			}
			labelSelector := labels.SelectorFromSet(labelMap)

			// check if pvcs is wiped out
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(pvcList.Items) > 0 {
				fmt.Println("PVCs have not wiped out yet")
				return fmt.Errorf("PVCs have not wiped out yet")
			}

			// check if snapshot is wiped out
			snapshotList, err := f.dbClient.KubedbV1alpha1().Snapshots(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(snapshotList.Items) > 0 {
				fmt.Println("all snapshots have not wiped out yet")
				return fmt.Errorf("all snapshots have not wiped out yet")
			}

			// check if secrets are wiped out
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(secretList.Items) > 0 {
				fmt.Println("secrets have not wiped out yet")
				return fmt.Errorf("secrets have not wiped out yet")
			}

			// check if appbinds are wiped out
			appBindingList, err := f.appCatalogClient.AppBindings(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(appBindingList.Items) > 0 {
				fmt.Println("appBindings have not wiped out yet")
				return fmt.Errorf("appBindings have not wiped out yet")
			}

			return nil
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) CleanDormantDatabase() {
	dormantDatabaseList, err := f.dbClient.KubedbV1alpha1().DormantDatabases(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, d := range dormantDatabaseList.Items {
		if _, _, err := util.PatchDormantDatabase(f.dbClient.KubedbV1alpha1(), &d, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.ObjectMeta.Finalizers = nil
			in.Spec.WipeOut = true
			return in
		}); err != nil {
			fmt.Printf("error Patching DormantDatabase. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().DormantDatabases(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Dormant Database. Error: %v", err)
	}
}
