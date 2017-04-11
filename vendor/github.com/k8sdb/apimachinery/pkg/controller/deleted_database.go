package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/appscode/go/wait"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

type Deleter interface {
	// Check Database TPR
	Exists(*tapi.DeletedDatabase) (bool, error)
	// Delete operation
	DeleteDatabase(*tapi.DeletedDatabase) error
	// Destroy operation
	DestroyDatabase(*tapi.DeletedDatabase) error
}

type DeletedDatabaseController struct {
	// Kubernetes client
	client clientset.Interface
	// ThirdPartyExtension client
	extClient tcs.ExtensionInterface
	// Deleter interface
	deleter Deleter
	// ListerWatcher
	lw *cache.ListWatch
	// Event Recorder
	eventRecorder eventer.EventRecorderInterface
	// sync time to sync the list.
	syncPeriod time.Duration
}

// NewDeletedDbController creates a new DeletedDatabase Controller
func NewDeletedDbController(
	client clientset.Interface,
	extClient tcs.ExtensionInterface,
	deleter Deleter,
	lw *cache.ListWatch,
	syncPeriod time.Duration,
) *DeletedDatabaseController {
	// return new DeletedDatabase Controller
	return &DeletedDatabaseController{
		client:        client,
		extClient:     extClient,
		deleter:       deleter,
		lw:            lw,
		eventRecorder: eventer.NewEventRecorder(client, "DeletedDatabase Controller"),
		syncPeriod:    syncPeriod,
	}
}

func (c *DeletedDatabaseController) Run() {
	// Ensure DeletedDatabase TPR
	c.ensureThirdPartyResource()
	// Watch DeletedDatabase with provided ListerWatcher
	c.watch()
}

// Ensure DeletedDatabase ThirdPartyResource
func (c *DeletedDatabaseController) ensureThirdPartyResource() {
	log.Infoln("Ensuring DeletedDatabase ThirdPartyResource")

	resourceName := tapi.ResourceNameDeletedDatabase + "." + tapi.V1beta1SchemeGroupVersion.Group
	var err error
	if _, err = c.client.Extensions().ThirdPartyResources().Get(resourceName); err == nil {
		return
	}
	if !k8serr.IsNotFound(err) {
		log.Fatalln(err)
	}

	thirdPartyResource := &extensions.ThirdPartyResource{
		TypeMeta: unversioned.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "ThirdPartyResource",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name: resourceName,
		},
		Versions: []extensions.APIVersion{
			{
				Name: tapi.V1beta1SchemeGroupVersion.Version,
			},
		},
	}
	if _, err := c.client.Extensions().ThirdPartyResources().Create(thirdPartyResource); err != nil {
		log.Fatalln(err)
	}
}

func (c *DeletedDatabaseController) watch() {
	_, cacheController := cache.NewInformer(c.lw,
		&tapi.DeletedDatabase{},
		c.syncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				deletedDb := obj.(*tapi.DeletedDatabase)
				if deletedDb.Status.Created == nil {
					c.create(deletedDb)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				oldDeletedDb, ok := old.(*tapi.DeletedDatabase)
				if !ok {
					return
				}
				newDeletedDb, ok := new.(*tapi.DeletedDatabase)
				if !ok {
					return
				}
				// TODO: Find appropriate checking
				// Only allow if Spec varies
				if !reflect.DeepEqual(oldDeletedDb.Spec, newDeletedDb.Spec) {
					c.update(newDeletedDb)
				}
			},
		},
	)
	cacheController.Run(wait.NeverStop)
}

func (c *DeletedDatabaseController) create(deletedDb *tapi.DeletedDatabase) {

	// Set DeletedDatabase Phase: Deleting
	unversionedNow := unversioned.Now()
	deletedDb.Status.Created = &unversionedNow
	_deletedDb, err := c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to update DeletedDatabase. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, deletedDb)
		return
	}
	deletedDb = _deletedDb

	// Check if DB TPR object exists
	found, err := c.deleter.Exists(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to delete Database. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb)
		return
	}

	if found {
		message := "Failed to delete Database. Delete Database TPR object first"
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb)

		// Delete DeletedDatabase object
		if err := c.extClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			message := fmt.Sprintf(`Failed to delete DeletedDatabase. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb,
			)
			log.Errorln(err)
		}
		return
	}

	// Set DeletedDatabase Phase: Deleting
	unversionedNow = unversioned.Now()
	deletedDb.Status.Phase = tapi.PhaseDatabaseDeleting
	_deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to update DeletedDatabase. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, deletedDb)
		return
	}
	deletedDb = _deletedDb

	c.eventRecorder.PushEvent(kapi.EventTypeNormal, eventer.EventReasonDeleting, "Deleting Database", deletedDb)

	// Delete Database workload
	if err := c.deleter.DeleteDatabase(deletedDb); err != nil {
		message := fmt.Sprintf(`Failed to delete. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb)
		log.Errorln(err)
		return
	}

	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonSuccessfulDelete, "Successfully deleted Database workload",
		deletedDb,
	)

	// Set DeletedDatabase Phase: Deleted
	unversionedNow = unversioned.Now()
	deletedDb.Status.Deleted = &unversionedNow
	deletedDb.Status.Phase = tapi.PhaseDatabaseDeleted
	_, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to update DeletedDatabase. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, deletedDb)
		return
	}

	return
}

func (c *DeletedDatabaseController) update(deletedDb *tapi.DeletedDatabase) {
	if !deletedDb.Spec.Destroy {
		message := fmt.Sprintf(`Invalid update`)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonInvalidUpdate, message, deletedDb)
		return
	}

	// Check if DB TPR object exists
	found, err := c.deleter.Exists(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to destroy Database. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb)
		return
	}

	if found {
		message := "Failed to destroy Database. Delete Database TPR object first"
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDestroy, message, deletedDb)

		// Delete DeletedDatabase object
		if err := c.extClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			message := fmt.Sprintf(`Failed to delete DeletedDatabase. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, deletedDb,
			)
			log.Errorln(err)
		}
		return
	}

	// Set DeletedDatabase Phase: Destroying
	unversionedNow := unversioned.Now()
	deletedDb.Status.Phase = tapi.PhaseDatabaseDestroying
	_deletedDb, err := c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to update DeletedDatabase. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, deletedDb)
		return
	}
	deletedDb = _deletedDb

	// Destroy Database workload
	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonDestroying, "Destroying Database", deletedDb,
	)
	if err := c.deleter.DestroyDatabase(deletedDb); err != nil {
		message := fmt.Sprintf(`Failed to destroy. Reason: %v`, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToDestroy, message, deletedDb,
		)
		log.Errorln(err)
		return
	}

	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonSuccessfulDestroy,
		"Successfully destroyed Database workload", deletedDb,
	)

	// Set DeletedDatabase Phase: Deleted
	unversionedNow = unversioned.Now()
	deletedDb.Status.Destroyed = &unversionedNow
	deletedDb.Status.Phase = tapi.PhaseDatabaseDestroyed
	_, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb)
	if err != nil {
		message := fmt.Sprintf(`Failed to update DeletedDatabase. Reason: "%v"`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, deletedDb)
		return
	}
}
