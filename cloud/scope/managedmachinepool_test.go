package scope

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

var (
	TestGCPMMP      *v1beta1.GCPManagedMachinePool
	TestMP          *clusterv1exp.MachinePool
	TestClusterName string
)

var _ = Describe("GCPManagedMachinePool Scope", func() {
	BeforeEach(func() {
		TestClusterName = "test-cluster"
		gcpmmpName := "test-gcpmmp"
		nodePoolName := "test-pool"
		namespace := "capg-system"
		replicas := int32(1)

		TestGCPMMP = &v1beta1.GCPManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gcpmmpName,
				Namespace: namespace,
			},
			Spec: v1beta1.GCPManagedMachinePoolSpec{
				NodePoolName: nodePoolName,
			},
		}
		TestMP = &clusterv1exp.MachinePool{
			Spec: clusterv1exp.MachinePoolSpec{
				Replicas: &replicas,
			},
		}
	})

	Context("Test NodePoolResourceLabels", func() {
		It("should append cluster owned label", func() {
			labels := infrav1.Labels{"test-key": "test-value"}

			Expect(NodePoolResourceLabels(labels, TestClusterName)).To(Equal(infrav1.Labels{
				"test-key":                             "test-value",
				infrav1.ClusterTagKey(TestClusterName): string(infrav1.ResourceLifecycleOwned),
			}))
		})
	})

	Context("Test ConvertToSdkNodePool", func() {
		It("should convert to SDK node pool with default values", func() {
			sdkNodePool := ConvertToSdkNodePool(*TestGCPMMP, *TestMP, false, TestClusterName)

			Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
				Name:             TestGCPMMP.Spec.NodePoolName,
				InitialNodeCount: *TestMP.Spec.Replicas,
				Config: &containerpb.NodeConfig{
					ResourceLabels:         NodePoolResourceLabels(nil, TestClusterName),
					ShieldedInstanceConfig: &containerpb.ShieldedInstanceConfig{},
				},
			}))
		})

		It("should convert to SDK node pool node count in a regional cluster", func() {
			replicas := int32(6)
			TestMP.Spec.Replicas = &replicas

			sdkNodePool := ConvertToSdkNodePool(*TestGCPMMP, *TestMP, true, TestClusterName)

			Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
				Name:             TestGCPMMP.Spec.NodePoolName,
				InitialNodeCount: replicas / cloud.DefaultNumRegionsPerZone,
				Config: &containerpb.NodeConfig{
					ResourceLabels:         NodePoolResourceLabels(nil, TestClusterName),
					ShieldedInstanceConfig: &containerpb.ShieldedInstanceConfig{},
				},
			}))
		})

		It("should convert to SDK node pool using GCPManagedMachinePool", func() {
			machineType := "n1-standard-1"
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

			TestGCPMMP.Spec.MachineType = &machineType
			TestGCPMMP.Spec.DiskSizeGb = &diskSizeGb
			TestGCPMMP.Spec.ImageType = &imageType
			TestGCPMMP.Spec.LocalSsdCount = &localSsdCount
			TestGCPMMP.Spec.DiskType = &diskType
			TestGCPMMP.Spec.Scaling = &scaling
			TestGCPMMP.Spec.MaxPodsPerNode = &maxPodsPerNode
			TestGCPMMP.Spec.KubernetesLabels = labels
			TestGCPMMP.Spec.KubernetesTaints = taints
			TestGCPMMP.Spec.AdditionalLabels = resourceLabels

			sdkNodePool := ConvertToSdkNodePool(*TestGCPMMP, *TestMP, false, TestClusterName)

			Expect(sdkNodePool).To(Equal(&containerpb.NodePool{
				Name:             TestGCPMMP.Spec.NodePoolName,
				InitialNodeCount: *TestMP.Spec.Replicas,
				Config: &containerpb.NodeConfig{
					Labels:                 labels,
					Taints:                 v1beta1.ConvertToSdkTaint(taints),
					ResourceLabels:         NodePoolResourceLabels(resourceLabels, TestClusterName),
					MachineType:            machineType,
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
	})
})
