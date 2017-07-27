package framework

import (
	"github.com/appscode/go/crypto/rand"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	clientset "k8s.io/client-go/kubernetes"
)

type Framework struct {
	kubeClient   clientset.Interface
	extClient    tcs.ExtensionInterface
	namespace    string
	name         string
	StorageClass string
}

func New(kubeClient clientset.Interface, extClient tcs.ExtensionInterface, storageClass string) *Framework {
	return &Framework{
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
