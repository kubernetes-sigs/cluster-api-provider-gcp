//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"io"
	"net/http"
	"os"
	"strings"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
)              

type VerifyMachineGPUAttachedSpecInput struct {
	GCPProjectID string
	GCPCredentials string
	BootstrapClusterProxy *framework.ClusterProxy
	Machine *clusterv1.Machine
	GPUType string
	GPUCount int
}

// resolveCIVersion resolves kubernetes version labels (e.g. latest, latest-1.xx) to the corresponding CI version numbers.
// Go implementation of https://github.com/kubernetes-sigs/cluster-api/blob/d1dc87d5df3ab12a15ae5b63e50541a191b7fec4/scripts/ci-e2e-lib.sh#L75-L95.
func resolveCIVersion(label string) (string, error) {
	if ciVersion, ok := os.LookupEnv("CI_VERSION"); ok {
		return ciVersion, nil
	}

	if strings.HasPrefix(label, "latest") {
		if kubernetesVersion, err := latestCIVersion(label); err == nil {
			return kubernetesVersion, nil
		}
	}
	// default to https://dl.k8s.io/ci/latest.txt if the label can't be resolved
	return kubernetesversions.LatestCIRelease()
}

// latestCIVersion returns the latest CI version of a given label in the form of latest-1.xx.
func latestCIVersion(label string) (string, error) {
	ciVersionURL := fmt.Sprintf("https://dl.k8s.io/ci/%s.txt", label)
	resp, err := http.Get(ciVersionURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

// VerifyMachineGPUAttachedSpec verifies that the machine has the expected GPU type and count.
func VerifyMachineGPUAttachedSpec(ctx context.Context, input VerifyMachineGPUAttachedSpecInput) {
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil")
	Expect(input.GCPCredentials).ToNot(BeEmpty(), "Invalid argument. input.GCPCredentials can't be empty")
	Expect(input.Machine).ToNot(BeNil(), "Invalid argument. input.Machine can't be nil")
	Expect(input.GPUType).ToNot(BeEmpty(), "Invalid argument. input.GPUType can't be empty")
	Expect(input.GCPProjectID).ToNot(BeEmpty(), "Invalid argument. input.GPUType can't be empty")
	Expect(input.GPUCount).ToNot(BeZero(), "Invalid argument. input.GPUCount can't be zero")

	Byf("Getting the GCPMachine for Machine %s/%s", input.Machine.Namespace, input.Machine.Name)
	key := types.NamespacedName{
		Namespace: input.Machine.Spec.InfrastructureRef.Namespace,
		Name:      input.Machine.Spec.InfrastructureRef.Name,
	}

	client := input.BootstrapClusterProxy.GetClient()
	gcpMachine := &infrav1.GCPMachine{}
	err := client.Get(ctx, key, gcpMachine)
	Expect(err).NotTo(HaveOccurred())

	Byf("Getting instance ID from providerID")
	parsed, err := noderefutil.NewProviderID(*input.Machine.Spec.ProviderID)
	Expect(err).NotTo(HaveOccurred())
	instanceID := pointer.StringPtr(parsed.ID())

	Byf("Getting the %s instance from GCP", *instanceID)
	computeSvc, err := compute.NewService(ctx, option.WithCredentialsFile(input.GCPCredentials))
	Expect(err).NotTo(HaveOccurred())
	getReq := computeSvc.Instances.Get(input.GCPProjectID, *input.Machine.Spec.FailureDomain, *instanceID)
	instance, err := getReq.Do()

	Expect(err).NotTo(HaveOccurred())
	Expect(instance.GuestAccelerators).ToNot(BeEmpty(), "No GuestAccelerators found in instance %s", instance.Name)
	for _, acceleratorConfig := range instance.GuestAccelerators {
		Expect(acceleratorConfig.AcceleratorType).To(Equal(input.GPUType))
		Expect(acceleratorConfig.AcceleratorCount).To(Equal(int64(1)), "AcceleratorCount is not 1")
	}
}