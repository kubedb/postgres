package controller

import (
	"fmt"
	"reflect"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

func (c *Controller) create(postgres *tapi.Postgres) {
	unversionedNow := unversioned.Now()
	postgres.Status.Created = &unversionedNow
	postgres.Status.DatabaseStatus = tapi.StatusDatabaseCreating
	var err error
	if postgres, err = c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
		message := fmt.Sprintf(`Fail to update Postgres: "%v". Reason: %v`, postgres.Name, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, postgres,
		)
		log.Errorln(err)
		return
	}

	if err := c.validatePostgres(postgres); err != nil {
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonInvalid, err.Error(), postgres)

		postgres.Status.DatabaseStatus = tapi.StatusDatabaseFailed
		postgres.Status.Reason = err.Error()
		if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
			message := fmt.Sprintf(`Fail to update Postgres: "%v". Reason: %v`, postgres.Name, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, postgres,
			)
			log.Errorln(err)
		}

		log.Errorln(err)
		return
	}
	// Event for successful validation
	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonSuccessfulValidate, "Successfully validate Postgres", postgres,
	)

	// Check if DeletedDatabase exists or not
	recovering := false
	deletedDb, err := c.ExtClient.DeletedDatabases(postgres.Namespace).Get(postgres.Name)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			message := fmt.Sprintf(`Fail to get DeletedDatabase: "%v". Reason: %v`, postgres.Name, err)
			c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToGet, message, postgres)
			log.Errorln(err)
			return
		}
	} else {
		var message string

		if deletedDb.Labels[amc.LabelDatabaseType] != tapi.ResourceNamePostgres {
			message = fmt.Sprintf(`Invalid Postgres: "%v". Exists irrelevant DeletedDatabase: "%v"`,
				postgres.Name, deletedDb.Name)
		} else {
			if deletedDb.Status.Phase == tapi.PhaseDatabaseRecovering {
				recovering = true
			} else {
				message = fmt.Sprintf(`Recover from DeletedDatabase: "%v"`, deletedDb.Name)
			}
		}
		if !recovering {
			// Set status to Failed
			postgres.Status.DatabaseStatus = tapi.StatusDatabaseFailed
			postgres.Status.Reason = message
			if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
				message := fmt.Sprintf(`Fail to update Postgres: "%v". Reason: %v`, postgres.Name, err)
				c.eventRecorder.PushEvent(
					kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, postgres,
				)
				log.Errorln(err)
			}
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres,
			)
			log.Infoln(message)
			return
		}
	}

	// Event for notification that kubernetes objects are creating
	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonCreating, "Creating Kubernetes objects", postgres,
	)

	// create Governing Service
	governingService := GoverningPostgres
	if postgres.Spec.ServiceAccountName != "" {
		governingService = postgres.Spec.ServiceAccountName
	}

	if err := c.CreateGoverningServiceAccount(governingService, postgres.Namespace); err != nil {
		message := fmt.Sprintf(`Failed to create ServiceAccount: "%v". Reason: %v`, governingService, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres)
		log.Errorln(err)
		return
	}
	postgres.Spec.ServiceAccountName = governingService

	// create database Service
	if err := c.createService(postgres.Name, postgres.Namespace); err != nil {
		message := fmt.Sprintf(`Failed to create Service. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres)
		log.Errorln(err)
		return
	}

	// Create statefulSet for Postgres database
	statefulSet, err := c.createStatefulSet(postgres)
	if err != nil {
		message := fmt.Sprintf(`Failed to create StatefulSet. Reason: %v`, err)
		c.eventRecorder.PushEvent(kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres)
		log.Errorln(err)
		return
	}

	// Check StatefulSet Pod status
	if err := c.CheckStatefulSetPodStatus(statefulSet, durationCheckStatefulSet); err != nil {
		message := fmt.Sprintf(`Failed to create StatefulSet. Reason: %v`, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToStart, message, postgres,
		)
		log.Errorln(err)
		return
	} else {
		c.eventRecorder.PushEvent(
			kapi.EventTypeNormal, eventer.EventReasonSuccessfulCreate, "Successfully created Postgres",
			postgres,
		)
	}

	if recovering {
		// Delete DeletedDatabase instance
		if err := c.ExtClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			message := fmt.Sprintf(`Failed to delete DeletedDatabase: "%v". Reason: %v`, deletedDb.Name, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, postgres,
			)
			log.Errorln(err)
		}
		message := fmt.Sprintf(`Successfully deleted DeletedDatabase: "%v"`, deletedDb.Name)
		c.eventRecorder.PushEvent(
			kapi.EventTypeNormal, eventer.EventReasonSuccessfulDelete, message, postgres,
		)
	}

	postgres.Status.DatabaseStatus = tapi.StatusDatabaseRunning
	if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
		message := fmt.Sprintf(`Fail to update Postgres: "%v". Reason: %v`, postgres.Name, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToUpdate, message, postgres,
		)
		log.Errorln(err)
	}

	// Setup Schedule backup
	if postgres.Spec.BackupSchedule != nil {
		err := c.cronController.ScheduleBackup(postgres, postgres.ObjectMeta, postgres.Spec.BackupSchedule)
		if err != nil {
			message := fmt.Sprintf(`Failed to schedule snapshot. Reason: %v`, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToSchedule, message, postgres,
			)
			log.Errorln(err)
		}
	}
}

func (c *Controller) delete(postgres *tapi.Postgres) {

	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonDeleting, "Deleting Postgres", postgres,
	)

	if postgres.Spec.DoNotDelete {
		message := fmt.Sprintf(`Postgres "%v" is locked.`, postgres.Name)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToDelete, message, postgres,
		)

		if err := c.reCreatePostgres(postgres); err != nil {
			message := fmt.Sprintf(`Failed to recreate Postgres: "%v". Reason: %v`, postgres, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres,
			)
			log.Errorln(err)
			return
		}
		return
	}

	if _, err := c.createDeletedDatabase(postgres); err != nil {
		message := fmt.Sprintf(`Failed to create DeletedDatabase: "%v". Reason: %v`, postgres.Name, err)
		c.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message, postgres,
		)
		log.Errorln(err)
		return
	}
	message := fmt.Sprintf(`Successfully created DeletedDatabase: "%v"`, postgres.Name)
	c.eventRecorder.PushEvent(
		kapi.EventTypeNormal, eventer.EventReasonSuccessfulCreate, message, postgres,
	)

	c.cronController.StopBackupScheduling(postgres.ObjectMeta)
}

func (c *Controller) update(oldPostgres, updatedPostgres *tapi.Postgres) {
	if (updatedPostgres.Spec.Replicas != oldPostgres.Spec.Replicas) && oldPostgres.Spec.Replicas >= 0 {
		statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, updatedPostgres.Name)
		statefulSet, err := c.Client.Apps().StatefulSets(updatedPostgres.Namespace).Get(statefulSetName)
		if err != nil {
			message := fmt.Sprintf(`Failed to get StatefulSet: "%v". Reason: %v`, statefulSetName, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeNormal, eventer.EventReasonFailedToGet, message, updatedPostgres,
			)
			log.Errorln(err)
			return
		}
		statefulSet.Spec.Replicas = oldPostgres.Spec.Replicas
		if _, err := c.Client.Apps().StatefulSets(statefulSet.Namespace).Update(statefulSet); err != nil {
			message := fmt.Sprintf(`Failed to update StatefulSet: "%v". Reason: %v`, statefulSetName, err)
			c.eventRecorder.PushEvent(
				kapi.EventTypeNormal, eventer.EventReasonFailedToUpdate, message, updatedPostgres,
			)
			log.Errorln(err)
			return
		}
	}

	if !reflect.DeepEqual(updatedPostgres.Spec.BackupSchedule, oldPostgres.Spec.BackupSchedule) {
		backupScheduleSpec := updatedPostgres.Spec.BackupSchedule
		if backupScheduleSpec != nil {
			if err := c.ValidateBackupSchedule(backupScheduleSpec); err != nil {
				c.eventRecorder.PushEvent(
					kapi.EventTypeNormal, eventer.EventReasonInvalid, err.Error(), updatedPostgres,
				)
				log.Errorln(err)
				return
			}

			if err := c.CheckBucketAccess(backupScheduleSpec.SnapshotSpec, oldPostgres.Namespace); err != nil {
				c.eventRecorder.PushEvent(
					kapi.EventTypeNormal, eventer.EventReasonInvalid, err.Error(), updatedPostgres,
				)
				log.Errorln(err)
				return
			}

			if err := c.cronController.ScheduleBackup(
				oldPostgres, oldPostgres.ObjectMeta, oldPostgres.Spec.BackupSchedule); err != nil {
				message := fmt.Sprintf(`Failed to schedule snapshot. Reason: %v`, err)
				c.eventRecorder.PushEvent(
					kapi.EventTypeWarning, eventer.EventReasonFailedToSchedule, message, updatedPostgres,
				)
				log.Errorln(err)
			}
		} else {
			c.cronController.StopBackupScheduling(oldPostgres.ObjectMeta)
		}
	}
}
