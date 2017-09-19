package framework

import (
	"errors"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func (f *Framework) EventuallyCRD() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			// Check Postgres TPR
			if _, err := f.extClient.Postgreses(apiv1.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD Postgres is not ready")
			}

			// Check Snapshots TPR
			if _, err := f.extClient.Snapshots(apiv1.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD Snapshot is not ready")
			}

			// Check DormantDatabases TPR
			if _, err := f.extClient.DormantDatabases(apiv1.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD DormantDatabase is not ready")
			}

			return nil
		},
		time.Minute*2,
		time.Second*10,
	)
}
