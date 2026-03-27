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

// Package cloudinit provides utilities for inspecting and patching cloud-init
// bootstrap data used by the GCP infrastructure provider.
package cloudinit

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	cloudConfigHeader       = "#cloud-config"
	kubeadmConfigPath       = "/run/kubeadm/kubeadm.yaml"
	kubeadmAPIGroupPrefix   = "kubeadm.k8s.io/"
	minPatchableBetaVersion = 4 // v1beta4 introduced the timeouts field
)

// PatchKubeadmTimeout inspects cloud-init bootstrap data for a kubeadm config
// (v1beta4 or later) written to /run/kubeadm/kubeadm.yaml and ensures
// timeouts.kubernetesAPICall is at least minTimeout so that kubeadm init has
// enough time to contact the API server through load balancers that may take
// time to converge. If the existing value is already equal or higher it is
// left untouched.
//
// minTimeout should be a Go duration string (e.g. "15m0s").
//
// If the data is not cloud-config, predates v1beta4, or doesn't contain the
// expected write_files entry the original data is returned unmodified.
func PatchKubeadmTimeout(bootstrapData, minTimeout string) (string, error) {
	if minTimeout == "" {
		return bootstrapData, nil
	}

	d, err := time.ParseDuration(minTimeout)
	if err != nil {
		return "", fmt.Errorf("invalid minTimeout %q: %w", minTimeout, err)
	}
	if d <= 0 {
		return "", fmt.Errorf("minTimeout must be positive, got %q", minTimeout)
	}

	// Split into preamble (e.g. "## template: jinja\n#cloud-config") and
	// the YAML body so we can reassemble the original format after patching.
	preamble, body, ok := extractCloudConfigBody(bootstrapData)
	if !ok {
		klog.V(4).Info("PatchKubeadmTimeout: not cloud-config, skipping")
		return bootstrapData, nil
	}

	// Cheap string check before expensive YAML parsing: if the path doesn't
	// appear in the raw body at all, there can't be a matching write_files entry.
	if !strings.Contains(body, kubeadmConfigPath) {
		klog.V(4).Info("PatchKubeadmTimeout: no reference to kubeadm.yaml in body, skipping")
		return bootstrapData, nil
	}

	// We unmarshal into a generic map to preserve all cloud-config directives
	// (runcmd, users, ntp, etc.) during the round-trip.
	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		klog.V(4).InfoS("PatchKubeadmTimeout: cloud-config body is not valid YAML, skipping", "err", err)
		return bootstrapData, nil
	}

	// Find the kubeadm.yaml write_files entry.
	// The write_files entry is a list of maps, each representing a file.
	files, _ := cc["write_files"].([]interface{})
	idx := findWriteFileEntry(files, kubeadmConfigPath)
	if idx < 0 {
		klog.V(4).Info("PatchKubeadmTimeout: no kubeadm.yaml write_files entry, skipping")
		return bootstrapData, nil
	}

	// Safe assertion: findWriteFileEntry only returns an index for entries
	// that already passed this type check.
	entry := files[idx].(map[string]interface{})
	content, _ := entry["content"].(string)
	encoding, _ := entry["encoding"].(string)

	decoded, err := decodeContent(content, encoding)
	if err != nil {
		klog.V(2).InfoS("PatchKubeadmTimeout: unable to decode content, skipping", "encoding", encoding, "err", err)
		return bootstrapData, nil
	}

	// Quick pre-check on decoded content before splitting into YAML documents.
	if !strings.Contains(decoded, kubeadmAPIGroupPrefix) {
		klog.V(4).Info("PatchKubeadmTimeout: kubeadm.yaml content has no kubeadm API version, skipping")
		return bootstrapData, nil
	}

	// Patch the kubeadm config with the minimum timeout.
	patched, changed, err := patchKubeadmConfig(decoded, minTimeout, d)
	if err != nil || !changed {
		return bootstrapData, err
	}

	klog.V(4).InfoS("PatchKubeadmTimeout: patched kubeadm timeouts.kubernetesAPICall", "minTimeout", minTimeout)

	// Re-encode the patched content with its original encoding and
	// reassemble the full cloud-init blob: preamble + patched YAML body.
	encoded, err := encodeContent(patched, encoding)
	if err != nil {
		return "", fmt.Errorf("re-encoding write_files content: %w", err)
	}
	entry["content"] = encoded
	files[idx] = entry
	cc["write_files"] = files

	// Re-serialize the cloud-config with the patched kubeadm config.
	out, err := yaml.Marshal(cc)
	if err != nil {
		return "", fmt.Errorf("re-serializing cloud-config: %w", err)
	}

	return preamble + "\n" + string(out), nil
}

// kubeadmSupportsTimeouts returns true if apiVersion is a kubeadm API version
// that supports the timeouts field. The field was introduced in v1beta4, so any
// v1betaN where N >= 4 qualifies, as do future major versions (v1, v2, etc.).
func kubeadmSupportsTimeouts(apiVersion string) bool {
	if !strings.HasPrefix(apiVersion, kubeadmAPIGroupPrefix) {
		return false
	}
	ver := strings.TrimPrefix(apiVersion, kubeadmAPIGroupPrefix)

	if strings.HasPrefix(ver, "v1beta") {
		n, err := strconv.Atoi(strings.TrimPrefix(ver, "v1beta"))
		return err == nil && n >= minPatchableBetaVersion
	}

	// GA versions (v1, v2, …) will also carry timeouts.
	if strings.HasPrefix(ver, "v") {
		_, err := strconv.Atoi(strings.TrimPrefix(ver, "v"))
		return err == nil
	}
	return false
}

