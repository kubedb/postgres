package job

import (
	"time"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	amc "github.com/kubedb/apimachinery/pkg/controller"
	"github.com/kubedb/apimachinery/pkg/eventer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type SnapshotDoer interface {
	GetDatabase(metav1.ObjectMeta) (runtime.Object, error)
	SetDatabaseStatus(metav1.ObjectMeta, api.DatabasePhase, string) error
}

type ControllerInterface interface {
	// client interface
	amc.ClientInterface
	// helper method for Job watcher
	SnapshotDoer
}

type Controller struct {
	ControllerInterface

	// ListOptions for watcher
	listOption metav1.ListOptions
	// Event Recorder
	eventRecorder record.EventRecorder
	// sync time to sync the list.
	syncPeriod time.Duration
	// Workqueue
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
	//Max number requests for retries
	maxNumRequests int
}

// NewController creates a new Controller
func NewController(
	controller ControllerInterface,
	listOption metav1.ListOptions,
	syncPeriod time.Duration,
) amc.ControllerInterface {

	// return new DormantDatabase Controller
	return &Controller{
		ControllerInterface: controller,
		listOption:          listOption,
		eventRecorder:       eventer.NewEventRecorder(controller.Client(), "Job Controller"),
		syncPeriod:          syncPeriod,
		maxNumRequests:      5,
	}
}

func (c *Controller) Run() {
	// Watch DormantDatabase with provided ListerWatcher
	c.watchJob()
}

func (c *Controller) watchJob() {

	c.initWatcher()

	stop := make(chan struct{})
	defer close(stop)

	c.runWatcher(5, stop)
	select {}
}
