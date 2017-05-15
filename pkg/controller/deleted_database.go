package controller

import (
	"fmt"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/labels"
)

func (c *Controller) Exists(om *kapi.ObjectMeta) (bool, error) {
	if _, err := c.ExtClient.Postgreses(om.Namespace).Get(om.Name); err != nil {
		if !k8serr.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func (c *Controller) DeleteDatabase(deletedDb *tapi.DeletedDatabase) error {
	// Delete Service
	if err := c.deleteService(deletedDb.Name, deletedDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	statefulSetName := fmt.Sprintf("%v-%v", amc.DatabaseNamePrefix, deletedDb.Name)
	if err := c.deleteStatefulSet(statefulSetName, deletedDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func (c *Controller) WipeOutDatabase(deletedDb *tapi.DeletedDatabase) error {
	labelMap := map[string]string{
		amc.LabelDatabaseName: deletedDb.Name,
		amc.LabelDatabaseKind: tapi.ResourceKindPostgres,
	}

	labelSelector := labels.SelectorFromSet(labelMap)

	if err := c.DeleteDatabaseSnapshots(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	if err := c.DeletePersistentVolumeClaims(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func (c *Controller) RecoverDatabase(deletedDb *tapi.DeletedDatabase) error {
	origin := deletedDb.Spec.Origin
	objectMeta := origin.ObjectMeta
	postgres := &tapi.Postgres{
		ObjectMeta: kapi.ObjectMeta{
			Name:        objectMeta.Name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Spec: *origin.Spec.Postgres,
	}
	_, err := c.ExtClient.Postgreses(postgres.Namespace).Create(postgres)
	return err
}
