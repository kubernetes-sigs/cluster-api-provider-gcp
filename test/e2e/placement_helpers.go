//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func createSpreadPolicy(ctx context.Context, project, region, policyName string, skipCleanup bool) string {
	GinkgoHelper()
	policyPath := fmt.Sprintf("projects/%s/regions/%s/resourcePolicies/%s", project, region, policyName)
	policies, err := compute.NewResourcePoliciesRESTClient(ctx)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(policies.Close)
	op, err := policies.Insert(ctx, &computepb.InsertResourcePolicyRequest{
		Project: project,
		Region:  region,
		ResourcePolicyResource: &computepb.ResourcePolicy{
			Name: ptr.To(policyName),
			GroupPlacementPolicy: &computepb.ResourcePolicyGroupPlacementPolicy{
				Collocation:             ptr.To("SPREAD"),
				AvailabilityDomainCount: ptr.To[int32](8),
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		if skipCleanup {
			return
		}
		By("Deleting the workers' placement policy")
		op, err := policies.Delete(ctx, &computepb.DeleteResourcePolicyRequest{
			Project: project, Region: region, ResourcePolicy: policyName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(op.Wait(ctx)).To(Succeed())
	})
	Expect(op.Wait(ctx)).To(Succeed())
	return policyPath
}

func verifySpreadPolicy(ctx context.Context, machines []clusterv1.Machine, policyPath string) {
	GinkgoHelper()
	instances, err := compute.NewInstancesRESTClient(ctx)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(instances.Close)
	for _, machine := range machines {
		project, zone, name, err := parseProviderID(machine.Spec.ProviderID)
		Expect(err).NotTo(HaveOccurred())
		instance, err := instances.Get(ctx, &computepb.GetInstanceRequest{
			Project: project, Zone: zone, Instance: name,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(instance.GetResourcePolicies()).To(ContainElement("https://www.googleapis.com/compute/v1/" + policyPath))
		scheduling := instance.GetResourceStatus().GetScheduling()
		Expect(scheduling).NotTo(BeNil())
		Expect(scheduling.AvailabilityDomain).NotTo(BeNil(), "VM %s should report an availability domain", name)
	}
}
