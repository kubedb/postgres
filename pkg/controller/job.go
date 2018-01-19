package controller

import (
	"fmt"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/kutil/tools/analytics"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/storage"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	snapshotProcessRestore = "restore"
	snapshotProcessBackup  = "backup"
)

func (c *Controller) createRestoreJob(postgres *api.Postgres, snapshot *api.Snapshot) (*batch.Job, error) {
	jobName := rand.WithUniqSuffix(snapshot.OffshootName())
	jobLabel := map[string]string{
		api.LabelDatabaseName: postgres.OffshootName(),
		api.LabelJobType:      snapshotProcessRestore,
	}
	backupSpec := snapshot.Spec.SnapshotStorageSpec
	bucket, err := backupSpec.Container()
	if err != nil {
		return nil, err
	}

	// Get PersistentVolume object for Backup Util pod.
	persistentVolume, err := c.getVolumeForSnapshot(postgres.Spec.Storage, jobName, postgres.Namespace)
	if err != nil {
		return nil, err
	}

	// Folder name inside Cloud bucket where backup will be uploaded
	folderName, _ := snapshot.Location()

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: jobLabel,
		},
		Spec: batch.JobSpec{
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabel,
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:            snapshotProcessRestore,
							Image:           c.opt.Docker.GetToolsImageWithTag(postgres),
							ImagePullPolicy: core.PullIfNotPresent,
							Args: []string{
								snapshotProcessRestore,
								fmt.Sprintf(`--host=%s`, postgres.PrimaryName()),
								fmt.Sprintf(`--bucket=%s`, bucket),
								fmt.Sprintf(`--folder=%s`, folderName),
								fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
								fmt.Sprintf("--analytics=%v", c.opt.EnableAnalytics),
							},
							Env: []core.EnvVar{
								{
									Name: KeyPostgresPassword,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: postgres.Spec.DatabaseSecret.SecretName,
											},
											Key: KeyPostgresPassword,
										},
									},
								},
								{
									Name:  analytics.Key,
									Value: c.opt.AnalyticsClientID,
								},
							},
							Resources: snapshot.Spec.Resources,
							VolumeMounts: []core.VolumeMount{
								{
									Name:      persistentVolume.Name,
									MountPath: "/var/data",
								},
								{
									Name:      "osmconfig",
									MountPath: storage.SecretMountPath,
									ReadOnly:  true,
								},
							},
						},
					},
					ImagePullSecrets: postgres.Spec.ImagePullSecrets,
					Volumes: []core.Volume{
						{
							Name:         persistentVolume.Name,
							VolumeSource: persistentVolume.VolumeSource,
						},
						{
							Name: "osmconfig",
							VolumeSource: core.VolumeSource{
								Secret: &core.SecretVolumeSource{
									SecretName: snapshot.OSMSecretName(),
								},
							},
						},
					},
					RestartPolicy: core.RestartPolicyNever,
				},
			},
		},
	}
	if snapshot.Spec.SnapshotStorageSpec.Local != nil {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
			Name:      "local",
			MountPath: snapshot.Spec.SnapshotStorageSpec.Local.MountPath,
			SubPath:   snapshot.Spec.SnapshotStorageSpec.Local.SubPath,
		})
		volume := core.Volume{
			Name:         "local",
			VolumeSource: snapshot.Spec.SnapshotStorageSpec.Local.VolumeSource,
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, volume)
	}
	return c.Client.BatchV1().Jobs(postgres.Namespace).Create(job)
}

func (c *Controller) GetSnapshotter(snapshot *api.Snapshot) (*batch.Job, error) {
	postgres, err := c.ExtClient.Postgreses(snapshot.Namespace).Get(snapshot.Spec.DatabaseName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	jobName := rand.WithUniqSuffix(snapshot.OffshootName())
	jobLabel := map[string]string{
		api.LabelDatabaseName: postgres.OffshootName(),
		api.LabelJobType:      snapshotProcessBackup,
	}
	backupSpec := snapshot.Spec.SnapshotStorageSpec
	bucket, err := backupSpec.Container()
	if err != nil {
		return nil, err
	}

	// Get PersistentVolume object for Backup Util pod.
	persistentVolume, err := c.getVolumeForSnapshot(postgres.Spec.Storage, jobName, snapshot.Namespace)
	if err != nil {
		return nil, err
	}

	// Folder name inside Cloud bucket where backup will be uploaded
	folderName, _ := snapshot.Location()
	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: jobLabel,
		},
		Spec: batch.JobSpec{
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabel,
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:            snapshotProcessBackup,
							Image:           c.opt.Docker.GetToolsImageWithTag(postgres),
							ImagePullPolicy: core.PullAlways,
							Args: []string{
								snapshotProcessBackup,
								fmt.Sprintf(`--host=%s`, postgres.PrimaryName()),
								fmt.Sprintf(`--bucket=%s`, bucket),
								fmt.Sprintf(`--folder=%s`, folderName),
								fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
								fmt.Sprintf("--analytics=%v", c.opt.EnableAnalytics),
							},
							Env: []core.EnvVar{
								{
									Name: KeyPostgresPassword,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: postgres.Spec.DatabaseSecret.SecretName,
											},
											Key: KeyPostgresPassword,
										},
									},
								},
								{
									Name:  analytics.Key,
									Value: c.opt.AnalyticsClientID,
								},
							},
							Resources: snapshot.Spec.Resources,
							VolumeMounts: []core.VolumeMount{
								{
									Name:      persistentVolume.Name,
									MountPath: "/var/data",
								},
								{
									Name:      "osmconfig",
									MountPath: storage.SecretMountPath,
									ReadOnly:  true,
								},
							},
						},
					},
					ImagePullSecrets: postgres.Spec.ImagePullSecrets,
					Volumes: []core.Volume{
						{
							Name:         persistentVolume.Name,
							VolumeSource: persistentVolume.VolumeSource,
						},
						{
							Name: "osmconfig",
							VolumeSource: core.VolumeSource{
								Secret: &core.SecretVolumeSource{
									SecretName: snapshot.OSMSecretName(),
								},
							},
						},
					},
					RestartPolicy: core.RestartPolicyNever,
				},
			},
		},
	}

	if snapshot.Spec.SnapshotStorageSpec.Local != nil {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
			Name:      "local",
			MountPath: snapshot.Spec.SnapshotStorageSpec.Local.MountPath,
			SubPath:   snapshot.Spec.SnapshotStorageSpec.Local.SubPath,
		})
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, core.Volume{
			Name:         "local",
			VolumeSource: snapshot.Spec.SnapshotStorageSpec.Local.VolumeSource,
		})
	}
	return job, nil
}
