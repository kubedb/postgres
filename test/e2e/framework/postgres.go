package framework

import (
	"fmt"
	"strconv"
	"time"

	"github.com/appscode/go/crypto/rand"
	jtypes "github.com/appscode/go/encoding/json/types"
	"github.com/appscode/go/types"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

var (
	JobPvcStorageSize = "2Gi"
	DBPvcStorageSize  = "1Gi"
)

const (
	kindEviction = "Eviction"
)

func (i *Invocation) Postgres() *api.Postgres {
	return &api.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(api.ResourceSingularPostgres),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.PostgresSpec{
			Version:  jtypes.StrYo(DBCatalogName),
			Replicas: types.Int32P(1),
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
		},
	}
}

func (f *Framework) CreatePostgres(obj *api.Postgres) error {
	_, err := f.dbClient.KubedbV1alpha1().Postgreses(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetPostgres(meta metav1.ObjectMeta) (*api.Postgres, error) {
	return f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchPostgres(meta metav1.ObjectMeta, transform func(postgres *api.Postgres) *api.Postgres) (*api.Postgres, error) {
	postgres, err := f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	postgres, _, err = util.PatchPostgres(f.dbClient.KubedbV1alpha1(), postgres, transform)
	return postgres, err
}

func (f *Framework) DeletePostgres(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) EventuallyPostgres(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
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

func (f *Framework) EventuallyPostgresPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresPodCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() int32 {
			st, err := f.kubeClient.AppsV1beta1().StatefulSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return -1
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return st.Status.ReadyReplicas
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			postgres, err := f.dbClient.KubedbV1alpha1().Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return postgres.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*15,
		time.Second*5,
	)
}

func (f *Framework) CleanPostgres() {
	postgresList, err := f.dbClient.KubedbV1alpha1().Postgreses(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range postgresList.Items {
		if _, _, err := util.PatchPostgres(f.dbClient.KubedbV1alpha1(), &e, func(in *api.Postgres) *api.Postgres {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}); err != nil {
			fmt.Printf("error Patching Postgres. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().Postgreses(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Postgres. Error: %v", err)
	}
}

func (f *Framework) EvictPodsFromStatefulSet(meta metav1.ObjectMeta) error {
	var err error
	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: api.GenericKey,
		api.LabelDatabaseKind:       api.ResourceKindPostgres,
		api.LabelDatabaseName:       meta.GetName(),
	}
	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable)
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}
