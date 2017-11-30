package framework

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/encoding/json/types"
	"github.com/go-xorm/xorm"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	kutildb "github.com/k8sdb/apimachinery/client/typed/kubedb/v1alpha1/util"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Invocation) Postgres() *tapi.Postgres {
	return &tapi.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("postgres"),
			Namespace: f.namespace,
			Labels: map[string]string{
				"app": f.app,
			},
		},
		Spec: tapi.PostgresSpec{
			Version:  types.StrYo("9.6.5"),
			Replicas: 1,
		},
	}
}

func (f *Framework) CreatePostgres(obj *tapi.Postgres) error {
	_, err := f.extClient.Postgreses(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetPostgres(meta metav1.ObjectMeta) (*tapi.Postgres, error) {
	return f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) TryPatchPostgres(meta metav1.ObjectMeta, transform func(*tapi.Postgres) *tapi.Postgres) (*tapi.Postgres, error) {
	return kutildb.TryPatchPostgres(f.extClient, meta, transform)
}

func (f *Framework) DeletePostgres(meta metav1.ObjectMeta) error {
	return f.extClient.Postgreses(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallyPostgres(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
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
			postgres, err := f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return postgres.Status.Phase == tapi.DatabasePhaseRunning
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresClientReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	db, err := f.GetPostgresClient(meta)
	Expect(err).NotTo(HaveOccurred())

	return Eventually(
		func() bool {
			if err := f.CheckPostgres(db); err != nil {
				fmt.Println("---- ,", err)
				return false
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresTableCount(db *xorm.Engine) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			count, err := f.CountTable(db)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(count))
			return count
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresArchiveCount(db *xorm.Engine) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			count, err := f.CountArchive(db)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(count))
			return count
		},
		time.Minute*5,
		time.Second*5,
	)
}
