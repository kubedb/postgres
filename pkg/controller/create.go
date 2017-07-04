package controller

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/apimachinery/pkg/docker"
	"github.com/k8sdb/apimachinery/pkg/storage"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	batch "k8s.io/client-go/pkg/apis/batch/v1"
	rbac "k8s.io/client-go/pkg/apis/rbac/v1beta1"
)

const (
	annotationDatabaseVersion = "postgres.kubedb.com/version"
	modeBasic                 = "basic"
	// Duration in Minute
	// Check whether pod under StatefulSet is running or not
	// Continue checking for this duration until failure
	durationCheckStatefulSet = time.Minute * 30
)

func (c *Controller) findService(name, namespace string) (bool, error) {
	service, err := c.Client.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	if service.Spec.Selector[amc.LabelDatabaseName] != name {
		return false, fmt.Errorf(`Intended service "%v" already exists`, name)
	}

	return true, nil
}

func (c *Controller) createService(postgres *tapi.Postgres) error {
	label := map[string]string{
		amc.LabelDatabaseName: postgres.Name,
	}
	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   postgres.Name,
			Labels: label,
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Name:       "db",
					Port:       5432,
					TargetPort: intstr.FromString("db"),
				},
			},
			Selector: label,
		},
	}
	if postgres.Spec.Monitor != nil &&
		postgres.Spec.Monitor.Agent == tapi.AgentCoreosPrometheus &&
		postgres.Spec.Monitor.Prometheus != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, apiv1.ServicePort{
			Name:       tapi.PrometheusExporterPortName,
			Port:       tapi.PrometheusExporterPortNumber,
			TargetPort: intstr.FromString(tapi.PrometheusExporterPortName),
		})
	}

	if _, err := c.Client.CoreV1().Services(postgres.Namespace).Create(svc); err != nil {
		return err
	}

	return nil
}

func (c *Controller) findStatefulSet(postgres *tapi.Postgres) (bool, error) {
	// SatatefulSet for Postgres database
	statefulSetName := getStatefulSetName(postgres.Name)
	statefulSet, err := c.Client.AppsV1beta1().StatefulSets(postgres.Namespace).Get(statefulSetName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	if statefulSet.Labels[amc.LabelDatabaseKind] != tapi.ResourceKindPostgres {
		return false, fmt.Errorf(`Intended statefulSet "%v" already exists`, statefulSetName)
	}

	return true, nil
}

func (c *Controller) ensureRBACStuff(namespace string) error {
	name := c.opt.ClusterRole
	// Ensure ServiceAccounts
	if _, err := c.Client.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{}); err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}
		sa := &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		if _, err := c.Client.CoreV1().ServiceAccounts(namespace).Create(sa); err != nil {
			return err
		}
	}

	var roleBindingRef = rbac.RoleRef{
		APIGroup: rbac.GroupName,
		Kind:     "ClusterRole",
		Name:     name,
	}
	var roleBindingSubjects = []rbac.Subject{
		{
			Kind:      rbac.ServiceAccountKind,
			Name:      name,
			Namespace: namespace,
		},
	}

	// Ensure ClusterRoleBindings
	roleBinding, err := c.Client.RbacV1beta1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}

		roleBinding := &rbac.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			RoleRef:  roleBindingRef,
			Subjects: roleBindingSubjects,
		}

		if _, err := c.Client.RbacV1beta1().ClusterRoleBindings().Create(roleBinding); err != nil {
			return err
		}

	} else {
		roleBinding.RoleRef = roleBindingRef
		roleBinding.Subjects = roleBindingSubjects
		if _, err := c.Client.RbacV1beta1().ClusterRoleBindings().Update(roleBinding); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) createStatefulSet(postgres *tapi.Postgres) (*apps.StatefulSet, error) {
	// Set labels
	labels := make(map[string]string)
	for key, val := range postgres.Labels {
		labels[key] = val
	}
	labels[amc.LabelDatabaseKind] = tapi.ResourceKindPostgres

	// Set Annotations
	annotations := make(map[string]string)
	for key, val := range postgres.Annotations {
		annotations[key] = val
	}
	annotations[annotationDatabaseVersion] = string(postgres.Spec.Version)

	podLabels := make(map[string]string)
	for key, val := range labels {
		podLabels[key] = val
	}
	podLabels[amc.LabelDatabaseName] = postgres.Name

	serviceAccount := c.opt.ClusterRole
	if serviceAccount != "default" {
		// Ensure ClusterRoles for database statefulsets
		if err := c.ensureRBACStuff(postgres.Namespace); err != nil {
			return nil, err
		}
	}

	// SatatefulSet for Postgres database
	statefulSetName := getStatefulSetName(postgres.Name)

	replicas := int32(1)
	statefulSet := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        statefulSetName,
			Namespace:   postgres.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: apps.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: c.opt.GoverningService,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            tapi.ResourceNamePostgres,
							Image:           fmt.Sprintf("%s:%s-db", docker.ImagePostgres, postgres.Spec.Version),
							ImagePullPolicy: apiv1.PullIfNotPresent,
							Ports: []apiv1.ContainerPort{
								{
									Name:          "db",
									ContainerPort: 5432,
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "secret",
									MountPath: "/srv/" + tapi.ResourceNamePostgres + "/secrets",
								},
								{
									Name:      "data",
									MountPath: "/var/pv",
								},
							},
							Args: []string{modeBasic},
						},
					},
					NodeSelector:       postgres.Spec.NodeSelector,
					ServiceAccountName: serviceAccount,
				},
			},
		},
	}

	if postgres.Spec.Monitor != nil &&
		postgres.Spec.Monitor.Agent == tapi.AgentCoreosPrometheus &&
		postgres.Spec.Monitor.Prometheus != nil {
		exporter := apiv1.Container{
			Name: "exporter",
			Args: []string{
				"export",
				fmt.Sprintf("--address=:%d", tapi.PrometheusExporterPortNumber),
				"--v=3",
			},
			Image:           docker.ImageOperator + ":" + c.opt.ExporterTag,
			ImagePullPolicy: apiv1.PullIfNotPresent,
			Ports: []apiv1.ContainerPort{
				{
					Name:          tapi.PrometheusExporterPortName,
					Protocol:      apiv1.ProtocolTCP,
					ContainerPort: int32(tapi.PrometheusExporterPortNumber),
				},
			},
		}
		statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, exporter)
	}

	if postgres.Spec.DatabaseSecret == nil {
		secretVolumeSource, err := c.createDatabaseSecret(postgres)
		if err != nil {
			return nil, err
		}
		if postgres, err = c.ExtClient.Postgreses(postgres.Namespace).Get(postgres.Name); err != nil {
			return nil, err
		}

		postgres.Spec.DatabaseSecret = secretVolumeSource

		if _, err := c.ExtClient.Postgreses(postgres.Namespace).Update(postgres); err != nil {
			return nil, err
		}
	}

	// Add secretVolume for authentication
	addSecretVolume(statefulSet, postgres.Spec.DatabaseSecret)

	// Add Data volume for StatefulSet
	addDataVolume(statefulSet, postgres.Spec.Storage)

	// Add InitialScript to run at startup
	if postgres.Spec.Init != nil && postgres.Spec.Init.ScriptSource != nil {
		addInitialScript(statefulSet, postgres.Spec.Init.ScriptSource)
	}

	if _, err := c.Client.AppsV1beta1().StatefulSets(statefulSet.Namespace).Create(statefulSet); err != nil {
		return nil, err
	}

	return statefulSet, nil
}

