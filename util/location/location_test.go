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

package location

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

const projectName = "my-proj"

var errSomethingBroke = errors.New("something broke")

type ListResponse struct {
	zones []*computepb.Zone
	err   error
}
type FakeZoneListingClient struct {
	responses   []ZoneIterator
	callCounter *int
}

type FakeZoneIterator struct {
	listResponses   ListResponse
	responsePointer int
}

func (f *FakeZoneIterator) Next() (*computepb.Zone, error) {
	if f.listResponses.err != nil {
		return nil, f.listResponses.err
	}
	if len(f.listResponses.zones) == f.responsePointer {
		return nil, iterator.Done
	}
	result := f.listResponses.zones[f.responsePointer]
	f.responsePointer++
	return result, nil
}

func (f FakeZoneListingClient) List(_ context.Context, _ *computepb.ListZonesRequest, _ ...gax.CallOption) ZoneIterator {
	itToReturn := f.responses[*f.callCounter]
	*f.callCounter++
	return itToReturn
}

func (f FakeZoneListingClient) Close() error {
	return nil
}

func NewZoneClient(responses []ListResponse, callCounter *int) FakeZoneListingClient {
	iteratorResponses := make([]ZoneIterator, 0, len(responses))
	for _, listResponses := range responses {
		zoneIterator := FakeZoneIterator{
			listResponses:   listResponses,
			responsePointer: 0,
		}
		iteratorResponses = append(iteratorResponses, &zoneIterator)
	}

	return FakeZoneListingClient{responses: iteratorResponses, callCounter: callCounter}
}

type ResolveRequest struct {
	Project  string
	Location string
}

type ResolveResponse struct {
	resolvedLocation *ResolvedLocation
	err              error
}

func TestResolver_Resolve(t *testing.T) {
	tests := []struct {
		name          string
		reqs          []ResolveRequest
		zoneResponses []ListResponse
		resps         []ResolveResponse
	}{
		{
			name: "empty project",
			reqs: []ResolveRequest{
				{
					Project:  "",
					Location: "my-location",
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              ErrInvalidLocation,
				},
			},
		},
		{
			name: "empty location",
			reqs: []ResolveRequest{
				{
					Project:  "my-project",
					Location: "",
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              ErrInvalidLocation,
				},
			},
		},
		{
			name: "zone found",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1-a",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
			},
		},
		{
			name: "zone found (not first list element)",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1-a",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-west1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-west1", projectName)),
						},
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
			},
		},
		{
			name: "region found",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     nil,
						Regional: true,
					},
					err: nil,
				},
			},
		},
		{
			name: "both 404",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "fake-location",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{},
					err:   nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              ErrInvalidLocation,
				},
			},
		},
		{
			name: "zone gives non-404",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: nil,
					err:   errSomethingBroke,
				},
			},
			resps: []ResolveResponse{
				{
					err: errSomethingBroke,
				},
			},
		},
		{
			name: "sovereign zone shape specifically",
			reqs: []ResolveRequest{{
				Project:  projectName,
				Location: "u-germany-northeast1-a",
			}},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("u-germany-northeast1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/u-germany-northeast1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "u-germany-northeast1",
						Zone:     new("u-germany-northeast1-a"),
						Regional: false,
					},
					err: nil,
				},
			},
		},
		{
			name: "retries with transient errors succeed",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1-a",
				},
				{
					Project:  projectName,
					Location: "us-central1-a",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: nil,
					err:   errSomethingBroke,
				},
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              errSomethingBroke,
				},
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
				},
			},
		},
		{
			name: "resolution keyed per project and location",
			reqs: []ResolveRequest{
				{
					Project:  projectName,
					Location: "us-central1-a",
				},
				{
					Project:  projectName,
					Location: "us-east1",
				},
			},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-east1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-east1", projectName)),
						},
					},
					err: nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-east1",
						Zone:     nil,
						Regional: true,
					},
					err: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			zoneResponseCounter := 0
			fakeZones := NewZoneClient(tt.zoneResponses, &zoneResponseCounter)

			r := Resolver{
				zoneLister: fakeZones,
				cache:      &sync.Map{},
			}

			for i, req := range tt.reqs {
				got, err := r.Resolve(t.Context(), req.Project, req.Location)
				// Check for a specific zone error
				if tt.resps[i].err != nil {
					if !errors.Is(err, tt.resps[i].err) {
						t.Fatalf("Resolve() error = %v, wantErr %v", err, tt.resps[i].err)
					}
					continue
				}
				// If all the error checking is fine, assert the value is what we want
				g.Expect(got).To(HaveValue(Equal(*tt.resps[i].resolvedLocation)))
			}
			// Validate the fakes are called the right number of times
			g.Expect(zoneResponseCounter).To(Equal(len(tt.zoneResponses)))
		})
	}
}

