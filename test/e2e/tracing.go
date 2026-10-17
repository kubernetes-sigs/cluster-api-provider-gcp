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
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const observabilityNamespace = "observability"

func tracingEnabled() bool {
	capgEnableTracing, err := strconv.ParseBool(e2eConfig.MustGetVariable("CAPG_ENABLE_TRACING"))
	Expect(err).ToNot(HaveOccurred())
	return capgEnableTracing
}

func applyTracingInfrastructure(ctx context.Context, clusterProxy framework.ClusterProxy, e2eConfig *clusterctl.E2EConfig, logFolder string) {
	By("Read in tracing infrastructure file")
	tracingInfraPath := e2eConfig.MustGetVariable("TRACING_INFRASTRUCTURE_CONFIGURATION")
	tracingManifest, err := os.ReadFile(tracingInfraPath)
	Expect(err).ToNot(HaveOccurred())

	By("Applying the observability manifests to the cluster")
	Eventually(func() error {
		return clusterProxy.CreateOrUpdate(ctx, tracingManifest)
	}, 30*time.Second).Should(Succeed(), "Failed to apply the tracing infrastructure")

	deploymentToWatch := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector",
			Namespace: observabilityNamespace,
		},
	}

	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     clusterProxy.GetClient(),
		Deployment: &deploymentToWatch,
	}, 3*time.Minute)

	framework.WatchDeploymentLogsByName(ctx, framework.WatchDeploymentLogsByNameInput{
		GetLister:  clusterProxy.GetClient(),
		Cache:      clusterProxy.GetCache(ctx),
		ClientSet:  clusterProxy.GetClientSet(),
		Deployment: &deploymentToWatch,
		LogPath:    filepath.Join(logFolder, "logs", observabilityNamespace),
	})
}

func assertTracesReceived(ctx context.Context, clusterProxy framework.ClusterProxy, clusterName string) {
	Eventually(func() string {
		pods, err := clusterProxy.GetClientSet().CoreV1().Pods(observabilityNamespace).
			List(ctx, metav1.ListOptions{LabelSelector: "app=otel-collector"})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Failed to find otel-collector pod for logs: %v\n", err)
			return ""
		}
		if len(pods.Items) == 0 {
			fmt.Fprint(GinkgoWriter, "No pods found to extract logs from")
			return ""
		}
		otelCollectorLogs, err := clusterProxy.GetClientSet().CoreV1().Pods(observabilityNamespace).
			GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{Container: "log-collector"}).
			Do(ctx).Raw()
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "Failed to get otel-collector logs: %v\n", err)
		}
		return string(otelCollectorLogs)
	}, time.Minute, 5*time.Second).Should(And(ContainSubstring("googleapis.com"), ContainSubstring(`"name":"GCPCluster/Reconcile"`), ContainSubstring(clusterName)))
}
