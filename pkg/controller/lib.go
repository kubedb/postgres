package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/labels"
)

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

func (w *Controller) createSecret(namespace, secretName string) error {
	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				"k8sdb.com/type": databaseType,
			},
		},
		Type: kapi.SecretTypeOpaque,
		Data: create_auth(),
	}
	_, err := w.Client.Core().Secrets(namespace).Create(secret)
	return err
}

// To create password secret
func create_auth() map[string][]byte {
	POSTGRES_PASSWORD := fmt.Sprintf("POSTGRES_PASSWORD=%s\n", rand.GeneratePassword())
	data := map[string][]byte{
		".admin": []byte(POSTGRES_PASSWORD),
	}
	return data
}

func (c *Controller) deleteStatefulSet(statefulSet *kapps.StatefulSet) error {
	// Update StatefulSet
	statefulSet.Spec.Replicas = 0
	if _, err := c.Client.Apps().StatefulSets(statefulSet.Namespace).Update(statefulSet); err != nil {
		return err
	}

	labelSelector := labels.SelectorFromSet(statefulSet.Spec.Selector.MatchLabels)

	check := 1
	for {
		time.Sleep(time.Second * 30)
		podList, err := c.Client.Core().Pods(kapi.NamespaceAll).List(kapi.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}
		if len(podList.Items) == 0 {
			break
		}

		if check == 5 {
			return errors.New("Fail to delete StatefulSet Pods")
		}
		check++
	}

	// Delete StatefulSet
	return c.Client.Apps().StatefulSets(statefulSet.Namespace).Delete(statefulSet.Name, nil)
}
