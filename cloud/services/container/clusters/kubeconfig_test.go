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

package clusters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialEmailResolver(t *testing.T) {
	r := credentialEmailResolver{email: "sa@my-project.iam.gserviceaccount.com"}
	email, err := r.Email(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sa@my-project.iam.gserviceaccount.com", email)
}

func TestMetadataEmailResolver_WIFMode(t *testing.T) {
	const wifEmail = "wif-sa@my-project.iam.gserviceaccount.com"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/service-accounts/default/email") {
			_, _ = w.Write([]byte(wifEmail))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))

	email, err := metadataEmailResolver{}.Email(context.Background())
	require.NoError(t, err)
	assert.Equal(t, wifEmail, email)
}

func TestMetadataEmailResolver_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))

	_, err := metadataEmailResolver{}.Email(context.Background())
	require.Error(t, err)
}
