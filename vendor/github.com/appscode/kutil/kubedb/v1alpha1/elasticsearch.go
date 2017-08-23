package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/appscode/jsonpatch"
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	aci "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func EnsureElasticsearch(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(alert *aci.Elasticsearch) *aci.Elasticsearch) (*aci.Elasticsearch, error) {
	return CreateOrPatchElasticsearch(c, meta, transform)
}

func CreateOrPatchElasticsearch(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(alert *aci.Elasticsearch) *aci.Elasticsearch) (*aci.Elasticsearch, error) {
	cur, err := c.Elasticsearches(meta.Namespace).Get(meta.Name)
	if kerr.IsNotFound(err) {
		return c.Elasticsearches(meta.Namespace).Create(transform(&aci.Elasticsearch{ObjectMeta: meta}))
	} else if err != nil {
		return nil, err
	}
	return PatchElasticsearch(c, cur, transform)
}

func PatchElasticsearch(c tcs.ExtensionInterface, cur *aci.Elasticsearch, transform func(*aci.Elasticsearch) *aci.Elasticsearch) (*aci.Elasticsearch, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(transform(cur))
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(curJson, modJson)
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 {
		return cur, nil
	}
	pb, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, err
	}
	glog.V(5).Infof("Patching Elasticsearch %s@%s with %s.", cur.Name, cur.Namespace, string(pb))
	result, err := c.Elasticsearches(cur.Namespace).Patch(cur.Name, types.JSONPatchType, pb)
	return result, err
}

func TryPatchElasticsearch(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Elasticsearch) *aci.Elasticsearch) (*aci.Elasticsearch, error) {
	attempt := 0
	for ; attempt < kutil.MaxAttempts; attempt = attempt + 1 {
		cur, err := c.Elasticsearches(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			return PatchElasticsearch(c, cur, transform)
		}
		glog.Errorf("Attempt %d failed to patch Elasticsearch %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(kutil.RetryInterval)
	}
	return nil, fmt.Errorf("Failed to patch Elasticsearch %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func TryUpdateElasticsearch(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Elasticsearch) *aci.Elasticsearch) (*aci.Elasticsearch, error) {
	attempt := 0
	for ; attempt < kutil.MaxAttempts; attempt = attempt + 1 {
		cur, err := c.Elasticsearches(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			oJson, err := json.Marshal(cur)
			if err != nil {
				return nil, err
			}
			modified := transform(cur)
			mJson, err := json.Marshal(modified)
			if err != nil {
				return nil, err
			}
			if bytes.Equal(oJson, mJson) {
				return cur, err
			}

			result, err := c.Elasticsearches(cur.Namespace).Update(transform(cur))
			return result, err
		}
		glog.Errorf("Attempt %d failed to update Elasticsearch %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(kutil.RetryInterval)
	}
	return nil, fmt.Errorf("Failed to update Elasticsearch %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}
