package controller

import (
	"errors"
	"time"

	amc "github.com/k8sdb/apimachinery/pkg/controller"
	kapi "k8s.io/kubernetes/pkg/api"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/labels"
)

func (c *Deleter) deleteService(name, namespace string) error {
	service, err := c.Client.Core().Services(namespace).Get(name)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	if service == nil {
		return nil
	}

	if service.Spec.Selector[amc.LabelDatabaseName] != name {
		return nil
	}

	return c.Client.Core().Services(namespace).Delete(name, nil)
}

func (c *Deleter) deleteStatefulSet(name, namespace string) error {
	statefulSet, err := c.Client.Apps().StatefulSets(namespace).Get(name)
	if err != nil {
		return err
	}

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
