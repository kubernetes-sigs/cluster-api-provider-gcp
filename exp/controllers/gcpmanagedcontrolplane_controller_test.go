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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"

	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

func Test_markNotReady(t *testing.T) {
	g := NewWithT(t)
	cp := &infrav1exp.GCPManagedControlPlane{}
	message := "foo bar"
	markNotReady(cp, infrav1exp.GKEControlPlaneAutopilotRequiresRegionalReason, message)

	for _, condType := range []clusterv1beta1.ConditionType{infrav1exp.GKEControlPlaneReadyCondition, clusterv1beta1.ReadyCondition} {
		v1beta1Cond := v1beta1conditions.Get(cp, condType)
		g.Expect(v1beta1Cond).NotTo(BeNil())
		g.Expect(v1beta1Cond.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(v1beta1Cond.Reason).To(Equal(infrav1exp.GKEControlPlaneAutopilotRequiresRegionalReason))
		g.Expect(v1beta1Cond.Message).To(Equal(message))
		g.Expect(v1beta1Cond.Severity).To(Equal(clusterv1beta1.ConditionSeverityError))
	}

	for _, condType := range []string{infrav1exp.GCPManagedControlPlaneReadyCondition, infrav1exp.GCPManagedControlPlaneGKEControlPlaneReadyCondition} {
		v1beta2Cond := v1beta2conditions.Get(cp, condType)
		g.Expect(v1beta2Cond).NotTo(BeNil())
		g.Expect(v1beta2Cond.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(v1beta2Cond.Reason).To(Equal(infrav1exp.GKEControlPlaneAutopilotRequiresRegionalReason))
		g.Expect(v1beta2Cond.Message).To(Equal(message))
	}
}
