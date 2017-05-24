package monitor

import (
	tapi "github.com/k8sdb/apimachinery/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

type Monitor interface {
	AddMonitor(*kapi.ObjectMeta, *tapi.MonitorSpec) error
	UpdateMonitor(*kapi.ObjectMeta, *tapi.MonitorSpec) error
	DeleteMonitor(*kapi.ObjectMeta, *tapi.MonitorSpec) error
}
