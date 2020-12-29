/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"time"

	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
)

func (f *Framework) EventuallyPVCCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	labelMap := map[string]string{
		meta_util.NameLabelKey:      api.Postgres{}.ResourceFQN(),
		meta_util.InstanceLabelKey:  meta.Name,
		meta_util.ManagedByLabelKey: kubedb.GroupName,
	}
	labelSelector := labels.SelectorFromSet(labelMap)
	return Eventually(
		func() int {
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			Expect(err).NotTo(HaveOccurred())
			return len(pvcList.Items)
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (i *Invocation) GetPersistentVolumeClaim() *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.app,
			Namespace: i.namespace,
			Annotations: map[string]string{
				"volume.beta.kubernetes.io/storage-class": i.StorageClass,
			},
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			StorageClassName: &i.StorageClass,
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("50Mi"),
				},
			},
		},
	}
}

func (i *Invocation) GetNamedPersistentVolumeClaim(name string) *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.app + name,
			Namespace: i.namespace,
			Annotations: map[string]string{
				"volume.beta.kubernetes.io/storage-class": i.StorageClass,
			},
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			StorageClassName: &i.StorageClass,
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("50Mi"),
				},
			},
		},
	}
}

func (i *Invocation) CreatePersistentVolumeClaim(pvc *core.PersistentVolumeClaim) error {
	_, err := i.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	return err
}

func (i *Invocation) DeletePersistentVolumeClaim(meta metav1.ObjectMeta) error {
	return i.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}