func TestCachingBehaviour(t *testing.T) {
	tests := []struct {
		name                 string
		reqs                 []ResolveRequest
		zoneResponses        []ListResponse
		initialCacheContents map[locationCacheKey]ResolvedLocation
		desiredCacheContents map[locationCacheKey]ResolvedLocation
		resps                []ResolveResponse
	}{
		{
			name: "non-404 error, result remains uncached",
			reqs: []ResolveRequest{{
				Project:  projectName,
				Location: "us-central1-a",
			}},
			zoneResponses: []ListResponse{
				{
					zones: nil,
					err:   errSomethingBroke,
				},
			},
			desiredCacheContents: map[locationCacheKey]ResolvedLocation{},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              errSomethingBroke,
				},
			},
		},
		{
			name: "cache hit makes no second API call",
			reqs: []ResolveRequest{{
				Project:  projectName,
				Location: "us-central1-a",
			}},
			initialCacheContents: map[locationCacheKey]ResolvedLocation{
				{
					project:  projectName,
					location: "us-central1-a",
				}: {
					Region:   "us-central1",
					Zone:     new("us-central1-a"),
					Regional: false,
				},
			},
			desiredCacheContents: map[locationCacheKey]ResolvedLocation{
				{
					project:  projectName,
					location: "us-central1-a",
				}: {
					Region:   "us-central1",
					Zone:     new("us-central1-a"),
					Regional: false,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
			},
		},
		{
			name: "invalid location leaves cache empty",
			reqs: []ResolveRequest{{
				Project:  projectName,
				Location: "special-region",
			}},
			desiredCacheContents: map[locationCacheKey]ResolvedLocation{},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{},
					err:   nil,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: nil,
					err:              ErrInvalidLocation,
				},
			},
		},
		{
			name: "potential collision",
			reqs: []ResolveRequest{{Project: "foo", Location: "us-central1"}, {Project: "foo-us", Location: "central1"}},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
				{
					zones: []*computepb.Zone{},
					err:   nil,
				},
			},
			desiredCacheContents: map[locationCacheKey]ResolvedLocation{
				{
					project:  "foo",
					location: "us-central1",
				}: {
					Region:   "us-central1",
					Zone:     nil,
					Regional: true,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     nil,
						Regional: true,
					},
					err: nil,
				},
				{
					resolvedLocation: nil,
					err:              ErrInvalidLocation,
				},
			},
		},
		{
			name: "ensure zonal response is cached",
			reqs: []ResolveRequest{{Project: "foo", Location: "us-central1-a"}, {Project: "foo", Location: "us-central1-a"}},
			zoneResponses: []ListResponse{
				{
					zones: []*computepb.Zone{
						{
							Name:   new("us-central1-a"),
							Region: new(fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1", projectName)),
						},
					},
					err: nil,
				},
			},
			desiredCacheContents: map[locationCacheKey]ResolvedLocation{
				{
					project:  "foo",
					location: "us-central1-a",
				}: {
					Region:   "us-central1",
					Zone:     new("us-central1-a"),
					Regional: false,
				},
			},
			resps: []ResolveResponse{
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
				{
					resolvedLocation: &ResolvedLocation{
						Region:   "us-central1",
						Zone:     new("us-central1-a"),
						Regional: false,
					},
					err: nil,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			zoneResponseCounter := 0
			fakeZones := NewZoneClient(test.zoneResponses, &zoneResponseCounter)

			testCache := sync.Map{}
			for k, v := range test.initialCacheContents {
				testCache.Store(k, v)
			}

			r := &Resolver{
				zoneLister: fakeZones,
				cache:      &testCache,
			}

			for i, req := range test.reqs {
				got, err := r.Resolve(t.Context(), req.Project, req.Location)
				// Check for a specific zone error
				if test.resps[i].err != nil {
					if !errors.Is(err, test.resps[i].err) {
						t.Fatalf("Resolve() error = %v, wantErr %v", err, test.resps[i].err)
					}
					continue
				}
				// If all the error checking is fine, assert the value is what we want
				g.Expect(got).To(HaveValue(Equal(*test.resps[i].resolvedLocation)))
			}
			// Validate the fakes are called the right number of times
			g.Expect(zoneResponseCounter).To(Equal(len(test.zoneResponses)))

			// Check the cache contents
			typedCache := map[locationCacheKey]ResolvedLocation{}
			testCache.Range(func(k, v any) bool {
				typedCache[k.(locationCacheKey)] = v.(ResolvedLocation)
				return true
			})
			g.Expect(typedCache).To(Equal(test.desiredCacheContents))
		})
	}
}
