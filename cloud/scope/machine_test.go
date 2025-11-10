package scope

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// This test verifies that if a user selects "local-ssd"
// as a disk type then the MachineScope correctly detects it as such.
func TestMachineLocalSSDDiskType(t *testing.T) {
	ctx := context.Background()

	// Register the GCPMachine and GCPMachineList in a schema.
	schema, err := infrav1.SchemeBuilder.Register(&infrav1.GCPMachine{}, &infrav1.GCPMachineList{}).Build()

	// Make sure no errors were triggered.
	assert.Nil(t, err)

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
	assert.Nil(t, err)
	assert.NotNil(t, testMachineScope)

	// Now make sure the local-ssd disk type is detected as SCRATCH.
	diskSpec := instanceAdditionalDiskSpec(ctx, testGCPMachine.Spec.AdditionalDisks, testGCPMachine.Spec.RootDiskEncryptionKey, testMachineScope.Zone(), testGCPMachine.Spec.ResourceManagerTags)
	assert.NotEmpty(t, diskSpec)

	// Get the local-ssd disk now.
	localSSDTest := diskSpec[0]
	assert.True(t, localSSDTest.AutoDelete)
	assert.Equal(t, "SCRATCH", localSSDTest.Type)
	assert.Equal(t, "NVME", localSSDTest.Interface)
	assert.Equal(t, int64(375), localSSDTest.InitializeParams.DiskSizeGb)
}

// TestInstanceNetworkInterfaceAliasIPRangesSpec tests the InstanceNetworkInterfaceAliasIPRangesSpec function
func TestInstanceNetworkInterfaceAliasIPRangesSpec(t *testing.T) {
	// Register the GCPMachine and GCPMachineList in a schema.
	schema, err := infrav1.SchemeBuilder.Register(&infrav1.GCPMachine{}, &infrav1.GCPMachineList{}).Build()
	assert.Nil(t, err)

	// Create a controller fake client.
	testClient := fake.NewClientBuilder().WithScheme(schema).Build()

	// Test machine parameter
	failureDomain := "us-central1-a"
	testMachine := clusterv1.Machine{
		Spec: clusterv1.MachineSpec{
			FailureDomain: failureDomain,
		},
	}

	t.Run("should return nil for empty alias IP ranges", func(t *testing.T) {
		testGCPMachine := infrav1.GCPMachine{
			Spec: infrav1.GCPMachineSpec{
				AliasIPRanges: []infrav1.AliasIPRange{},
			},
		}

		testScopeParams := MachineScopeParams{
			Client:     testClient,
			Machine:    &testMachine,
			GCPMachine: &testGCPMachine,
		}

		testMachineScope, err := NewMachineScope(testScopeParams)
		assert.Nil(t, err)
		assert.NotNil(t, testMachineScope)

		result := testMachineScope.InstanceNetworkInterfaceAliasIPRangesSpec()
		assert.Nil(t, result)
	})

	t.Run("should convert single alias IP range", func(t *testing.T) {
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

		testScopeParams := MachineScopeParams{
			Client:     testClient,
			Machine:    &testMachine,
			GCPMachine: &testGCPMachine,
		}

		testMachineScope, err := NewMachineScope(testScopeParams)
		assert.Nil(t, err)
		assert.NotNil(t, testMachineScope)

		result := testMachineScope.InstanceNetworkInterfaceAliasIPRangesSpec()
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, "10.0.0.0/24", result[0].IpCidrRange)
		assert.Equal(t, "pods", result[0].SubnetworkRangeName)
	})

	t.Run("should convert multiple alias IP ranges", func(t *testing.T) {
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

		testScopeParams := MachineScopeParams{
			Client:     testClient,
			Machine:    &testMachine,
			GCPMachine: &testGCPMachine,
		}

		testMachineScope, err := NewMachineScope(testScopeParams)
		assert.Nil(t, err)
		assert.NotNil(t, testMachineScope)

		result := testMachineScope.InstanceNetworkInterfaceAliasIPRangesSpec()
		assert.NotNil(t, result)
		assert.Len(t, result, 2)
		assert.Equal(t, "10.0.0.0/24", result[0].IpCidrRange)
		assert.Equal(t, "pods", result[0].SubnetworkRangeName)
		assert.Equal(t, "10.1.0.0/24", result[1].IpCidrRange)
		assert.Equal(t, "services", result[1].SubnetworkRangeName)
	})

	t.Run("should handle alias IP range without SubnetworkRangeName", func(t *testing.T) {
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

		testScopeParams := MachineScopeParams{
			Client:     testClient,
			Machine:    &testMachine,
			GCPMachine: &testGCPMachine,
		}

		testMachineScope, err := NewMachineScope(testScopeParams)
		assert.Nil(t, err)
		assert.NotNil(t, testMachineScope)

		result := testMachineScope.InstanceNetworkInterfaceAliasIPRangesSpec()
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, "10.100.0.0/24", result[0].IpCidrRange)
		assert.Equal(t, "", result[0].SubnetworkRangeName)
	})
}
