package framework

import (
	"time"

	tapi "github.com/k8sdb/apimachinery/api"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) EventuallyTPR() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			// Check Postgres TPR
			_, err := f.kubeClient.ExtensionsV1beta1().ThirdPartyResources().Get(
				tapi.ResourceNamePostgres+"."+tapi.V1alpha1SchemeGroupVersion.Group,
				metav1.GetOptions{},
			)
			if err != nil {
				if kerr.IsNotFound(err) {
					return err
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Check DormantDatabase TPR
			_, err = f.kubeClient.ExtensionsV1beta1().ThirdPartyResources().Get(
				tapi.ResourceNameDormantDatabase+"."+tapi.V1alpha1SchemeGroupVersion.Group,
				metav1.GetOptions{},
			)
			if err != nil {
				if kerr.IsNotFound(err) {
					return err
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}

			// Check Snapshot TPR
			_, err = f.kubeClient.ExtensionsV1beta1().ThirdPartyResources().Get(
				tapi.ResourceNameSnapshot+"."+tapi.V1alpha1SchemeGroupVersion.Group,
				metav1.GetOptions{},
			)
			if err != nil {
				if kerr.IsNotFound(err) {
					return err
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}

			return nil
		},
		time.Minute*2,
		time.Second*5,
	)
}
