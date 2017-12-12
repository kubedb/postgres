package controller

import (
	"github.com/appscode/go/log"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

func (c *Controller) deleteConfigMap(name, namespace string) error {
	if err := c.Client.CoreV1().ConfigMaps(namespace).Delete(name, nil); !kerr.IsNotFound(err) {
		log.Errorln(err)
		return err
	}
	return nil
}
