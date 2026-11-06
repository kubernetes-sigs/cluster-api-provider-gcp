package scope

import (
	"testing"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	clusterName            = "test-cluster"
	managedMachinePoolName = "test-gcpmmp"
	nodePoolName           = "test-pool"
	namespace              = "capg-system"
	replicas               = int32(1)
)

func createTestManagedMachinePool() *v1beta1.GCPManagedMachinePool {
	return &v1beta1.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedMachinePoolName,
			Namespace: namespace,
		},
		Spec: v1beta1.GCPManagedMachinePoolSpec{
			GCPManagedMachinePoolClassSpec: v1beta1.GCPManagedMachinePoolClassSpec{
				NodePoolName: nodePoolName,
			},
		},
	}
}

func createTestMachinePool() *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		Spec: clusterv1.MachinePoolSpec{
			Replicas: new(replicas),
		},
	}
}

func Test_NodePoolResourceLabels(t *testing.T) {
	t.Run("append cluster owned label", func(t *testing.T) {
		g := NewWithT(t)
		labels := infrav1.Labels{"test-key": "test-value"}

		g.Expect(NodePoolResourceLabels(labels, clusterName)).To(Equal(infrav1.Labels{
			"test-key":                         "test-value",
			infrav1.ClusterTagKey(clusterName): string(infrav1.ResourceLifecycleOwned),
		}))
	})
}

func Test_MachinePool_InfrastructureMachineKind(t *testing.T) {
	t.Run("set infrastructure machine kind when empty", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testManagedMachinePool.Status = v1beta1.GCPManagedMachinePoolStatus{}
		machinePoolScope := ManagedMachinePoolScope{
			GCPManagedMachinePool: testManagedMachinePool,
		}

		update := machinePoolScope.SetInfrastructureMachineKind()
		g.Expect(machinePoolScope.GCPManagedMachinePool.Status.InfrastructureMachineKind).To(Equal(v1beta1.GCPManagedMachinePoolMachineKind))
		g.Expect(update).To(BeTrue())
	})

	t.Run("do not update infrastructure machine kind if already set", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testManagedMachinePool.Status = v1beta1.GCPManagedMachinePoolStatus{
			InfrastructureMachineKind: v1beta1.GCPManagedMachinePoolMachineKind,
		}
		machinePoolScope := ManagedMachinePoolScope{
			GCPManagedMachinePool: testManagedMachinePool,
		}

		update := machinePoolScope.SetInfrastructureMachineKind()
		g.Expect(machinePoolScope.GCPManagedMachinePool.Status.InfrastructureMachineKind).To(Equal(v1beta1.GCPManagedMachinePoolMachineKind))
		g.Expect(update).To(BeFalse())
	})
}

