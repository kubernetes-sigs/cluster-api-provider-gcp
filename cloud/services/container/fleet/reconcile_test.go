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

package fleet

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func newTestService(controlPlane *infrav1exp.GCPManagedControlPlane) *Service {
	s := new(scope.ManagedControlPlaneScope)
	s.GCPManagedControlPlane = controlPlane
	return &Service{scope: s}
}

func testControlPlane() *infrav1exp.GCPManagedControlPlane {
	return &infrav1exp.GCPManagedControlPlane{
		Spec: infrav1exp.GCPManagedControlPlaneSpec{
			GCPManagedControlPlaneClassSpec: infrav1exp.GCPManagedControlPlaneClassSpec{
				Project:     "cluster-project",
				Location:    "us-central1",
				ClusterName: "test-cluster",
			},
		},
	}
}

func TestMembershipName(t *testing.T) {
	g := NewWithT(t)
	s := newTestService(testControlPlane())

	g.Expect(s.membershipName("fleet-project")).To(Equal("projects/fleet-project/locations/us-central1/memberships/test-cluster"))
}

func TestBuildCreateMembershipRequest(t *testing.T) {
	g := NewWithT(t)
	s := newTestService(testControlPlane())

	req := s.buildCreateMembershipRequest("fleet-project")

	g.Expect(req.GetParent()).To(Equal("projects/fleet-project/locations/us-central1"))
	g.Expect(req.GetMembershipId()).To(Equal("test-cluster"))

	gkeCluster := req.GetResource().GetEndpoint().GetGkeCluster()
	g.Expect(gkeCluster).NotTo(BeNil())
	g.Expect(gkeCluster.GetResourceLink()).To(Equal("//container.googleapis.com/projects/cluster-project/locations/us-central1/clusters/test-cluster"))
}

func TestBuildCreateMembershipRequest_CrossProject(t *testing.T) {
	g := NewWithT(t)
	s := newTestService(testControlPlane())

	req := s.buildCreateMembershipRequest("fleet-host-project")

	// The Membership itself is created in the fleet-host project...
	g.Expect(req.GetParent()).To(Equal("projects/fleet-host-project/locations/us-central1"))
	// ...but the resource link always points back at the cluster's own project.
	gkeCluster := req.GetResource().GetEndpoint().GetGkeCluster()
	g.Expect(gkeCluster).NotTo(BeNil())
	g.Expect(gkeCluster.GetResourceLink()).To(Equal("//container.googleapis.com/projects/cluster-project/locations/us-central1/clusters/test-cluster"))
}
