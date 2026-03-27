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
			name: "v1beta4 kubeadm config gets timeout patched",
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
			name: "v1beta4 with lower timeout gets raised",
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
			name: "v1beta4 with higher timeout is preserved",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
      timeouts:
        kubernetesAPICall: 30m0s
`,
			wantChanged: false,
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
			name: "jinja preamble before cloud-config gets patched",
			input: `## template: jinja
#cloud-config
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
			name: "v1beta5 (future) kubeadm config also gets patched",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta5
      kind: ClusterConfiguration
      kubernetesVersion: v1.36.0
`,
			wantChanged:       true,
			wantTimeoutInBody: "15m0s",
		},
		{
			name: "GA v1 (future) kubeadm config also gets patched",
			input: `#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1
      kind: ClusterConfiguration
      kubernetesVersion: v1.40.0
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
			result, err := PatchKubeadmTimeout(tt.input, "15m0s")
			if (err != nil) != tt.wantErr {
				t.Fatalf("PatchKubeadmTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantChanged {
				if result != tt.input {
					t.Errorf("expected data to be unchanged, but got diff.\nInput:\n%s\nOutput:\n%s", tt.input, result)
				}
				return
			}

			if !strings.Contains(result, cloudConfigHeader) {
				t.Errorf("expected result to contain #cloud-config header, got: %s", result[:50])
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
	result, err := PatchKubeadmTimeout(input, "15m0s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, body, ok := extractCloudConfigBody(result)
	if !ok {
		t.Fatal("result is not a cloud-config")
	}
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

func TestPatchPreservesJinjaPreamble(t *testing.T) {
	input := `## template: jinja
#cloud-config
write_files:
  - path: /run/kubeadm/kubeadm.yaml
    content: |
      apiVersion: kubeadm.k8s.io/v1beta4
      kind: ClusterConfiguration
      kubernetesVersion: v1.34.0
`
	result, err := PatchKubeadmTimeout(input, "15m0s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(result, "## template: jinja\n#cloud-config\n") {
		t.Errorf("jinja preamble not preserved, result starts with: %q", result[:60])
	}
}

func TestKubeadmSupportsTimeouts(t *testing.T) {
	tests := []struct {
		apiVersion string
		want       bool
	}{
		{"kubeadm.k8s.io/v1beta3", false},
		{"kubeadm.k8s.io/v1beta2", false},
		{"kubeadm.k8s.io/v1beta1", false},
		{"kubeadm.k8s.io/v1beta4", true},
		{"kubeadm.k8s.io/v1beta5", true},
		{"kubeadm.k8s.io/v1beta10", true},
		{"kubeadm.k8s.io/v1", true},
		{"kubeadm.k8s.io/v2", true},
		{"apps/v1", false},
		{"", false},
		{"kubeadm.k8s.io/v1alpha1", false},
	}
	for _, tt := range tests {
		t.Run(tt.apiVersion, func(t *testing.T) {
			if got := kubeadmSupportsTimeouts(tt.apiVersion); got != tt.want {
				t.Errorf("kubeadmSupportsTimeouts(%q) = %v, want %v", tt.apiVersion, got, tt.want)
			}
		})
	}
}

func verifyTimeoutInResult(t *testing.T, result, expectedTimeout string) {
	t.Helper()

	_, body, ok := extractCloudConfigBody(result)
	if !ok {
		t.Fatal("result is not a cloud-config")
	}
	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		t.Fatalf("failed to parse result cloud-config: %v", err)
	}

	files, _ := cc["write_files"].([]interface{})
	idx := findWriteFileEntry(files, kubeadmConfigPath)
	if idx < 0 {
		t.Fatal("no kubeadm.yaml write_files entry in result")
	}

	entry := files[idx].(map[string]interface{})
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
		apiVer, _ := m["apiVersion"].(string)
		if !kubeadmSupportsTimeouts(apiVer) {
			continue
		}
		timeouts, ok := m["timeouts"].(map[string]interface{})
		if !ok {
			t.Errorf("expected timeouts map in %s document, got none", apiVer)
			return
		}
		if got, _ := timeouts["kubernetesAPICall"].(string); got != expectedTimeout {
			t.Errorf("expected kubernetesAPICall = %q, got %q", expectedTimeout, got)
		}
		return
	}
	t.Error("did not find patchable kubeadm document with timeouts in result")
}
