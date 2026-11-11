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
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	bootstrapv1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/bootstrap/gke/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	testNamespace       = "default"
	testGKEConfigName   = "gke-config"
	testMachinePoolName = "mp-0"
	testGCPMPName       = "gcp-mp-0"
)

func newMachinePool() *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: testMachinePoolName, Namespace: testNamespace},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName: "cluster",
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: "cluster",
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: infrav1exp.GroupVersion.Group,
						Kind:     "GCPManagedMachinePool",
						Name:     testGCPMPName,
					},
				},
			},
		},
	}
}

func newGCPManagedMachinePool(ready bool) *infrav1exp.GCPManagedMachinePool {
	return &infrav1exp.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: testGCPMPName, Namespace: testNamespace},
		Status:     infrav1exp.GCPManagedMachinePoolStatus{Ready: ready},
	}
}

func newGKEConfig(withOwner bool) *bootstrapv1exp.GKEConfig {
	config := &bootstrapv1exp.GKEConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testGKEConfigName, Namespace: testNamespace},
	}
	if withOwner {
		config.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachinePool",
			Name:       testMachinePoolName,
			UID:        "uid",
		}}
	}
	return config
}

func TestGKEConfigReconcile(t *testing.T) {
	tests := []struct {
		name               string
		objects            []client.Object
		wantReady          bool
		wantDataSecretName *string
		wantSecretCreated  *bool
	}{
		{
			name: "GKEConfig is marked ready and data secret created when GCPManagedMachinePool is ready",
			objects: []client.Object{
				newGKEConfig(true),
				newMachinePool(),
				newGCPManagedMachinePool(true),
			},
			wantReady:          true,
			wantDataSecretName: new(""),
			wantSecretCreated:  new(true),
		},
		{
			name: "GKEConfig is not marked ready while GCPManagedMachinePool is not ready",
			objects: []client.Object{
				newGKEConfig(true),
				newMachinePool(),
				newGCPManagedMachinePool(false),
			},
		},
		{
			name: "GKEConfig is not marked ready while GCPManagedMachinePool does not exist",
			objects: []client.Object{
				newGKEConfig(true),
				newMachinePool(),
			},
		},
		{
			name: "GKEConfig without an owner MachinePool is ignored",
			objects: []client.Object{
				newGKEConfig(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			scheme := runtime.NewScheme()
			g.Expect(bootstrapv1exp.AddToScheme(scheme)).To(Succeed())
			g.Expect(infrav1exp.AddToScheme(scheme)).To(Succeed())
			g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())

			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.objects...).
				WithStatusSubresource(&bootstrapv1exp.GKEConfig{}).
				Build()

			r := &GKEConfigReconciler{Client: c}
			key := client.ObjectKey{Name: testGKEConfigName, Namespace: testNamespace}
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			g.Expect(err).NotTo(HaveOccurred())

			config := &bootstrapv1exp.GKEConfig{}
			g.Expect(c.Get(ctx, key, config)).To(Succeed())
			g.Expect(config.Status.Ready).To(Equal(tt.wantReady))
			g.Expect(config.Status.DataSecretName).To(Equal(tt.wantDataSecretName))
			g.Expect(config.Status.Initialization.DataSecretCreated).To(Equal(tt.wantSecretCreated))
		})
	}
}
