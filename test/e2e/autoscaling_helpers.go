//go:build e2e
// +build e2e

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

package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed data/scale-from-zero/rbac.yaml
var clusterAutoscalerRBAC string

//go:embed data/scale-from-zero/deployment.yaml.tmpl
var clusterAutoscalerDeploymentTemplate string

//go:embed data/scale-from-zero/autoscale-trigger-deployment.yaml.tmpl
var autoscaleTriggerDeploymentTemplate string

// renderTemplate executes a template with the given data and returns the rendered bytes.
func renderTemplate(name, templateStr string, data interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeYAML decodes a single YAML document into a runtime.Object.
func decodeYAML(data []byte) (runtime.Object, error) {
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}
	return obj, nil
}

// decodeUnstructuredYAML decodes a single YAML document into an unstructured object.
// Use for CRDs not registered in the standard k8s scheme.
func decodeUnstructuredYAML(data []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := dec.Decode(data, nil, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to decode unstructured YAML: %w", err)
	}
	return obj, nil
}

// decodeMultiYAML decodes multiple YAML documents separated by "---".
func decodeMultiYAML(data []byte) ([]runtime.Object, error) {
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	objects := []runtime.Object{}

	docs := bytes.Split(data, []byte("\n---\n"))
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decode YAML document: %w", err)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}

// deployClusterAutoscalerRBAC deploys Cluster Autoscaler RBAC resources to the management cluster.
func deployClusterAutoscalerRBAC(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	rendered, err := renderTemplate("cluster-autoscaler-rbac", clusterAutoscalerRBAC, map[string]string{
		"Namespace":   namespace,
		"ClusterName": clusterName,
	})
	if err != nil {
		return err
	}

	objects, err := decodeMultiYAML(rendered)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		clientObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("object is not a client.Object: %T", obj)
		}

		err := mgmtClient.Create(ctx, clientObj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %s %s: %w",
				obj.GetObjectKind().GroupVersionKind().Kind, clientObj.GetName(), err)
		}
	}

	return nil
}

// deployClusterAutoscaler deploys the Cluster Autoscaler deployment to the management cluster.
func deployClusterAutoscaler(ctx context.Context, mgmtClient client.Client, namespace, clusterName, clusterAutoscalerVersion string) error {
	rendered, err := renderTemplate("cluster-autoscaler", clusterAutoscalerDeploymentTemplate, map[string]string{
		"Namespace":                namespace,
		"ClusterName":              clusterName,
		"ClusterAutoscalerVersion": clusterAutoscalerVersion,
	})
	if err != nil {
		return err
	}

	obj, err := decodeYAML(rendered)
	if err != nil {
		return err
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("decoded object is not a Deployment")
	}

	err = mgmtClient.Create(ctx, deployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Cluster Autoscaler deployment %s: %w", deployment.Name, err)
	}

	return nil
}

// createTriggerWorkload creates a Deployment in the workload cluster that requires
// nodes with the autoscale-group=from-zero label to trigger autoscaler scale-up.
func createTriggerWorkload(ctx context.Context, workloadClient client.Client, clusterName string) (client.Object, error) {
	rendered, err := renderTemplate("trigger-deployment", autoscaleTriggerDeploymentTemplate, map[string]string{
		"ClusterName": clusterName,
	})
	if err != nil {
		return nil, err
	}

	obj, err := decodeYAML(rendered)
	if err != nil {
		return nil, err
	}

	deployment, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("decoded object is not a client.Object")
	}

	err = workloadClient.Create(ctx, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to create trigger deployment: %w", err)
	}

	return deployment, nil
}
