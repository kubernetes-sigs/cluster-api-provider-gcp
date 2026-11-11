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

package scope

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// credentialsSecretFor builds a fake client holding a credentials Secret with
// rawData, plus the reference pointing at it.
func credentialsSecretFor(t *testing.T, rawData []byte) (*infrav1.ObjectReference, client.Client) {
	t.Helper()
	g := NewWithT(t)

	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gcp-creds", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"credentials": rawData},
	}
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credSecret).Build()

	return &infrav1.ObjectReference{Name: "gcp-creds", Namespace: corev1.NamespaceDefault}, fakeClient
}

// adcCredentialsFile writes rawData to a temp file and points ConfigFileEnvVar
// at it, so credentials resolve through an Application Default Credentials
// file rather than through a Secret reference.
func adcCredentialsFile(t *testing.T, rawData []byte) {
	t.Helper()
	g := NewWithT(t)

	path := filepath.Join(t.TempDir(), "adc.json")
	g.Expect(os.WriteFile(path, rawData, 0o600)).To(Succeed())
	t.Setenv(ConfigFileEnvVar, path)
}

// withUniverseDomain adds an explicit universe_domain to a credential blob, as
// credentials issued for a sovereign (GCD) universe carry.
func withUniverseDomain(t *testing.T, rawData []byte, universeDomain string) []byte {
	t.Helper()
	g := NewWithT(t)

	fields := map[string]any{}
	g.Expect(json.Unmarshal(rawData, &fields)).To(Succeed())
	fields["universe_domain"] = universeDomain

	data, err := json.Marshal(fields)
	g.Expect(err).NotTo(HaveOccurred())
	return data
}

func TestDefaultClientOptions(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(t *testing.T) (*infrav1.ObjectReference, client.Client)
		wantUniverseDomain string
		wantErr            bool
	}{
		{
			name: "no credentials reference reads the application default credentials file",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				adcCredentialsFile(t, withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com"))
				return nil, nil
			},
			wantUniverseDomain: "gcd.example.com",
		},
		{
			name: "application default credentials without a universe domain",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				adcCredentialsFile(t, serviceAccountJSON(t, ""))
				return nil, nil
			},
			wantUniverseDomain: DefaultUniverseDomain,
		},
		{
			name: "application default credentials cannot be read",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				t.Setenv(ConfigFileEnvVar, filepath.Join(t.TempDir(), "missing.json"))
				return nil, nil
			},
			wantErr: true,
		},
		{
			name: "credentials without a universe domain",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				return credentialsSecretFor(t, serviceAccountJSON(t, ""))
			},
			wantUniverseDomain: DefaultUniverseDomain,
		},
		{
			name: "credentials with a sovereign universe domain",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				return credentialsSecretFor(t, withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com"))
			},
			wantUniverseDomain: "gcd.example.com",
		},
		{
			name: "credentials secret not found",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				scheme := runtime.NewScheme()
				NewWithT(t).Expect(corev1.AddToScheme(scheme)).To(Succeed())

				return &infrav1.ObjectReference{Name: "missing", Namespace: corev1.NamespaceDefault},
					fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			credentialsRef, crClient := tt.setup(t)

			opts, universeDomain, err := defaultClientOptions(t.Context(), credentialsRef, crClient)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(universeDomain).To(BeEmpty())
				g.Expect(opts).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(universeDomain).To(Equal(tt.wantUniverseDomain))
			g.Expect(opts).NotTo(BeEmpty())
		})
	}
}

// TestTagBindingsEndpoint also guards the property that matters for existing
// users: in the default universe the endpoint stays byte-identical to the one
// the previously hardcoded expression produced, so only GCD behaviour changes.
func TestTagBindingsEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		location       string
		universeDomain string
		endpoints      *infrav1.ServiceEndpoints
		want           string
	}{
		{
			name:           "default universe domain",
			location:       "us-central1",
			universeDomain: DefaultUniverseDomain,
			want:           "us-central1-cloudresourcemanager.googleapis.com:443",
		},
		{
			name:           "sovereign universe domain",
			location:       "us-central1",
			universeDomain: "gcd.example.com",
			want:           "us-central1-cloudresourcemanager.gcd.example.com:443",
		},
		{
			name:     "empty universe domain falls back to the default",
			location: "europe-west1",
			want:     "europe-west1-cloudresourcemanager.googleapis.com:443",
		},
		{
			name:           "default universe domain in another region",
			location:       "europe-west1",
			universeDomain: DefaultUniverseDomain,
			want:           "europe-west1-cloudresourcemanager.googleapis.com:443",
		},
		{
			name:           "default universe domain with a zone",
			location:       "us-central1-a",
			universeDomain: DefaultUniverseDomain,
			want:           "us-central1-a-cloudresourcemanager.googleapis.com:443",
		},
		{
			name:           "explicit endpoint overrides the sovereign universe domain",
			location:       "us-central1",
			universeDomain: "gcd.example.com",
			endpoints:      &infrav1.ServiceEndpoints{ResourceManagerServiceEndpoint: "cloudresourcemanager.override.example.com:443"},
			want:           "cloudresourcemanager.override.example.com:443",
		},
		{
			name:           "empty endpoint override falls back to the derived endpoint",
			location:       "us-central1",
			universeDomain: "gcd.example.com",
			endpoints:      &infrav1.ServiceEndpoints{},
			want:           "us-central1-cloudresourcemanager.gcd.example.com:443",
		},
		{
			name:           "an unrelated endpoint override does not change the derived endpoint",
			location:       "us-central1",
			universeDomain: "gcd.example.com",
			endpoints:      &infrav1.ServiceEndpoints{ComputeServiceEndpoint: "https://compute.gcd.example.com"},
			want:           "us-central1-cloudresourcemanager.gcd.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewWithT(t).Expect(tagBindingsEndpoint(tt.location, tt.universeDomain, tt.endpoints)).To(Equal(tt.want))
		})
	}
}

// TestNewComputeService asserts the base URL the service ends up dialling,
// which is what the universe domain plumbing actually has to get right: a
// sovereign (GCD) credential must move the client off googleapis.com, and an
// explicit endpoint override must still win over it.
func TestNewComputeService(t *testing.T) {
	tests := []struct {
		name         string
		rawData      func(t *testing.T) []byte
		endpoints    *infrav1.ServiceEndpoints
		wantBasePath string
	}{
		{
			name:         "default universe domain",
			rawData:      func(t *testing.T) []byte { return serviceAccountJSON(t, "") },
			wantBasePath: "https://compute.googleapis.com/compute/v1/",
		},
		{
			name: "sovereign universe domain",
			rawData: func(t *testing.T) []byte {
				return withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com")
			},
			wantBasePath: "https://compute.gcd.example.com/compute/v1/",
		},
		{
			name: "explicit service endpoint overrides the sovereign universe domain",
			rawData: func(t *testing.T) []byte {
				return withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com")
			},
			endpoints:    &infrav1.ServiceEndpoints{ComputeServiceEndpoint: "https://compute.override.example.com"},
			wantBasePath: "https://compute.override.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			credentialsRef, crClient := credentialsSecretFor(t, tt.rawData(t))

			computeSvc, err := newComputeService(t.Context(), credentialsRef, crClient, tt.endpoints)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(computeSvc).NotTo(BeNil())
			g.Expect(computeSvc.BasePath).To(Equal(tt.wantBasePath))
		})
	}
}
