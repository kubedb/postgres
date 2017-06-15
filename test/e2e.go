package test

import (
	"fmt"
	"sync"
	"time"

	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/postgres/pkg/controller"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

			var config *rest.Config
			config, err = clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				err = fmt.Errorf("Could not get kubernetes config: %s", err)
				return
			}

			client := clientset.NewForConfigOrDie(config)
			extClient := tcs.NewForConfigOrDie(config)
			promClient, err := pcm.NewForConfig(config)
			if err != nil {
				err = err
				return
			}

			cronController := amc.NewCronController(client, extClient)
			// Start Cron
			cronController.StartCron()
			// Stop Cron
			e2eController.controller = controller.New(client, extClient, promClient, cronController, controller.Options{
				GoverningService: "kubedb",
			})
			e2eController.isControllerRunning = true
			go c.RunAndHold()

			time.Sleep(time.Second * 30)
		},
	)
	return
}