func Test_MachinePool_ConvertToSdkNodePool(t *testing.T) {
	t.Run("convert to SDK node pool with default values", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
			Name:             testManagedMachinePool.Spec.NodePoolName,
			InitialNodeCount: *testMachinePool.Spec.Replicas,
			Config: &containerpb.NodeConfig{
				ResourceLabels:         NodePoolResourceLabels(nil, clusterName),
				ShieldedInstanceConfig: &containerpb.ShieldedInstanceConfig{},
			},
		}))
	})

	t.Run("convert to SDK node pool node count in a regional cluster", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		newReplicas := 6
		testMachinePool.Spec.Replicas = new(int32(newReplicas))

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, true, clusterName)

		g.Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
			Name:             testManagedMachinePool.Spec.NodePoolName,
			InitialNodeCount: int32(newReplicas / cloud.DefaultNumRegionsPerZone),
			Config: &containerpb.NodeConfig{
				ResourceLabels:         NodePoolResourceLabels(nil, clusterName),
				ShieldedInstanceConfig: &containerpb.ShieldedInstanceConfig{},
			},
		}))
	})

	t.Run("convert to SDK node pool using GCPManagedMachinePool", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		instanceType := "n1-standard-1"
		diskSizeGb := int32(128)
		imageType := "ubuntu_containerd"
		localSsdCount := int32(2)
		diskType := v1beta1.SSD
		maxPodsPerNode := int64(20)
		enableAutoscaling := false
		scaling := v1beta1.NodePoolAutoScaling{
			EnableAutoscaling: &enableAutoscaling,
		}
		labels := infrav1.Labels{"test-key": "test-value"}
		taints := v1beta1.Taints{
			{
				Key:    "test-key",
				Value:  "test-value",
				Effect: "NoSchedule",
			},
		}
		resourceLabels := infrav1.Labels{"test-key": "test-value"}

		testManagedMachinePool.Spec.InstanceType = &instanceType
		testManagedMachinePool.Spec.DiskSizeGb = &diskSizeGb
		testManagedMachinePool.Spec.ImageType = &imageType
		testManagedMachinePool.Spec.LocalSsdCount = &localSsdCount
		testManagedMachinePool.Spec.DiskType = &diskType
		testManagedMachinePool.Spec.Scaling = &scaling
		testManagedMachinePool.Spec.MaxPodsPerNode = &maxPodsPerNode
		testManagedMachinePool.Spec.KubernetesLabels = labels
		testManagedMachinePool.Spec.KubernetesTaints = taints
		testManagedMachinePool.Spec.AdditionalLabels = resourceLabels

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
			Name:             testManagedMachinePool.Spec.NodePoolName,
			InitialNodeCount: *testMachinePool.Spec.Replicas,
			Config: &containerpb.NodeConfig{
				Labels:                 labels,
				Taints:                 v1beta1.ConvertToSdkTaint(taints),
				ResourceLabels:         NodePoolResourceLabels(resourceLabels, clusterName),
				MachineType:            instanceType,
				DiskSizeGb:             diskSizeGb,
				ImageType:              imageType,
				LocalSsdCount:          localSsdCount,
				DiskType:               string(diskType),
				ShieldedInstanceConfig: &containerpb.ShieldedInstanceConfig{},
			},
			Autoscaling: v1beta1.ConvertToSdkAutoscaling(&scaling),
			MaxPodsConstraint: &containerpb.MaxPodsConstraint{
				MaxPodsPerNode: maxPodsPerNode,
			},
		}))
	})

	t.Run("let DiskSizeGb take precedence over DiskSizeGB when both are set", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		diskSizeGb := int32(64)
		diskSizeGB := int64(128)
		testManagedMachinePool.Spec.DiskSizeGb = &diskSizeGb
		testManagedMachinePool.Spec.DiskSizeGB = &diskSizeGB //nolint:staticcheck // SA1019: exercising deprecated field for backward-compatibility test

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetConfig().GetDiskSizeGb()).To(Equal(diskSizeGb))
	})

	t.Run("fall back to DiskSizeGB when DiskSizeGb is unset", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		diskSizeGB := int64(128)
		testManagedMachinePool.Spec.DiskSizeGB = &diskSizeGB //nolint:staticcheck // SA1019: exercising deprecated field for backward-compatibility test

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetConfig().GetDiskSizeGb()).To(Equal(int32(diskSizeGB)))
	})

	t.Run("prefer InstanceType over the deprecated MachineType when both are set", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		machineType := "n1-standard-1"
		instanceType := "n1-standard-2"
		testManagedMachinePool.Spec.MachineType = &machineType //nolint:staticcheck // SA1019: deprecated field set intentionally to test precedence
		testManagedMachinePool.Spec.InstanceType = &instanceType

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetConfig().GetMachineType()).To(Equal(instanceType))
	})

	t.Run("use the deprecated MachineType when InstanceType is unset", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		machineType := "n1-standard-2"
		testManagedMachinePool.Spec.MachineType = &machineType //nolint:staticcheck // SA1019: deprecated field set intentionally to test backward compatibility

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetConfig().GetMachineType()).To(Equal(machineType))
	})

	t.Run("leave UpgradeSettings unset on the SDK node pool when not specified", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings()).To(BeNil())
	})

	t.Run("UpgradeSettings should take default values if nothing is set", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		strategy := containerpb.NodePoolUpdateStrategy_SURGE
		g.Expect(sdkNodePool.GetUpgradeSettings()).To(Equal(&containerpb.NodePool_UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: 0,
			Strategy:       &strategy,
		}))
	})

	t.Run("UpgradeSettings should respect maxSurge if it is set and default everything else", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		maxSurge := int32(100)
		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			MaxSurge: &maxSurge,
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		strategy := containerpb.NodePoolUpdateStrategy_SURGE
		g.Expect(sdkNodePool.GetUpgradeSettings()).To(Equal(&containerpb.NodePool_UpgradeSettings{
			MaxSurge:       maxSurge,
			MaxUnavailable: 0,
			Strategy:       &strategy,
		}))
	})

	t.Run("UpgradeSettings should respect maxUnavailable if it is set and default everything else", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		maxUnavailable := int32(100)
		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			MaxUnavailable: &maxUnavailable,
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		strategy := containerpb.NodePoolUpdateStrategy_SURGE
		g.Expect(sdkNodePool.GetUpgradeSettings()).To(Equal(&containerpb.NodePool_UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: maxUnavailable,
			Strategy:       &strategy,
		}))
	})

	t.Run("UpgradeSettings should accept SURGE explicitly", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("SURGE"),
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		strategy := containerpb.NodePoolUpdateStrategy_SURGE
		g.Expect(sdkNodePool.GetUpgradeSettings()).To(Equal(&containerpb.NodePool_UpgradeSettings{
			MaxSurge:       1,
			MaxUnavailable: 0,
			Strategy:       &strategy,
		}))
	})

	t.Run("Unset strategy defaults to SURGE", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		maxSurge := int32(10)
		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			MaxSurge: &maxSurge,
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		strategy := containerpb.NodePoolUpdateStrategy_SURGE
		g.Expect(sdkNodePool.GetUpgradeSettings()).To(Equal(&containerpb.NodePool_UpgradeSettings{
			MaxSurge:       maxSurge,
			MaxUnavailable: 0,
			Strategy:       &strategy,
		}))
	})

	t.Run("SURGE strategies should not have BlueGreenSettings", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("SURGE"),
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings()).To(BeNil())
	})

	t.Run("BLUE_GREEN converts percentages correctly", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		percentage := int32(50)

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &percentage,
				},
			},
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetStandardRolloutPolicy().GetBatchPercentage()).To(Equal(float32(0.5)))
	})

	t.Run("BLUE_GREEN retains node count", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		nodeCount := int32(50)

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchNodeCount: &nodeCount,
				},
			},
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetStandardRolloutPolicy().GetBatchNodeCount()).To(Equal(int32(50)))
	})

	t.Run("BLUE_GREEN adheres to soak defaults", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		nodeCount := int32(50)

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchNodeCount: &nodeCount,
				},
			},
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetNodePoolSoakDuration()).To(Equal(durationpb.New(1 * time.Hour)))
		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetStandardRolloutPolicy().GetBatchSoakDuration()).To(Equal(durationpb.New(0)))
	})

	t.Run("BLUE_GREEN respects explicit durations", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		nodeCount := int32(50)

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchNodeCount:    &nodeCount,
					BatchSoakDuration: &metav1.Duration{Duration: 2 * time.Hour},
				},
				NodePoolSoakDuration: &metav1.Duration{Duration: 4 * time.Hour},
			},
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetNodePoolSoakDuration()).To(Equal(durationpb.New(4 * time.Hour)))
		g.Expect(sdkNodePool.GetUpgradeSettings().GetBlueGreenSettings().GetStandardRolloutPolicy().GetBatchSoakDuration()).To(Equal(durationpb.New(2 * time.Hour)))
	})

	t.Run("BLUE_GREEN doesn't send maxSurge or maxUnavailable", func(t *testing.T) {
		g := NewWithT(t)
		testManagedMachinePool := createTestManagedMachinePool()
		testMachinePool := createTestMachinePool()
		nodeCount := int32(50)

		testManagedMachinePool.Spec.UpgradeSettings = &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchNodeCount: &nodeCount,
				},
			},
		}
		sdkNodePool := ConvertToSdkNodePool(*testManagedMachinePool, *testMachinePool, false, clusterName)

		g.Expect(sdkNodePool.GetUpgradeSettings().GetMaxSurge()).To(Equal(int32(0)))
		g.Expect(sdkNodePool.GetUpgradeSettings().GetMaxUnavailable()).To(Equal(int32(0)))
	})
}
