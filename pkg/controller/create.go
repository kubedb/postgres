package controller

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/ghodss/yaml"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/util/intstr"
)

const (
	annotationDatabaseVersion = "postgres.k8sdb.com/version"
	GoverningPostgres         = "governing-postgres"
	imagePostgres             = "appscode/postgres"
	modeBasic                 = "basic"
	// Duration in Minute
	// Check whether pod under StatefulSet is running or not
	// Continue checking for this duration until failure
	durationCheckStatefulSet = time.Minute * 30
)

func (c *Controller) checkService(name, namespace string) (bool, error) {
	service, err := c.Client.Core().Services(namespace).Get(name)
	if err != nil {
		if k8serr.IsNotFound(err) {
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

func (w *Controller) createService(name, namespace string) error {
	// Check if service name exists
	found, err := w.checkService(name, namespace)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	label := map[string]string{
		amc.LabelDatabaseName: name,
	}
	service := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: label,
		},
		Spec: kapi.ServiceSpec{
			Ports: []kapi.ServicePort{
				{
					Name:       "port",
					Port:       5432,
					TargetPort: intstr.FromString("port"),
				},
			},
			Selector: label,
		},
	}

	if _, err := w.Client.Core().Services(namespace).Create(service); err != nil {
		return err
	}

	return nil
}

func (c *Controller) checkStatefulSet(postgres *tapi.Postgres) (*kapps.StatefulSet, error) {
	// SatatefulSet for Postgres database
	statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, postgres.Name)
	statefulSet, err := c.Client.Apps().StatefulSets(postgres.Namespace).Get(statefulSetName)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	if statefulSet.Labels[amc.LabelDatabaseType] != tapi.ResourceNamePostgres {
		return nil, fmt.Errorf(`Intended statefulSet "%v" already exists`, statefulSetName)
	}

	return statefulSet, nil
}

func (c *Controller) createStatefulSet(postgres *tapi.Postgres) (*kapps.StatefulSet, error) {
	_statefulSet, err := c.checkStatefulSet(postgres)
	if err != nil {
		return nil, err
	}
	if _statefulSet != nil {
		return _statefulSet, nil
	}

	// Set labels
	if postgres.Labels == nil {
		postgres.Labels = make(map[string]string)
	}
	postgres.Labels[amc.LabelDatabaseType] = tapi.ResourceNamePostgres
	// Set Annotations
	if postgres.Annotations == nil {
		postgres.Annotations = make(map[string]string)
	}
	postgres.Annotations[annotationDatabaseVersion] = postgres.Spec.Version

	podLabels := make(map[string]string)
	for key, val := range postgres.Labels {
		podLabels[key] = val
	}
	podLabels[amc.LabelDatabaseName] = postgres.Name

	dockerImage := fmt.Sprintf("%v:%v", imagePostgres, postgres.Spec.Version)

	// SatatefulSet for Postgres database
	statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, postgres.Name)

	replicas := int32(1)
	statefulSet := &kapps.StatefulSet{
		ObjectMeta: kapi.ObjectMeta{
			Name:        statefulSetName,
			Namespace:   postgres.Namespace,
			Labels:      postgres.Labels,
			Annotations: postgres.Annotations,
		},
		Spec: kapps.StatefulSetSpec{
			Replicas:    replicas,
			ServiceName: postgres.Spec.ServiceAccountName,
			Template: kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels:      podLabels,
					Annotations: postgres.Annotations,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:            tapi.ResourceNamePostgres,
							Image:           dockerImage,
							ImagePullPolicy: kapi.PullIfNotPresent,
							Ports: []kapi.ContainerPort{
								{
									Name:          "port",
									ContainerPort: 5432,
								},
							},
							Args: []string{modeBasic},
						},
					},
					NodeSelector: postgres.Spec.NodeSelector,
				},
			},
		},
	}

	if postgres.Spec.DatabaseSecret == nil {
		secretVolumeSource, err := c.createDatabaseSecret(postgres)
		if err != nil {
			return nil, err
		}
		postgres.Spec.DatabaseSecret = secretVolumeSource
	}

	// Add secretVolume for authentication
	addSecretVolume(statefulSet, postgres.Spec.DatabaseSecret)

	// Add Data volume for StatefulSet
	addDataVolume(statefulSet, postgres.Spec.Storage)

	// Add InitialScript to run at startup
	addInitialScript(statefulSet, postgres.Spec.InitialScript)

	if _, err := c.Client.Apps().StatefulSets(statefulSet.Namespace).Create(statefulSet); err != nil {
		return nil, err
	}

	return statefulSet, nil
}

