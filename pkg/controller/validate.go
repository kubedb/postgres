package controller

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/api"
)

func (c *Controller) validatePostgres(postgres *tapi.Postgres) error {
	if postgres.Spec.Version == "" {
		return fmt.Errorf(`Object 'Version' is missing in '%v'`, postgres.Spec)
	}

	storage := postgres.Spec.Storage
	if storage != nil {
		var err error
		if storage, err = c.ValidateStorageSpec(storage); err != nil {
			return err
		}
	}

	backupScheduleSpec := postgres.Spec.BackupSchedule
	if postgres.Spec.BackupSchedule != nil {
		if err := c.ValidateBackupSchedule(backupScheduleSpec); err != nil {
			return err
		}

		if err := c.CheckBucketAccess(backupScheduleSpec.SnapshotSpec, postgres.Namespace); err != nil {
			return err
		}
	}
	return nil
}