func (c *Controller) findSecret(namespace, secretName string) (bool, error) {
	secret, err := c.Client.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	if secret == nil {
		return false, nil
	}

	return true, nil
}

func (c *Controller) createDatabaseSecret(postgres *tapi.Postgres) (*apiv1.SecretVolumeSource, error) {
	authSecretName := postgres.Name + "-admin-auth"

	found, err := c.findSecret(postgres.Namespace, authSecretName)
	if err != nil {
		return nil, err
	}

	if !found {
		POSTGRES_PASSWORD := fmt.Sprintf("POSTGRES_PASSWORD=%s\n", rand.GeneratePassword())
		data := map[string][]byte{
			".admin": []byte(POSTGRES_PASSWORD),
		}
		secret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: authSecretName,
				Labels: map[string]string{
					amc.LabelDatabaseKind: tapi.ResourceKindPostgres,
				},
			},
			Type: apiv1.SecretTypeOpaque,
			Data: data,
		}
		if _, err := c.Client.CoreV1().Secrets(postgres.Namespace).Create(secret); err != nil {
			return nil, err
		}
	}

	return &apiv1.SecretVolumeSource{
		SecretName: authSecretName,
	}, nil
}

func addSecretVolume(statefulSet *apps.StatefulSet, secretVolume *apiv1.SecretVolumeSource) error {
	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes,
		apiv1.Volume{
			Name: "secret",
			VolumeSource: apiv1.VolumeSource{
				Secret: secretVolume,
			},
		},
	)
	return nil
}

func addDataVolume(statefulSet *apps.StatefulSet, storage *tapi.StorageSpec) {
	if storage != nil {
		// volume claim templates
		// Dynamically attach volume
		storageClassName := storage.Class
		statefulSet.Spec.VolumeClaimTemplates = []apiv1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
					Annotations: map[string]string{
						"volume.beta.kubernetes.io/storage-class": storageClassName,
					},
				},
				Spec: storage.PersistentVolumeClaimSpec,
			},
		}
	} else {
		// Attach Empty directory
		statefulSet.Spec.Template.Spec.Volumes = append(
			statefulSet.Spec.Template.Spec.Volumes,
			apiv1.Volume{
				Name: "data",
				VolumeSource: apiv1.VolumeSource{
					EmptyDir: &apiv1.EmptyDirVolumeSource{},
				},
			},
		)
	}
}

