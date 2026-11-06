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

type tagValueGetter interface {
	GetNamespacedTagValue(context.Context, *rmpb.GetNamespacedTagValueRequest, ...gax.CallOption) (*rmpb.TagValue, error)
	io.Closer
}

type tagBinder interface {
	CreateTagBinding(context.Context, *rmpb.CreateTagBindingRequest, ...gax.CallOption) (*resourcemanager.CreateTagBindingOperation, error)
	io.Closer
}

// TagManager is responsible for centralising dealing with the ResourceManager Tags on different objects.
type TagManager struct {
	valueGetter tagValueGetter
	binder      tagBinder
}

// NewTagManager constructs a TagManager.
func NewTagManager(tvg tagValueGetter) *TagManager {
	return &TagManager{valueGetter: tvg}
}

// NewTagManagerWithBinder constructs a TagManager, but with a tag binder as well.
func NewTagManagerWithBinder(tvg tagValueGetter, tb tagBinder) *TagManager {
	return &TagManager{valueGetter: tvg, binder: tb}
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

// TagCluster applies a set of ResourceManager Tagst to a GKE Cluster object
func (m *TagManager) TagCluster(ctx context.Context, clusterFullName string, tags infrav1.ResourceManagerTags) error {
	if len(tags) == 0 {
		return nil
	}
	if m.binder == nil {
		return errNoBinder
	}
	resolvedTags, err := m.resolve(ctx, tags)
	if err != nil {
		return err
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
	errList := []error{m.valueGetter.Close()}
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
