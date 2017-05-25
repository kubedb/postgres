package monitor

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	_ "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	prom "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tapi "github.com/k8sdb/apimachinery/api"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/api/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	kerr "k8s.io/kubernetes/pkg/api/errors"
	kepi "k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/intstr"
)

const (
	exporterName = "kubedb-exporter"
	portName     = "exporter"
	portNumber   = 8080
)

var exporterLabel = map[string]string{
	"app": exporterName,
}

type PrometheusController struct {
	kubeClient          clientset.Interface
	promClient          *v1alpha1.MonitoringV1alpha1Client
	exporterNamespace   string
	exporterDockerImage string
}

func NewPrometheusController(kubeClient clientset.Interface, promClient *v1alpha1.MonitoringV1alpha1Client, exporterNamespace, exporterDockerImage string) Monitor {
	return &PrometheusController{
		kubeClient:          kubeClient,
		promClient:          promClient,
		exporterNamespace:   exporterNamespace,
		exporterDockerImage: exporterDockerImage,
	}
}

func (c *PrometheusController) AddMonitor(meta *kapi.ObjectMeta, spec *tapi.MonitorSpec) error {
	if !c.SupportsCoreOSOperator() {
		return errors.New("Cluster does not support CoreOS Prometheus operator")
	}
	err := c.ensureExporter(meta)
	if err != nil {
		return err
	}
	return c.ensureServiceMonitor(meta, spec, spec)
}

func (c *PrometheusController) UpdateMonitor(meta *kapi.ObjectMeta, old, new *tapi.MonitorSpec) error {
	if new == nil {
		return c.DeleteMonitor(meta, old)
	}
	if old == nil {
		old = new
	}

	if !c.SupportsCoreOSOperator() {
		return errors.New("Cluster does not support CoreOS Prometheus operator")
	}
	err := c.ensureExporter(meta)
	if err != nil {
		return err
	}
	return c.ensureServiceMonitor(meta, old, new)
}

func (c *PrometheusController) DeleteMonitor(meta *kapi.ObjectMeta, spec *tapi.MonitorSpec) error {
	if !c.SupportsCoreOSOperator() {
		return errors.New("Cluster does not support CoreOS Prometheus operator")
	}
	if err := c.promClient.ServiceMonitors(spec.Prometheus.Namespace).Delete(getServiceMonitorName(meta), nil); !kerr.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *PrometheusController) SupportsCoreOSOperator() bool {
	_, err := c.kubeClient.Extensions().ThirdPartyResources().Get("prometheus." + prom.TPRGroup)
	if err != nil {
		return false
	}
	_, err = c.kubeClient.Extensions().ThirdPartyResources().Get("service-monitor." + prom.TPRGroup)
	if err != nil {
		return false
	}
	return true
}

func (c *PrometheusController) ensureExporter(meta *kapi.ObjectMeta) error {
	if err := c.ensureExporterPods(); err != nil {
		return err
	}
	if err := c.ensureExporterService(); err != nil {
		return err
	}
	return nil
}

func (c *PrometheusController) ensureExporterPods() error {
	if _, err := c.kubeClient.Extensions().Deployments(c.exporterNamespace).Get(exporterName); !kerr.IsNotFound(err) {
		return err
	}
	d := &kepi.Deployment{
		ObjectMeta: kapi.ObjectMeta{
			Name:      exporterName,
			Namespace: c.exporterNamespace,
			Labels:    exporterLabel,
		},
		Spec: kepi.DeploymentSpec{
			Replicas: 1,
			Template: kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name: "exporter",
							Args: []string{
								fmt.Sprintf("--address=:%d", portNumber),
							},
							Image:           c.exporterDockerImage,
							ImagePullPolicy: kapi.PullIfNotPresent,
							Ports: []kapi.ContainerPort{
								{
									Name:          portName,
									Protocol:      kapi.ProtocolTCP,
									ContainerPort: portNumber,
								},
							},
						},
					},
				},
			},
		},
	}
	if _, err := c.kubeClient.Extensions().Deployments(c.exporterNamespace).Create(d); !kerr.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *PrometheusController) ensureExporterService() error {
	if _, err := c.kubeClient.Core().Services(c.exporterNamespace).Get(exporterName); !kerr.IsNotFound(err) {
		return err
	}
	svc := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name:      exporterName,
			Namespace: c.exporterNamespace,
			Labels:    exporterLabel,
		},
		Spec: kapi.ServiceSpec{
			Type: kapi.ServiceTypeClusterIP,
			Ports: []kapi.ServicePort{
				{
					Name:       portName,
					Port:       portNumber,
					Protocol:   kapi.ProtocolTCP,
					TargetPort: intstr.FromString(portName),
				},
			},
			Selector: exporterLabel,
		},
	}
	if _, err := c.kubeClient.Core().Services(c.exporterNamespace).Create(svc); !kerr.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *PrometheusController) ensureServiceMonitor(meta *kapi.ObjectMeta, old, new *tapi.MonitorSpec) error {
	name := getServiceMonitorName(meta)
	if old.Prometheus.Namespace != new.Prometheus.Namespace {
		err := c.promClient.ServiceMonitors(old.Prometheus.Namespace).Delete(name, nil)
		if err != nil && !kerr.IsNotFound(err) {
			return err
		}
	}
	actual, err := c.promClient.ServiceMonitors(new.Prometheus.Namespace).Get(name)
	if kerr.IsNotFound(err) {
		return c.createServiceMonitor(meta, new)
	} else if err != nil {
		return err
	}
	if !reflect.DeepEqual(old.Prometheus.Labels, new.Prometheus.Labels) ||
		old.Prometheus.Interval != new.Prometheus.Interval {
		actual.Labels = new.Prometheus.Labels
		for _, e := range actual.Spec.Endpoints {
			e.Interval = new.Prometheus.Interval
		}
		_, err := c.promClient.ServiceMonitors(new.Prometheus.Namespace).Update(actual)
		return err
	}
	return nil
}

func (c *PrometheusController) createServiceMonitor(meta *kapi.ObjectMeta, spec *tapi.MonitorSpec) error {
	sm := &prom.ServiceMonitor{
		ObjectMeta: v1.ObjectMeta{
			Name:      getServiceMonitorName(meta),
			Namespace: spec.Prometheus.Namespace,
			Labels:    spec.Prometheus.Labels,
		},
		Spec: prom.ServiceMonitorSpec{
			NamespaceSelector: prom.NamespaceSelector{
				MatchNames: []string{c.exporterNamespace},
			},
			Endpoints: []prom.Endpoint{
				{
					Port:     portName,
					Interval: spec.Prometheus.Interval,
					Path:     fmt.Sprintf("/kubedb.com/v1beta1/namespaces/:%s/:%s/:%s/metrics", meta.Namespace, getTypeFromSelfLink(meta.SelfLink), meta.Name),
				},
			},
			Selector: unversioned.LabelSelector{
				MatchLabels: exporterLabel,
			},
		},
	}
	if _, err := c.promClient.ServiceMonitors(spec.Prometheus.Namespace).Create(sm); !kerr.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func getTypeFromSelfLink(selfLink string) string {
	if len(selfLink) == 0 {
		return ""
	}
	s := strings.Split(selfLink, "/")
	return s[len(s)-2]
}

func getServiceMonitorName(meta *kapi.ObjectMeta) string {
	return fmt.Sprintf("kubedb-%s-%s", meta.Namespace, meta.Name)
}