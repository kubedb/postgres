package test

import (
	"fmt"
	"sync"
	"time"

	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/postgres/pkg/controller"
	cgcmd "k8s.io/client-go/tools/clientcmd"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

type postgresController struct {
	isControllerRunning bool
	controller          *controller.Controller
	once                sync.Once
}

var e2eController = postgresController{isControllerRunning: false}

const (
	configPath = ""
)

func getController() (c *controller.Controller, err error) {

	// Controller is already running..
	if e2eController.isControllerRunning {
		c = e2eController.controller
		return
	}

	e2eController.once.Do(
		func() {
			fmt.Println("-- TestE2E: Waiting for controller")

			var config *restclient.Config
			config, err = clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				err = fmt.Errorf("Could not get kubernetes config: %s", err)
				return
			}

			client := clientset.NewForConfigOrDie(config)
			extClient := tcs.NewExtensionsForConfigOrDie(config)

			cgConfig, _err := cgcmd.BuildConfigFromFlags("", configPath)
			if _err != nil {
				err = _err
				return
			}

			promClient, err := pcm.NewForConfig(cgConfig)
			if err != nil {
				err = err
				return
			}

			c = controller.New(client, extClient, promClient, "canary-util", "kubedb")

			e2eController.controller = c
			e2eController.isControllerRunning = true
			go c.RunAndHold()

			time.Sleep(time.Second * 30)
		},
	)
	return
}
