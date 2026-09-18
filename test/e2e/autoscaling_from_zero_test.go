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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed data/cluster-autoscaler/rbac.yaml
var clusterAutoscalerRBAC string

//go:embed data/cluster-autoscaler/deployment.yaml.tmpl
var clusterAutoscalerDeploymentTemplate string

var _ = Describe("Autoscaling from zero", func() {
	var (
		ctx                 = context.TODO()
		specName            = "autoscale-from-zero"
		namespace           *corev1.Namespace
		cancelWatches       context.CancelFunc
		result              *clusterctl.ApplyClusterTemplateAndWaitResult
		clusterName         string
		clusterctlLogFolder string
		additionalCleanup   func()
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, bootstrapClusterProxy, artifactFolder)
		result = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		clusterctlLogFolder = filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		cleanInput := cleanupInput{
			SpecName:             specName,
			Cluster:              result.Cluster,
			ClusterProxy:         bootstrapClusterProxy,
			ClusterctlConfigPath: clusterctlConfigPath,
			Namespace:            namespace,
			CancelWatches:        cancelWatches,
			IntervalsGetter:      e2eConfig.GetIntervals,
			SkipCleanup:          skipCleanup,
			ArtifactFolder:       artifactFolder,
			AdditionalCleanup:    additionalCleanup,
		}

		dumpSpecResourcesAndCleanup(ctx, cleanInput)
	})

	It("Should scale MachineDeployment from 0 to 1 via cluster-autoscaler", func() {
		// Setup cleanup for Cluster Autoscaler ClusterRole/ClusterRoleBinding
		additionalCleanup = func() {
			By("Cleaning up Cluster Autoscaler ClusterRole and ClusterRoleBinding")
			mgmtClient := bootstrapClusterProxy.GetClient()

			clusterRoles := []string{
				fmt.Sprintf("cluster-autoscaler-%s", clusterName),
				fmt.Sprintf("cluster-autoscaler-management-%s", clusterName),
			}

			for _, name := range clusterRoles {
				cr := &rbacv1.ClusterRole{}
				cr.Name = name
				_ = mgmtClient.Delete(ctx, cr)

				crb := &rbacv1.ClusterRoleBinding{}
				crb.Name = name
				_ = mgmtClient.Delete(ctx, crb)
			}
		}

		By("Creating a cluster with 0 worker nodes")
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: bootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder:                clusterctlLogFolder,
				ClusterctlConfigPath:     clusterctlConfigPath,
				KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
				Flavor:                   clusterctl.DefaultFlavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        e2eConfig.MustGetVariable(KubernetesVersion),
				ControlPlaneMachineCount: ptr.To[int64](1),
				WorkerMachineCount:       ptr.To[int64](0),
			},
			WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
		}, result)

		mgmtClient := bootstrapClusterProxy.GetClient()
		workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace.Name, clusterName)

		By("Adding autoscaler annotations to MachineDeployment")
		Expect(result.MachineDeployments).To(HaveLen(1))
		md := result.MachineDeployments[0]

		mdPatch := client.MergeFrom(md.DeepCopy())
		if md.Annotations == nil {
			md.Annotations = make(map[string]string)
		}
		md.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"] = "0"
		md.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"] = "3"
		Expect(mgmtClient.Patch(ctx, md, mdPatch)).To(Succeed())

		By("Verifying GCPMachineTemplate capacity is populated with 0 replicas")
		templateRef := md.Spec.Template.Spec.InfrastructureRef
		template := &infrav1.GCPMachineTemplate{}

		Eventually(func(g Gomega) {
			err := mgmtClient.Get(ctx, client.ObjectKey{Namespace: md.Namespace, Name: templateRef.Name}, template)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(template.Status.Capacity).NotTo(BeNil(), "Status.Capacity should be populated even with 0 replicas")
			g.Expect(template.Status.Capacity.Cpu().IsZero()).To(BeFalse(), "CPU capacity should be set")
			g.Expect(template.Status.Capacity.Memory().IsZero()).To(BeFalse(), "Memory capacity should be set")
			g.Expect(template.Status.NodeInfo).NotTo(BeNil(), "Status.NodeInfo should be populated even with 0 replicas")
			g.Expect(template.Status.NodeInfo.Architecture).To(BeElementOf(infrav1.ArchitectureAmd64, infrav1.ArchitectureArm64))
			g.Expect(template.Status.NodeInfo.OperatingSystem).To(Equal(infrav1.OperatingSystemLinux))
		}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Succeed())

		By("Deploying Cluster Autoscaler RBAC to management cluster")
		Expect(deployClusterAutoscalerRBAC(ctx, mgmtClient, namespace.Name, clusterName)).To(Succeed())

		By("Deploying Cluster Autoscaler to management cluster")
		Expect(deployClusterAutoscaler(ctx, mgmtClient, namespace.Name, clusterName)).To(Succeed())

		By("Waiting for Cluster Autoscaler pod to be ready")
		Eventually(func(g Gomega) {
			pods, err := bootstrapClusterProxy.GetClientSet().CoreV1().Pods(namespace.Name).
				List(ctx, metav1.ListOptions{LabelSelector: "app=cluster-autoscaler"})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pods.Items).To(HaveLen(1))
			pod := pods.Items[0]
			g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))

			hasReadyCondition := false
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady {
					hasReadyCondition = true
					g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))
					break
				}
			}
			g.Expect(hasReadyCondition).To(BeTrue(), "Pod Ready condition not found")
		}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

		By("Creating workload to trigger autoscaler scale-up")
		workloadClientset := workloadClusterProxy.GetClientSet()

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "autoscale-trigger",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "autoscale-trigger"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "autoscale-trigger"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "registry.k8s.io/pause:3.9",
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

		_, err := workloadClientset.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		defer func() {
			By("Cleaning up workload deployment")
			_ = workloadClientset.AppsV1().Deployments("default").Delete(ctx, deployment.Name, metav1.DeleteOptions{})
		}()

		By("Waiting for Cluster Autoscaler to scale MachineDeployment from 0 to 1+")
		Eventually(func(g Gomega) {
			updatedMD := &clusterv1.MachineDeployment{}
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(md), updatedMD)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updatedMD.Spec.Replicas).NotTo(BeNil())
			g.Expect(*updatedMD.Spec.Replicas).To(BeNumerically(">=", 1))
			g.Expect(updatedMD.Status.Replicas).To(HaveValue(BeNumerically(">=", 1)))
			g.Expect(updatedMD.Status.ReadyReplicas).To(HaveValue(BeNumerically(">=", 1)))
		}, e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...).Should(Succeed())

		By("Verifying workload deployment is ready")
		Eventually(func(g Gomega) {
			dep, err := workloadClientset.AppsV1().Deployments("default").Get(ctx, deployment.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(dep.Status.ReadyReplicas).To(Equal(int32(2)))
		}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
	})
})

