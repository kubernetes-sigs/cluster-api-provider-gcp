//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"io"
	"net/http"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resolveCIVersion resolves kubernetes version labels (e.g. latest, latest-1.xx) to the corresponding CI version numbers.
// Go implementation of https://github.com/kubernetes-sigs/cluster-api/blob/d1dc87d5df3ab12a15ae5b63e50541a191b7fec4/scripts/ci-e2e-lib.sh#L75-L95.
func resolveCIVersion(label string) (string, error) {
	if ciVersion, ok := os.LookupEnv("CI_VERSION"); ok {
		return ciVersion, nil
	}

	if strings.HasPrefix(label, "latest") {
		if kubernetesVersion, err := latestCIVersion(label); err == nil {
			return kubernetesVersion, nil
		}
	}
	// default to https://dl.k8s.io/ci/latest.txt if the label can't be resolved
	return kubernetesversions.LatestCIRelease()
}

// latestCIVersion returns the latest CI version of a given label in the form of latest-1.xx.
func latestCIVersion(label string) (string, error) {
	ciVersionURL := fmt.Sprintf("https://dl.k8s.io/ci/%s.txt", label)
	resp, err := http.Get(ciVersionURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

// verifyMachinePoolConditions verifies a set of conditions that get set on GCPMachinePool objects eventually
func verifyMachinePoolConditions(ctx context.Context, getter framework.Getter, key client.ObjectKey, intervals ...interface{}) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		gcpMachinePool := &infrav1exp.GCPMachinePool{}
		g.Expect(getter.Get(ctx, key, gcpMachinePool)).To(Succeed())
		for _, conditionType := range []string{
			string(infrav1exp.MIGReadyCondition), string(infrav1exp.InstanceTemplateReadyCondition), string(clusterv1.ReadyCondition),
		} {
			condition := meta.FindStatusCondition(gcpMachinePool.Status.Conditions, conditionType)
			g.Expect(condition).ToNot(BeNil(), fmt.Sprintf("expected condition %s to not be nil", conditionType))
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue), fmt.Sprintf("condition %s (Reason: '%s', Message: '%s') is not ready", conditionType, condition.Reason, condition.Message))
		}
	}, intervals...).Should(Succeed())
}
