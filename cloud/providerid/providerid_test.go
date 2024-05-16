/*
Copyright 2023 The Kubernetes Authors.

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

package providerid_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/providerid"
)

func TestProviderID_New(t *testing.T) {
	RegisterTestingT(t)

	testCases := []struct {
		testname           string
		project            string
		location           string
		name               string
		expectedProviderID string
		expectError        bool
	}{
		{
			testname:    "no project, should fail",
			project:     "",
			location:    "eu-west4",
			name:        "vm1",
			expectError: true,
		},
		{
			testname:    "no location, should fail",
			project:     "proj1",
			location:    "",
			name:        "vm1",
			expectError: true,
		},
		{
			testname:    "no name, should fail",
			project:     "proj1",
			location:    "eu-west4",
			name:        "",
			expectError: true,
		},
		{
			testname:           "with all details, should pass",
			project:            "proj1",
			location:           "eu-west4",
			name:               "vm1",
			expectError:        false,
			expectedProviderID: "gce://proj1/eu-west4/vm1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testname, func(_ *testing.T) {
			providerID, err := providerid.New(tc.project, tc.location, tc.name)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(providerID.String()).To(Equal(tc.expectedProviderID))
			}
		})
	}
}

func TestProviderID_NewFromResourceURL(t *testing.T) {
	RegisterTestingT(t)

	testCases := []struct {
		testname           string
		resourceURL        string
		expectedProviderID string
		expectError        bool
	}{
		{
			testname:    "invalid url, should fail",
			resourceURL: "hvfnhdkdk",
			expectError: true,
		},
		{
			testname:           "valid instance url, should pass",
			resourceURL:        "https://www.googleapis.com/compute/v1/projects/myproject/zones/europe-west2-a/instances/gke-capg-dskczmdculd-capg-e2e-ebs0oy--014f89ba-sx2p",
			expectError:        false,
			expectedProviderID: "gce://myproject/europe-west2-a/gke-capg-dskczmdculd-capg-e2e-ebs0oy--014f89ba-sx2p",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testname, func(_ *testing.T) {
			providerID, err := providerid.NewFromResourceURL(tc.resourceURL)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(providerID.String()).To(Equal(tc.expectedProviderID))
			}
		})
	}
}
