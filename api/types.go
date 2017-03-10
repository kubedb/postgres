package api

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// Postgres defines a Postgres database.
type Postgres struct {
	unversioned.TypeMeta `json:",inline,omitempty"`
	api.ObjectMeta       `json:"metadata,omitempty"`
	Spec                 PostgresSpec    `json:"spec,omitempty"`
	Status               *PostgresStatus `json:"status,omitempty"`
}

type PostgresSpec struct {
	// Version of Postgres to be deployed.
	Version string `json:"version,omitempty"`
	// Number of instances to deploy for a Postgres database.
	Replicas int32 `json:"replicas,omitempty"`
	// Storage spec to specify how storage shall be used.
	Storage *StorageSpec `json:"storage,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run the
	// Prometheus Pods.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Secret for Authentication
	AuthSecret *api.SecretVolumeSource `json:"authSecret,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Run initial script when starting Postgres master
	InitialScript *InitialScriptSpec `json:"initialScript,omitempty"`
}

// StorageSpec defines storage provisioning
type StorageSpec struct {
	// Name of the StorageClass to use when requesting storage provisioning.
	Class string `json:"class"`
	// Persistent Volume Claim
	api.PersistentVolumeClaimSpec `json:",inline,omitempty"`
}

type InitialScriptSpec struct {
	ScriptPath       string `json:"scriptPath,omitempty"`
	api.VolumeSource `json:",inline,omitempty"`
}

type PostgresStatus struct {
	// Total number of non-terminated pods targeted by this Postgres TPR
	Replicas int32 `json:"replicas"`
	// Total number of available pods targeted by this Postgres TPR.
	AvailableReplicas int32 `json:"availableReplicas"`
}

type PostgresList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	// Items is a list of Postgres TPR objects
	Items []*Postgres `json:"items,omitempty"`
}
