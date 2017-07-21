package framework

import (
	"errors"
	"time"

	tapi "github.com/k8sdb/apimachinery/api"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (f *Framework) EventuallyTPR() GomegaAsyncAssertion {
	label := map[string]string{
		"app": tapi.DatabaseNamePrefix,
	}

	return Eventually(
		func() error {
			tprList, err := f.kubeClient.ExtensionsV1beta1().ThirdPartyResources().List(
				metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(label).String(),
				},
			)
			if err != nil {
				return err
			}

			if len(tprList.Items) != 3 {
				return errors.New("All ThirdPartyResources are not ready")
			}
			return nil
		},
		time.Minute*2,
		time.Second*5,
	)
}
