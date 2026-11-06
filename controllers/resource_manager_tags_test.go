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

package controllers

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
)

var _ = Describe("ResourceManagerTags Tests", func() {
	It("should error when duplicate tag keys are used", func(ctx context.Context) {
		obj := infrav1.GCPMachine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-machine-",
				Namespace:    "default",
			},
			Spec: infrav1.GCPMachineSpec{
				InstanceType: "n2-standard-4",
				ResourceManagerTags: infrav1.ResourceManagerTags{
					infrav1.ResourceManagerTag{
						ParentID: "my-proj",
						Key:      "my-key",
						Value:    "foo",
					},
					infrav1.ResourceManagerTag{
						ParentID: "my-proj",
						Key:      "my-key",
						Value:    "bar",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, &obj)
		var invalidErr apierrors.APIStatus
		Expect(errors.As(err, &invalidErr)).To(BeTrue())
		Expect(invalidErr.Status().Details.Causes).To(ContainElement(
			And(HaveField("Type", metav1.CauseTypeFieldValueDuplicate), HaveField("Field", "spec.resourceManagerTags[1]"))))
	})
	It("should error when more than 50 tags are used", func(ctx context.Context) {
		obj := infrav1.GCPMachine{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-machine-",
				Namespace:    "default",
			},
			Spec: infrav1.GCPMachineSpec{
				InstanceType:        "n2-standard-4",
				ResourceManagerTags: make(infrav1.ResourceManagerTags, 51),
			},
		}

		for i := range 51 {
			obj.Spec.ResourceManagerTags[i] = infrav1.ResourceManagerTag{
				ParentID: "my-proj",
				Key:      fmt.Sprintf("my-key-%d", i),
				Value:    "bar",
			}
		}

		err := k8sClient.Create(ctx, &obj)
		var invalidErr apierrors.APIStatus
		Expect(errors.As(err, &invalidErr)).To(BeTrue())
		Expect(invalidErr.Status().Details.Causes).To(ContainElement(
			And(HaveField("Type", metav1.CauseTypeTooMany), HaveField("Field", "spec.resourceManagerTags"))))
	})
})
