/*
 Copyright 2026 The Kubernetes Authors.

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
	"testing"

	. "github.com/onsi/gomega"

	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/pkg/errors"

	"google.golang.org/api/compute/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

const (
	instanceName         = "my-instance"
	instanceTemplateName = "my-instance-template"
)

type fakeGetter struct {
	fakeResults map[string]*rmpb.TagValue
	CallCounter *int
}

var errNotFound = errors.New("not found")

func (f fakeGetter) GetNamespacedTagValue(_ context.Context, request *rmpb.GetNamespacedTagValueRequest, _ ...gax.CallOption) (*rmpb.TagValue, error) {
	*f.CallCounter++
	if res, ok := f.fakeResults[request.GetName()]; ok {
		return res, nil
	}
	return nil, errNotFound
}

func (f fakeGetter) Close() error {
	return nil
}

func createInstance(params *compute.InstanceParams, disks []*compute.AttachedDisk) *compute.Instance {
	return &compute.Instance{
		Name:   instanceName,
		Params: params,
		Disks:  disks,
	}
}

func createInstanceTemplate(properties *compute.InstanceProperties) *compute.InstanceTemplate {
	return &compute.InstanceTemplate{
		Name:       instanceTemplateName,
		Properties: properties,
	}
}

func assertDiskTagsAreCorrect(g *WithT, disks []*compute.AttachedDisk, expectation map[string]map[string]string) {
	initializeParamsReduced := map[string]map[string]string{}
	for _, disk := range disks {
		if disk.InitializeParams != nil {
			initializeParamsReduced[disk.DeviceName] = disk.InitializeParams.ResourceManagerTags
		}
	}

	g.Expect(initializeParamsReduced).To(Equal(expectation))
}

func TestTagManager_TagInstance(t *testing.T) {
	tests := []struct {
		name                       string
		disks                      []*compute.AttachedDisk
		params                     *compute.InstanceParams
		tagsInput                  infrav1.ResourceManagerTags
		knownTags                  map[string]*rmpb.TagValue
		wantedInstanceParamsTags   map[string]string
		wantedInitializeParamsTags map[string]map[string]string
		wantedCallCounter          *int
		wantErr                    bool
	}{
		{
			name:   "happy path",
			params: &compute.InstanceParams{},
			disks: []*compute.AttachedDisk{
				{
					DeviceName: "/dev/sda1",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						DiskName: "disk-1",
					},
				},
				{
					DeviceName: "/dev/sda2",
				},
				{
					DeviceName: "/dev/sda3",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						DiskName: "disk-1",
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
				"bar/key1/value2": {
					Name:   "tag456",
					Parent: "bar",
				},
			},
			wantedInstanceParamsTags: map[string]string{
				"foo": "tag123",
				"bar": "tag456",
			},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": {
					"foo": "tag123",
					"bar": "tag456",
				},
				"/dev/sda3": {
					"foo": "tag123",
					"bar": "tag456",
				},
			},
			wantedCallCounter: new(2),
		},
		{
			name: "lookup failure",
			disks: []*compute.AttachedDisk{
				{
					DeviceName: "/dev/sda1",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						DiskName: "disk-1",
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
			},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": nil,
			},
			wantErr: true,
		},
		{
			name: "empty tags",
			disks: []*compute.AttachedDisk{
				{
					DeviceName: "/dev/sda1",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						DiskName: "disk-1",
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": nil,
			},
			wantedCallCounter: new(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			callCounter := 0
			m := &TagManager{
				valueGetter: fakeGetter{
					fakeResults: tt.knownTags,
					CallCounter: &callCounter,
				},
			}
			instance := createInstance(tt.params, tt.disks)

			if err := m.TagInstance(t.Context(), instance, tt.tagsInput); (err != nil) != tt.wantErr {
				t.Errorf("TagInstance() error = %v, wantErr %v", err, tt.wantErr)
			}

			if instance.Params != nil {
				g.Expect(instance.Params.ResourceManagerTags).To(Equal(tt.wantedInstanceParamsTags))
			} else if tt.params != nil {
				t.Fatalf("instance.Params was nil when this was unexpected")
			}

			assertDiskTagsAreCorrect(g, instance.Disks, tt.wantedInitializeParamsTags)

			if tt.wantedCallCounter != nil {
				g.Expect(callCounter).To(Equal(*tt.wantedCallCounter))
			}
		})
	}
}

func TestTagManager_TagInstanceTemplate(t *testing.T) {
	tests := []struct {
		name                       string
		properties                 *compute.InstanceProperties
		tagsInput                  infrav1.ResourceManagerTags
		knownTags                  map[string]*rmpb.TagValue
		wantedPropertiesTags       map[string]string
		wantedInitializeParamsTags map[string]map[string]string
		wantedCallCounter          *int
		wantErr                    bool
	}{
		{
			name: "happy path",
			properties: &compute.InstanceProperties{
				Disks: []*compute.AttachedDisk{
					{
						DeviceName: "/dev/sda1",
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskName: "disk-1",
						},
					},
					{
						DeviceName: "/dev/sda2",
					},
					{
						DeviceName: "/dev/sda3",
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskName: "disk-1",
						},
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
				"bar/key1/value2": {
					Name:   "tag456",
					Parent: "bar",
				},
			},
			wantedPropertiesTags: map[string]string{
				"foo": "tag123",
				"bar": "tag456",
			},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": {
					"foo": "tag123",
					"bar": "tag456",
				},
				"/dev/sda3": {
					"foo": "tag123",
					"bar": "tag456",
				},
			},
		},
		{
			name: "lookup failure",
			properties: &compute.InstanceProperties{
				Disks: []*compute.AttachedDisk{
					{
						DeviceName: "/dev/sda1",
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskName: "disk-1",
						},
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
			},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": nil,
			},
			wantErr: true,
		},
		{
			name: "empty tags",
			properties: &compute.InstanceProperties{
				Disks: []*compute.AttachedDisk{
					{
						DeviceName: "/dev/sda1",
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskName: "disk-1",
						},
					},
				},
			},
			tagsInput: infrav1.ResourceManagerTags{},
			wantedInitializeParamsTags: map[string]map[string]string{
				"/dev/sda1": nil,
			},
			wantedCallCounter: new(0),
		},
		{
			name:       "nil properties",
			properties: nil,
			wantErr:    true,
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			wantedCallCounter: new(0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			callCounter := 0
			m := &TagManager{
				valueGetter: fakeGetter{
					fakeResults: tt.knownTags,
					CallCounter: &callCounter,
				},
			}
			instanceTemplate := createInstanceTemplate(tt.properties)

			if err := m.TagInstanceTemplate(t.Context(), instanceTemplate, tt.tagsInput); (err != nil) != tt.wantErr {
				t.Errorf("TagInstance() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.properties != nil {
				g.Expect(instanceTemplate.Properties.ResourceManagerTags).To(Equal(tt.wantedPropertiesTags))

				assertDiskTagsAreCorrect(g, instanceTemplate.Properties.Disks, tt.wantedInitializeParamsTags)

				if tt.wantedCallCounter != nil {
					g.Expect(callCounter).To(Equal(*tt.wantedCallCounter))
				}
			} else if !tt.wantErr {
				t.Fatalf("instanceTemplate.Properties is nil when no error has occurred")
			}
		})
	}
}

func TestTagManager_resolve(t *testing.T) {
	tests := []struct {
		name       string
		tagsInput  infrav1.ResourceManagerTags
		knownTags  map[string]*rmpb.TagValue
		wantedTags infrav1.ResourceManagerTagsMap
		wantErr    bool
	}{
		{
			name: "success",
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
				"bar/key1/value2": {
					Name:   "tag456",
					Parent: "bar",
				},
			},
			wantedTags: infrav1.ResourceManagerTagsMap{
				"bar": "tag456",
				"foo": "tag123",
			},
			wantErr: false,
		},
		{
			name: "failed lookup produces error",
			tagsInput: infrav1.ResourceManagerTags{
				{
					ParentID: "foo",
					Key:      "key1",
					Value:    "value1",
				},
				{
					ParentID: "bar",
					Key:      "key1",
					Value:    "value2",
				},
			},
			knownTags: map[string]*rmpb.TagValue{
				"foo/key1/value1": {
					Name:   "tag123",
					Parent: "foo",
				},
			},
			wantErr: true,
		},
		{
			name:       "empty tags = empty result",
			tagsInput:  infrav1.ResourceManagerTags{},
			knownTags:  map[string]*rmpb.TagValue{},
			wantedTags: map[string]string{},
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			m := &TagManager{
				valueGetter: fakeGetter{
					fakeResults: tt.knownTags,
					CallCounter: new(0),
				},
			}
			got, err := m.resolve(t.Context(), tt.tagsInput)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.wantErr && !errors.Is(err, errNotFound) {
				t.Errorf("resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			g.Expect(got).To(Equal(tt.wantedTags))
		})
	}
}
