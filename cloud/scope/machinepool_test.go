package scope_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/processors"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GCPManagedMachinePool Scope", func() {
	var TestMachinePoolScope *scope.MachinePoolScope
	var getter cloud.ClusterGetter
	var mpscopeparams scope.MachinePoolScopeParams
	var t *testing.T

	BeforeEach(func() {
		// Register the MachinePool, GCPMachinePool and GCPMachinePoolList in a schema.
		schema, err := infrav1.SchemeBuilder.Register(&clusterv1exp.MachinePool{}, &infrav1exp.GCPMachinePool{}, &infrav1exp.GCPMachinePoolList{}).Build()
		// Make sure no errors were triggered.
		assert.Nil(t, err)

		// Create a controller fake client.
		// It's best to use envtest but in this case we are not really using the client
		// just passing it as parameter to the NewMachinePoolScope to test the mincpu.
		testClient := fake.NewClientBuilder().WithScheme(schema).Build()

		// Create the machinepool scope
		mpscopeparams = scope.MachinePoolScopeParams{
			Client:        testClient,
			ClusterGetter: getter,
			MachinePool: &clusterv1exp.MachinePool{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       clusterv1exp.MachinePoolSpec{},
			},
			GCPMachinePool: &infrav1exp.GCPMachinePool{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       infrav1exp.GCPMachinePoolSpec{},
			},
		}
		TestMachinePoolScope, _ = scope.NewMachinePoolScope(mpscopeparams)

		// Make sure the machinepool scope is created correctly.
		assert.Nil(t, err)
		assert.NotNil(t, TestMachinePoolScope)

	})

	Describe("Min CPU Mappings", func() {
		Context("instance types", func() {
			It("should match all", func() {
				for k := range processors.Processors {
					TestMachinePoolScope.GCPMachinePool.Spec.InstanceType = fmt.Sprintf("%sstandard-8", k)
					Expect(TestMachinePoolScope.MinCPUPlatform()).To(Equal(processors.Processors[k]))
				}
			})
			It("should not match n2", func() {
				TestMachinePoolScope.GCPMachinePool.Spec.InstanceType = "n2d-standard-8"
				Expect(TestMachinePoolScope.MinCPUPlatform()).NotTo(Equal(processors.Processors["n2"]))
			})
		})
	})

	Describe("GCPMachinePool Spec has no ShieldedInstanceConfig passed", func() {
		It("should have Integrity Monitoring set to true", func() {
			shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
			Expect(shieldedVMConfig.EnableIntegrityMonitoring).To(BeTrue())
		})
		It("should have Secure Boot set to false", func() {
			shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
			Expect(shieldedVMConfig.EnableSecureBoot).To(BeFalse())
		})
		It("should have vTPM set to true", func() {
			shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
			Expect(shieldedVMConfig.EnableVtpm).To(BeTrue())
		})
	})

	Describe("GCPMachinePool Spec has ShieldedInstanceConfig passed", func() {
		Context("Secure Boot is enabled in gcpmacninepool.spec", func() {
			It("should have secure boot set to true", func() {
				mpscopeparams.GCPMachinePool.Spec = infrav1exp.GCPMachinePoolSpec{
					ShieldedInstanceConfig: &infrav1exp.GCPShieldedInstanceConfig{
						SecureBoot: infrav1exp.SecureBootPolicyEnabled,
					},
				}
				shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
				Expect(shieldedVMConfig.EnableIntegrityMonitoring).To(BeTrue())
			})
		})

		Context("vTPM is disabled in gcpmacninepool.spec", func() {
			It("should have secure boot set to false", func() {
				mpscopeparams.GCPMachinePool.Spec = infrav1exp.GCPMachinePoolSpec{
					ShieldedInstanceConfig: &infrav1exp.GCPShieldedInstanceConfig{
						VirtualizedTrustedPlatformModule: infrav1exp.VirtualizedTrustedPlatformModulePolicyDisabled,
					},
				}
				shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
				Expect(shieldedVMConfig.EnableVtpm).To(BeFalse())
			})
		})

		Context("Integrity Monitoring is disabled in gcpmacninepool.spec", func() {
			It("should have Integrity Monitoring set to false", func() {
				mpscopeparams.GCPMachinePool.Spec = infrav1exp.GCPMachinePoolSpec{
					ShieldedInstanceConfig: &infrav1exp.GCPShieldedInstanceConfig{
						IntegrityMonitoring: infrav1exp.IntegrityMonitoringPolicyDisabled,
					},
				}
				shieldedVMConfig := TestMachinePoolScope.GetShieldedInstanceConfigSpec()
				Expect(shieldedVMConfig.EnableIntegrityMonitoring).To(BeFalse())
			})
		})
	})
})
