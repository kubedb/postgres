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
	postgres, err := mini.CreatePostgres(controller, "default")
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
}

func TestReCreate(t *testing.T) {
	controller, err := getController()
	if !assert.Nil(t, err) {
		return
	}

	fmt.Println("--> Running Postgres Controller")

	// Postgres
	fmt.Println()
	fmt.Println("-- >> Testing postgres")
	fmt.Println("---- >> Creating Postgres")
	postgres, err := mini.CreatePostgres(controller, "default")
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
	postgres, err := mini.CreatePostgres(controller, "default")
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

	postgres, _ = controller.ExtClient.Postgreses(postgres.Namespace).Get(postgres.Name)
	postgres.Spec.DoNotDelete = true
	postgres, err = mini.UpdatePostres(controller, postgres)
	if !assert.Nil(t, err) {
		return
	}
	time.Sleep(time.Second * 10)

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
	postgres, err := mini.CreatePostgres(controller, "default")
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

	const (
		bucket     = "database-test"
		secretName = "google-cred"
	)

	snapshotSpec := tapi.DatabaseSnapshotSpec{
		DatabaseName: postgres.Name,
		SnapshotSpec: tapi.SnapshotSpec{
			BucketName: bucket,
			StorageSecret: &kapi.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}

	err = controller.CheckBucketAccess(bucket, &kapi.SecretVolumeSource{SecretName: secretName}, postgres.Namespace)
	if !assert.Nil(t, err) {
		return
	}

	dbSnapshot, err := mini.CreateDatabaseSnapshot(controller, postgres.Namespace, snapshotSpec)
	if !assert.Nil(t, err) {
		return
	}

	done, err := mini.CheckDatabaseSnapshot(controller, dbSnapshot)
	assert.Nil(t, err)
	if !assert.True(t, done) {
		fmt.Println("---- >> Failed to take snapshot")
		return
	}
}
