package controller

import (
	"fmt"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/labels"
)

type Deleter struct {
	*amc.Controller
}

func NewDeleter(c *amc.Controller) amc.Deleter {
	return &Deleter{c}
}

func (d *Deleter) Exists(deletedDb *tapi.DeletedDatabase) (bool, error) {
	if _, err := d.ExtClient.Postgreses(deletedDb.Namespace).Get(deletedDb.Name); err != nil {
		if !k8serr.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func (d *Deleter) DeleteDatabase(deletedDb *tapi.DeletedDatabase) error {
	// Delete Service
	if err := d.deleteService(deletedDb.Name, deletedDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, deletedDb.Name)
	if err := d.deleteStatefulSet(statefulSetName, deletedDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func (d *Deleter) DestroyDatabase(deletedDb *tapi.DeletedDatabase) error {
	labelMap := map[string]string{
		amc.LabelDatabaseName: deletedDb.Name,
		amc.LabelDatabaseType: DatabasePostgres,
	}

	labelSelector := labels.SelectorFromSet(labelMap)

	if err := d.DeleteDatabaseSnapshots(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	if err := d.DeletePersistentVolumeClaims(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}