func addInitialScript(statefulSet *apps.StatefulSet, script *tapi.ScriptSourceSpec) {
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
		apiv1.VolumeMount{
			Name:      "initial-script",
			MountPath: "/var/db-script",
		},
	)
	statefulSet.Spec.Template.Spec.Containers[0].Args = []string{
		modeBasic,
		script.ScriptPath,
	}

	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes,
		apiv1.Volume{
			Name:         "initial-script",
			VolumeSource: script.VolumeSource,
		},
	)
}

func (c *Controller) createDormantDatabase(postgres *tapi.Postgres) (*tapi.DormantDatabase, error) {
	dormantDb := &tapi.DormantDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      postgres.Name,
			Namespace: postgres.Namespace,
			Labels: map[string]string{
				amc.LabelDatabaseKind: tapi.ResourceKindPostgres,
			},
		},
		Spec: tapi.DormantDatabaseSpec{
			Origin: tapi.Origin{
				ObjectMeta: metav1.ObjectMeta{
					Name:        postgres.Name,
					Namespace:   postgres.Namespace,
					Labels:      postgres.Labels,
					Annotations: postgres.Annotations,
				},
				Spec: tapi.OriginSpec{
					Postgres: &postgres.Spec,
				},
			},
		},
	}
	return c.ExtClient.DormantDatabases(dormantDb.Namespace).Create(dormantDb)
}

func (c *Controller) reCreatePostgres(postgres *tapi.Postgres) error {
	_postgres := &tapi.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:        postgres.Name,
			Namespace:   postgres.Namespace,
			Labels:      postgres.Labels,
			Annotations: postgres.Annotations,
		},
		Spec:   postgres.Spec,
		Status: postgres.Status,
	}

	if _, err := c.ExtClient.Postgreses(_postgres.Namespace).Create(_postgres); err != nil {
		return err
	}

	return nil
}

const (
	SnapshotProcess_Restore  = "restore"
	snapshotType_DumpRestore = "dump-restore"
)

func (c *Controller) createRestoreJob(postgres *tapi.Postgres, snapshot *tapi.Snapshot) (*batch.Job, error) {
	databaseName := postgres.Name
	jobName := snapshot.Name
	jobLabel := map[string]string{
		amc.LabelDatabaseName: databaseName,
		amc.LabelJobType:      SnapshotProcess_Restore,
	}
	backupSpec := snapshot.Spec.SnapshotStorageSpec
	bucket, err := storage.GetContainer(backupSpec)
	if err != nil {
		return nil, err
	}

	// Get PersistentVolume object for Backup Util pod.
	persistentVolume, err := c.getVolumeForSnapshot(postgres.Spec.Storage, jobName, postgres.Namespace)
	if err != nil {
		return nil, err
	}

	// Folder name inside Cloud bucket where backup will be uploaded
	folderName := fmt.Sprintf("%v/%v/%v", amc.DatabaseNamePrefix, snapshot.Namespace, snapshot.Spec.DatabaseName)

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: jobLabel,
		},
		Spec: batch.JobSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabel,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  SnapshotProcess_Restore,
							Image: fmt.Sprintf("%s:%s-util", docker.ImagePostgres, postgres.Spec.Version),
							Args: []string{
								fmt.Sprintf(`--process=%s`, SnapshotProcess_Restore),
								fmt.Sprintf(`--host=%s`, databaseName),
								fmt.Sprintf(`--bucket=%s`, bucket),
								fmt.Sprintf(`--folder=%s`, folderName),
								fmt.Sprintf(`--snapshot=%s`, snapshot.Name),
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "secret",
									MountPath: "/srv/" + tapi.ResourceNamePostgres + "/secrets",
								},
								{
									Name:      persistentVolume.Name,
									MountPath: "/var/" + snapshotType_DumpRestore + "/",
								},
								{
									Name:      "osmconfig",
									MountPath: storage.SecretMountPath,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "secret",
							VolumeSource: apiv1.VolumeSource{
								Secret: &apiv1.SecretVolumeSource{
									SecretName: postgres.Spec.DatabaseSecret.SecretName,
								},
							},
						},
						{
							Name:         persistentVolume.Name,
							VolumeSource: persistentVolume.VolumeSource,
						},
						{
							Name: "osmconfig",
							VolumeSource: apiv1.VolumeSource{
								Secret: &apiv1.SecretVolumeSource{
									SecretName: snapshot.Name,
								},
							},
						},
					},
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}
	if snapshot.Spec.SnapshotStorageSpec.Local != nil {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, apiv1.VolumeMount{
			Name:      snapshot.Spec.SnapshotStorageSpec.Local.Volume.Name,
			MountPath: snapshot.Spec.SnapshotStorageSpec.Local.Path,
		})
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, snapshot.Spec.SnapshotStorageSpec.Local.Volume)
	}
	return c.Client.BatchV1().Jobs(postgres.Namespace).Create(job)
}

func getStatefulSetName(databaseName string) string {
	return fmt.Sprintf("%v-%v", databaseName, tapi.ResourceCodePostgres)
}
