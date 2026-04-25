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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCredentialDataUsingADC_EnvVarNotSet(t *testing.T) {
	t.Setenv(ConfigFileEnvVar, "")
	data, err := getCredentialDataUsingADC()
	assert.Nil(t, err)
	assert.Nil(t, data)
}

func TestGetCredentialDataUsingADC_ValidFile(t *testing.T) {
	content := []byte(`{"type":"service_account","project_id":"test-project"}`)
	f, err := os.CreateTemp(t.TempDir(), "creds-*.json")
	require.NoError(t, err)
	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	t.Setenv(ConfigFileEnvVar, f.Name())
	data, err := getCredentialDataUsingADC()
	assert.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestGetCredentialDataUsingADC_MissingFile(t *testing.T) {
	t.Setenv(ConfigFileEnvVar, "/nonexistent/path/credentials.json")
	data, err := getCredentialDataUsingADC()
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestGetCredentials_WIFMode(t *testing.T) {
	t.Setenv(ConfigFileEnvVar, "")
	// nil credentialsRef + no env var → WIF/implicit ADC mode: both nil
	cred, err := getCredentials(t.Context(), nil, nil)
	assert.Nil(t, err)
	assert.Nil(t, cred)
}
