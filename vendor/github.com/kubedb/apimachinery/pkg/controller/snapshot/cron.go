package snapshot

import (
	"fmt"
	"sync"
	"time"

	"github.com/appscode/go/log"
	discovery_util "github.com/appscode/kutil/discovery"
	meta_util "github.com/appscode/kutil/meta"
	apiCatalog "github.com/kubedb/apimachinery/apis/catalog/v1alpha1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/orcaman/concurrent-map"
	"gopkg.in/robfig/cron.v2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

type CronControllerInterface interface {
	StartCron()
	// ScheduleBackup takes parameter DB-runtime object, DB-Object-Meta,  DB.spec.BackupSchedule and DB-Version-Catalog
	ScheduleBackup(runtime.Object, metav1.ObjectMeta, *api.BackupScheduleSpec, runtime.Object) error
	StopBackupScheduling(metav1.ObjectMeta)
	StopCron()
}

type cronController struct {
	// kube client
	kubeClient kubernetes.Interface
	// ThirdPartyExtension client
	extClient cs.Interface
	// dynamic client
	dynamicClient dynamic.Interface
	// For Internal Cron Job
	cron *cron.Cron
	// Store Cron Job EntryID for further use
	cronEntryIDs cmap.ConcurrentMap
	// Event Recorder
	eventRecorder record.EventRecorder
	// To perform start operation once
	once sync.Once
}

/*
 NewCronController returns CronControllerInterface.
 Need to call StartCron() method to start Cron.
*/
func NewCronController(client kubernetes.Interface, extClient cs.Interface, dc dynamic.Interface) CronControllerInterface {
	return &cronController{
		kubeClient:    client,
		extClient:     extClient,
		dynamicClient: dc,
		cron:          cron.New(),
		cronEntryIDs:  cmap.New(),
		eventRecorder: eventer.NewEventRecorder(client, "Cron controller"),
	}
}

func (c *cronController) StartCron() {
	c.once.Do(func() {
		c.cron.Start()
	})
}

func (c *cronController) ScheduleBackup(
	// Runtime Object to push event
	runtimeObj runtime.Object,
	// ObjectMeta of Database TPR object
	om metav1.ObjectMeta,
	// BackupScheduleSpec
	spec *api.BackupScheduleSpec,
	// DBVersion catalog
	catalog runtime.Object,
) error {
	// cronEntry name
	cronEntryName := fmt.Sprintf("%v@%v", om.Name, om.Namespace)

	invoker := &snapshotInvoker{
		kubeclient:    c.kubeClient,
		extClient:     c.extClient,
		dynamicClient: c.dynamicClient,
		runtimeObject: runtimeObj,
		om:            om,
		spec:          spec,
		catalog:       catalog,
		eventRecorder: c.eventRecorder,
	}

	// Remove previous cron job if exist
	if id, exists := c.cronEntryIDs.Pop(cronEntryName); exists {
		c.cron.Remove(id.(cron.EntryID))
	} else {
		invoker.createScheduledSnapshot()
	}

	// Set cron job
	entryID, err := c.cron.AddFunc(spec.CronExpression, invoker.createScheduledSnapshot)
	if err != nil {
		return err
	}

	// Add job entryID
	c.cronEntryIDs.Set(cronEntryName, entryID)

	return nil
}

func (c *cronController) StopBackupScheduling(om metav1.ObjectMeta) {
	// cronEntry name
	cronEntryName := fmt.Sprintf("%v@%v", om.Name, om.Namespace)

	if id, exists := c.cronEntryIDs.Pop(cronEntryName); exists {
		c.cron.Remove(id.(cron.EntryID))
	}
}

func (c *cronController) StopCron() {
	c.cron.Stop()
}

type snapshotInvoker struct {
	kubeclient    kubernetes.Interface
	extClient     cs.Interface
	dynamicClient dynamic.Interface
	runtimeObject runtime.Object
	om            metav1.ObjectMeta
	spec          *api.BackupScheduleSpec
	catalog       runtime.Object
	eventRecorder record.EventRecorder
}

func (s *snapshotInvoker) createScheduledSnapshot() {
	kind := meta_util.GetKind(s.runtimeObject)
	name := s.om.Name
	catalogKind := meta_util.GetKind(s.catalog)
	catalogName, err := meta.NewAccessor().Name(s.catalog)

	gvk := apiCatalog.SchemeGroupVersion.WithKind(catalogKind)
	gvr, err := discovery_util.ResourceForGVK(s.kubeclient.Discovery(), gvk)
	if err != nil {
		log.Errorf("Failed to get 'gvr' for %v/%v. Reason: %v",
			catalogKind, catalogName, err)
		return
	}

	updateCatalog, err := s.dynamicClient.Resource(gvr).Get(catalogName, metav1.GetOptions{})
	if err != nil {
		s.eventRecorder.Eventf(
			s.runtimeObject,
			core.EventTypeWarning,
			eventer.EventReasonFailedToList,
			"Failed to get DB Catalog %v/%v. Reason: %v",
			catalogKind, catalogName, err,
		)
		log.Errorf("Failed to get DB Catalog %v/%v. Reason: %v",
			catalogKind, catalogName, err)
		return
	}

	if val, found, err := unstructured.NestedBool(updateCatalog.UnstructuredContent(), "spec", "deprecated"); err != nil {
		s.eventRecorder.Eventf(
			s.runtimeObject,
			core.EventTypeWarning,
			eventer.EventReasonFailedToList,
			"Failed to get spec.Deprecated value. Reason: %v",
			err,
		)
		log.Errorf("Failed to get spec.Deprecated value. Reason: %v", err)
		return
	} else if found && val == true {
		s.eventRecorder.Eventf(
			s.runtimeObject,
			core.EventTypeWarning,
			eventer.EventReasonFailedToList,
			"%v %s/%s is using deprecated version %v. Skipped processing scheduler",
			kind, s.om.Namespace, s.om.Name, catalogName,
		)
		log.Errorf("%v %s/%s is using deprecated version %v. Skipped processing scheduler",
			kind, s.om.Namespace, s.om.Name, catalogName)
		return
	}

	labelMap := map[string]string{
		api.LabelDatabaseKind:   kind,
		api.LabelDatabaseName:   name,
		api.LabelSnapshotStatus: string(api.SnapshotPhaseRunning),
	}

	snapshotList, err := s.extClient.KubedbV1alpha1().Snapshots(s.om.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(labelMap).AsSelector().String(),
	})
	if err != nil {
		s.eventRecorder.Eventf(
			s.runtimeObject,
			core.EventTypeWarning,
			eventer.EventReasonFailedToList,
			"Failed to list Snapshots. Reason: %v",
			err,
		)
		log.Errorln(err)
		return
	}

	if len(snapshotList.Items) > 0 {
		s.eventRecorder.Event(
			s.runtimeObject,
			core.EventTypeNormal,
			eventer.EventReasonIgnoredSnapshot,
			"Skipping scheduled Backup. One is still active.",
		)
		log.Debugln("Skipping scheduled Backup. One is still active.")
		return
	}

	// Set label. Elastic controller will detect this using label selector
	labelMap = map[string]string{
		api.LabelDatabaseKind: kind,
		api.LabelDatabaseName: name,
	}

	now := time.Now().UTC()
	snapshotName := fmt.Sprintf("%v-%v", s.om.Name, now.Format("20060102-150405"))

	if _, err = s.createSnapshot(snapshotName); err != nil {
		log.Errorln(err)
	}
}

func (s *snapshotInvoker) createSnapshot(snapshotName string) (*api.Snapshot, error) {
	labelMap := map[string]string{
		api.LabelDatabaseKind: meta_util.GetKind(s.runtimeObject),
		api.LabelDatabaseName: s.om.Name,
	}

	snapshot := &api.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: s.om.Namespace,
			Labels:    labelMap,
		},
		Spec: api.SnapshotSpec{
			DatabaseName: s.om.Name,
			Backend:      s.spec.Backend,
			PodTemplate:  s.spec.PodTemplate,
		},
	}

	snapshot, err := s.extClient.KubedbV1alpha1().Snapshots(snapshot.Namespace).Create(snapshot)
	if err != nil {
		s.eventRecorder.Eventf(
			s.runtimeObject,
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to create Snapshot. Reason: %v",
			err,
		)
		return nil, err
	}

	return snapshot, nil
}
