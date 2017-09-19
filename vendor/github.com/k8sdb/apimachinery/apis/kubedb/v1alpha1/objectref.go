package v1alpha1

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func (d DormantDatabase) ObjectReference() *apiv1.ObjectReference {
	return &apiv1.ObjectReference{
		APIVersion:      SchemeGroupVersion.String(),
		Kind:            ResourceKindDormantDatabase,
		Namespace:       d.Namespace,
		Name:            d.Name,
		UID:             d.UID,
		ResourceVersion: d.ResourceVersion,
	}
}

func (p Postgres) ObjectReference() *apiv1.ObjectReference {
	return &apiv1.ObjectReference{
		APIVersion:      SchemeGroupVersion.String(),
		Kind:            ResourceKindPostgres,
		Namespace:       p.Namespace,
		Name:            p.Name,
		UID:             p.UID,
		ResourceVersion: p.ResourceVersion,
	}
}

func (e Elasticsearch) ObjectReference() *apiv1.ObjectReference {
	return &apiv1.ObjectReference{
		APIVersion:      SchemeGroupVersion.String(),
		Kind:            ResourceKindElasticsearch,
		Namespace:       e.Namespace,
		Name:            e.Name,
		UID:             e.UID,
		ResourceVersion: e.ResourceVersion,
	}
}

func (s Snapshot) ObjectReference() *apiv1.ObjectReference {
	return &apiv1.ObjectReference{
		APIVersion:      SchemeGroupVersion.String(),
		Kind:            ResourceKindSnapshot,
		Namespace:       s.Namespace,
		Name:            s.Name,
		UID:             s.UID,
		ResourceVersion: s.ResourceVersion,
	}
}
