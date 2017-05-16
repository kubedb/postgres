package controller

import (
	"errors"
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
	"k8s.io/kubernetes/pkg/client/record"
)

type Deleter interface {
	// Check Database TPR
	Exists(*kapi.ObjectMeta) (bool, error)
	// Delete operation
	DeleteDatabase(*tapi.DeletedDatabase) error
	// Wipe out operation
	WipeOutDatabase(*tapi.DeletedDatabase) error
	// Recover operation
	RecoverDatabase(*tapi.DeletedDatabase) error
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
	eventRecorder record.EventRecorder
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
				if deletedDb.Status.CreationTime == nil {
					if err := c.create(deletedDb); err != nil {
						log.Errorln(err)
					}
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
					if err := c.update(oldDeletedDb, newDeletedDb); err != nil {
						log.Errorln(err)
					}
				}
			},
		},
	)
	cacheController.Run(wait.NeverStop)
}

func (c *DeletedDatabaseController) create(deletedDb *tapi.DeletedDatabase) error {

	var err error
	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	// Set DeletedDatabase Phase: Deleting
	t := unversioned.Now()
	deletedDb.Status.CreationTime = &t
	if _, err := c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	// Check if DB TPR object exists
	found, err := c.deleter.Exists(&deletedDb.ObjectMeta)
	if err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			"Failed to delete Database. Reason: %v",
			err,
		)
		return err
	}

	if found {
		message := "Failed to delete Database. Delete Database TPR object first"
		c.eventRecorder.Event(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			message,
		)

		// Delete DeletedDatabase object
		if err := c.extClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			c.eventRecorder.Eventf(
				deletedDb,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToDelete,
				"Failed to delete DeletedDatabase. Reason: %v",
				err,
			)
			log.Errorln(err)
		}
		return errors.New(message)
	}

	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	// Set DeletedDatabase Phase: Deleting
	t = unversioned.Now()
	deletedDb.Status.Phase = tapi.DeletedDatabasePhaseDeleting
	if _, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	c.eventRecorder.Event(deletedDb, kapi.EventTypeNormal, eventer.EventReasonDeleting, "Deleting Database")

	// Delete Database workload
	if err := c.deleter.DeleteDatabase(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			"Failed to delete. Reason: %v",
			err,
		)
		return err
	}

	c.eventRecorder.Event(
		deletedDb,
		kapi.EventTypeNormal,
		eventer.EventReasonSuccessfulDelete,
		"Successfully deleted Database workload",
	)

	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	// Set DeletedDatabase Phase: Deleted
	t = unversioned.Now()
	deletedDb.Status.DeletionTime = &t
	deletedDb.Status.Phase = tapi.DeletedDatabasePhaseDeleted
	if _, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	return nil
}

func (c *DeletedDatabaseController) update(oldDeletedDb, updatedDeletedDb *tapi.DeletedDatabase) error {
	if oldDeletedDb.Spec.WipeOut != updatedDeletedDb.Spec.WipeOut && updatedDeletedDb.Spec.WipeOut {
		return c.wipeOut(updatedDeletedDb)
	}

	if oldDeletedDb.Spec.Recover != updatedDeletedDb.Spec.Recover && updatedDeletedDb.Spec.Recover {
		if oldDeletedDb.Status.Phase == tapi.DeletedDatabasePhaseDeleted {
			return c.recover(updatedDeletedDb)
		} else {
			message := "Failed to recover Database. " +
				"Only DeletedDatabase of \"Deleted\" Phase can be recovered"
			c.eventRecorder.Event(
				updatedDeletedDb,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				message,
			)
		}
	}
	return nil
}

func (c *DeletedDatabaseController) wipeOut(deletedDb *tapi.DeletedDatabase) error {
	// Check if DB TPR object exists
	found, err := c.deleter.Exists(&deletedDb.ObjectMeta)
	if err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			"Failed to wipeOut Database. Reason: %v",
			err,
		)
		return err
	}

	if found {
		message := "Failed to wipeOut Database. Delete Database TPR object first"
		c.eventRecorder.Event(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToWipeOut,
			message,
		)

		// Delete DeletedDatabase object
		if err := c.extClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			c.eventRecorder.Eventf(
				deletedDb,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToDelete,
				"Failed to delete DeletedDatabase. Reason: %v",
				err,
			)
			log.Errorln(err)
		}
		return errors.New(message)
	}

	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	// Set DeletedDatabase Phase: Wiping out
	t := unversioned.Now()
	deletedDb.Status.Phase = tapi.DeletedDatabasePhaseWipingOut

	if _, err := c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	// Wipe out Database workload
	c.eventRecorder.Event(deletedDb, kapi.EventTypeNormal, eventer.EventReasonWipingOut, "Wiping out Database")
	if err := c.deleter.WipeOutDatabase(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToWipeOut,
			"Failed to wipeOut. Reason: %v",
			err,
		)
		return err
	}

	c.eventRecorder.Event(
		deletedDb,
		kapi.EventTypeNormal,
		eventer.EventReasonSuccessfulWipeOut,
		"Successfully wiped out Database workload",
	)

	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	// Set DeletedDatabase Phase: Deleted
	t = unversioned.Now()
	deletedDb.Status.WipeOutTime = &t
	deletedDb.Status.Phase = tapi.DeletedDatabasePhaseWipedOut
	if _, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	return nil
}

func (c *DeletedDatabaseController) recover(deletedDb *tapi.DeletedDatabase) error {
	// Check if DB TPR object exists
	found, err := c.deleter.Exists(&deletedDb.ObjectMeta)
	if err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToRecover,
			"Failed to recover Database. Reason: %v",
			err,
		)
		return err
	}

	if found {
		message := "Failed to recover Database. One Database TPR object exists with same name"
		c.eventRecorder.Event(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToRecover,
			message,
		)
		return errors.New(message)
	}

	if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		return err
	}

	deletedDb.Status.Phase = tapi.DeletedDatabasePhaseRecovering
	if _, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DeletedDatabase. Reason: %v",
			err,
		)
		return err
	}

	if err = c.deleter.RecoverDatabase(deletedDb); err != nil {
		c.eventRecorder.Eventf(
			deletedDb,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToRecover,
			"Failed to recover Database. Reason: %v",
			err,
		)

		if deletedDb, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
			return err
		}

		deletedDb.Status.Phase = tapi.DeletedDatabasePhaseDeleted
		if _, err = c.extClient.DeletedDatabases(deletedDb.Namespace).Update(deletedDb); err != nil {
			c.eventRecorder.Eventf(
				deletedDb,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				"Failed to update DeletedDatabase. Reason: %v",
				err,
			)
			log.Errorln(err)
		}

		return err
	}

	return nil
}
