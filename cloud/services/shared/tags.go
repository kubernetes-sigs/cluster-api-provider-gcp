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
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type tagValueGetter interface {
	GetNamespacedTagValue(context.Context, *rmpb.GetNamespacedTagValueRequest, ...gax.CallOption) (*rmpb.TagValue, error)
	io.Closer
}

// TagManager is responsible for centralising dealing with the ResourceManager Tags on different objects.
type TagManager struct {
	valueGetter tagValueGetter
}

// NewTagManager constructs a TagManager.
func NewTagManager(tvg tagValueGetter) *TagManager {
	return &TagManager{valueGetter: tvg}
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

// Close is responsible for closing the TagManager and any underlying clients.
func (m *TagManager) Close() error {
	return m.valueGetter.Close()
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

// ResourceTagBinding creates a TagBinding between a TagValue and a Google Cloud resource.
// If any of the SDK calls fail, the error is logged and no action is taken.
func ResourceTagBinding(ctx context.Context, client *resourcemanager.TagBindingsClient, spec infrav1exp.GCPManagedClusterSpec, name string) error {
	for _, tag := range spec.ResourceManagerTags {
		tagValue, err := getTagValues(ctx, tag)
		if err != nil {
			return fmt.Errorf("failed to retrieve tag value: %w", err)
		}
		req := &rmpb.CreateTagBindingRequest{
			TagBinding: &rmpb.TagBinding{
				Parent:   fmt.Sprintf("//container.googleapis.com/projects/%s/locations/%s/clusters/%s", tag.ParentID, spec.Region, name),
				TagValue: tagValue.GetName(),
			},
		}
		op, err := client.CreateTagBinding(ctx, req)
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

func getTagValues(ctx context.Context, tag infrav1.ResourceManagerTag) (*rmpb.TagValue, error) {
	log := log.FromContext(ctx)
	client, err := resourcemanager.NewTagValuesClient(ctx)
	if err != nil {
		log.Error(err, "failed to create tag values client")
		return &rmpb.TagValue{}, err
	}
	defer client.Close()

	req := &rmpb.GetNamespacedTagValueRequest{
		Name: fmt.Sprintf("%s/%s/%s", tag.ParentID, tag.Key, tag.Value),
	}
	tagValue, err := client.GetNamespacedTagValue(ctx, req)
	if err != nil {
		return &rmpb.TagValue{}, err
	}

	return tagValue, nil
}