// patchKubeadmConfig patches all patchable kubeadm documents (v1beta4+) in a
// (potentially multi-document) kubeadm config YAML, setting
// timeouts.kubernetesAPICall to the given minimum timeout. If the existing
// value is already equal or higher, it is left untouched.
func patchKubeadmConfig(raw, minTimeout string, minDuration time.Duration) (string, bool, error) {
	// kubeadm configs are multi-document YAML (e.g. InitConfiguration +
	// ClusterConfiguration); each document is inspected independently.
	docs := splitYAMLDocuments(raw)
	changed := false

	for i, doc := range docs {
		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			continue
		}

		apiVer, _ := m["apiVersion"].(string)
		if !kubeadmSupportsTimeouts(apiVer) {
			continue
		}

		// Create the timeouts map if it doesn't exist yet, which is the
		// common case for freshly generated kubeadm configs.
		timeouts, ok := m["timeouts"].(map[string]interface{})
		if !ok {
			timeouts = make(map[string]interface{})
			m["timeouts"] = timeouts
		}

		// Only raise the timeout; never lower a value that is already sufficient.
		if existing, ok := timeouts["kubernetesAPICall"].(string); ok {
			if d, err := time.ParseDuration(existing); err == nil && d >= minDuration {
				continue
			}
		}

		timeouts["kubernetesAPICall"] = minTimeout
		changed = true

		// Re-serialize the kubeadm document with the patched timeouts.
		out, err := yaml.Marshal(m)
		if err != nil {
			return "", false, fmt.Errorf("re-serializing kubeadm document: %w", err)
		}
		docs[i] = string(out)
	}

	if !changed {
		return raw, false, nil
	}
	return joinYAMLDocuments(docs), true, nil
}

// extractCloudConfigBody finds the #cloud-config header in the bootstrap data,
// allowing for preamble lines like "## template: jinja" that the CAPI kubeadm
// bootstrap provider always prepends. Returns the full preamble (up to and
// including the #cloud-config line), the body after it, and whether it was found.
func extractCloudConfigBody(data string) (preamble, body string, ok bool) {
	trimmed := strings.TrimSpace(data)
	lines := strings.Split(trimmed, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == cloudConfigHeader {
			return strings.Join(lines[:i+1], "\n"),
				strings.Join(lines[i+1:], "\n"),
				true
		}
		// Allow "## template: jinja" and similar cloud-init preamble directives.
		if s := strings.TrimSpace(line); s != "" && !strings.HasPrefix(s, "##") {
			return "", "", false
		}
	}
	return "", "", false
}

// findWriteFileEntry returns the index of the write_files entry whose path
// field matches, or -1 if not found.
func findWriteFileEntry(files []interface{}, path string) int {
	for i, f := range files {
		entry, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if p, _ := entry["path"].(string); p == path {
			return i
		}
	}
	return -1
}

// splitYAMLDocuments splits a multi-document YAML string on "---" separators,
// dropping empty segments.
func splitYAMLDocuments(raw string) []string {
	parts := strings.Split(raw, "\n---")
	docs := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			docs = append(docs, trimmed)
		}
	}
	return docs
}

// joinYAMLDocuments joins a slice of YAML documents back together with "---" separators.
func joinYAMLDocuments(docs []string) string {
	return strings.Join(docs, "\n---\n")
}

// decodeContent handles the cloud-init write_files encoding field.
// The standard CAPI kubeadm bootstrap provider never encodes kubeadm.yaml,
// but we handle it defensively for custom bootstrap providers.
func decodeContent(content, encoding string) (string, error) {
	switch strings.ToLower(encoding) {
	case "", "text/plain":
		return content, nil
	case "base64", "b64":
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", fmt.Errorf("base64 decoding: %w", err)
		}
		return string(decoded), nil
	case "gzip", "gz":
		return gunzipString([]byte(content))
	case "gz+base64", "gzip+base64", "gz+b64", "gzip+b64":
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", fmt.Errorf("base64 decoding: %w", err)
		}
		return gunzipString(decoded)
	default:
		return "", fmt.Errorf("unsupported encoding %q", encoding)
	}
}

// encodeContent is the inverse of decodeContent, re-applying the original
// encoding so the round-tripped write_files entry stays in its original format.
func encodeContent(content, encoding string) (string, error) {
	switch strings.ToLower(encoding) {
	case "", "text/plain":
		return content, nil
	case "base64", "b64":
		return base64.StdEncoding.EncodeToString([]byte(content)), nil
	case "gzip", "gz":
		compressed, err := gzipBytes([]byte(content))
		if err != nil {
			return "", err
		}
		return string(compressed), nil
	case "gz+base64", "gzip+base64", "gz+b64", "gzip+b64":
		compressed, err := gzipBytes([]byte(content))
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(compressed), nil
	default:
		return "", fmt.Errorf("unsupported encoding %q", encoding)
	}
}

// gunzipString decompresses a gzip-compressed string.
func gunzipString(data []byte) (string, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("gzip read: %w", err)
	}
	return string(out), nil
}

// gzipBytes compresses a byte slice using gzip.
func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), nil
}
