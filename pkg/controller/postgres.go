package controller

import (
	"fmt"
	"reflect"
	"time"

	"errors"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

func (c *Controller) create(postgres *tapi.Postgres) error {
	t := unversioned.Now()
	postgres.Status.CreationTime = &t
	postgres.Status.Phase = tapi.DatabasePhaseCreating
	var _postgres *tapi.Postgres
	var err error
	if _postgres, err = c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			`Fail to update Postgres: "%v". Reason: %v`,
			postgres.Name,
			err,
		)
		return err
	}
	postgres = _postgres

	if err := c.validatePostgres(postgres); err != nil {
		c.eventRecorder.Event(postgres, kapi.EventTypeWarning, eventer.EventReasonInvalid, err.Error())

		postgres.Status.Phase = tapi.DatabasePhaseFailed
		postgres.Status.Reason = err.Error()
		if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				`Fail to update Postgres: "%v". Reason: %v`,
				postgres.Name,
				err,
			)
			log.Errorln(err)
		}
		return err
	}
	// Event for successful validation
	c.eventRecorder.Event(
		postgres,
		kapi.EventTypeNormal,
		eventer.EventReasonSuccessfulValidate,
		"Successfully validate Postgres",
	)

	// Check if DeletedDatabase exists or not
	recovering := false
	deletedDb, err := c.ExtClient.DeletedDatabases(postgres.Namespace).Get(postgres.Name)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToGet,
				`Fail to get DeletedDatabase: "%v". Reason: %v`,
				postgres.Name,
				err,
			)
			return err
		}
	} else {
		var message string

		if deletedDb.Labels[amc.LabelDatabaseKind] != tapi.ResourceKindPostgres {
			message = fmt.Sprintf(`Invalid Postgres: "%v". Exists irrelevant DeletedDatabase: "%v"`,
				postgres.Name, deletedDb.Name)
		} else {
			if deletedDb.Status.Phase == tapi.DeletedDatabasePhaseRecovering {
				recovering = true
			} else {
				message = fmt.Sprintf(`Recover from DeletedDatabase: "%v"`, deletedDb.Name)
			}
		}
		if !recovering {
			// Set status to Failed
			postgres.Status.Phase = tapi.DatabasePhaseFailed
			postgres.Status.Reason = message
			if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
				c.eventRecorder.Eventf(
					postgres,
					kapi.EventTypeWarning,
					eventer.EventReasonFailedToUpdate,
					`Fail to update Postgres: "%v". Reason: %v`, postgres.Name, err,
				)
				log.Errorln(err)
			}
			c.eventRecorder.Event(postgres, kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message)
			return errors.New(message)
		}
	}

	// Event for notification that kubernetes objects are creating
	c.eventRecorder.Event(postgres, kapi.EventTypeNormal, eventer.EventReasonCreating, "Creating Kubernetes objects")

	// create Governing Service
	governingService := c.governingService
	if err := c.CreateGoverningService(governingService, postgres.Namespace); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			`Failed to create ServiceAccount: "%v". Reason: %v`,
			governingService,
			err,
		)
		return err
	}

	// create database Service
	if err := c.createService(postgres.Name, postgres.Namespace); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to create Service. Reason: %v",
			err,
		)
		return err
	}

	// Create statefulSet for Postgres database
	statefulSet, err := c.createStatefulSet(postgres)
	if err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to create StatefulSet. Reason: %v",
			err,
		)
		return err
	}

	// Check StatefulSet Pod status
	if err := c.CheckStatefulSetPodStatus(statefulSet, durationCheckStatefulSet); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToStart,
			`Failed to create StatefulSet. Reason: %v`,
			err,
		)
		return err
	} else {
		c.eventRecorder.Event(
			postgres,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulCreate,
			"Successfully created Postgres",
		)
	}

	if postgres.Spec.Init != nil && postgres.Spec.Init.SnapshotSource != nil {
		postgres.Status.Phase = tapi.DatabasePhaseInitializing
		if _postgres, err = c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToUpdate,
				`Fail to update Postgres: "%v". Reason: %v`,
				postgres.Name,
				err,
			)
			return err
		}
		postgres = _postgres

		if err := c.initialize(postgres); err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToInitialize,
				"Failed to initialize. Reason: %v",
				err,
			)
		}
	}

	if recovering {
		// Delete DeletedDatabase instance
		if err := c.ExtClient.DeletedDatabases(deletedDb.Namespace).Delete(deletedDb.Name); err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToDelete,
				`Failed to delete DeletedDatabase: "%v". Reason: %v`,
				deletedDb.Name,
				err,
			)
			log.Errorln(err)
		}
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulDelete,
			`Successfully deleted DeletedDatabase: "%v"`,
			deletedDb.Name,
		)
	}

	postgres.Status.Phase = tapi.DatabasePhaseRunning
	if _postgres, err = c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			`Fail to update Postgres: "%v". Reason: %v`,
			postgres.Name,
			err,
		)
		log.Errorln(err)
	}
	postgres = _postgres

	// Setup Schedule backup
	if postgres.Spec.BackupSchedule != nil {
		err := c.cronController.ScheduleBackup(postgres, postgres.ObjectMeta, postgres.Spec.BackupSchedule)
		if err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToSchedule,
				"Failed to schedule snapshot. Reason: %v",
				err,
			)
			log.Errorln(err)
		}
	}

	return nil
}

