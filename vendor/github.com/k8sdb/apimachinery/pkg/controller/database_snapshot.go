package controller

import (
	"fmt"
	"time"

	"github.com/appscode/go/wait"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

type Snapshotter interface {
	ValidateSnapshot(*tapi.DatabaseSnapshot) error
	GetDatabase(*tapi.DatabaseSnapshot) (runtime.Object, error)
	GetSnapshotter(*tapi.DatabaseSnapshot) (*kbatch.Job, error)
	WipeOutSnapshot(*tapi.DatabaseSnapshot) error
}

type DatabaseSnapshotController struct {
	// Kubernetes client
	client clientset.Interface
	// ThirdPartyExtension client
	extClient tcs.ExtensionInterface
	// Snapshotter interface
	snapshoter Snapshotter
	// ListerWatcher
	lw *cache.ListWatch
	// Event Recorder
	eventRecorder record.EventRecorder
	// sync time to sync the list.
	syncPeriod time.Duration
}

const (
	LabelJobType        = "job.k8sdb.com/type"
	LabelSnapshotStatus = "snapshot.k8sdb.com/status"
)

// NewDatabaseSnapshotController creates a new NewDatabaseSnapshot Controller
func NewDatabaseSnapshotController(
	client clientset.Interface,
	extClient tcs.ExtensionInterface,
	snapshoter Snapshotter,
	lw *cache.ListWatch,
	syncPeriod time.Duration,
) *DatabaseSnapshotController {

	// return new DeletedDatabase Controller
	return &DatabaseSnapshotController{
		client:        client,
		extClient:     extClient,
		snapshoter:    snapshoter,
		lw:            lw,
		eventRecorder: eventer.NewEventRecorder(client, "DatabaseSnapshot Controller"),
		syncPeriod:    syncPeriod,
	}
}

func (c *DatabaseSnapshotController) Run() {
	// Ensure DeletedDatabase TPR
	c.ensureThirdPartyResource()
	// Watch DeletedDatabase with provided ListerWatcher
	c.watch()
}

// Ensure DatabaseSnapshot ThirdPartyResource
func (c *DatabaseSnapshotController) ensureThirdPartyResource() {
	log.Infoln("Ensuring DatabaseSnapshot ThirdPartyResource")

	resourceName := tapi.ResourceNameDatabaseSnapshot + "." + tapi.V1beta1SchemeGroupVersion.Group
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

func (c *DatabaseSnapshotController) watch() {
	_, cacheController := cache.NewInformer(c.lw,
		&tapi.DatabaseSnapshot{},
		c.syncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dbSnapshot := obj.(*tapi.DatabaseSnapshot)
				if dbSnapshot.Status.StartTime == nil {
					if err := c.create(dbSnapshot); err != nil {
						log.Errorln(err)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				dbSnapshot := obj.(*tapi.DatabaseSnapshot)
				if err := c.delete(dbSnapshot); err != nil {
					log.Errorln(err)
				}
			},
		},
	)
	cacheController.Run(wait.NeverStop)
}

const (
	durationCheckSnapshotJob = time.Minute * 30
)

func (c *DatabaseSnapshotController) create(dbSnapshot *tapi.DatabaseSnapshot) error {
	// Validate DatabaseSnapshot spec
	if err := c.snapshoter.ValidateSnapshot(dbSnapshot); err != nil {
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonInvalid, err.Error())
		return err
	}

	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error())
		return err
	}

	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name); err != nil {
		return err
	}

	dbSnapshot.Labels[LabelDatabaseName] = dbSnapshot.Spec.DatabaseName
	if _, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error())
		return err
	}

	c.eventRecorder.Event(runtimeObj, kapi.EventTypeNormal, eventer.EventReasonStarting, "Backup running")
	c.eventRecorder.Event(dbSnapshot, kapi.EventTypeNormal, eventer.EventReasonStarting, "Backup running")

	job, err := c.snapshoter.GetSnapshotter(dbSnapshot)
	if err != nil {
		message := fmt.Sprintf("Failed to take snapshot. Reason: %v", err)
		c.eventRecorder.Event(runtimeObj, kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message)
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message)
		return err
	}

	if _, err := c.client.Batch().Jobs(dbSnapshot.Namespace).Create(job); err != nil {
		message := fmt.Sprintf("Failed to take snapshot. Reason: %v", err)
		c.eventRecorder.Event(runtimeObj, kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message)
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message)
		return err
	}

	go func() {
		if err := c.checkDatabaseSnapshotJob(dbSnapshot, job.Name, durationCheckSnapshotJob); err != nil {
			log.Errorln(err)
		}
	}()

	return nil
}

