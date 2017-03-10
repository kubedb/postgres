package controller

import (
	"fmt"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/postgres/api"
)

func (w *Controller) delete(postgres *tapi.Postgres) {
	statefulSetName := fmt.Sprintf("%v-%v", DatabaseNamePrefix, postgres.Name)
	statefulSet, err := w.Client.Apps().StatefulSets(postgres.Namespace).Get(statefulSetName)
	if err != nil {
		log.Errorln(err)
		return
	}
	// Delete StatefulSet
	if err := w.deleteStatefulSet(statefulSet); err != nil {
		log.Errorln(err)
		return
	}
	// Delete Service
	if err := w.deleteService(postgres.Namespace, postgres.Name); err != nil {
		log.Errorln(err)
		return
	}
}
