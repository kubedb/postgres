package controller

import (
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	appcat_util "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1/util"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

func (c *Controller) ensureAppBinding(db *api.Postgres) (kutil.VerbType, error) {
	appmeta := db.AppBindingMeta()

	meta := metav1.ObjectMeta{
		Name:      appmeta.Name(),
		Namespace: db.Namespace,
	}

	ref, err := reference.GetReference(clientsetscheme.Scheme, db)
	if err != nil {
		return kutil.VerbUnchanged, err
	}

	_, vt, err := appcat_util.CreateOrPatchAppBinding(c.AppCatalogClient, meta, func(in *appcat.AppBinding) *appcat.AppBinding {
		core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
		in.Labels = db.OffshootSelectors()
		in.Annotations = db.Spec.ServiceTemplate.Annotations

		in.Spec.Type = appmeta.Type()
		in.Spec.ClientConfig.Service = &appcat.ServiceReference{
			Name: db.ServiceName(),
		}
		in.Spec.ClientConfig.InsecureSkipTLSVerify = true

		svcPorts := ofst.MergeServicePorts([]core.ServicePort{defaultDBPort}, db.Spec.ServiceTemplate.Spec.Ports)
		appPorts := make([]appcat.AppPort, len(svcPorts))
		for i, sp := range svcPorts {
			appPorts[i] = appcat.AppPort{
				Name: sp.Name,
				Port: sp.Port,
			}
		}
		in.Spec.ClientConfig.Ports = appPorts

		return in
	})

	if err != nil {
		c.recorder.Eventf(
			db,
			core.EventTypeWarning,
			eventer.EventReasonFailedToCreate,
			"Failed to reconcile appbinding. Reason: %v",
			err,
		)
		return kutil.VerbUnchanged, err
	} else if vt != kutil.VerbUnchanged {
		c.recorder.Eventf(
			db,
			core.EventTypeNormal,
			eventer.EventReasonSuccessful,
			"Successfully %s appbinding",
			vt,
		)
	}
	return vt, nil
}
