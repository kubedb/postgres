package test

import (
	"fmt"
	"testing"
	"time"

	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/postgres/test/mini"
	"github.com/stretchr/testify/assert"
	kapi "k8s.io/kubernetes/pkg/api"
)

func TestCreate(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running Postgres Controller")

	// Postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating Postgres")
	postgres := mini.NewPostgres()
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		assert.Nil(t, err)
	}

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err := mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}


	fmt.Println("---- >> WipingOut Database")
	err = mini.WipeOutDormantDatabase(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be wipedout")
	}

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseWipedOut)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be wipedout")
	}

	fmt.Println("---- >> Deleting DormantDatabase")
	err = mini.DeleteDormantDatabase(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}

func TestDoNotDelete(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running Postgres Controller")

	// Postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating Postgres")
	postgres := mini.NewPostgres()
	postgres.Spec.DoNotDelete = true
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		assert.Nil(t, err)
	}

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err = mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		assert.Nil(t, err)
	}

	postgres, _ = controller.ExtClient.Postgreses(postgres.Namespace).Get(postgres.Name)
	postgres.Spec.DoNotDelete = false
	postgres, err = mini.UpdatePostgres(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}
	time.Sleep(time.Second * 10)

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err := mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}

func TestSnapshot(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running Postgres Controller")

	// Postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating Postgres")
	postgres := mini.NewPostgres()
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		if !assert.Nil(t, err) {
			fmt.Println("---- >> Failed to check PostgresWorkload")
			return
		}
	}

	const (
		bucket     = ""
		secretName = ""
	)

	snapshotSpec := tapi.SnapshotSpec{
		DatabaseName: postgres.Name,
		SnapshotStorageSpec: tapi.SnapshotStorageSpec{
			BucketName: bucket,
			StorageSecret: &kapi.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}

	err = controller.CheckBucketAccess(snapshotSpec.SnapshotStorageSpec, postgres.Namespace)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Creating Snapshot")
	snapshot, err := mini.CreateSnapshot(controller, postgres.Namespace, snapshotSpec)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Checking Snapshot")
	done, err := mini.CheckSnapshot(controller, snapshot)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to take snapshot")
		return
	}

	fmt.Println("---- >> Checking Snapshot data")
	count, err := mini.CheckSnapshotData(controller, snapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.NotZero(t, count)

	fmt.Println("---- >> Deleting Snapshot")
	err = controller.ExtClient.Snapshots(snapshot.Namespace).Delete(snapshot.Name)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to delete Snapshot")
		return
	}

	time.Sleep(time.Second * 30)

	fmt.Println("---- >> Checking Snapshot data")
	count, err = mini.CheckSnapshotData(controller, snapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.Zero(t, count)

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}

func TestDatabaseResumey(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running Postgres Controller")

	// Postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating Postgres")
	postgres := mini.NewPostgres()
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		if !assert.Nil(t, err) {
			fmt.Println("---- >> Failed to check PostgresWorkload")
			return
		}
	}

	fmt.Println("---- >> Deleting Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err := mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be delete")
	}

	fmt.Println("---- >> Updating DormantDatabase")
	dormantDb, err := controller.ExtClient.DormantDatabases(postgres.Namespace).Get(postgres.Name)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to get DormantDatabase")
		return
	}

	dormantDb.Spec.Resume = true
	_, err = controller.ExtClient.DormantDatabases(dormantDb.Namespace).Update(dormantDb)
	assert.Nil(t, err)

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking Postgres")
	running, err = mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> Postgres fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		if !assert.Nil(t, err) {
			fmt.Println("---- >> Failed to check PostgresWorkload")
			return
		}
	}

	fmt.Println("---- >> Deleting Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be delete")
	}
}

func TestInitialize(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running postgres Controller")

	// postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating postgres")
	postgres := mini.NewPostgres()
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> postgres fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		if !assert.Nil(t, err) {
			fmt.Println("---- >> Failed to check postgresWorkload")
			return
		}
	}

	const (
		bucket     = ""
		secretName = ""
	)

	snapshotSpec := tapi.SnapshotSpec{
		DatabaseName: postgres.Name,
		SnapshotStorageSpec: tapi.SnapshotStorageSpec{
			BucketName: bucket,
			StorageSecret: &kapi.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}

	fmt.Println("---- >> Creating Snapshot")
	snapshot, err := mini.CreateSnapshot(controller, postgres.Namespace, snapshotSpec)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Checking Snapshot")
	done, err := mini.CheckSnapshot(controller, snapshot)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to take snapshot")
		return
	}

	fmt.Println("---- >> Checking Snapshot data")
	count, err := mini.CheckSnapshotData(controller, snapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.NotZero(t, count)

	// postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres_init")
	fmt.Println("---- >> Creating postgres_init")
	postgres_init := mini.NewPostgres()
	postgres_init.Spec.Init = &tapi.InitSpec{
		SnapshotSource: &tapi.SnapshotSourceSpec{
			Name: snapshot.Name,
		},
	}

	postgres_init, err = controller.ExtClient.Postgreses("default").Create(postgres_init)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking postgres")
	running, err = mini.CheckPostgresStatus(controller, postgres_init)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> postgres_init fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres_init)
		if !assert.Nil(t, err) {
			fmt.Println("---- >> Failed to check postgresWorkload")
			return
		}
	}

	fmt.Println("---- >> Deleting Snapshot")
	err = controller.ExtClient.Snapshots(snapshot.Namespace).Delete(snapshot.Name)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to delete Snapshot")
		return
	}

	time.Sleep(time.Second * 30)

	fmt.Println("---- >> Checking Snapshot data")
	count, err = mini.CheckSnapshotData(controller, snapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.Zero(t, count)

	fmt.Println("---- >> Deleted postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}

	fmt.Println("---- >> Deleted postgres_init")
	err = mini.DeletePostgres(controller, postgres_init)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres_init, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}

func TestUpdateScheduler(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running postgres Controller")

	// postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating postgres")
	postgres := mini.NewPostgres()
	postgres, err = controller.ExtClient.Postgreses("default").Create(postgres)
	if !assert.Nil(t, err) {
		return
	}

	time.Sleep(time.Second * 30)
	fmt.Println("---- >> Checking postgres")
	running, err := mini.CheckPostgresStatus(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, running) {
		fmt.Println("---- >> postgres fails to be Ready")
		return
	} else {
		err := mini.CheckPostgresWorkload(controller, postgres)
		if !assert.Nil(t, err){
			return
		}
	}

	postgres, err = controller.ExtClient.Postgreses("default").Get(postgres.Name)
	if !assert.Nil(t, err) {
		return
	}

	postgres.Spec.BackupSchedule = &tapi.BackupScheduleSpec{
		CronExpression: "@every 30s",
		SnapshotStorageSpec: tapi.SnapshotStorageSpec{
			BucketName: "",
			StorageSecret: &kapi.SecretVolumeSource{
				SecretName: "",
			},
		},
	}

	postgres, err = mini.UpdatePostgres(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}

	err = mini.CheckSnapshotScheduler(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DormantDatabase")
	done, err := mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseStopped)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}


	fmt.Println("---- >> WipingOut Database")
	err = mini.WipeOutDormantDatabase(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be wipedout")
	}

	fmt.Println("---- >> Checking DormantDatabase")
	done, err = mini.CheckDormantDatabasePhase(controller, postgres, tapi.DormantDatabasePhaseWipedOut)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be wipedout")
	}

	fmt.Println("---- >> Deleting DormantDatabase")
	err = mini.DeleteDormantDatabase(controller, postgres)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}