func (c *DatabaseSnapshotController) delete(dbSnapshot *tapi.DatabaseSnapshot) error {
	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			c.eventRecorder.Event(
				dbSnapshot,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToGet,
				err.Error(),
			)
			return err
		}
	}

	if runtimeObj != nil {
		c.eventRecorder.Eventf(
			runtimeObj,
			kapi.EventTypeNormal,
			eventer.EventReasonWipingOut,
			"Wiping out DatabaseSnapshot: %v",
			dbSnapshot.Name,
		)
	}

	if err := c.snapshoter.WipeOutSnapshot(dbSnapshot); err != nil {
		if runtimeObj != nil {
			c.eventRecorder.Eventf(
				runtimeObj,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToWipeOut,
				"Failed to  wipeOut. Reason: %v",
				err,
			)
		}
		return err
	}

	if runtimeObj != nil {
		c.eventRecorder.Eventf(
			runtimeObj,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulWipeOut,
			"Successfully wiped out DatabaseSnapshot: %v",
			dbSnapshot.Name,
		)
	}
	return nil
}

func (c *DatabaseSnapshotController) checkDatabaseSnapshotJob(dbSnapshot *tapi.DatabaseSnapshot, jobName string, checkDuration time.Duration) error {

	var err error
	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name); err != nil {
		return err
	}

	t := unversioned.Now()
	dbSnapshot.Status.StartTime = &t
	dbSnapshot.Status.Phase = tapi.SnapshotPhaseRunning
	dbSnapshot.Labels[LabelSnapshotStatus] = string(tapi.SnapshotPhaseRunning)

	if _, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		c.eventRecorder.Eventf(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DatabaseSnapshot. Reason: %v",
			err,
		)
		return err
	}

	var jobSuccess bool = false
	var job *kbatch.Job

	then := time.Now()
	now := time.Now()
	for now.Sub(then) < checkDuration {
		log.Debugln("Checking for Job ", jobName)
		job, err = c.client.Batch().Jobs(dbSnapshot.Namespace).Get(jobName)
		if err != nil {
			c.eventRecorder.Eventf(
				dbSnapshot,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToList,
				"Failed to get Job. Reason: %v",
				err,
			)
			return err
		}
		log.Debugf("Pods Statuses:	%d Running / %d Succeeded / %d Failed",
			job.Status.Active, job.Status.Succeeded, job.Status.Failed)
		// If job is success
		if job.Status.Succeeded > 0 {
			jobSuccess = true
			break
		} else if job.Status.Failed > 0 {
			break
		}

		time.Sleep(time.Minute)
		now = time.Now()
	}

	podList, err := c.client.Core().Pods(job.Namespace).List(
		kapi.ListOptions{
			LabelSelector: labels.SelectorFromSet(job.Spec.Selector.MatchLabels),
		},
	)
	if err != nil {
		c.eventRecorder.Eventf(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToList,
			"Failed to list Pods. Reason: %v",
			err,
		)
		return err
	}

	for _, pod := range podList.Items {
		if err := c.client.Core().Pods(pod.Namespace).Delete(pod.Name, nil); err != nil {
			c.eventRecorder.Eventf(
				dbSnapshot,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToDelete,
				"Failed to delete Pod. Reason: %v",
				err,
			)
			log.Errorln(err)
		}
	}

	for _, volume := range job.Spec.Template.Spec.Volumes {
		claim := volume.PersistentVolumeClaim
		if claim != nil {
			err := c.client.Core().PersistentVolumeClaims(job.Namespace).Delete(claim.ClaimName, nil)
			if err != nil {
				c.eventRecorder.Eventf(
					dbSnapshot,
					kapi.EventTypeWarning,
					eventer.EventReasonFailedToDelete,
					"Failed to delete PersistentVolumeClaim. Reason: %v",
					err,
				)
				log.Errorln(err)
			}
		}
	}

	if err := c.client.Batch().Jobs(job.Namespace).Delete(job.Name, nil); err != nil {
		c.eventRecorder.Eventf(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			"Failed to delete Job. Reason: %v",
			err,
		)
		log.Errorln(err)
	}

	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name); err != nil {
		c.eventRecorder.Eventf(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToGet,
			"Failed to get DatabaseSnapshot. Reason: %v",
			err,
		)
		return err
	}

	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		c.eventRecorder.Event(dbSnapshot, kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error())
		return err
	}

	t = unversioned.Now()
	dbSnapshot.Status.CompletionTime = &t
	if jobSuccess {
		dbSnapshot.Status.Phase = tapi.SnapshotPhaseSuccessed
		c.eventRecorder.Event(
			runtimeObj,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulSnapshot,
			"Successfully completed snapshot",
		)
		c.eventRecorder.Event(
			dbSnapshot,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulSnapshot,
			"Successfully completed snapshot",
		)
	} else {
		dbSnapshot.Status.Phase = tapi.SnapshotPhaseFailed
		c.eventRecorder.Event(
			runtimeObj,
			kapi.EventTypeWarning,
			eventer.EventReasonSnapshotFailed,
			"Failed to complete snapshot",
		)
		c.eventRecorder.Event(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonSnapshotFailed,
			"Failed to complete snapshot",
		)
	}

	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name); err != nil {
		return err
	}
	delete(dbSnapshot.Labels, LabelSnapshotStatus)
	if _, err := c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		c.eventRecorder.Eventf(
			dbSnapshot,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update DatabaseSnapshot. Reason: %v",
			err,
		)
		log.Errorln(err)
	}
	return nil
}
