/*
Copyright 2019 The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	scheme "kubedb.dev/apimachinery/client/clientset/versioned/scheme"
)

// PerconaVersionsGetter has a method to return a PerconaVersionInterface.
// A group's client should implement this interface.
type PerconaVersionsGetter interface {
	PerconaVersions() PerconaVersionInterface
}

// PerconaVersionInterface has methods to work with PerconaVersion resources.
type PerconaVersionInterface interface {
	Create(*v1alpha1.PerconaVersion) (*v1alpha1.PerconaVersion, error)
	Update(*v1alpha1.PerconaVersion) (*v1alpha1.PerconaVersion, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PerconaVersion, error)
	List(opts v1.ListOptions) (*v1alpha1.PerconaVersionList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PerconaVersion, err error)
	PerconaVersionExpansion
}

// perconaVersions implements PerconaVersionInterface
type perconaVersions struct {
	client rest.Interface
}

// newPerconaVersions returns a PerconaVersions
func newPerconaVersions(c *CatalogV1alpha1Client) *perconaVersions {
	return &perconaVersions{
		client: c.RESTClient(),
	}
}

// Get takes name of the perconaVersion, and returns the corresponding perconaVersion object, and an error if there is any.
func (c *perconaVersions) Get(name string, options v1.GetOptions) (result *v1alpha1.PerconaVersion, err error) {
	result = &v1alpha1.PerconaVersion{}
	err = c.client.Get().
		Resource("perconaversions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PerconaVersions that match those selectors.
func (c *perconaVersions) List(opts v1.ListOptions) (result *v1alpha1.PerconaVersionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PerconaVersionList{}
	err = c.client.Get().
		Resource("perconaversions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested perconaVersions.
func (c *perconaVersions) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("perconaversions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a perconaVersion and creates it.  Returns the server's representation of the perconaVersion, and an error, if there is any.
func (c *perconaVersions) Create(perconaVersion *v1alpha1.PerconaVersion) (result *v1alpha1.PerconaVersion, err error) {
	result = &v1alpha1.PerconaVersion{}
	err = c.client.Post().
		Resource("perconaversions").
		Body(perconaVersion).
		Do().
		Into(result)
	return
}

// Update takes the representation of a perconaVersion and updates it. Returns the server's representation of the perconaVersion, and an error, if there is any.
func (c *perconaVersions) Update(perconaVersion *v1alpha1.PerconaVersion) (result *v1alpha1.PerconaVersion, err error) {
	result = &v1alpha1.PerconaVersion{}
	err = c.client.Put().
		Resource("perconaversions").
		Name(perconaVersion.Name).
		Body(perconaVersion).
		Do().
		Into(result)
	return
}

// Delete takes name of the perconaVersion and deletes it. Returns an error if one occurs.
func (c *perconaVersions) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("perconaversions").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *perconaVersions) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("perconaversions").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched perconaVersion.
func (c *perconaVersions) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PerconaVersion, err error) {
	result = &v1alpha1.PerconaVersion{}
	err = c.client.Patch(pt).
		Resource("perconaversions").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
