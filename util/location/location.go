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

// Package location implements methods to resolve locations against GCP.
package location

import (
	"context"
	"errors"
	"path"
	"sync"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"
)

// ErrInvalidLocation represents a location that we cannot validate via the GCP APIs
var ErrInvalidLocation = errors.New("invalid location")

// ResolvedLocation indicates a location that is validated via the GCP APIs so we know it's valid
type ResolvedLocation struct {
	Region string
	// Zone will be nil if a ResolvedLocation represents a region
	Zone     *string
	Regional bool
}

// Resolving is an interface to help passing in stubs in tests that don't need the full capabilities of a Resolver
type Resolving interface {
	Resolve(ctx context.Context, project, location string) (*ResolvedLocation, error)
}

// ZoneLister allows us to mock a client that gets Zones during testing
type ZoneLister interface {
	List(ctx context.Context, req *computepb.ListZonesRequest, opts ...gax.CallOption) ZoneIterator
	Close() error
}

// ZoneIterator allows us to mock the iterator interface without relying on GCP
type ZoneIterator interface {
	Next() (*computepb.Zone, error)
}

// Resolver is a struct that allows us to resolve a location into a concrete region/zone via the GCP APIs, it contains
// caching so it doesn't repeat unnecessary API calls
type Resolver struct {
	zoneLister ZoneLister
	cache      *sync.Map
}

var cache = sync.Map{}

type locationCacheKey struct {
	project  string
	location string
}

// NewResolver is a constructor for Resolver, this uses the package level cache variable so it can be shared
// across all versions of the Resolver. If you need one with an isolated cache, construct it directly
func NewResolver(zoneLister ZoneLister) *Resolver {
	return &Resolver{
		zoneLister: zoneLister,
		cache:      &cache,
	}
}

// Resolve takes the project and location and validates them against the GCP APIs to ensure they are valid
// before we attempt to use them.
func (r *Resolver) Resolve(ctx context.Context, project, location string) (*ResolvedLocation, error) {
	if project == "" || location == "" {
		return nil, ErrInvalidLocation
	}

	// Get the resolved location from the cache if we've done it before
	if v, ok := r.cache.Load(locationCacheKey{
		project:  project,
		location: location,
	}); ok {
		cachedLocation, _ := v.(ResolvedLocation)
		return &cachedLocation, nil
	}

	// Otherwise grab the zones for the project and iterate through them
	it := r.zoneLister.List(ctx, &computepb.ListZonesRequest{
		Project: project,
	})

	for {
		zone, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		baseRegion := path.Base(zone.GetRegion())
		if zone.GetName() == location {
			resolvedZone := ResolvedLocation{
				Zone:     zone.Name,
				Region:   baseRegion,
				Regional: false,
			}
			r.cache.Store(locationCacheKey{
				project:  project,
				location: location,
			}, resolvedZone)
			return &resolvedZone, nil
		}
		if baseRegion == location {
			resolvedRegion := ResolvedLocation{
				Region:   baseRegion,
				Zone:     nil,
				Regional: true,
			}
			r.cache.Store(locationCacheKey{
				project:  project,
				location: location,
			}, resolvedRegion)
			return &resolvedRegion, nil
		}
	}
	return nil, ErrInvalidLocation
}

// Close closes the underlying list client
func (r *Resolver) Close() error {
	return r.zoneLister.Close()
}
