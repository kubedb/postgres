package controller

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/appscode/log"
	"github.com/ghodss/yaml"
	"github.com/graymeta/stow"
	_ "github.com/graymeta/stow/google"
	_ "github.com/graymeta/stow/s3"
	tapi "github.com/k8sdb/apimachinery/api"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/labels"
)

func (c *Controller) ValidateStorageSpec(spec *tapi.StorageSpec) (*tapi.StorageSpec, error) {
	if spec == nil {
		return nil, nil
	}

	if spec.Class == "" {
		return nil, fmt.Errorf(`Object 'Class' is missing in '%v'`, *spec)
	}

	if _, err := c.Client.Storage().StorageClasses().Get(spec.Class); err != nil {
		if k8serr.IsNotFound(err) {
			return nil, fmt.Errorf(`Spec.Storage.Class "%v" not found`, spec.Class)
		}
		return nil, err
	}

	if len(spec.AccessModes) == 0 {
		spec.AccessModes = []kapi.PersistentVolumeAccessMode{
			kapi.ReadWriteOnce,
		}
		log.Infof(`Using "%v" as AccessModes in "%v"`, kapi.ReadWriteOnce, *spec)
	}

	if val, found := spec.Resources.Requests[kapi.ResourceStorage]; found {
		if val.Value() <= 0 {
			return nil, errors.New("Invalid ResourceStorage request")
		}
	} else {
		return nil, errors.New("Missing ResourceStorage request")
	}

	return spec, nil
}

func (c *Controller) ValidateBackupSchedule(spec *tapi.BackupScheduleSpec) error {
	if spec == nil {
		return nil
	}
	// CronExpression can't be empty
	if spec.CronExpression == "" {
		return errors.New("Invalid cron expression")
	}

	return c.ValidateSnapshotSpec(spec.SnapshotSpec)
}

func (c *Controller) ValidateSnapshotSpec(spec tapi.SnapshotSpec) error {
	// BucketName can't be empty
	bucketName := spec.BucketName
	if bucketName == "" {
		return fmt.Errorf(`Object 'BucketName' is missing in '%v'`, spec)
	}

	// Need to provide Storage credential secret
	storageSecret := spec.StorageSecret
	if storageSecret == nil {
		return fmt.Errorf(`Object 'StorageSecret' is missing in '%v'`, spec)
	}

	// Credential SecretName  can't be empty
	storageSecretName := storageSecret.SecretName
	if storageSecretName == "" {
		return fmt.Errorf(`Object 'SecretName' is missing in '%v'`, *spec.StorageSecret)
	}
	return nil
}

const (
	keyProvider = "provider"
	keyConfig   = "config"
)

func (c *Controller) CheckBucketAccess(snapshotSpec tapi.SnapshotSpec, namespace string) error {
	secret, err := c.Client.Core().Secrets(namespace).Get(snapshotSpec.StorageSecret.SecretName)
	if err != nil {
		return err
	}

	provider := secret.Data[keyProvider]
	if provider == nil {
		return errors.New("Missing provider key")
	}
	configData := secret.Data[keyConfig]
	if configData == nil {
		return errors.New("Missing config key")
	}

	var config stow.ConfigMap
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return err
	}

	loc, err := stow.Dial(string(provider), config)
	if err != nil {
		return err
	}

	container, err := loc.Container(snapshotSpec.BucketName)
	if err != nil {
		return err
	}

	r := bytes.NewReader([]byte("CheckBucketAccess"))
	item, err := container.Put(".k8sdb", r, r.Size(), nil)
	if err != nil {
		return err
	}

	if err := container.RemoveItem(item.ID()); err != nil {
		return err
	}
	return nil
}

func (c *Controller) CreateGoverningServiceAccount(name, namespace string) error {
	var err error
	if _, err = c.Client.Core().ServiceAccounts(namespace).Get(name); err == nil {
		return nil
	}
	if !k8serr.IsNotFound(err) {
		return err
	}

	serviceAccount := &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
	_, err = c.Client.Core().ServiceAccounts(namespace).Create(serviceAccount)
	return err
}

func (c *Controller) CheckStatefulSetPodStatus(statefulSet *kapps.StatefulSet, checkDuration time.Duration) error {
	podName := fmt.Sprintf("%v-%v", statefulSet.Name, 0)

	podReady := false
	then := time.Now()
	now := time.Now()
	for now.Sub(then) < checkDuration {
		pod, err := c.Client.Core().Pods(statefulSet.Namespace).Get(podName)
		if err != nil {
			if k8serr.IsNotFound(err) {
				_, err := c.Client.Apps().StatefulSets(statefulSet.Namespace).Get(statefulSet.Name)
				if k8serr.IsNotFound(err) {
					break
				}

				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return err
			}
		}
		log.Debugf("Pod Phase: %v", pod.Status.Phase)

		// If job is success
		if pod.Status.Phase == kapi.PodRunning {
			podReady = true
			break
		}

		time.Sleep(time.Minute)
		now = time.Now()
	}
	if !podReady {
		return errors.New("Database fails to be Ready")
	}
	return nil
}

func (c *Controller) DeletePersistentVolumeClaims(namespace string, selector labels.Selector) error {
	pvcList, err := c.Client.Core().PersistentVolumeClaims(namespace).List(
		kapi.ListOptions{
			LabelSelector: selector,
		},
	)
	if err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if err := c.Client.Core().PersistentVolumeClaims(pvc.Namespace).Delete(pvc.Name, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) DeleteSnapshotData(dbSnapshot *tapi.DatabaseSnapshot) error {
	secret, err := c.Client.Core().Secrets(dbSnapshot.Namespace).Get(dbSnapshot.Spec.StorageSecret.SecretName)
	if err != nil {
		return err
	}

	provider := secret.Data[keyProvider]
	if provider == nil {
		return errors.New("Missing provider key")
	}
	configData := secret.Data[keyConfig]
	if configData == nil {
		return errors.New("Missing config key")
	}

	var config stow.ConfigMap
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return err
	}

	loc, err := stow.Dial(string(provider), config)
	if err != nil {
		return err
	}

	container, err := loc.Container(dbSnapshot.Spec.BucketName)
	if err != nil {
		return err
	}

	folderName := dbSnapshot.Labels[LabelDatabaseType] + "-" + dbSnapshot.Spec.DatabaseName
	prefix := fmt.Sprintf("%v/%v", folderName, dbSnapshot.Name)
	cursor := stow.CursorStart
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := container.RemoveItem(item.ID()); err != nil {
				return err
			}
		}
		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return nil
}

func (c *Controller) DeleteDatabaseSnapshots(namespace string, selector labels.Selector) error {
	dbSnapshotList, err := c.ExtClient.DatabaseSnapshots(namespace).List(
		kapi.ListOptions{
			LabelSelector: selector,
		},
	)
	if err != nil {
		return err
	}

	for _, dbsnapshot := range dbSnapshotList.Items {
		if err := c.ExtClient.DatabaseSnapshots(dbsnapshot.Namespace).Delete(dbsnapshot.Name); err != nil {
			return err
		}
	}
	return nil
}
