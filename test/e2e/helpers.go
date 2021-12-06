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
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"sigs.k8s.io/cluster-api/test/framework/kubernetesversions"
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
