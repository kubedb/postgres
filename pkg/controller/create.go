package controller

import (
	"fmt"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/postgres/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
)

const (
	dockerImage    = "appscode/postgres"
	databaseType   = "postgres"
	modeBasic      = "basic"
	databasePrefix = "k8sdb"
)

func (w *Controller) create(postgres *tapi.Postgres) {
	if !w.validatePostgres(postgres) {
		return
	}

	if postgres.Labels == nil {
		postgres.Labels = make(map[string]string)
	}
	if postgres.Annotations == nil {
		postgres.Annotations = make(map[string]string)
	}

	postgres.Labels["postgres.k8sdb.com/name"] = postgres.Name
	postgres.Annotations["k8sdb.com/type"] = databaseType

	dockerImage := fmt.Sprintf("%v:%v", dockerImage, postgres.Spec.Version)

	statefulSet := &kapps.StatefulSet{
		ObjectMeta: kapi.ObjectMeta{
			Name:        fmt.Sprintf("%v-%v", databasePrefix, postgres.Name),
			Namespace:   postgres.Namespace,
			Labels:      postgres.Labels,
			Annotations: postgres.Annotations,
		},
		Spec: kapps.StatefulSetSpec{
			Replicas:    postgres.Spec.Replicas,
			ServiceName: postgres.Spec.ServiceAccountName,
			Template: kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels:      postgres.Labels,
					Annotations: postgres.Annotations,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:            databaseType,
							Image:           dockerImage,
							ImagePullPolicy: kapi.PullIfNotPresent,
							Ports: []kapi.ContainerPort{
								{
									Name:          "http",
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

	statefulSet.Spec.Template.Annotations["pod.alpha.kubernetes.io/initialized"] = "true"

	// Add secretVolume for authentication
	if err := w.addSecretVolume(statefulSet, postgres.Spec.AuthSecret); err != nil {
		log.Error(err)
		return
	}

	// Add PersistentVolumeClaim for StatefulSet
	w.addPersistentVolumeClaim(statefulSet, postgres.Spec.Storage)

	// Add InitialScript to run at startup
	w.addInitialScript(statefulSet, postgres.Spec.InitialScript)

	if _, err := w.Client.Apps().StatefulSets(statefulSet.Namespace).Create(statefulSet); err != nil {
		log.Errorln(err)
		return
	}
}

func (w *Controller) validatePostgres(postgres *tapi.Postgres) bool {
	if postgres.Spec.Version == "" {
		log.Errorln(fmt.Sprintf(`Object 'Version' is missing in '%v'`, postgres.Spec))
		return false
	}

	storage := postgres.Spec.Storage
	if storage != nil {
		if storage.Class == "" {
			log.Errorln(fmt.Sprintf(`Object 'Class' is missing in '%v'`, *storage))
			return false
		}
		storageClass, err := w.Client.Storage().StorageClasses().Get(storage.Class)
		if err != nil {
			log.Errorln(err)
			return false
		}
		if storageClass == nil {
			log.Errorln(fmt.Sprintf(`Spec.Storage.Class "%v" not found`, storage.Class))
			return false
		}
	}

	authSecret := postgres.Spec.AuthSecret
	if authSecret != nil {
		if authSecret.SecretName == "" {
			log.Errorln(fmt.Sprintf(`Object 'SecretName' is missing in '%v'`, *authSecret))
			return false
		}

		found, err := w.checkSecret(postgres.Namespace, authSecret.SecretName)
		if err != nil {
			log.Errorln(err)
			return false
		}

		if !found {
			log.Errorln(fmt.Sprintf(`Spec.AuthSecret.SecretName "%v" not found`, authSecret.SecretName))
			return false
		}
	}

	initialScritp := postgres.Spec.InitialScript
	if initialScritp != nil {
		if initialScritp.ScriptPath == "" {
			log.Errorln(fmt.Sprintf(`Object 'ScriptPath' is missing in '%v'`, *initialScritp))
			return false
		}
	}
	return true
}

func (w *Controller) addSecretVolume(statefulSet *kapps.StatefulSet, secretVolume *kapi.SecretVolumeSource) error {
	if secretVolume == nil {
		authSecretName := statefulSet.Name + "-admin-auth"

		found, err := w.checkSecret(statefulSet.Namespace, authSecretName)
		if err != nil {
			return err
		}

		if !found {
			if err := w.createSecret(statefulSet.Namespace, authSecretName); err != nil {
				return err
			}
		}

		secretVolume = &kapi.SecretVolumeSource{
			SecretName: authSecretName,
		}
	}

	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts,
		kapi.VolumeMount{
			Name:      "secret",
			MountPath: "/srv/" + databaseType + "/secrets",
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

func (w *Controller) addPersistentVolumeClaim(statefulSet *kapps.StatefulSet, storage *tapi.StorageSpec) {
	if storage != nil {
		// volume claim templates
		storageClassName := storage.Class
		statefulSet.Spec.VolumeClaimTemplates = []kapi.PersistentVolumeClaim{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "volume",
					Annotations: map[string]string{
						"volume.beta.kubernetes.io/storage-class":  storageClassName,
					},
				},
				Spec: storage.PersistentVolumeClaimSpec,
			},
		}
	}
}

func (w *Controller) addInitialScript(statefulSet *kapps.StatefulSet, script *tapi.InitialScriptSpec) {
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
