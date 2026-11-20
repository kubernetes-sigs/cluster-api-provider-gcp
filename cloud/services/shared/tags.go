/*
Copyright 2023 The Kubernetes Authors.

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

package shared

import (
	"context"
	"fmt"
	"io"
	"maps"

	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var errNoBinder = errors.New("tag manager has no tag bindings client")

// TagValueGetter allows us to only use concrete client interfaces when in production code
type TagValueGetter interface {
	GetNamespacedTagValue(context.Context, *rmpb.GetNamespacedTagValueRequest, ...gax.CallOption) (*rmpb.TagValue, error)
	io.Closer
}

// TagBinder allows us to only use concrete client interfaces when in production code
type TagBinder interface {
	CreateTagBinding(context.Context, *rmpb.CreateTagBindingRequest, ...gax.CallOption) (*resourcemanager.CreateTagBindingOperation, error)
	io.Closer
}

// TagManager is responsible for centralising dealing with the ResourceManager Tags on different objects.
type TagManager struct {
	valueGetter    TagValueGetter
	binder         TagBinder
	newValueGetter func(ctx context.Context) (TagValueGetter, error)
	newBinder      func(ctx context.Context) (TagBinder, error)
}

// NewLazyTagManager constructs a TagManager that lazily instantiates its clients rather than creating them upfront
func NewLazyTagManager(valueGetterFactory func(ctx context.Context) (TagValueGetter, error), binderFactory func(ctx context.Context) (TagBinder, error)) *TagManager {
	return &TagManager{
		newValueGetter: valueGetterFactory,
		newBinder:      binderFactory,
	}
}

// TagInstance applies a set of tags to a GCE compute instance.
func (m *TagManager) TagInstance(ctx context.Context, instance *compute.Instance, tags infrav1.ResourceManagerTags) error {
	if len(tags) == 0 {
		return nil
	}
	// Resolve the tags first
	tagMap, err := m.resolve(ctx, tags)
	if err != nil {
		return err
	}
	// Set the tags on the instance
	if instance.Params == nil {
		instance.Params = &compute.InstanceParams{}
	}
	instance.Params.ResourceManagerTags = maps.Clone(tagMap)

	// Set the tags on each disk
	for _, disk := range instance.Disks {
		if disk.InitializeParams != nil {
			disk.InitializeParams.ResourceManagerTags = maps.Clone(tagMap)
		}
	}
	return nil
}

// TagInstanceTemplate applies a set of tags to an InstanceTemplate.
func (m *TagManager) TagInstanceTemplate(ctx context.Context, instanceTemplate *compute.InstanceTemplate, tags infrav1.ResourceManagerTags) error {
	if len(tags) == 0 {
		return nil
	}
	if instanceTemplate.Properties == nil {
		return errors.New("no instance template properties provided")
	}

	// Resolve the tags first
	tagMap, err := m.resolve(ctx, tags)
	if err != nil {
		return err
	}
	// Set the tags on the instance
	instanceTemplate.Properties.ResourceManagerTags = maps.Clone(tagMap)

	// Set the tags on each disk
	for _, disk := range instanceTemplate.Properties.Disks {
		if disk.InitializeParams != nil {
			disk.InitializeParams.ResourceManagerTags = maps.Clone(tagMap)
		}
	}
	return nil
}

// TagCluster applies a set of ResourceManager Tags to a GKE Cluster object
func (m *TagManager) TagCluster(ctx context.Context, clusterFullName string, tags infrav1.ResourceManagerTags) error {
	if len(tags) == 0 {
		return nil
	}

	if m.binder == nil && m.newBinder == nil {
		return errNoBinder
	}

	resolvedTags, err := m.resolve(ctx, tags)
	if err != nil {
		return err
	}

	if m.binder == nil {
		binder, err := m.newBinder(ctx)
		if err != nil {
			return fmt.Errorf("could not create tag binder: %w", err)
		}
		m.binder = binder
	}

	for _, tagValue := range resolvedTags {
		op, err := m.binder.CreateTagBinding(ctx, &rmpb.CreateTagBindingRequest{
			TagBinding: &rmpb.TagBinding{
				TagValue: tagValue,
				Parent:   "//container.googleapis.com/" + clusterFullName,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create tag binding: %w", err)
		}

		_, err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("tag binding operation failed: %w", err)
		}
	}
	return nil
}

// Close is responsible for closing the TagManager and any underlying clients.
func (m *TagManager) Close() error {
	var errList []error

	if m.valueGetter != nil {
		errList = append(errList, m.valueGetter.Close())
	}
	if m.binder != nil {
		errList = append(errList, m.binder.Close())
	}
	return kerrors.NewAggregate(errList)
}

func (m *TagManager) resolve(ctx context.Context, t infrav1.ResourceManagerTags) (infrav1.ResourceManagerTagsMap, error) {
	tagValueList := make(infrav1.ResourceManagerTagsMap, len(t))
	if len(t) == 0 {
		return tagValueList, nil
	}

	// Lazily instantiate valueGetter if it's not set
	err := m.upsertValueGetter(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate TagValue getter: %w", err)
	}

	for _, tag := range t {
		tagValue, err := m.valueGetter.GetNamespacedTagValue(ctx, &rmpb.GetNamespacedTagValueRequest{
			Name: tag.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get tag values for tag %s: %w", tag, err)
		}
		tagValueList[tagValue.GetParent()] = tagValue.GetName()
	}

	return tagValueList, nil
}

func (m *TagManager) upsertValueGetter(ctx context.Context) error {
	if m.valueGetter == nil && m.newValueGetter == nil {
		return errors.New("nil getter and getter factory")
	}
	if m.valueGetter == nil && m.newValueGetter != nil {
		vg, err := m.newValueGetter(ctx)
		if err != nil {
			return err
		}
		m.valueGetter = vg
	}
	return nil
}
