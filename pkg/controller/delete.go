package controller

import (
	"fmt"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/postgres/api"
)

func (w *Controller) delete(postgres *tapi.Postgres) {
	statefulSetName := fmt.Sprintf("%v-%v", databasePrefix, postgres.Name)

	statefulSet, err := w.Client.Apps().StatefulSets(postgres.Namespace).Get(statefulSetName)
	if err != nil {
		log.Errorln(err)
	} else {
		if err := w.deleteStatefulSet(statefulSet); err != nil {
			log.Errorln(err)
		}
	}
}
