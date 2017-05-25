package controller

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/apimachinery/pkg/monitor"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	kapi "k8s.io/kubernetes/pkg/api"
	"github.com/appscode/log"
)

const ImageExporter = "kubedb/exporter"

func (c *Controller) newMonitorController(postgres *tapi.Postgres) (monitor.Monitor, error) {
	monitorSpec := postgres.Spec.Monitor

	if monitorSpec == nil {
		return nil, fmt.Errorf("MonitorSpec not found in %v", postgres.Spec)
	}

	if monitorSpec.Prometheus != nil {
		image := fmt.Sprintf("%v:%v", ImageExporter, c.option.ExporterTag)
		return monitor.NewPrometheusController(c.Client, c.promClient, c.option.ExporterNamespace, image), nil
	}

	return nil, fmt.Errorf("Monitoring controller not found for %v", monitorSpec)
}

const (
	Prometheus int = 1 + iota
)

func (c *Controller) createMonitor(postgres *tapi.Postgres) {
	monitor, err := c.newMonitorController(postgres)
	if err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToInitialize,
			"Failed to initialize monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
		return
	}
	if err := monitor.AddMonitor(postgres.ObjectMeta, postgres.Spec.Monitor); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to create monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
	}
}

func (c *Controller) deleteMonitor(postgres *tapi.Postgres) {
	m, err := c.newMonitorController(postgres)
	if err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToInitialize,
			"Failed to initialize monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
		return
	}
	if err = m.DeleteMonitor(postgres.ObjectMeta, postgres.Spec.Monitor); err != nil {
		c.eventRecorder.Eventf(
			postgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToDelete,
			"Failed to delete monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
	}
}

func (c *Controller) updateMonitor(oldPostgres, updatedPostgres *tapi.Postgres) {
	var err error
	var monitor monitor.Monitor
	if updatedPostgres.Spec.Monitor == nil {
		monitor, err = c.newMonitorController(oldPostgres)
	} else {
		monitor, err = c.newMonitorController(updatedPostgres)
	}
	if err != nil {
		c.eventRecorder.Eventf(
			updatedPostgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToInitialize,
			"Failed to initialize monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
		return
	}
	if err = monitor.UpdateMonitor(updatedPostgres.ObjectMeta, oldPostgres.Spec.Monitor, updatedPostgres.Spec.Monitor); err != nil {
		c.eventRecorder.Eventf(
			updatedPostgres,
			kapi.EventTypeWarning,
			eventer.EventReasonFailedToUpdate,
			"Failed to update monitoring system. Reason: %v",
			err,
		)
		log.Errorln(err)
	}
}