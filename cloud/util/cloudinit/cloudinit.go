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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	cloudConfigHeader     = "#cloud-config"
	kubeadmConfigPath     = "/run/kubeadm/kubeadm.yaml"
	kubeadmV1Beta4API     = "kubeadm.k8s.io/v1beta4"
	kubeadmAPICallTimeout = "15m0s"
)

// PatchKubeadmTimeout inspects cloud-init bootstrap data for a kubeadm v1beta4
// config written to /run/kubeadm/kubeadm.yaml and sets
// timeouts.kubernetesAPICall to 15m0s. If the data is not cloud-init, not
// v1beta4, or doesn't contain the expected write_files entry, the original
// data is returned unmodified.
func PatchKubeadmTimeout(bootstrapData string) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(bootstrapData), cloudConfigHeader) {
		return bootstrapData, nil
	}

	body := strings.TrimPrefix(strings.TrimSpace(bootstrapData), cloudConfigHeader)

	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		return bootstrapData, nil
	}

	rawFiles, ok := cc["write_files"]
	if !ok {
		return bootstrapData, nil
	}

	files, ok := rawFiles.([]interface{})
	if !ok {
		return bootstrapData, nil
	}

	idx := -1
	for i, f := range files {
		entry, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if path, _ := entry["path"].(string); path == kubeadmConfigPath {
			idx = i
			break
		}
	}
	if idx < 0 {
		return bootstrapData, nil
	}

	entry := files[idx].(map[string]interface{})
	content, _ := entry["content"].(string)
	encoding, _ := entry["encoding"].(string)

	decoded, err := decodeContent(content, encoding)
	if err != nil {
		return bootstrapData, nil
	}

	patched, changed, err := patchKubeadmConfig(decoded)
	if err != nil || !changed {
		return bootstrapData, err
	}

	encoded, err := encodeContent(patched, encoding)
	if err != nil {
		return "", fmt.Errorf("re-encoding write_files content: %w", err)
	}

	entry["content"] = encoded
	files[idx] = entry
	cc["write_files"] = files

	out, err := yaml.Marshal(cc)
	if err != nil {
		return "", fmt.Errorf("re-serializing cloud-config: %w", err)
	}

	return cloudConfigHeader + "\n" + string(out), nil
}

// patchKubeadmConfig parses a kubeadm config YAML (potentially multi-document),
// looks for a v1beta4 document, and sets timeouts.kubernetesAPICall.
// Returns the patched YAML, whether anything changed, and any error.
func patchKubeadmConfig(raw string) (string, bool, error) {
	docs := splitYAMLDocuments(raw)
	changed := false

	for i, doc := range docs {
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
			timeouts = make(map[string]interface{})
			m["timeouts"] = timeouts
		}

		if timeouts["kubernetesAPICall"] == kubeadmAPICallTimeout {
			continue
		}

		timeouts["kubernetesAPICall"] = kubeadmAPICallTimeout
		changed = true

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

func splitYAMLDocuments(raw string) []string {
	parts := strings.Split(raw, "\n---")
	docs := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			docs = append(docs, trimmed)
		}
	}
	return docs
}

func joinYAMLDocuments(docs []string) string {
	return strings.Join(docs, "\n---\n")
}
