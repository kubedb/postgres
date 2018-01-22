package controller

import (
	"github.com/graymeta/stow"
	_ "github.com/graymeta/stow/azure"
	_ "github.com/graymeta/stow/google"
	_ "github.com/graymeta/stow/s3"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/storage"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) DeletePersistentVolumeClaims(namespace string, selector labels.Selector) error {
	pvcList, err := c.Client.CoreV1().PersistentVolumeClaims(namespace).List(
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if err := c.Client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(pvc.Name, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) DeleteSnapshotData(snapshot *api.Snapshot) error {
	cfg, err := storage.NewOSMContext(c.Client, snapshot.Spec.SnapshotStorageSpec, snapshot.Namespace)
	if err != nil {
		return err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return err
	}
	bucket, err := snapshot.Spec.SnapshotStorageSpec.Container()
	if err != nil {
		return err
	}
	container, err := loc.Container(bucket)
	if err != nil {
		return err
	}

	prefix, _ := snapshot.Location() // error checked by .Container()
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

func (c *Controller) DeleteSnapshots(namespace string, selector labels.Selector) error {
	snapshotList, err := c.ExtClient.Snapshots(namespace).List(
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		return err
	}

	for _, snapshot := range snapshotList.Items {
		if err := c.ExtClient.Snapshots(snapshot.Namespace).Delete(snapshot.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) checkGoverningService(name, namespace string) (bool, error) {
	_, err := c.Client.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func (c *Controller) CreateGoverningService(name, namespace string) error {
	// Check if service name exists
	found, err := c.checkGoverningService(name, namespace)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: core.ServiceSpec{
			Type:      core.ServiceTypeClusterIP,
			ClusterIP: core.ClusterIPNone,
		},
	}
	_, err = c.Client.CoreV1().Services(namespace).Create(service)
	return err
}
