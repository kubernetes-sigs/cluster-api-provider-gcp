package scope

import (
	"testing"

	"github.com/stretchr/testify/assert"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// This test verifies that if a user selects "local-ssd"
// as a disk type then the MachineScope correctly detects it as such.
func TestMachineLocalSSDDiskType(t *testing.T) {
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
			FailureDomain: &failureDomain,
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
	diskSpec := testMachineScope.InstanceAdditionalDiskSpec()
	assert.NotEmpty(t, diskSpec)

	// Get the local-ssd disk now.
	localSSDTest := diskSpec[0]
	assert.True(t, localSSDTest.AutoDelete)
	assert.Equal(t, "SCRATCH", localSSDTest.Type)
	assert.Equal(t, "NVME", localSSDTest.Interface)
	assert.Equal(t, int64(375), localSSDTest.InitializeParams.DiskSizeGb)
}
