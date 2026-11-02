/*
 Copyright 2026 The Kubernetes Authors.

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

package webhooks

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dryRunTestEntry struct {
	name         string
	constructor  func() client.Object
	mutate       func(obj client.Object)
	errorMatcher types.GomegaMatcher
}

type validationTestEntry struct {
	name        string
	constructor func() client.Object
	invalidate  func(obj client.Object)
}

func createUnstructuredObject(obj client.Object, annotate bool) *unstructured.Unstructured {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	Expect(err).To(Not(HaveOccurred()))
	gvk, err := k8sClient.GroupVersionKindFor(obj)
	Expect(err).To(Not(HaveOccurred()))
	u["apiVersion"] = gvk.GroupVersion().String()
	u["kind"] = gvk.Kind
	unstructured.RemoveNestedField(u, "metadata", "managedFields")
	unstructured.RemoveNestedField(u, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(u, "metadata", "uid")
	unstructured.RemoveNestedField(u, "metadata", "creationTimestamp")
	result := unstructured.Unstructured{Object: u}
	if annotate {
		annotations := map[string]string{
			clusterv1.TopologyDryRunAnnotation: "true",
		}
		maps.Copy(annotations, result.GetAnnotations())
		result.SetAnnotations(annotations)
	}
	return &result
}

func applyChange(ctx context.Context, obj client.Object, annotate, dryRun bool) error {
	u := createUnstructuredObject(obj, annotate)

	options := []client.ApplyOption{client.ForceOwnership, client.FieldOwner("cluster-api-gcp")}
	if dryRun {
		options = append(options, client.DryRunAll)
	}

	return k8sClient.Apply(ctx, client.ApplyConfigurationFromUnstructured(u), options...)
}

var _ = Describe("Webhook Dry Run Tests", func() {
	DescribeTableSubtree("All webhooks run dry-runs correctly", func(dryRunEntry dryRunTestEntry) {
		Describe(dryRunEntry.name, func() {
			var obj client.Object

			BeforeEach(func(ctx SpecContext) {
				obj = dryRunEntry.constructor()
				err := k8sClient.Create(ctx, obj)
				Expect(err).To(Not(HaveOccurred()))
			})

			It("rejects a plain update", func(ctx SpecContext) {
				dryRunEntry.mutate(obj)
				err := k8sClient.Update(ctx, obj)
				Expect(err).To(dryRunEntry.errorMatcher)
			})
			It("succeeds when using dry-run SSA and dry-run annotation is used", func(ctx SpecContext) {
				dryRunEntry.mutate(obj)
				err := applyChange(ctx, obj, true, true)
				Expect(err).To(Not(HaveOccurred()))
			})
			It("rejects request when using dry-run SSA but no annotation is used", func(ctx SpecContext) {
				dryRunEntry.mutate(obj)
				err := applyChange(ctx, obj, false, true)
				Expect(err).To(dryRunEntry.errorMatcher)
			})
			It("rejects request when using non-dry-run SSA and annotation is used", func(ctx SpecContext) {
				dryRunEntry.mutate(obj)
				err := applyChange(ctx, obj, true, false)
				Expect(err).To(dryRunEntry.errorMatcher)
			})
		})
	},
		Entry("GCPClusterTemplate", dryRunTestEntry{
			name: "GCPClusterTemplate",
			constructor: func() client.Object {
				return &infrav1.GCPClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "cluster-template-",
						Namespace:    "default",
					},
					Spec: infrav1.GCPClusterTemplateSpec{
						Template: infrav1.GCPClusterTemplateResource{
							Spec: infrav1.GCPClusterSpec{
								Project: "my-proj",
								Region:  "us-central1",
							},
						},
					},
				}
			},
			mutate: func(obj client.Object) {
				obj.(*infrav1.GCPClusterTemplate).Spec.Template.Spec.Project = "my-new-proj"
			},
			errorMatcher: MatchError(apierrors.IsBadRequest, "be a BadRequest (400)"),
		}),
		Entry("GCPManagedClusterTemplate", dryRunTestEntry{
			name: "GCPManagedClusterTemplate",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedClusterTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "cluster-template-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedClusterTemplateSpec{
						Template: expinfrav1.GCPManagedClusterTemplateResource{
							Spec: expinfrav1.GCPManagedClusterTemplateResourceSpec{
								Project: "my-proj",
								Region:  "us-central1",
							},
						},
					},
				}
			},
			mutate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedClusterTemplate).Spec.Template.Spec.Project = "my-new-proj"
			},
			errorMatcher: MatchError(apierrors.IsBadRequest, "be a BadRequest (400)"),
		}),
		Entry("GCPMachineTemplate", dryRunTestEntry{
			name: "GCPMachineTemplate",
			constructor: func() client.Object {
				return &infrav1.GCPMachineTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "machine-template-",
						Namespace:    "default",
					},
					Spec: infrav1.GCPMachineTemplateSpec{
						Template: infrav1.GCPMachineTemplateResource{
							Spec: infrav1.GCPMachineSpec{
								InstanceType: "n1-standard-2",
							},
						},
					},
				}
			},
			mutate: func(obj client.Object) {
				obj.(*infrav1.GCPMachineTemplate).Spec.Template.Spec.InstanceType = "n1-standard-16"
			},
			errorMatcher: MatchError(apierrors.IsInvalid, "be Invalid (422)"),
		}),
		Entry("GCPManagedControlPlaneTemplate", dryRunTestEntry{
			name: "GCPManagedControlPlaneTemplate",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedControlPlaneTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "control-plane-template-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedControlPlaneTemplateSpec{
						Template: expinfrav1.GCPManagedControlPlaneTemplateResource{
							Spec: expinfrav1.GCPManagedControlPlaneTemplateResourceSpec{
								GCPManagedControlPlaneClassSpec: expinfrav1.GCPManagedControlPlaneClassSpec{
									Project: "my-proj",
								},
							},
						},
					},
				}
			},
			mutate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedControlPlaneTemplate).Spec.Template.Spec.Project = "my-proj-2"
			},
			errorMatcher: MatchError(apierrors.IsInvalid, "be Invalid (422)"),
		}),
		Entry("GCPManagedMachinePoolTemplate", dryRunTestEntry{
			name: "GCPManagedMachinePoolTemplate",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedMachinePoolTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "managed-machine-pool-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
						Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
							Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
								GCPManagedMachinePoolClassSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
									NodePoolName: "my-pool",
								},
							},
						},
					},
				}
			},
			mutate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedMachinePoolTemplate).Spec.Template.Spec.NodePoolName = "my-pool-2"
			},
			errorMatcher: MatchError(apierrors.IsInvalid, "be Invalid (422)"),
		}),
	)
	DescribeTableSubtree("Validation still runs during a dry-run", func(validationTestEntry validationTestEntry) {
		Describe(validationTestEntry.name, func() {
			var obj client.Object

			BeforeEach(func(ctx SpecContext) {
				obj = validationTestEntry.constructor()
				err := k8sClient.Create(ctx, obj)
				Expect(err).To(Not(HaveOccurred()))
			})

			It("rejects an update for non-immutability reasons", func(ctx SpecContext) {
				validationTestEntry.invalidate(obj)
				err := applyChange(ctx, obj, true, true)
				Expect(err).To(MatchError(apierrors.IsInvalid, "be Invalid (422)"))
			})
		})
	},
		Entry("GCPManagedControlPlaneTemplate - Autopilot Enabled/LoggingService Valid", validationTestEntry{
			name: "GCPManagedControlPlaneTemplate - Autopilot Enabled/LoggingService Valid",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedControlPlaneTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "control-plane-template-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedControlPlaneTemplateSpec{
						Template: expinfrav1.GCPManagedControlPlaneTemplateResource{
							Spec: expinfrav1.GCPManagedControlPlaneTemplateResourceSpec{
								GCPManagedControlPlaneClassSpec: expinfrav1.GCPManagedControlPlaneClassSpec{
									Project:         "my-proj",
									Location:        "us-central1",
									EnableAutopilot: true,
									ReleaseChannel:  new(expinfrav1.Regular),
								},
							},
						},
					},
				}
			},
			invalidate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedControlPlaneTemplate).Spec.Template.Spec.LoggingService = new(expinfrav1.LoggingService("logging.googleapis.com/kubernetes"))
			},
		}),
		Entry("GCPManagedControlPlaneTemplate - Invalid LoggingService", validationTestEntry{
			name: "GCPManagedControlPlaneTemplate - Invalid LoggingService",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedControlPlaneTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "control-plane-template-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedControlPlaneTemplateSpec{
						Template: expinfrav1.GCPManagedControlPlaneTemplateResource{
							Spec: expinfrav1.GCPManagedControlPlaneTemplateResourceSpec{
								GCPManagedControlPlaneClassSpec: expinfrav1.GCPManagedControlPlaneClassSpec{
									Project:  "my-proj",
									Location: "us-central1",
								},
							},
						},
					},
				}
			},
			invalidate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedControlPlaneTemplate).Spec.Template.Spec.LoggingService = new(expinfrav1.LoggingService("bogus"))
			},
		}),
		Entry("GCPManagedMachinePoolTemplate - Invalid Scaling Policy", validationTestEntry{
			name: "GCPManagedMachinePoolTemplate - Invalid Scaling Policy",
			constructor: func() client.Object {
				return &expinfrav1.GCPManagedMachinePoolTemplate{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "managed-machine-pool-",
						Namespace:    "default",
					},
					Spec: expinfrav1.GCPManagedMachinePoolTemplateSpec{
						Template: expinfrav1.GCPManagedMachinePoolTemplateResource{
							Spec: expinfrav1.GCPManagedMachinePoolTemplateResourceSpec{
								GCPManagedMachinePoolClassSpec: expinfrav1.GCPManagedMachinePoolClassSpec{
									NodePoolName: "my-pool",
								},
							},
						},
					},
				}
			},
			invalidate: func(obj client.Object) {
				obj.(*expinfrav1.GCPManagedMachinePoolTemplate).Spec.Template.Spec.Scaling = &expinfrav1.NodePoolAutoScaling{
					MinCount: new(int32(5)),
					MaxCount: new(int32(3)),
				}
			},
		}))
})
