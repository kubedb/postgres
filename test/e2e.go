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
	"net"
	"strconv"
)

type postgresController struct {
	isControllerRunning bool
	controller          *controller.Controller
	once                sync.Once
}

var e2eController = postgresController{isControllerRunning: false}

const (
	configPath = ""
	enableRbac = true
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
			port, _ := getAvailablePort()
			e2eController.controller = controller.New(client, extClient, promClient, cronController, controller.Options{
				GoverningService: "kubedb",
				EnableRbac:       enableRbac,
				EnableAnalytics:  false,
				Address:          fmt.Sprintf("127.0.0.1:%v", port),
			})
			e2eController.isControllerRunning = true
			c = e2eController.controller
			go c.RunAndHold()

			time.Sleep(time.Second * 30)
		},
	)
	return
}

func getAvailablePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}
