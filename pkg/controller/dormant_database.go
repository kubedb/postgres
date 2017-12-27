package controller

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/appscode/go/log"
	apps_util "github.com/appscode/kutil/apps/v1beta1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) Exists(om *metav1.ObjectMeta) (bool, error) {
	if _, err := c.ExtClient.Postgreses(om.Namespace).Get(om.Name, metav1.GetOptions{}); err != nil {
		if !kerr.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func (c *Controller) PauseDatabase(dormantDb *api.DormantDatabase) error {
	postgres := &api.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dormantDb.OffshootName(),
			Namespace: dormantDb.Namespace,
		},
	}
	// Delete Service
	if err := c.deleteService(postgres.OffshootName(), dormantDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}
	if err := c.deleteService(postgres.PrimaryName(), dormantDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	err := apps_util.DeleteStatefulSet(c.Client, metav1.ObjectMeta{
		Name:      dormantDb.OffshootName(),
		Namespace: dormantDb.Namespace,
	})
	if err != nil {
		return err
	}

	if err := c.deleteRBACStuff(postgres); err != nil {
		log.Errorln(err)
		return err
	}

	configMapName := fmt.Sprintf("%v-leader-lock", dormantDb.OffshootName())
	if err := c.deleteConfigMap(configMapName, dormantDb.Namespace); err != nil {
		return err
	}

	return nil
}

func (c *Controller) WipeOutDatabase(dormantDb *api.DormantDatabase) error {
	labelMap := map[string]string{
		api.LabelDatabaseName: dormantDb.Name,
		api.LabelDatabaseKind: api.ResourceKindPostgres,
	}

	labelSelector := labels.SelectorFromSet(labelMap)

	if err := c.DeleteSnapshots(dormantDb.Namespace, labelSelector); err != nil {
		return err
	}

	if err := c.DeletePersistentVolumeClaims(dormantDb.Namespace, labelSelector); err != nil {
		return err
	}

	if dormantDb.Spec.Origin.Spec.Postgres.DatabaseSecret != nil {
		if err := c.deleteSecret(dormantDb, dormantDb.Spec.Origin.Spec.Postgres.DatabaseSecret); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) deleteSecret(dormantDb *api.DormantDatabase, secretVolume *core.SecretVolumeSource) error {

	var secretFound bool = false

	postgresList, err := c.ExtClient.Postgreses(dormantDb.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, postgres := range postgresList.Items {
		databaseSecret := postgres.Spec.DatabaseSecret
		if databaseSecret != nil {
			if databaseSecret.SecretName == secretVolume.SecretName {
				secretFound = true
				break
			}
		}
	}

	if !secretFound {
		labelMap := map[string]string{
			api.LabelDatabaseKind: api.ResourceKindPostgres,
		}
		dormantDatabaseList, err := c.ExtClient.DormantDatabases(dormantDb.Namespace).List(
			metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			},
		)
		if err != nil {
			return err
		}

		for _, ddb := range dormantDatabaseList.Items {
			if ddb.Name == dormantDb.Name {
				continue
			}

			databaseSecret := ddb.Spec.Origin.Spec.Postgres.DatabaseSecret
			if databaseSecret != nil {
				if databaseSecret.SecretName == secretVolume.SecretName {
					secretFound = true
					break
				}
			}
		}
	}

	if !secretFound {
		if err := c.Client.CoreV1().Secrets(dormantDb.Namespace).Delete(secretVolume.SecretName, nil); !kerr.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (c *Controller) ResumeDatabase(dormantDb *api.DormantDatabase) error {
	origin := dormantDb.Spec.Origin
	objectMeta := origin.ObjectMeta

	if origin.Spec.Postgres.Init != nil {
		return errors.New("do not support InitSpec in spec.origin.postgres")
	}

	postgres := &api.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:        objectMeta.Name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Spec: *origin.Spec.Postgres,
	}

	if postgres.Annotations == nil {
		postgres.Annotations = make(map[string]string)
	}

	for key, val := range dormantDb.Annotations {
		postgres.Annotations[key] = val
	}

	_, err := c.ExtClient.Postgreses(postgres.Namespace).Create(postgres)
	return err
}

func (c *Controller) createDormantDatabase(postgres *api.Postgres) (*api.DormantDatabase, error) {
	dormantDb := &api.DormantDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      postgres.Name,
			Namespace: postgres.Namespace,
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			},
		},
		Spec: api.DormantDatabaseSpec{
			Origin: api.Origin{
				ObjectMeta: metav1.ObjectMeta{
					Name:        postgres.Name,
					Namespace:   postgres.Namespace,
					Labels:      postgres.Labels,
					Annotations: postgres.Annotations,
				},
				Spec: api.OriginSpec{
					Postgres: &postgres.Spec,
				},
			},
		},
	}

	if postgres.Spec.Init != nil {
		initSpec, err := json.Marshal(postgres.Spec.Init)
		if err == nil {
			dormantDb.Annotations = map[string]string{
				api.PostgresInitSpec: string(initSpec),
			}
		}
	}

	dormantDb.Spec.Origin.Spec.Postgres.Init = nil

	return c.ExtClient.DormantDatabases(dormantDb.Namespace).Create(dormantDb)
}
