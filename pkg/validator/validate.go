package validator

import (
	"errors"
	"fmt"
	"strings"

	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/k8sdb/apimachinery/pkg/docker"
	amv "github.com/k8sdb/apimachinery/pkg/validator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ValidatePostgres(client kubernetes.Interface, postgres *tapi.Postgres) error {
	if postgres.Spec.Version == "" {
		return fmt.Errorf(`object 'Version' is missing in '%v'`, postgres.Spec)
	}

	version := fmt.Sprintf("%v-db", postgres.Spec.Version)
	if err := docker.CheckDockerImageVersion(docker.ImagePostgres, version); err != nil {
		return fmt.Errorf(`image %v:%v not found`, docker.ImagePostgres, version)
	}

	if postgres.Spec.Storage != nil {
		var err error
		if err = amv.ValidateStorage(client, postgres.Spec.Storage); err != nil {
			return err
		}
	}

	configuration := postgres.Spec.Configuration
	if configuration.Standby != "" {
		if strings.ToLower(configuration.Standby) != "hot" &&
			strings.ToLower(configuration.Standby) != "warm" {
			return fmt.Errorf(`configuration.Standby "%v" invalid`, configuration.Standby)
		}
	}
	if configuration.Streaming != "" {
		// TODO: synchronous Streaming is unavailable due to lack of support
		if /*strings.ToLower(configuration.Streaming) != "synchronous" &&
		 */strings.ToLower(configuration.Streaming) != "asynchronous" {
			return fmt.Errorf(`configuration.Streaming "%v" invalid`, configuration.Streaming)
		}
	}

	archive := postgres.Spec.Archive
	if postgres.Spec.Archive != nil {
		switch archive.Type {
		case "wal-g":
			if archive.Secret == nil {
				return errors.New("archive.Secret not found")
			}
			if _, err := client.CoreV1().Secrets(postgres.Namespace).Get(archive.Secret.SecretName, metav1.GetOptions{}); err != nil {
				return err
			}
		case "":
			return errors.New("archive.Type not found")
		default:
			return fmt.Errorf(`archive.Type "%v" invalid`, archive.Type)

		}
	}

	databaseSecret := postgres.Spec.DatabaseSecret
	if databaseSecret != nil {
		if _, err := client.CoreV1().Secrets(postgres.Namespace).Get(databaseSecret.SecretName, metav1.GetOptions{}); err != nil {
			return err
		}
	}

	backupScheduleSpec := postgres.Spec.BackupSchedule
	if backupScheduleSpec != nil {
		if err := amv.ValidateBackupSchedule(client, backupScheduleSpec, postgres.Namespace); err != nil {
			return err
		}
	}

	monitorSpec := postgres.Spec.Monitor
	if monitorSpec != nil {
		if err := amv.ValidateMonitorSpec(monitorSpec); err != nil {
			return err
		}

	}
	return nil
}
