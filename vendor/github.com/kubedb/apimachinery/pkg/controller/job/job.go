package job

import (
	"fmt"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/eventer"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) completeJob(job *batch.Job) error {
	var snapshotName string
	for _, o := range job.OwnerReferences {
		if o.Kind == api.ResourceKindSnapshot {
			snapshotName = o.Name
		}
	}
	if snapshotName == "" {
		return fmt.Errorf(`resource Job "%s/%s" doesn't have any OwnerReference for Snapshot`, job.Namespace, job.Name)
	}

	snapshot, err := c.ExtClient.Snapshots(job.Namespace).Get(snapshotName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	jobSucceeded := job.Status.Succeeded > 0

	_, _, err = util.PatchSnapshot(c.ExtClient, snapshot, func(in *api.Snapshot) *api.Snapshot {
		t := metav1.Now()
		if jobSucceeded {
			in.Status.Phase = api.SnapshotPhaseSucceeded
		} else {
			in.Status.Phase = api.SnapshotPhaseFailed
		}
		in.Status.CompletionTime = &t
		delete(in.Labels, api.LabelSnapshotStatus)
		return in
	})
	if err != nil {
		c.eventRecorder.Eventf(snapshot.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToUpdate, err.Error())
		return err
	}

	deletePolicy := metav1.DeletePropagationBackground
	err = c.Client.BatchV1().Jobs(job.Namespace).Delete(job.Name, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})

	if err != nil && !kerr.IsNotFound(err) {
		return fmt.Errorf("failed to delete job: %s, reason: %s", job.Name, err)
	}

	return nil
}
