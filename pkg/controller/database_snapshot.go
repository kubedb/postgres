package controller

import (
	"errors"
	"fmt"

	"github.com/appscode/go/crypto/rand"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

const (
	SnapshotProcess_Backup = "backup"
	storageSecretMountPath = "/var/credentials/"
	tagPostgresUtil        = "9.5-v4-util"
)

type Snapshotter struct {
	*amc.Controller
}

func NewSnapshotter(c *amc.Controller) amc.Snapshotter {
	return &Snapshotter{c}
}

func (s *Snapshotter) Validate(snapshot *tapi.DatabaseSnapshot) error {
	// Database name can't empty
	databaseName := snapshot.Spec.DatabaseName
	if databaseName == "" {
		return fmt.Errorf(`Object 'DatabaseName' is missing in '%v'`, snapshot.Spec)
	}

	labelMap := map[string]string{
		amc.LabelDatabaseType:   DatabasePostgres,
		amc.LabelDatabaseName:   snapshot.Spec.DatabaseName,
		amc.LabelSnapshotStatus: string(tapi.StatusSnapshotRunning),
	}

	snapshotList, err := s.ExtClient.DatabaseSnapshots(snapshot.Namespace).List(kapi.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(labelMap)),
	})
	if err != nil {
		return err
	}

	if len(snapshotList.Items) > 0 {
		unversionedNow := unversioned.Now()
		snapshot.Status.StartTime = &unversionedNow
		snapshot.Status.CompletionTime = &unversionedNow
		snapshot.Status.Status = tapi.StatusSnapshotFailed
		snapshot.Status.Reason = "One DatabaseSnapshot is already Running"
		if _, err := s.ExtClient.DatabaseSnapshots(snapshot.Namespace).Update(snapshot); err != nil {
			return err
		}
		return errors.New("One DatabaseSnapshot is already Running")
	}

	snapshotSpec := snapshot.Spec.SnapshotSpec
	if err := s.ValidateSnapshotSpec(snapshotSpec); err != nil {
		return err
	}

	if err := s.CheckBucketAccess(snapshotSpec.BucketName, snapshotSpec.StorageSecret,
		snapshot.Namespace); err != nil {
		return err
	}
	return nil
}

func (s *Snapshotter) GetDatabase(snapshot *tapi.DatabaseSnapshot) (runtime.Object, error) {
	return s.ExtClient.Postgreses(snapshot.Namespace).Get(snapshot.Spec.DatabaseName)
}

func (s *Snapshotter) GetSnapshotter(snapshot *tapi.DatabaseSnapshot) (*kbatch.Job, error) {
	databaseName := snapshot.Spec.DatabaseName
	jobName := rand.WithUniqSuffix(SnapshotProcess_Backup + "-" + databaseName)
	jobLabel := map[string]string{
		amc.LabelDatabaseName: databaseName,
		amc.LabelJobType:      SnapshotProcess_Backup,
	}
	backupSpec := snapshot.Spec.SnapshotSpec

	postgres, err := s.ExtClient.Postgreses(snapshot.Namespace).Get(databaseName)
	if err != nil {
		return nil, err
	}

	// Get PersistentVolume object for Backup Util pod.
	persistentVolume, err := s.getVolumeForSnapshot(postgres.Spec.Storage, jobName, snapshot.Namespace)
	if err != nil {
		return nil, err
	}

	// Folder name inside Cloud bucket where backup will be uploaded
	folderName := DatabasePostgres + "-" + databaseName

	job := &kbatch.Job{
		ObjectMeta: kapi.ObjectMeta{
			Name:   jobName,
			Labels: jobLabel,
		},
		Spec: kbatch.JobSpec{
			Template: kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: jobLabel,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  SnapshotProcess_Backup,
							Image: imagePostgres + ":" + tagPostgresUtil,
							Args: []string{
								fmt.Sprintf(`--process=%s`, SnapshotProcess_Backup),
								fmt.Sprintf(`--host=%s`, databaseName),
								fmt.Sprintf(`--bucket=%s`, backupSpec.BucketName),
								fmt.Sprintf(`--folder=%s`, folderName),
								fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
							},
							VolumeMounts: []kapi.VolumeMount{
								{
									Name:      "secret",
									MountPath: "/srv/" + DatabasePostgres + "/secrets",
								},
								{
									Name:      "cloud",
									MountPath: storageSecretMountPath,
								},
								{
									Name:      persistentVolume.Name,
									MountPath: "/var/" + SnapshotProcess_Backup + "/",
								},
							},
						},
					},
					Volumes: []kapi.Volume{
						{
							Name: "secret",
							VolumeSource: kapi.VolumeSource{
								Secret: &kapi.SecretVolumeSource{
									SecretName: postgres.Spec.DatabaseSecret.SecretName,
								},
							},
						},
						{
							Name: "cloud",
							VolumeSource: kapi.VolumeSource{
								Secret: backupSpec.StorageSecret,
							},
						},
						{
							Name:         persistentVolume.Name,
							VolumeSource: persistentVolume.VolumeSource,
						},
					},
					RestartPolicy: kapi.RestartPolicyNever,
				},
			},
		},
	}
	return job, nil
}

func (s *Snapshotter) DestroySnapshot(dbSnapshot *tapi.DatabaseSnapshot) error {
	return s.DeleteSnapshotData(dbSnapshot)
}

func (s *Snapshotter) getVolumeForSnapshot(storage *tapi.StorageSpec, jobName, namespace string) (*kapi.Volume, error) {
	volume := &kapi.Volume{
		Name: "util-volume",
	}
	if storage != nil {
		claim := &kapi.PersistentVolumeClaim{
			ObjectMeta: kapi.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
				Annotations: map[string]string{
					"volume.beta.kubernetes.io/storage-class": storage.Class,
				},
			},
			Spec: storage.PersistentVolumeClaimSpec,
		}

		if _, err := s.Client.Core().PersistentVolumeClaims(claim.Namespace).Create(claim); err != nil {
			return nil, err
		}

		volume.PersistentVolumeClaim = &kapi.PersistentVolumeClaimVolumeSource{
			ClaimName: claim.Name,
		}
	} else {
		volume.EmptyDir = &kapi.EmptyDirVolumeSource{}
	}
	return volume, nil
}
