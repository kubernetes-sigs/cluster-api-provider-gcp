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

	"k8s.io/klog/v2"
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
	klog.V(4).InfoS("PatchKubeadmTimeout: called", "dataLen", len(bootstrapData), "first80", truncate(strings.TrimSpace(bootstrapData), 80))
	if !strings.HasPrefix(strings.TrimSpace(bootstrapData), cloudConfigHeader) {
		klog.V(4).Info("PatchKubeadmTimeout: not cloud-config, skipping")
		return bootstrapData, nil
	}

	body := strings.TrimPrefix(strings.TrimSpace(bootstrapData), cloudConfigHeader)
	klog.V(4).InfoS("PatchKubeadmTimeout: cloud-config body extracted", "bodyLen", len(body))

	var cc map[string]interface{}
	if err := yaml.Unmarshal([]byte(body), &cc); err != nil {
		klog.V(4).InfoS("PatchKubeadmTimeout: failed to parse cloud-config body, skipping", "err", err)
		return bootstrapData, nil
	}

	klog.V(4).InfoS("PatchKubeadmTimeout: cloud-config parsed", "topLevelKeys", mapKeys(cc))

	rawFiles, ok := cc["write_files"]
	if !ok {
		klog.V(4).Info("PatchKubeadmTimeout: no write_files in cloud-config, skipping")
		return bootstrapData, nil
	}

	files, ok := rawFiles.([]interface{})
	if !ok {
		klog.V(4).Info("PatchKubeadmTimeout: write_files is not a list, skipping")
		return bootstrapData, nil
	}

	var paths []string
	idx := -1
	for i, f := range files {
		entry, ok := f.(map[string]interface{})
		if !ok {
			klog.V(4).InfoS("PatchKubeadmTimeout: write_files entry is not a map", "index", i, "type", fmt.Sprintf("%T", f))
			continue
		}
		p, _ := entry["path"].(string)
		enc, _ := entry["encoding"].(string)
		contentLen := 0
		if c, ok := entry["content"].(string); ok {
			contentLen = len(c)
		}
		paths = append(paths, p)
		klog.V(4).InfoS("PatchKubeadmTimeout: write_files entry", "index", i, "path", p, "encoding", enc, "contentLen", contentLen, "keys", mapKeys(entry))
		if p == kubeadmConfigPath {
			idx = i
		}
	}
	klog.V(4).InfoS("PatchKubeadmTimeout: write_files summary", "totalEntries", len(files), "paths", paths, "kubeadmIdx", idx)
	if idx < 0 {
		return bootstrapData, nil
	}

	entry := files[idx].(map[string]interface{})
	content, _ := entry["content"].(string)
	encoding, _ := entry["encoding"].(string)
	klog.V(4).InfoS("PatchKubeadmTimeout: kubeadm entry found", "encoding", encoding, "contentLen", len(content))

	decoded, err := decodeContent(content, encoding)
	if err != nil {
		klog.V(2).InfoS("PatchKubeadmTimeout: failed to decode content, skipping", "encoding", encoding, "err", err)
		return bootstrapData, nil
	}
	klog.V(4).InfoS("PatchKubeadmTimeout: decoded kubeadm config", "content", decoded)

	patched, changed, err := patchKubeadmConfig(decoded)
	if err != nil || !changed {
		klog.V(4).InfoS("PatchKubeadmTimeout: patchKubeadmConfig result", "changed", changed, "err", err)
		return bootstrapData, err
	}

	klog.V(2).InfoS("PatchKubeadmTimeout: kubeadm config patched", "patchedContent", patched)

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

	result := cloudConfigHeader + "\n" + string(out)
	klog.V(4).InfoS("PatchKubeadmTimeout: final cloud-config produced", "resultLen", len(result))
	klog.V(4).InfoS("PatchKubeadmTimeout: final cloud-config content", "result", truncate(result, 2000))
	return result, nil
}

// patchKubeadmConfig parses a kubeadm config YAML (potentially multi-document),
// looks for a v1beta4 document, and sets timeouts.kubernetesAPICall.
// Returns the patched YAML, whether anything changed, and any error.
func patchKubeadmConfig(raw string) (string, bool, error) {
	docs := splitYAMLDocuments(raw)
	klog.V(4).InfoS("patchKubeadmConfig: split documents", "docCount", len(docs))
	changed := false

	for i, doc := range docs {
		var m map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			klog.V(4).InfoS("patchKubeadmConfig: failed to unmarshal document, skipping", "docIndex", i, "err", err, "docSnippet", truncate(doc, 200))
			continue
		}

		apiVersion, _ := m["apiVersion"].(string)
		kind, _ := m["kind"].(string)
		klog.V(4).InfoS("patchKubeadmConfig: inspecting document", "docIndex", i, "apiVersion", apiVersion, "kind", kind)

		if apiVersion != kubeadmV1Beta4API {
			continue
		}

		timeouts, ok := m["timeouts"].(map[string]interface{})
		if !ok {
			klog.V(4).InfoS("patchKubeadmConfig: no existing timeouts, creating", "docIndex", i, "kind", kind)
			timeouts = make(map[string]interface{})
			m["timeouts"] = timeouts
		} else {
			klog.V(4).InfoS("patchKubeadmConfig: existing timeouts found", "docIndex", i, "kind", kind, "timeouts", timeouts)
		}

		if timeouts["kubernetesAPICall"] == kubeadmAPICallTimeout {
			klog.V(4).InfoS("patchKubeadmConfig: timeout already set, skipping", "docIndex", i, "kind", kind)
			continue
		}

		klog.V(4).InfoS("patchKubeadmConfig: setting timeout", "docIndex", i, "kind", kind, "oldValue", timeouts["kubernetesAPICall"], "newValue", kubeadmAPICallTimeout)
		timeouts["kubernetesAPICall"] = kubeadmAPICallTimeout
		changed = true

		out, err := yaml.Marshal(m)
		if err != nil {
			return "", false, fmt.Errorf("re-serializing kubeadm document: %w", err)
		}
		klog.V(4).InfoS("patchKubeadmConfig: re-serialized document", "docIndex", i, "kind", kind, "yaml", string(out))
		docs[i] = string(out)
	}

	if !changed {
		klog.V(4).Info("patchKubeadmConfig: no v1beta4 documents needed patching")
		return raw, false, nil
	}

	result := joinYAMLDocuments(docs)
	klog.V(4).InfoS("patchKubeadmConfig: final joined result", "result", result)
	return result, true, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