// deployClusterAutoscalerRBAC deploys the RBAC resources for Cluster Autoscaler to management cluster.
func deployClusterAutoscalerRBAC(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	tmpl, err := template.New("cluster-autoscaler-rbac").Parse(clusterAutoscalerRBAC)
	if err != nil {
		return fmt.Errorf("failed to parse RBAC template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Namespace":   namespace,
		"ClusterName": clusterName,
	})
	if err != nil {
		return fmt.Errorf("failed to execute RBAC template: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	objects := []runtime.Object{}

	docs := bytes.Split(buf.Bytes(), []byte("\n---\n"))
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode RBAC YAML: %w", err)
		}
		objects = append(objects, obj)
	}

	for _, obj := range objects {
		clientObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("object is not a client.Object: %T", obj)
		}

		err := mgmtClient.Create(ctx, clientObj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %s %s: %w",
				obj.GetObjectKind().GroupVersionKind().Kind, clientObj.GetName(), err)
		}
	}

	return nil
}

// deployClusterAutoscaler deploys the Cluster Autoscaler deployment to management cluster.
func deployClusterAutoscaler(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	tmpl, err := template.New("cluster-autoscaler").Parse(clusterAutoscalerDeploymentTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse deployment template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Namespace":   namespace,
		"ClusterName": clusterName,
	})
	if err != nil {
		return fmt.Errorf("failed to execute deployment template: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to decode deployment YAML: %w", err)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("decoded object is not a Deployment")
	}

	err = mgmtClient.Create(ctx, deployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Cluster Autoscaler deployment %s: %w", deployment.Name, err)
	}

	return nil
}
