package controller

import (
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
	if err := c.DeleteService(deletedDb.Name, deletedDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	statefulSetName := getStatefulSetName(deletedDb.Name)
	if err := c.DeleteStatefulSet(statefulSetName, deletedDb.Namespace); err != nil {
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

	if err := c.DeleteSnapshots(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	if err := c.DeletePersistentVolumeClaims(deletedDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	if deletedDb.Spec.Origin.Spec.Postgres.DatabaseSecret != nil {
		if err := c.deleteSecret(deletedDb); err != nil {
			return err
		}

	}

	return nil
}

func (c *Controller) deleteSecret(deletedDb *tapi.DeletedDatabase) error {

	var secretFound bool = false
	deletedDatabaseSecret := deletedDb.Spec.Origin.Spec.Postgres.DatabaseSecret

	postgresList, err := c.ExtClient.Postgreses(deletedDb.Namespace).List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	for _, postgres := range postgresList.Items {
		databaseSecret := postgres.Spec.DatabaseSecret
		if databaseSecret != nil {
			if databaseSecret.SecretName == deletedDatabaseSecret.SecretName {
				secretFound = true
				break
			}
		}
	}

	if !secretFound {
		labelMap := map[string]string{
			amc.LabelDatabaseKind: tapi.ResourceKindPostgres,
		}

		labelSelector := labels.SelectorFromSet(labelMap)

		deletedDatabaseList, err := c.ExtClient.DeletedDatabases(deletedDb.Namespace).List(
			kapi.ListOptions{
				LabelSelector: labelSelector,
			},
		)
		if err != nil {
			return err
		}

		for _, ddb := range deletedDatabaseList.Items {
			if ddb.Name == deletedDb.Name {
				continue
			}

			databaseSecret := ddb.Spec.Origin.Spec.Postgres.DatabaseSecret
			if databaseSecret != nil {
				if databaseSecret.SecretName == deletedDatabaseSecret.SecretName {
					secretFound = true
					break
				}
			}
		}
	}

	if !secretFound {
		if err := c.DeleteSecret(deletedDatabaseSecret.SecretName, deletedDb.Namespace); err != nil {
			return err
		}
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
