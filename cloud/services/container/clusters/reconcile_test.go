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

package clusters

import (
	"context"

	"cloud.google.com/go/container/apiv1/containerpb"

	"go.uber.org/mock/gomock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	cloudScope "sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	mockContainer "sigs.k8s.io/cluster-api-provider-gcp/cloud/scope/mocks"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
	_ = infrav1.AddToScheme(scheme.Scheme)
}

var (
	fakeCluster                *clusterv1.Cluster
	fakeGCPManagedCluster      *infrav1exp.GCPManagedCluster
	fakeGCPManagedControlPlane *infrav1exp.GCPManagedControlPlane
	ctx                        context.Context
)

var _ = Describe("ManagedControlPlaneScope Reconcile", func() {
	BeforeEach(func() {
		fakeCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster",
				Namespace: "default",
			},
			Spec: clusterv1.ClusterSpec{},
		}

		fakeGCPManagedCluster = &infrav1exp.GCPManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster",
				Namespace: "default",
			},
			Spec: infrav1exp.GCPManagedClusterSpec{
				CredentialsRef: &infrav1.ObjectReference{
					Namespace: "default",
					Name:      "my-cluster",
				},
			},
		}

		fakeGCPManagedControlPlane = &infrav1exp.GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster",
				Namespace: "default",
			},
			Spec: infrav1exp.GCPManagedControlPlaneSpec{
				Location:    "us-west1-a",
				Project:     "foo-project",
				ClusterName: "my-cluster",
			},
		}
	})

	Context("When GCPManagedCluster passed doesn't have a name present in the network spec", func() {
		It("should log an error message and return from the reconcile", func() {
			fakec := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				Build()

			mockCtrl := gomock.NewController(GinkgoT())
			defer mockCtrl.Finish()

			mockInternalClusterManagerClient := mockContainer.NewMockContainer(mockCtrl)
			getClusterRequest := &containerpb.GetClusterRequest{
				Name: "projects/foo-project/locations/us-west1/clusters/my-cluster",
			}
			mockInternalClusterManagerClient.EXPECT().GetCluster(ctx, getClusterRequest).Return(nil, nil).Times(1)

			testManagedControlPlaneScope, err := cloudScope.NewMockManagedControlPlaneScope(ctx, cloudScope.ManagedControlPlaneScopeParams{
				Client:                 fakec,
				Cluster:                fakeCluster,
				GCPManagedCluster:      fakeGCPManagedCluster,
				GCPManagedControlPlane: fakeGCPManagedControlPlane,
				ManagedClusterClient:   mockInternalClusterManagerClient,
			})

			reconcile := New(testManagedControlPlaneScope)
			result, err := reconcile.Reconcile(ctx)

			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).NotTo(BeNil())
			Expect(err).To(Equal(ErrGCPManagedClusterHasNoNetworkDefined))
		})
	})
})
