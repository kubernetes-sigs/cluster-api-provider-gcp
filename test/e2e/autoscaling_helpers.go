//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deployClusterAutoscalerRBAC deploys Cluster Autoscaler RBAC resources to the management cluster.
func deployClusterAutoscalerRBAC(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-autoscaler",
			Namespace: namespace,
		},
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-autoscaler-management-" + clusterName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.x-k8s.io"},
				Resources: []string{
					"machinedeployments",
					"machinedeployments/scale",
					"machines",
					"machinesets",
					"machinesets/scale",
					"machinepools",
					"machinepools/scale",
				},
				Verbs: []string{"get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"infrastructure.cluster.x-k8s.io"},
				Resources: []string{
					"gcpmachines",
					"gcpmachinetemplates",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-autoscaler-management-" + clusterName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-autoscaler-management-" + clusterName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "cluster-autoscaler",
				Namespace: namespace,
			},
		},
	}

	objects := []client.Object{sa, clusterRole, clusterRoleBinding}
	for _, obj := range objects {
		if err := mgmtClient.Create(ctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %T %s: %w", obj, obj.GetName(), err)
		}
	}

	return nil
}

// deployClusterAutoscaler deploys the Cluster Autoscaler deployment to the management cluster.
func deployClusterAutoscaler(ctx context.Context, mgmtClient client.Client, namespace, clusterName, clusterAutoscalerVersion string) error {
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-autoscaler-" + clusterName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "cluster-autoscaler",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "cluster-autoscaler",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "cluster-autoscaler",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "cluster-autoscaler",
					Tolerations: []corev1.Toleration{
						{
							Effect: corev1.TaintEffectNoSchedule,
							Key:    "node-role.kubernetes.io/control-plane",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "cluster-autoscaler",
							Image: "registry.k8s.io/autoscaling/cluster-autoscaler:" + clusterAutoscalerVersion,
							Command: []string{
								"/cluster-autoscaler",
								"--cloud-provider=clusterapi",
								"--node-group-auto-discovery=clusterapi:namespace=" + namespace + ",clusterName=" + clusterName,
								"--kubeconfig=/etc/kubernetes/value",
								"--clusterapi-cloud-config-authoritative",
								"--scale-down-delay-after-add=1m",
								"--scale-down-unneeded-time=2m",
								"--scale-down-delay-after-delete=30s",
								"--max-node-provision-time=10m",
								"--balance-similar-node-groups",
								"--skip-nodes-with-system-pods=false",
								"--skip-nodes-with-local-storage=false",
								"--expander=random",
								"--kube-client-qps=20",
								"--kube-client-burst=30",
								"--v=4",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CAPI_GROUP",
									Value: "cluster.x-k8s.io",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workload-kubeconfig",
									MountPath: "/etc/kubernetes",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health-check",
										Port: intstr.FromInt(8085),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health-check",
										Port: intstr.FromInt(8085),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workload-kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: clusterName + "-kubeconfig",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := mgmtClient.Create(ctx, deployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Cluster Autoscaler deployment %s: %w", deployment.Name, err)
	}

	return nil
}

// createTriggerWorkload creates a Deployment in the workload cluster that requires
// nodes with the autoscale-group=from-zero label to trigger autoscaler scale-up.
func createTriggerWorkload(ctx context.Context, workloadClient client.Client, clusterName string) (client.Object, error) {
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-trigger",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "autoscale-trigger",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "autoscale-trigger",
					},
				},
				Spec: corev1.PodSpec{
					SchedulerName: "default-scheduler",
					Containers: []corev1.Container{
						{
							Name:  "pause",
							Image: "registry.k8s.io/pause:3.10",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := workloadClient.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create trigger deployment: %w", err)
	}

	return deployment, nil
}
