package controller

import (
	"reflect"
	"time"

	"github.com/golang/glog"
	tapi "github.com/k8sdb/postgres/api"
	tcs "github.com/k8sdb/postgres/client/clientset"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

type Controller struct {
	Client tcs.ExtensionInterface
	// sync time to sync the list.
	SyncPeriod time.Duration
}

func New(c *rest.Config) *Controller {
	return &Controller{
		Client:     tcs.NewExtensionsForConfigOrDie(c),
		SyncPeriod: time.Minute * 2,
	}
}

// Blocks caller. Intended to be called as a Go routine.
func (w *Controller) RunAndHold() {
	lw := &cache.ListWatch{
		ListFunc: func(opts api.ListOptions) (runtime.Object, error) {
			return w.Client.Postgres(api.NamespaceAll).List(api.ListOptions{})
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return w.Client.Postgres(api.NamespaceAll).Watch(api.ListOptions{})
		},
	}
	_, controller := cache.NewInformer(lw,
		&tapi.Postgres{},
		w.SyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				glog.Infoln("Got one added TPR", obj.(*tapi.Postgres))
				w.doStuff(obj.(*tapi.Postgres))
			},
			DeleteFunc: func(obj interface{}) {
				glog.Infoln("Got one deleted TPR", obj.(*tapi.Postgres))
				w.doStuff(obj.(*tapi.Postgres))
			},
			UpdateFunc: func(old, new interface{}) {
				oldObj, ok := old.(*tapi.Postgres)
				if !ok {
					return
				}
				newObj, ok := new.(*tapi.Postgres)
				if !ok {
					return
				}
				if !reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
					glog.Infoln("Got one updated TPR", newObj)
					w.doStuff(newObj)
				}
			},
		},
	)
	controller.Run(wait.NeverStop)
}

func (pl *Controller) doStuff(release *tapi.Postgres) {

}
