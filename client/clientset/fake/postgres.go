package fake

import (
	aci "github.com/k8sdb/postgres/api"
	"k8s.io/kubernetes/pkg/api"
	schema "k8s.io/kubernetes/pkg/api/unversioned"
	testing "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"
)

type FakePostgres struct {
	Fake *testing.Fake
	ns   string
}

var postgresResource = schema.GroupVersionResource{Group: "k8sdb.com", Version: "v1beta1", Resource: "postgreses"}

// Get returns the Postgres by name.
func (mock *FakePostgres) Get(name string) (*aci.Postgres, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewGetAction(postgresResource, mock.ns, name), &aci.Postgres{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Postgres), err
}

// List returns the a of Postgress.
func (mock *FakePostgres) List(opts api.ListOptions) (*aci.PostgresList, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewListAction(postgresResource, mock.ns, opts), &aci.Postgres{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &aci.PostgresList{}
	for _, item := range obj.(*aci.PostgresList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Create creates a new Postgres.
func (mock *FakePostgres) Create(svc *aci.Postgres) (*aci.Postgres, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewCreateAction(postgresResource, mock.ns, svc), &aci.Postgres{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Postgres), err
}

// Update updates a Postgres.
func (mock *FakePostgres) Update(svc *aci.Postgres) (*aci.Postgres, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewUpdateAction(postgresResource, mock.ns, svc), &aci.Postgres{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Postgres), err
}

// Delete deletes a Postgres by name.
func (mock *FakePostgres) Delete(name string, _ *api.DeleteOptions) error {
	_, err := mock.Fake.
		Invokes(testing.NewDeleteAction(postgresResource, mock.ns, name), &aci.Postgres{})

	return err
}

func (mock *FakePostgres) UpdateStatus(srv *aci.Postgres) (*aci.Postgres, error) {
	obj, err := mock.Fake.
		Invokes(testing.NewUpdateSubresourceAction(postgresResource, "status", mock.ns, srv), &aci.Postgres{})

	if obj == nil {
		return nil, err
	}
	return obj.(*aci.Postgres), err
}

func (mock *FakePostgres) Watch(opts api.ListOptions) (watch.Interface, error) {
	return mock.Fake.
		InvokesWatch(testing.NewWatchAction(postgresResource, mock.ns, opts))
}
