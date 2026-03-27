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

package cloudinit

import (
	"encoding/base64"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestPatchKubeadmTimeout(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantChanged       bool
		wantErr           bool
		wantTimeoutInBody string
	}{
		{
			name:        "not cloud-init data is returned as-is",
			input:       "#!/bin/bash\necho hello",
			wantChanged: false,
		},
		{
			name: "cloud-init without write_files is returned as-is",
			input: `#cloud-config
runcmd:
  - echo hello
`,
			wantChanged: false,
		},
		{
			name: "cloud-init with write_files but no kubeadm path is returned as-is",
			input: `#cloud-config
write_files:
  - path: /etc/some-other-file
    content: |
      foo: bar
`,
			wantChanged: false,
		},
		{
			name: "cloud-init with v1beta3 kubeadm config is returned as-is",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta3
      kind: ClusterConfiguration
      kubernetesVersion: v1.28.0
`,
			wantChanged: false,
		},
		{
			name: "cloud-init with v1beta4 kubeadm config gets timeout patched",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
`,
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
		{
			name: "v1beta4 with existing different timeout gets patched",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
      timeouts:
        kubernetesAPICall: 5m0s
`,
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
		{
			name: "v1beta4 already at 15m0s is idempotent",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
      timeouts:
        kubernetesAPICall: 15m0s
`,
			wantChanged: false,
		},
		{
			name: "multi-document kubeadm config with v1beta4 gets patched",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: InitConfiguration
      nodeRegistration:
        name: node1
      ---
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
`,
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
		{
			name: "base64-encoded v1beta4 content gets patched",
			input: "#cloud-config\nwrite_files:\n  - path: /run/kubeadm/kubeadm.yaml\n    encoding: base64\n    content: " +
				base64.StdEncoding.EncodeToString([]byte("apiVersion: kubeadm.k8s.io/v1beta4\nkind: ClusterConfiguration\nkubernetesVersion: v1.34.0\n")) + "\n",
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
		{
			name: "gzip+base64-encoded v1beta4 content gets patched",
			input: func() string {
				raw := "apiVersion: kubeadm.k8s.io/v1beta4\nkind: ClusterConfiguration\nkubernetesVersion: v1.34.0\n"
				compressed, _ := gzipBytes([]byte(raw))
				encoded := base64.StdEncoding.EncodeToString(compressed)
				return "#cloud-config\nwrite_files:\n  - path: /run/kubeadm/kubeadm.yaml\n    encoding: gzip+base64\n    content: " + encoded + "\n"
			}(),
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PatchKubeadmTimeout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PatchKubeadmTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantChanged {
				if result != tt.input {
					t.Errorf("expected data to be unchanged, but got diff.\nInput:\n%s\nOutput:\n%s", tt.input, result)
				}
				return
			}

			if !strings.HasPrefix(result, "#cloud-config\n") {
				t.Errorf("expected result to start with #cloud-config header, got: %s", result[:50])
			}

			if tt.wantTimeoutInBody != "" {
				verifyTimeoutInResult(t, result, tt.wantTimeoutInBody)
			}
		})
	}
}

func TestPatchPreservesOtherFields(t *testing.T) {
	input := `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
  - path: /etc/some-other-file
    content: |
      hello: world
runcmd:
  - /tmp/bootstrap.sh
users:
  - name: root
ntp:
  enabled: true
`
	result, err := PatchKubeadmTimeout(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := strings.TrimPrefix(strings.TrimSpace(result), "#cloud-config")
	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	for _, key := range []string{"write_files", "runcmd", "users", "ntp"} {
		if _, ok := cc[key]; !ok {
			t.Errorf("expected key %q to be preserved in output, but it was missing", key)
		}
	}

	files, ok := cc["write_files"].([]interface{})
	if !ok {
		t.Fatal("write_files is not a list")
	}
	if len(files) != 2 {
		t.Errorf("expected 2 write_files entries, got %d", len(files))
	}

	runcmd, ok := cc["runcmd"].([]interface{})
	if !ok || len(runcmd) != 1 {
		t.Errorf("expected runcmd with 1 entry, got %v", cc["runcmd"])
	}
}

func verifyTimeoutInResult(t *testing.T, result, expectedTimeout string) {
	t.Helper()

	body := strings.TrimPrefix(strings.TrimSpace(result), "#cloud-config")
	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		t.Fatalf("failed to parse result cloud-config: %v", err)
	}

	rawFiles, ok := cc["write_files"]
	if !ok {
		t.Fatal("no write_files in result")
	}
	files, ok := rawFiles.([]interface{})
	if !ok {
		t.Fatal("write_files is not a list")
	}

	for _, f := range files {
		entry, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		path, _ := entry["path"].(string)
		if path != kubeadmConfigPath {
			continue
		}

		content, _ := entry["content"].(string)
		encoding, _ := entry["encoding"].(string)

		decoded, err := decodeContent(content, encoding)
		if err != nil {
			t.Fatalf("failed to decode content: %v", err)
		}

		docs := splitYAMLDocuments(decoded)
		for _, doc := range docs {
			var m map[string]interface{}
			if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
				continue
			}
			apiVersion, _ := m["apiVersion"].(string)
			if apiVersion != kubeadmV1Beta4API {
				continue
			}
			timeouts, ok := m["timeouts"].(map[string]interface{})
			if !ok {
				t.Errorf("expected timeouts map in v1beta4 document, got none")
				return
			}
			got, _ := timeouts["kubernetesAPICall"].(string)
			if got != expectedTimeout {
				t.Errorf("expected kubernetesAPICall = %q, got %q", expectedTimeout, got)
			}
			return
		}
	}

	t.Error("did not find kubeadm v1beta4 document with timeouts in result")
}
