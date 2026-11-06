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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	expinfrav1 "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	expwebhooks "sigs.k8s.io/cluster-api-provider-gcp/exp/webhooks"
	"sigs.k8s.io/cluster-api-provider-gcp/webhooks"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func init() {
	klog.InitFlags(nil)
	klog.SetOutput(GinkgoWriter)
	logf.SetLogger(klog.Background())
}

func TestWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhooks Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook", "manifests.yaml")},
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).ToNot(BeNil())
	DeferCleanup(func() {
		stopErr := testEnv.Stop()
		Expect(stopErr).NotTo(HaveOccurred())
	})

	scheme := runtime.NewScheme()
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	Expect(expinfrav1.AddToScheme(scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Metrics: metrics.Options{
			BindAddress: "0",
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&webhooks.GCPClusterTemplate{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&webhooks.GCPMachineTemplate{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&expwebhooks.GCPManagedClusterTemplate{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = expwebhooks.SetupGCPManagedControlPlaneTemplateWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = expwebhooks.SetupGCPManagedMachinePoolTemplateWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&expwebhooks.GCPManagedMachinePool{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithCancel(context.Background())
	DeferCleanup(cancel)

	go func() {
		defer GinkgoRecover()
		startErr := mgr.Start(ctx)
		Expect(startErr).To(Succeed())
	}()

	checker := mgr.GetWebhookServer().StartedChecker()
	Eventually(func() error { return checker(nil) }).WithTimeout(5 * time.Second).Should(Succeed())
})
