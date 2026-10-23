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

	. "github.com/onsi/gomega"
)

func TestCredentialsClient_FromADCFile_EnvVarNotSet(t *testing.T) {
	g := NewWithT(t)
	t.Setenv(ConfigFileEnvVar, "")
	data, err := CredentialsClient{}.fromADCFile()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).To(BeNil())
}

func TestCredentialsClient_FromADCFile_ValidFile(t *testing.T) {
	g := NewWithT(t)
	content := []byte(`{"type":"service_account","project_id":"test-project"}`)
	f, err := os.CreateTemp(t.TempDir(), "creds-*.json")
	g.Expect(err).NotTo(HaveOccurred())
	_, err = f.Write(content)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(f.Close()).To(Succeed())

	t.Setenv(ConfigFileEnvVar, f.Name())
	data, err := CredentialsClient{}.fromADCFile()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).To(Equal(content))
}

func TestCredentialsClient_FromADCFile_MissingFile(t *testing.T) {
	g := NewWithT(t)
	t.Setenv(ConfigFileEnvVar, "/nonexistent/path/credentials.json")
	data, err := CredentialsClient{}.fromADCFile()
	g.Expect(err).To(HaveOccurred())
	g.Expect(data).To(BeNil())
}
