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
	"bytes"
	"context"
	"flag"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	gcompute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/services/compute/mocks"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("GCPMachineReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an GCPMachine", func() {
		It("should not error with minimal set up", func() {
			reconciler := &GCPMachineReconciler{
				Client:     k8sClient,
				Log:        log.Log,
				computeSvc: nil,
			}
			By("Calling reconcile")
			instance := &infrav1.GCPMachine{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
			result, err := reconciler.Reconcile(ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})

	Context("reconcile non-deleted GCPMachines", func() {
		var (
			reconciler   *GCPMachineReconciler
			ctx          context.Context
			clusterScope *scope.ClusterScope
			machineScope *scope.MachineScope
			mockCtrl     *gomock.Controller
			gcpService   *mocks.MockServiceInterface
		)
		BeforeEach(func() {
			flag.Set("logtostderr", "false")
			flag.Set("v", "2")
			klog.SetOutput(GinkgoWriter)
			var err error
			bootstrapSecretName := "bootstrapsecret"

			mockCtrl = gomock.NewController(GinkgoT())
			gcpService = mocks.NewMockServiceInterface(mockCtrl)

			reconciler = &GCPMachineReconciler{
				Client:     k8sClient,
				Log:        log.Log,
				computeSvc: gcpService,
			}

			gcpMachine := &infrav1.GCPMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: infrav1.GCPMachineSpec{
					Zone:         "us-west4-a",
					InstanceType: "n1-standard-1",
				},
			}

			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fooMachine",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Kind:       "GCPMachine",
						Name:       gcpMachine.Name,
					},
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: &bootstrapSecretName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, gcpMachine)).Should(Succeed())

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "default",
				},
				Status: clusterv1.ClusterStatus{
					InfrastructureReady: true,
				},
			}

			gcpCluster := &infrav1.GCPCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gcpcluster",
					Namespace: "default",
				},
				Spec: infrav1.GCPClusterSpec{
					Project: "gcp-project",
				},
			}

			// Create the cluster scope
			clusterScope, err = scope.NewClusterScope(scope.ClusterScopeParams{
				Client:     k8sClient,
				Logger:     log.Log,
				Cluster:    cluster,
				GCPCluster: gcpCluster,
			})
			Expect(err).To(BeNil())

			// Create the machine scope
			machineScope, err = scope.NewMachineScope(scope.MachineScopeParams{
				Logger:     log.Log,
				Client:     k8sClient,
				Cluster:    cluster,
				Machine:    machine,
				GCPCluster: gcpCluster,
				GCPMachine: gcpMachine,
			})
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, machineScope.GCPMachine)).Should(Succeed())
		})

		It("should return early if gcp machine is in error state", func() {
			machineScope.SetFailureReason(capierrors.UpdateMachineError)
			machineScope.SetFailureMessage(errors.New("GCE error"))

			buf := new(bytes.Buffer)
			klog.SetOutput(buf)

			_, err := reconciler.reconcile(ctx, machineScope, clusterScope)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("Error state detected, skipping reconciliation"))
		})

		It("should return if infrastructure is not ready", func() {
			clusterStatus := clusterv1.ClusterStatus{InfrastructureReady: false}
			machineScope.Cluster.Status = clusterStatus

			buf := new(bytes.Buffer)
			klog.SetOutput(buf)

			_, err := reconciler.reconcile(ctx, machineScope, clusterScope)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("Cluster infrastructure is not ready yet"))
		})

		It("should return if bootstrap data is not available", func() {
			spec := clusterv1.MachineSpec{
				Bootstrap: clusterv1.Bootstrap{
					Data: nil,
				}}
			machineScope.Machine.Spec = spec

			buf := new(bytes.Buffer)
			klog.SetOutput(buf)

			_, err := reconciler.reconcile(ctx, machineScope, clusterScope)
			Expect(err).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("Bootstrap data secret reference is not yet available"))
		})

		It("should try to create a new machine if none exists", func() {
			instance := &gcompute.Instance{
				Name:        "gce-instance",
				Status:      "RUNNING",
				MachineType: "n1-standard-1",
			}
			gcpService.EXPECT().InstanceIfExists(gomock.Any()).Return(nil, nil)
			gcpService.EXPECT().CreateInstance(gomock.Any()).Return(instance, nil)

			_, err := reconciler.reconcile(ctx, machineScope, clusterScope)
			Expect(err).To(BeNil())

			By("Checking providerId")
			Expect(machineScope.GetProviderID()).To(Equal("gce://gcp-project/us-west4-a/gce-instance"))
			By("Checking instanceStatus")
			Expect(*machineScope.GetInstanceStatus()).To(Equal(infrav1.InstanceStatus("RUNNING")))
			By("Checking a finalizer is added")
			Expect(machineScope.GCPMachine.Finalizers).To(ContainElement(infrav1.MachineFinalizer))
		})
	})
})
