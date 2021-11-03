/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var _ = Describe("GCPClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an GCPCluster", func() {
		It("should not error and not requeue the request with insufficient set up", func() {
			ctx := context.Background()

			reconciler := &GCPClusterReconciler{
				Client: k8sClient,
			}

			instance := &infrav1.GCPCluster{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

			// Create the GCPCluster object and expect the Reconcile and Deployment to be created
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
			defer func() {
				err := k8sClient.Delete(ctx, instance)
				Expect(err).NotTo(HaveOccurred())
			}()

			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
			Expect(result.Requeue).To(BeFalse())
		})
	})
})
