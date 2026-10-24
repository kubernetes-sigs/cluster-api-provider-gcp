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
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// credentialsSecretFor builds a fake client holding a credentials Secret with
// rawData, plus the reference pointing at it.
func credentialsSecretFor(t *testing.T, rawData []byte) (*infrav1.ObjectReference, client.Client) {
	t.Helper()

	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gcp-creds", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"credentials": rawData},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credSecret).Build()

	return &infrav1.ObjectReference{Name: "gcp-creds", Namespace: corev1.NamespaceDefault}, fakeClient
}

// adcCredentialsFile writes rawData to a temp file and points ConfigFileEnvVar
// at it, so credentials resolve through an Application Default Credentials
// file rather than through a Secret reference.
func adcCredentialsFile(t *testing.T, rawData []byte) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "adc.json")
	require.NoError(t, os.WriteFile(path, rawData, 0o600))
	t.Setenv(ConfigFileEnvVar, path)
}

// withUniverseDomain adds an explicit universe_domain to a credential blob, as
// credentials issued for a sovereign (GCD) universe carry.
func withUniverseDomain(t *testing.T, rawData []byte, universeDomain string) []byte {
	t.Helper()

	fields := map[string]any{}
	require.NoError(t, json.Unmarshal(rawData, &fields))
	fields["universe_domain"] = universeDomain

	data, err := json.Marshal(fields)
	require.NoError(t, err)
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
				require.NoError(t, corev1.AddToScheme(scheme))

				return &infrav1.ObjectReference{Name: "missing", Namespace: corev1.NamespaceDefault},
					fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsRef, crClient := tt.setup(t)

			opts, universeDomain, err := defaultClientOptions(t.Context(), credentialsRef, crClient)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, universeDomain)
				assert.Nil(t, opts)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantUniverseDomain, universeDomain)
			assert.NotEmpty(t, opts)
		})
	}
}

func TestTagBindingsEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		location       string
		universeDomain string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tagBindingsEndpoint(tt.location, tt.universeDomain))
		})
	}

	// Guards the property that actually matters for existing users: in the
	// default universe this must stay byte-identical to the endpoint the
	// previously hardcoded expression produced, so only GCD behaviour changes.
	for _, location := range []string{"us-central1", "europe-west1", "us-central1-a"} {
		t.Run("unchanged in the default universe/"+location, func(t *testing.T) {
			assert.Equal(t, location+"-cloudresourcemanager.googleapis.com:443",
				tagBindingsEndpoint(location, DefaultUniverseDomain))
		})
	}
}

func TestNewTagBindingsClient(t *testing.T) {
	tests := []struct {
		name      string
		rawData   func(t *testing.T) []byte
		endpoints *infrav1.ServiceEndpoints
	}{
		{
			name:    "default universe domain",
			rawData: func(t *testing.T) []byte { return serviceAccountJSON(t, "") },
		},
		{
			name: "sovereign universe domain",
			rawData: func(t *testing.T) []byte {
				return withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com")
			},
		},
		{
			name: "explicit service endpoint override",
			rawData: func(t *testing.T) []byte {
				return withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com")
			},
			endpoints: &infrav1.ServiceEndpoints{ResourceManagerServiceEndpoint: "cloudresourcemanager.gcd.example.com:443"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsRef, crClient := credentialsSecretFor(t, tt.rawData(t))

			tagBindingsClient, err := newTagBindingsClient(t.Context(), credentialsRef, crClient, "us-central1", tt.endpoints)
			require.NoError(t, err)
			require.NotNil(t, tagBindingsClient)
			t.Cleanup(func() { _ = tagBindingsClient.Close() })
		})
	}
}

func TestNewComputeServiceLogsUniverseDomain(t *testing.T) {
	tests := []struct {
		name       string
		rawData    func(t *testing.T) []byte
		wantLogged bool
	}{
		{
			name:    "default universe domain",
			rawData: func(t *testing.T) []byte { return serviceAccountJSON(t, "") },
		},
		{
			name: "sovereign universe domain",
			rawData: func(t *testing.T) []byte {
				return withUniverseDomain(t, serviceAccountJSON(t, ""), "gcd.example.com")
			},
			wantLogged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsRef, crClient := credentialsSecretFor(t, tt.rawData(t))

			var logged strings.Builder
			logger := funcr.New(func(_, args string) { logged.WriteString(args + "\n") }, funcr.Options{})
			ctx := log.IntoContext(t.Context(), logger)

			_, err := newComputeService(ctx, credentialsRef, crClient, nil)
			require.NoError(t, err)

			if tt.wantLogged {
				assert.Contains(t, logged.String(), "Operating in a non-default universe domain")
				assert.Contains(t, logged.String(), "gcd.example.com")
			} else {
				assert.Empty(t, logged.String())
			}
		})
	}
}
