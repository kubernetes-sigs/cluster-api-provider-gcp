package scope

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// This test verifies that if a user selects "local-ssd"
// as a disk type then the MachineScope correctly detects it as such.
func TestMachineLocalSSDDiskType(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Register the GCPMachine types in a scheme.
	schema := runtime.NewScheme()
	err := infrav1.AddToScheme(schema)

	// Make sure no errors were triggered.
	g.Expect(err).NotTo(HaveOccurred())

	// Create a controller fake client.
	// It's best to use envtest but in this case we are not really using the client
	// just passing it as parameter to the NewMachineScope to test the dik functionality.
	testClient := fake.NewClientBuilder().WithScheme(schema).Build()

	// New test machine, needed as parameter.
	failureDomain := "example.com"
	testMachine := clusterv1.Machine{
		Spec: clusterv1.MachineSpec{
			FailureDomain: failureDomain,
		},
	}

	// GCPMachine for mocking the disks.
	// Set the disk to local-ssd.
	diskType := infrav1.LocalSsdDiskType
	diskSize := int64(100)

	testGCPMachine := infrav1.GCPMachine{
		Spec: infrav1.GCPMachineSpec{
			AdditionalDisks: []infrav1.AttachedDiskSpec{
				{
					DeviceType: &diskType,
					Size:       &diskSize,
				},
			},
		},
	}

	// Finally put together the parameters.
	testScopeParams := MachineScopeParams{
		Client:     testClient,
		Machine:    &testMachine,
		GCPMachine: &testGCPMachine,
	}

	// Create the scope
	testMachineScope, err := NewMachineScope(testScopeParams)

	// Make sure the machine scope is create correctly.
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testMachineScope).NotTo(BeNil())

	// Now make sure the local-ssd disk type is detected as SCRATCH.
	diskSpec := instanceAdditionalDiskSpec(ctx, testGCPMachine.Spec.AdditionalDisks, testGCPMachine.Spec.RootDiskEncryptionKey, testMachineScope.Zone(), testGCPMachine.Spec.ResourceManagerTags)
	g.Expect(diskSpec).NotTo(BeEmpty())

	// Get the local-ssd disk now.
	localSSDTest := diskSpec[0]
	g.Expect(localSSDTest.AutoDelete).To(BeTrue())
	g.Expect(localSSDTest.Type).To(Equal("SCRATCH"))
	g.Expect(localSSDTest.Interface).To(Equal("NVME"))
	g.Expect(localSSDTest.InitializeParams.DiskSizeGb).To(Equal(int64(375)))
}

// TestInstanceNetworkInterfaceAliasIPRangesSpec tests the InstanceNetworkInterfaceAliasIPRangesSpec function
func TestInstanceNetworkInterfaceAliasIPRangesSpec(t *testing.T) {
	t.Run("should return nil for empty alias IP ranges", func(t *testing.T) {
		g := NewWithT(t)
		testGCPMachine := infrav1.GCPMachine{
			Spec: infrav1.GCPMachineSpec{
				AliasIPRanges: []infrav1.AliasIPRange{},
			},
		}

		result := InstanceNetworkInterfaceAliasIPRangesSpec(testGCPMachine.Spec.AliasIPRanges)
		g.Expect(result).To(BeNil())
	})

	t.Run("should convert single alias IP range", func(t *testing.T) {
		g := NewWithT(t)
		testGCPMachine := infrav1.GCPMachine{
			Spec: infrav1.GCPMachineSpec{
				AliasIPRanges: []infrav1.AliasIPRange{
					{
						IPCidrRange:         "10.0.0.0/24",
						SubnetworkRangeName: "pods",
					},
				},
			},
		}

		result := InstanceNetworkInterfaceAliasIPRangesSpec(testGCPMachine.Spec.AliasIPRanges)
		g.Expect(result).NotTo(BeNil())
		g.Expect(result).To(HaveLen(1))
		g.Expect(result[0].IpCidrRange).To(Equal("10.0.0.0/24"))
		g.Expect(result[0].SubnetworkRangeName).To(Equal("pods"))
	})

	t.Run("should convert multiple alias IP ranges", func(t *testing.T) {
		g := NewWithT(t)
		testGCPMachine := infrav1.GCPMachine{
			Spec: infrav1.GCPMachineSpec{
				AliasIPRanges: []infrav1.AliasIPRange{
					{
						IPCidrRange:         "10.0.0.0/24",
						SubnetworkRangeName: "pods",
					},
					{
						IPCidrRange:         "10.1.0.0/24",
						SubnetworkRangeName: "services",
					},
				},
			},
		}

		result := InstanceNetworkInterfaceAliasIPRangesSpec(testGCPMachine.Spec.AliasIPRanges)
		g.Expect(result).NotTo(BeNil())
		g.Expect(result).To(HaveLen(2))
		g.Expect(result[0].IpCidrRange).To(Equal("10.0.0.0/24"))
		g.Expect(result[0].SubnetworkRangeName).To(Equal("pods"))
		g.Expect(result[1].IpCidrRange).To(Equal("10.1.0.0/24"))
		g.Expect(result[1].SubnetworkRangeName).To(Equal("services"))
	})

	t.Run("should handle alias IP range without SubnetworkRangeName", func(t *testing.T) {
		g := NewWithT(t)
		testGCPMachine := infrav1.GCPMachine{
			Spec: infrav1.GCPMachineSpec{
				AliasIPRanges: []infrav1.AliasIPRange{
					{
						IPCidrRange:         "10.100.0.0/24",
						SubnetworkRangeName: "",
					},
				},
			},
		}

		result := InstanceNetworkInterfaceAliasIPRangesSpec(testGCPMachine.Spec.AliasIPRanges)
		g.Expect(result).NotTo(BeNil())
		g.Expect(result).To(HaveLen(1))
		g.Expect(result[0].IpCidrRange).To(Equal("10.100.0.0/24"))
		g.Expect(result[0].SubnetworkRangeName).To(Equal(""))
	})
}
