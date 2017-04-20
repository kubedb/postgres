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
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

type Snapshotter interface {
	ValidateSnapshot(*tapi.DatabaseSnapshot) error
	GetDatabase(*tapi.DatabaseSnapshot) (runtime.Object, error)
	GetSnapshotter(*tapi.DatabaseSnapshot) (*kbatch.Job, error)
	DestroySnapshot(*tapi.DatabaseSnapshot) error
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
	eventRecorder eventer.EventRecorderInterface
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
					c.create(dbSnapshot)
				}
			},
			DeleteFunc: func(obj interface{}) {
				dbSnapshot := obj.(*tapi.DatabaseSnapshot)
				c.delete(dbSnapshot)
			},
		},
	)
	cacheController.Run(wait.NeverStop)
}

const (
	durationCheckSnapshotJob = time.Minute * 30
)

func (c *DatabaseSnapshotController) create(dbSnapshot *tapi.DatabaseSnapshot) {
	// Validate DatabaseSnapshot spec
	if err := c.snapshoter.ValidateSnapshot(dbSnapshot); err != nil {
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonInvalid, err.Error(), dbSnapshot)
		log.Errorln(err)
		return
	}

	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error(), dbSnapshot)
		log.Errorln(err)
		return
	}

	dbSnapshot.Labels[LabelDatabaseName] = dbSnapshot.Spec.DatabaseName
	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error(), dbSnapshot)
		log.Errorln(err)
		return
	}

	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonStarting, "Backup running",
		runtimeObj, dbSnapshot,
	)

	job, err := c.snapshoter.GetSnapshotter(dbSnapshot)
	if err != nil {
		message := fmt.Sprintf(`Failed to take snapshot. Reason: %v`, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message,
			runtimeObj, dbSnapshot,
		)
		log.Errorln(err)
		return
	}

	if _, err := c.client.Batch().Jobs(dbSnapshot.Namespace).Create(job); err != nil {
		message := fmt.Sprintf(`Failed to take snapshot. Reason: %v`, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, message,
			runtimeObj, dbSnapshot,
		)
		log.Errorln(err)
		return
	}

	go c.checkDatabaseSnapshotJob(dbSnapshot, job.Name, durationCheckSnapshotJob)
}

func (c *DatabaseSnapshotController) delete(dbSnapshot *tapi.DatabaseSnapshot) {
	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error(), dbSnapshot,
			)
			log.Errorln(err)
			return
		}
	}

	if runtimeObj != nil {
		message := fmt.Sprintf(`Destroying DatabaseSnapshot: "%v"`, dbSnapshot.Name)
		c.eventRecorder.PushEvent(kapi.EventTypeNormal, eventer.EventReasonDestroying, message, runtimeObj)
	}

	if err := c.snapshoter.DestroySnapshot(dbSnapshot); err != nil {
		if runtimeObj != nil {
			message := fmt.Sprintf(`Failed to  destroy. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToDestroy, message, runtimeObj,
			)
		}
		log.Errorln(err)
		return
	}

	if runtimeObj != nil {
		message := fmt.Sprintf(`Successfully destroyed DatabaseSnapshot: "%v"`, dbSnapshot.Name)
		c.eventRecorder.PushEvent(
			kapi.EventTypeNormal, eventer.EventReasonSuccessfulDestroy, message, runtimeObj,
		)
	}
}

func (c *DatabaseSnapshotController) checkDatabaseSnapshotJob(dbSnapshot *tapi.DatabaseSnapshot, jobName string, checkDuration time.Duration) {
	t := unversioned.Now()
	dbSnapshot.Status.StartTime = &t
	dbSnapshot.Status.Status = tapi.StatusSnapshotRunning
	dbSnapshot.Labels[LabelSnapshotStatus] = string(tapi.StatusSnapshotRunning)
	var err error
	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		message := fmt.Sprintf(`Failed to update DatabaseSnapshot. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, dbSnapshot)
		log.Errorln(err)
		return
	}

	var jobSuccess bool = false
	var job *kbatch.Job

	then := time.Now()
	now := time.Now()
	for now.Sub(then) < checkDuration {
		log.Debugln("Checking for Job ", jobName)
		job, err = c.client.Batch().Jobs(dbSnapshot.Namespace).Get(jobName)
		if err != nil {
			message := fmt.Sprintf(`Failed to get Job. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToList, message, dbSnapshot,
			)
			log.Errorln(err)
			return
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
		message := fmt.Sprintf(`Failed to list Pods. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToList, message, dbSnapshot)
		log.Errorln(err)
		return
	}

	for _, pod := range podList.Items {
		if err := c.client.Core().Pods(pod.Namespace).Delete(pod.Name, nil); err != nil {
			message := fmt.Sprintf(`Failed to delete Pod. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, dbSnapshot,
			)
			log.Errorln(err)
		}
	}

	for _, volume := range job.Spec.Template.Spec.Volumes {
		claim := volume.PersistentVolumeClaim
		if claim != nil {
			err := c.client.Core().PersistentVolumeClaims(job.Namespace).Delete(claim.ClaimName, nil)
			if err != nil {
				message := fmt.Sprintf(`Failed to delete PersistentVolumeClaim. Reason: %v`, err)
				c.eventRecorder.PushEvent(
					kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, dbSnapshot,
				)
				log.Errorln(err)
			}
		}
	}

	if err := c.client.Batch().Jobs(job.Namespace).Delete(job.Name, nil); err != nil {
		message := fmt.Sprintf(`Failed to delete Job. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, dbSnapshot)
		log.Errorln(err)
	}

	if dbSnapshot, err = c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Get(dbSnapshot.Name); err != nil {
		message := fmt.Sprintf(`Failed to get DatabaseSnapshot. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToGet, message, dbSnapshot)
		log.Errorln(err)
		return
	}

	runtimeObj, err := c.snapshoter.GetDatabase(dbSnapshot)
	if err != nil {
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToGet, err.Error(), dbSnapshot)
		log.Errorln(err)
		return
	}

	t = unversioned.Now()
	dbSnapshot.Status.CompletionTime = &t
	if jobSuccess {
		dbSnapshot.Status.Status = tapi.StatusSnapshotSuccessed
		c.eventRecorder.PushEvent(
			kapi.EventTypeNormal, eventer.EventReasonSuccessfulSnapshot, "Successfully completed snapshot",
			runtimeObj, dbSnapshot,
		)
	} else {
		dbSnapshot.Status.Status = tapi.StatusSnapshotFailed
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonSnapshotFailed, "Failed to complete snapshot",
			runtimeObj, dbSnapshot,
		)
	}

	delete(dbSnapshot.Labels, LabelSnapshotStatus)

	if _, err := c.extClient.DatabaseSnapshots(dbSnapshot.Namespace).Update(dbSnapshot); err != nil {
		message := fmt.Sprintf(`Failed to update DatabaseSnapshot. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, dbSnapshot)
		log.Errorln(err)
	}
}