func (w *Controller) checkSecret(namespace, secretName string) (bool, error) {
	secret, err := w.Client.Core().Secrets(namespace).Get(secretName)
	if err != nil {
		if k8serr.IsNotFound(err) {
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

func (c *Controller) createDatabaseSecret(postgres *tapi.Postgres) (*kapi.SecretVolumeSource, error) {
	authSecretName := postgres.Name + "-admin-auth"

	found, err := c.checkSecret(postgres.Namespace, authSecretName)
	if err != nil {
		return nil, err
	}

	if !found {
		POSTGRES_PASSWORD := fmt.Sprintf("POSTGRES_PASSWORD=%s\n", rand.GeneratePassword())
		data := map[string][]byte{
			".admin": []byte(POSTGRES_PASSWORD),
		}
		secret := &kapi.Secret{
			ObjectMeta: kapi.ObjectMeta{
				Name: authSecretName,
				Labels: map[string]string{
					amc.LabelDatabaseType: tapi.ResourceNamePostgres,
				},
			},
			Type: kapi.SecretTypeOpaque,
			Data: data,
		}
		if _, err := c.Client.Core().Secrets(postgres.Namespace).Create(secret); err != nil {
			return nil, err
		}
	}

	return &kapi.SecretVolumeSource{
		SecretName: authSecretName,
	}, nil
}

func addSecretVolume(statefulSet *kapps.StatefulSet, secretVolume *kapi.SecretVolumeSource) error {
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
		kapi.VolumeMount{
			Name:      "secret",
			MountPath: "/srv/" + tapi.ResourceNamePostgres + "/secrets",
		},
	)

	statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes,
		kapi.Volume{
			Name: "secret",
			VolumeSource: kapi.VolumeSource{
				Secret: secretVolume,
			},
		},
	)
	return nil
}

func addDataVolume(statefulSet *kapps.StatefulSet, storage *tapi.StorageSpec) {
	if storage != nil {
		// volume claim templates
		// Dynamically attach volume
		storageClassName := storage.Class
		statefulSet.Spec.VolumeClaimTemplates = []kapi.PersistentVolumeClaim{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "volume",
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
			kapi.Volume{
				Name: "volume",
				VolumeSource: kapi.VolumeSource{
					EmptyDir: &kapi.EmptyDirVolumeSource{},
				},
			},
		)
	}
}

func addInitialScript(statefulSet *kapps.StatefulSet, script *tapi.InitialScriptSpec) {
	if script != nil {
		statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
			kapi.VolumeMount{
				Name:      "initial-script",
				MountPath: "/var/db-script",
			},
		)
		statefulSet.Spec.Template.Spec.Containers[0].Args = []string{
			modeBasic,
			script.ScriptPath,
		}

		statefulSet.Spec.Template.Spec.Volumes = append(statefulSet.Spec.Template.Spec.Volumes,
			kapi.Volume{
				Name:         "initial-script",
				VolumeSource: script.VolumeSource,
			},
		)
	}
}

func (w *Controller) createDeletedDatabase(postgres *tapi.Postgres) (*tapi.DeletedDatabase, error) {
	deletedDb := &tapi.DeletedDatabase{
		ObjectMeta: kapi.ObjectMeta{
			Name:      postgres.Name,
			Namespace: postgres.Namespace,
			Labels: map[string]string{
				amc.LabelDatabaseType: tapi.ResourceNamePostgres,
			},
		},
	}

	yamlDataByte, _ := yaml.Marshal(postgres)
	if yamlDataByte != nil {
		deletedDb.Annotations = map[string]string{
			tapi.ResourceNamePostgres: string(yamlDataByte),
		}
	}
	return w.ExtClient.DeletedDatabases(deletedDb.Namespace).Create(deletedDb)
}

func (w *Controller) reCreatePostgres(postgres *tapi.Postgres) error {
	_postgres := &tapi.Postgres{
		ObjectMeta: kapi.ObjectMeta{
			Name:        postgres.Name,
			Namespace:   postgres.Namespace,
			Labels:      postgres.Labels,
			Annotations: postgres.Annotations,
		},
		Spec:   postgres.Spec,
		Status: postgres.Status,
	}

	if _, err := w.ExtClient.Postgreses(_postgres.Namespace).Create(_postgres); err != nil {
		return err
	}

	return nil
}
