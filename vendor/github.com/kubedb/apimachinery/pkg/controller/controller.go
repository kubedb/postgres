package controller

import (
	cs "github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type ControllerInterface interface {
	Run()
}

type ClientInterface interface {
	Client() kubernetes.Interface
	ApiExtKubeClient() crd_cs.ApiextensionsV1beta1Interface
	ExtClient() cs.KubedbV1alpha1Interface
}

type Controller struct {
	// Kubernetes client
	client kubernetes.Interface
	// Api Extension Client
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface
	// ThirdPartyExtension client
	extClient cs.KubedbV1alpha1Interface
}

func (c *Controller) Client() kubernetes.Interface {
	return c.client
}

func (c *Controller) ApiExtKubeClient() crd_cs.ApiextensionsV1beta1Interface {
	return c.apiExtKubeClient
}

func (c *Controller) ExtClient() cs.KubedbV1alpha1Interface {
	return c.extClient
}
