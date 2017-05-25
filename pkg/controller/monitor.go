package controller

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/apimachinery/pkg/monitor"
)

const ImageExporter = "k8sdb/exporter"

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

func (c *Controller) isMonitorControllerChanged(old, new *tapi.Postgres) bool {
	oldMonitorSpec := old.Spec.Monitor
	newMonitorSpec := new.Spec.Monitor

	if oldMonitorSpec == nil || newMonitorSpec == nil {
		return false
	}

	var oldI, newI int
	if oldMonitorSpec.Prometheus != nil {
		oldI = Prometheus
	}
	if newMonitorSpec.Prometheus != nil {
		newI = Prometheus
	}

	return oldI != newI
}