const (
	durationCheckRestoreJob = time.Minute * 30
)

func (c *Controller) initialize(postgres *tapi.Postgres) error {
	snapshotSource := postgres.Spec.Init.SnapshotSource
	// Event for notification that kubernetes objects are creating
	c.eventRecorder.Eventf(
		postgres,
		kapi.EventTypeNormal,
		eventer.EventReasonInitializing,
		`Initializing from DatabaseSnapshot: "%v"`,
		snapshotSource.Name,
	)

	namespace := snapshotSource.Namespace
	if namespace == "" {
		namespace = postgres.Namespace
	}
	dbSnapshot, err := c.ExtClient.DatabaseSnapshots(namespace).Get(snapshotSource.Name)
	if err != nil {
		return err
	}

	job, err := c.createRestoreJob(postgres, dbSnapshot)
	if err != nil {
		return err
	}

	jobSuccess := c.CheckDatabaseRestoreJob(job, postgres, c.eventRecorder, durationCheckRestoreJob)
	if jobSuccess {
		c.eventRecorder.Event(
			postgres,
			kapi.EventTypeNormal,
			eventer.EventReasonSuccessfulInitialize,
			"Successfully completed initialization",
		)
	} else {
		c.eventRecorder.Event(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToInitialize,
			"Failed to complete initialization",
		)
	}
	return nil
}

func (c *Controller) delete(postgres *tapi.Postgres) error {

	c.eventRecorder.Event(postgres, kapi.EventTypeNormal, eventer.EventReasonDeleting, "Deleting Postgres")

	if postgres.Spec.DoNotDelete {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			`Postgres "%v" is locked.`,
			postgres.Name,
		)

		if err := c.reCreatePostgres(postgres); err != nil {
			c.eventRecorder.Eventf(
				postgres,
				kapi.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				`Failed to recreate Postgres: "%v". Reason: %v`,
				postgres,
				err,
			)
			return err
		}
		return nil
	}

	if _, err := c.createDeletedDatabase(postgres); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			`Failed to create DeletedDatabase: "%v". Reason: %v`,
			postgres.Name,
			err,
		)
		return err
	}
	c.eventRecorder.Eventf(
		postgres,
		kapi.EventTypeNormal,
		eventer.EventReasonSuccessfulCreate,
		`Successfully created DeletedDatabase: "%v"`,
		postgres.Name,
	)

	c.cronController.StopBackupScheduling(postgres.ObjectMeta)
	return nil
}

func (c *Controller) update(oldPostgres, updatedPostgres *tapi.Postgres) error {
	if !reflect.DeepEqual(updatedPostgres.Spec.BackupSchedule, oldPostgres.Spec.BackupSchedule) {
		backupScheduleSpec := updatedPostgres.Spec.BackupSchedule
		if backupScheduleSpec != nil {
			if err := c.ValidateBackupSchedule(backupScheduleSpec); err != nil {
				c.eventRecorder.Event(
					updatedPostgres,
					kapi.EventTypeNormal,
					eventer.EventReasonInvalid,
					err.Error(),
				)
				return err
			}

			if err := c.CheckBucketAccess(backupScheduleSpec.SnapshotSpec, oldPostgres.Namespace); err != nil {
				c.eventRecorder.Event(
					updatedPostgres,
					kapi.EventTypeNormal,
					eventer.EventReasonInvalid,
					err.Error(),
				)
				return err
			}

			if err := c.cronController.ScheduleBackup(
				oldPostgres, oldPostgres.ObjectMeta, oldPostgres.Spec.BackupSchedule); err != nil {
				c.eventRecorder.Eventf(
					updatedPostgres,
					kapi.EventTypeWarning,
					eventer.EventReasonFailedToSchedule,
					"Failed to schedule snapshot. Reason: %v", err,
				)
				log.Errorln(err)
			}
		} else {
			c.cronController.StopBackupScheduling(oldPostgres.ObjectMeta)
		}
	}
	return nil
}
