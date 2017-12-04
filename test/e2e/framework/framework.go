package framework

import (
	"github.com/appscode/go/crypto/rand"
	cs "github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Framework struct {
	restConfig   *rest.Config
	kubeClient   kubernetes.Interface
	extClient    cs.KubedbV1alpha1Interface
	namespace    string
	name         string
	StorageClass string
}

func New(restConfig *rest.Config, kubeClient kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, storageClass string) *Framework {
	return &Framework{
		restConfig:   restConfig,
		kubeClient:   kubeClient,
		extClient:    extClient,
		name:         "postgres-operator",
		namespace:    rand.WithUniqSuffix("postgres"),
		StorageClass: storageClass,
	}
}

func (f *Framework) Invoke() *Invocation {
	return &Invocation{
		Framework: f,
		app:       rand.WithUniqSuffix("postgres-e2e"),
	}
}

type Invocation struct {
	*Framework
	app string
}
