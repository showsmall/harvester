/*
Copyright 2022 Rancher Labs, Inc.

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

// Code generated by main. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/client-go/api/v1"
)

type VirtualMachineInstanceMigrationHandler func(string, *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)

type VirtualMachineInstanceMigrationController interface {
	generic.ControllerMeta
	VirtualMachineInstanceMigrationClient

	OnChange(ctx context.Context, name string, sync VirtualMachineInstanceMigrationHandler)
	OnRemove(ctx context.Context, name string, sync VirtualMachineInstanceMigrationHandler)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, duration time.Duration)

	Cache() VirtualMachineInstanceMigrationCache
}

type VirtualMachineInstanceMigrationClient interface {
	Create(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	Update(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	UpdateStatus(*v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error)
	List(namespace string, opts metav1.ListOptions) (*v1.VirtualMachineInstanceMigrationList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.VirtualMachineInstanceMigration, err error)
}

type VirtualMachineInstanceMigrationCache interface {
	Get(namespace, name string) (*v1.VirtualMachineInstanceMigration, error)
	List(namespace string, selector labels.Selector) ([]*v1.VirtualMachineInstanceMigration, error)

	AddIndexer(indexName string, indexer VirtualMachineInstanceMigrationIndexer)
	GetByIndex(indexName, key string) ([]*v1.VirtualMachineInstanceMigration, error)
}

type VirtualMachineInstanceMigrationIndexer func(obj *v1.VirtualMachineInstanceMigration) ([]string, error)

type virtualMachineInstanceMigrationController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewVirtualMachineInstanceMigrationController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) VirtualMachineInstanceMigrationController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &virtualMachineInstanceMigrationController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromVirtualMachineInstanceMigrationHandlerToHandler(sync VirtualMachineInstanceMigrationHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v1.VirtualMachineInstanceMigration
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v1.VirtualMachineInstanceMigration))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *virtualMachineInstanceMigrationController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v1.VirtualMachineInstanceMigration))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateVirtualMachineInstanceMigrationDeepCopyOnChange(client VirtualMachineInstanceMigrationClient, obj *v1.VirtualMachineInstanceMigration, handler func(obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error)) (*v1.VirtualMachineInstanceMigration, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *virtualMachineInstanceMigrationController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *virtualMachineInstanceMigrationController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *virtualMachineInstanceMigrationController) OnChange(ctx context.Context, name string, sync VirtualMachineInstanceMigrationHandler) {
	c.AddGenericHandler(ctx, name, FromVirtualMachineInstanceMigrationHandlerToHandler(sync))
}

func (c *virtualMachineInstanceMigrationController) OnRemove(ctx context.Context, name string, sync VirtualMachineInstanceMigrationHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromVirtualMachineInstanceMigrationHandlerToHandler(sync)))
}

func (c *virtualMachineInstanceMigrationController) Enqueue(namespace, name string) {
	c.controller.Enqueue(namespace, name)
}

func (c *virtualMachineInstanceMigrationController) EnqueueAfter(namespace, name string, duration time.Duration) {
	c.controller.EnqueueAfter(namespace, name, duration)
}

func (c *virtualMachineInstanceMigrationController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *virtualMachineInstanceMigrationController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *virtualMachineInstanceMigrationController) Cache() VirtualMachineInstanceMigrationCache {
	return &virtualMachineInstanceMigrationCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *virtualMachineInstanceMigrationController) Create(obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	result := &v1.VirtualMachineInstanceMigration{}
	return result, c.client.Create(context.TODO(), obj.Namespace, obj, result, metav1.CreateOptions{})
}

func (c *virtualMachineInstanceMigrationController) Update(obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	result := &v1.VirtualMachineInstanceMigration{}
	return result, c.client.Update(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *virtualMachineInstanceMigrationController) UpdateStatus(obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	result := &v1.VirtualMachineInstanceMigration{}
	return result, c.client.UpdateStatus(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *virtualMachineInstanceMigrationController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), namespace, name, *options)
}

func (c *virtualMachineInstanceMigrationController) Get(namespace, name string, options metav1.GetOptions) (*v1.VirtualMachineInstanceMigration, error) {
	result := &v1.VirtualMachineInstanceMigration{}
	return result, c.client.Get(context.TODO(), namespace, name, result, options)
}

func (c *virtualMachineInstanceMigrationController) List(namespace string, opts metav1.ListOptions) (*v1.VirtualMachineInstanceMigrationList, error) {
	result := &v1.VirtualMachineInstanceMigrationList{}
	return result, c.client.List(context.TODO(), namespace, result, opts)
}

func (c *virtualMachineInstanceMigrationController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), namespace, opts)
}

func (c *virtualMachineInstanceMigrationController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*v1.VirtualMachineInstanceMigration, error) {
	result := &v1.VirtualMachineInstanceMigration{}
	return result, c.client.Patch(context.TODO(), namespace, name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type virtualMachineInstanceMigrationCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *virtualMachineInstanceMigrationCache) Get(namespace, name string) (*v1.VirtualMachineInstanceMigration, error) {
	obj, exists, err := c.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v1.VirtualMachineInstanceMigration), nil
}

func (c *virtualMachineInstanceMigrationCache) List(namespace string, selector labels.Selector) (ret []*v1.VirtualMachineInstanceMigration, err error) {

	err = cache.ListAllByNamespace(c.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.VirtualMachineInstanceMigration))
	})

	return ret, err
}

func (c *virtualMachineInstanceMigrationCache) AddIndexer(indexName string, indexer VirtualMachineInstanceMigrationIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v1.VirtualMachineInstanceMigration))
		},
	}))
}

func (c *virtualMachineInstanceMigrationCache) GetByIndex(indexName, key string) (result []*v1.VirtualMachineInstanceMigration, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v1.VirtualMachineInstanceMigration, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v1.VirtualMachineInstanceMigration))
	}
	return result, nil
}

type VirtualMachineInstanceMigrationStatusHandler func(obj *v1.VirtualMachineInstanceMigration, status v1.VirtualMachineInstanceMigrationStatus) (v1.VirtualMachineInstanceMigrationStatus, error)

type VirtualMachineInstanceMigrationGeneratingHandler func(obj *v1.VirtualMachineInstanceMigration, status v1.VirtualMachineInstanceMigrationStatus) ([]runtime.Object, v1.VirtualMachineInstanceMigrationStatus, error)

func RegisterVirtualMachineInstanceMigrationStatusHandler(ctx context.Context, controller VirtualMachineInstanceMigrationController, condition condition.Cond, name string, handler VirtualMachineInstanceMigrationStatusHandler) {
	statusHandler := &virtualMachineInstanceMigrationStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromVirtualMachineInstanceMigrationHandlerToHandler(statusHandler.sync))
}

func RegisterVirtualMachineInstanceMigrationGeneratingHandler(ctx context.Context, controller VirtualMachineInstanceMigrationController, apply apply.Apply,
	condition condition.Cond, name string, handler VirtualMachineInstanceMigrationGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &virtualMachineInstanceMigrationGeneratingHandler{
		VirtualMachineInstanceMigrationGeneratingHandler: handler,
		apply: apply,
		name:  name,
		gvk:   controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterVirtualMachineInstanceMigrationStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type virtualMachineInstanceMigrationStatusHandler struct {
	client    VirtualMachineInstanceMigrationClient
	condition condition.Cond
	handler   VirtualMachineInstanceMigrationStatusHandler
}

func (a *virtualMachineInstanceMigrationStatusHandler) sync(key string, obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type virtualMachineInstanceMigrationGeneratingHandler struct {
	VirtualMachineInstanceMigrationGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *virtualMachineInstanceMigrationGeneratingHandler) Remove(key string, obj *v1.VirtualMachineInstanceMigration) (*v1.VirtualMachineInstanceMigration, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1.VirtualMachineInstanceMigration{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *virtualMachineInstanceMigrationGeneratingHandler) Handle(obj *v1.VirtualMachineInstanceMigration, status v1.VirtualMachineInstanceMigrationStatus) (v1.VirtualMachineInstanceMigrationStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.VirtualMachineInstanceMigrationGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
