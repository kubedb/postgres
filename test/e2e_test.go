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

	fmt.Println("---- >> Checking DeletedDatabase")
	done, err := mini.CheckDeletedDatabasePhase(controller, postgres, tapi.PhaseDatabaseDeleted)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}

	fmt.Println("---- >> ReCreating Postgres")
	postgres, err = mini.ReCreatePostgres(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}

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

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DeletedDatabase")
	done, err = mini.CheckDeletedDatabasePhase(controller, postgres, tapi.PhaseDatabaseDeleted)
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
	postgres, err = mini.UpdatePostres(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}
	time.Sleep(time.Second * 10)

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DeletedDatabase")
	done, err := mini.CheckDeletedDatabasePhase(controller, postgres, tapi.PhaseDatabaseDeleted)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}

func TestDatabaseSnapshot(t *testing.T) {
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

	dbSnapshotSpec := tapi.DatabaseSnapshotSpec{
		DatabaseName: postgres.Name,
		SnapshotSpec: tapi.SnapshotSpec{
			BucketName: bucket,
			StorageSecret: &kapi.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}

	err = controller.CheckBucketAccess(dbSnapshotSpec.SnapshotSpec, postgres.Namespace)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Creating DatabaseSnapshot")
	dbSnapshot, err := mini.CreateDatabaseSnapshot(controller, postgres.Namespace, dbSnapshotSpec)
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("---- >> Checking DatabaseSnapshot")
	done, err := mini.CheckDatabaseSnapshot(controller, dbSnapshot)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to take snapshot")
		return
	}

	fmt.Println("---- >> Checking Snapshot data")
	count, err := mini.CheckSnapshotData(controller, dbSnapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.NotZero(t, count)

	fmt.Println("---- >> Deleting DatabaseSnapshot")
	err = controller.ExtClient.DatabaseSnapshots(dbSnapshot.Namespace).Delete(dbSnapshot.Name)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to delete DatabaseSnapshot")
		return
	}

	time.Sleep(time.Second * 30)

	fmt.Println("---- >> Checking Snapshot data")
	count, err = mini.CheckSnapshotData(controller, dbSnapshot)
	if !assert.Nil(t, err) {
		fmt.Println("---- >> Failed to check snapshot data")
		return
	}
	assert.Zero(t, count)

	fmt.Println("---- >> Deleted Postgres")
	err = mini.DeletePostgres(controller, postgres)
	assert.Nil(t, err)

	fmt.Println("---- >> Checking DeletedDatabase")
	done, err = mini.CheckDeletedDatabasePhase(controller, postgres, tapi.PhaseDatabaseDeleted)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to be deleted")
	}
}
