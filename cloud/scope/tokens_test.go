/*
Copyright 2025 The Kubernetes Authors.

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// serviceAccountJSON builds a syntactically valid service-account credential
// blob, complete with a freshly generated (test-only) RSA key, since
// google.CredentialsFromJSON parses and validates the private key eagerly.
// tokenURI defaults to the real Google endpoint when empty; tests that need
// to intercept token refresh calls can point it at a local test server.
func serviceAccountJSON(t *testing.T, tokenURI string) []byte {
	t.Helper()

	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token" //nolint:gosec // public, well-known endpoint, not a credential
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	data, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "test-project",
		"private_key":  string(keyPEM),
		"client_email": "sa@test-project.iam.gserviceaccount.com",
		"token_uri":    tokenURI,
	})
	require.NoError(t, err)
	return data
}

func TestNewTokenClient(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*infrav1.ObjectReference, client.Client)
		wantErr bool
	}{
		{
			name: "implicit ADC",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				t.Setenv(ConfigFileEnvVar, "")
				// Isolate from any real gcloud ADC file on the host running the test.
				t.Setenv("CLOUDSDK_CONFIG", t.TempDir())

				// nil credentialsRef → implicit ADC (including WIF via the GKE
				// metadata server). google.FindDefaultCredentials only succeeds
				// here if it can detect a metadata server, so fake one up.
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Metadata-Flavor", "Google")
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)
				t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))

				return nil, nil
			},
		},
		{
			name: "explicit credentials",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				credSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "gcp-creds", Namespace: corev1.NamespaceDefault},
					Data:       map[string][]byte{"credentials": serviceAccountJSON(t, "")},
				}
				scheme := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(scheme))
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credSecret).Build()

				return &infrav1.ObjectReference{Name: "gcp-creds", Namespace: corev1.NamespaceDefault}, fakeClient
			},
		},
		{
			name: "explicit credentials, secret not found",
			setup: func(t *testing.T) (*infrav1.ObjectReference, client.Client) {
				scheme := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(scheme))
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

				return &infrav1.ObjectReference{Name: "missing", Namespace: corev1.NamespaceDefault}, fakeClient
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsRef, crClient := tt.setup(t)

			tc, err := NewTokenClient(t.Context(), credentialsRef, crClient)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, tc.source)
		})
	}
}

func TestNewTokenClientWithTimeout_TokenRefreshIsBounded(t *testing.T) {
	// Regression test: oauth2.TokenSource.Token() takes no context of its
	// own, so a stalled token-endpoint request must not be able to hang the
	// calling reconcile indefinitely (this is what was blocking the GKE
	// e2e cluster deletion/finalizer path from ever completing).
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // simulate a request that never gets a response
	}))
	// Deliberately not calling srv.Close(): it waits for in-flight handlers
	// to return, but this one is designed to never return on its own. The
	// listener is cleaned up when the test binary exits.

	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "gcp-creds", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"credentials": serviceAccountJSON(t, srv.URL)},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credSecret).Build()

	tc, err := NewTokenClientWithTimeout(t.Context(), &infrav1.ObjectReference{Name: "gcp-creds", Namespace: corev1.NamespaceDefault}, fakeClient, 200*time.Millisecond)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		_, _ = tc.Token() // only timing matters here; explicit double-blank already satisfies errcheck
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Token() did not return within a generous margin of the configured timeout - it's hanging")
	}
}
