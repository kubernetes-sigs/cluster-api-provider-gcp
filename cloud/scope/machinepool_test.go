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
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/processors"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GCPManagedMachinePool Scope", func() {
	var TestMachinePoolScope *scope.MachinePoolScope
	var getter cloud.ClusterGetter
	var t *testing.T

	BeforeEach(func() {
		// Register the MachinePool, GCPMachinePool and GCPMachinePoolList in a schema.
		schema, err := infrav1.SchemeBuilder.Register(&clusterv1exp.MachinePool{}, &v1beta1.GCPMachinePool{}, &v1beta1.GCPMachinePoolList{}).Build()
		// Make sure no errors were triggered.
		assert.Nil(t, err)

		// Create a controller fake client.
		// It's best to use envtest but in this case we are not really using the client
		// just passing it as parameter to the NewMachinePoolScope to test the mincpu.
		testClient := fake.NewClientBuilder().WithScheme(schema).Build()

		// Create the machinepool scope
		params := scope.MachinePoolScopeParams{
			Client:        testClient,
			ClusterGetter: getter,
			MachinePool: &clusterv1exp.MachinePool{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       clusterv1exp.MachinePoolSpec{},
			},
			GCPMachinePool: &v1beta1.GCPMachinePool{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1beta1.GCPMachinePoolSpec{},
			},
		}
		TestMachinePoolScope, _ = scope.NewMachinePoolScope(params)

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
})
