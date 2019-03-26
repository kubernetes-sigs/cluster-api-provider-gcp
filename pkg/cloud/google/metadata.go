/*
Copyright 2018 The Kubernetes Authors.

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

package google

import (
	"bytes"
	"text/template"

	"fmt"

	"sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/google/machinesetup"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type metadataParams struct {
	Token        string
	Cluster      *clusterv1.Cluster
	Machine      *clusterv1.Machine
	DockerImages []string
	Project      string
	Metadata     *[]machinesetup.MetadataItem

	// These fields are set when executing the template if they are necessary.
	PodCIDR        string
	ServiceCIDR    string
	MasterEndpoint string
}

func nodeMetadata(token string, cluster *clusterv1.Cluster, machine *clusterv1.Machine, project string, metadata *[]machinesetup.MetadataItem) (map[string]string, error) {
	if len(cluster.Status.APIEndpoints) == 0 {
		return nil, fmt.Errorf("master endpoint not found in apiEndpoints for cluster %v", cluster)
	}
	params := metadataParams{
		Token:          token,
		Cluster:        cluster,
		Machine:        machine,
		Project:        project,
		Metadata:       metadata,
		PodCIDR:        getSubnet(cluster.Spec.ClusterNetwork.Pods),
		ServiceCIDR:    getSubnet(cluster.Spec.ClusterNetwork.Services),
		MasterEndpoint: getEndpoint(cluster.Status.APIEndpoints[0]),
	}

	nodeMetadata := map[string]string{}
	var buf bytes.Buffer
	if err := nodeEnvironmentVarsTemplate.Execute(&buf, params); err != nil {
		return nil, err
	}
	nodeMetadata["startup-script"] = buf.String()

	for _, item := range *params.Metadata {
		buf.Reset()
		buf.WriteString(item.Value)
		nodeMetadata[item.Name] += buf.String()
	}
	return nodeMetadata, nil
}

func masterMetadata(cluster *clusterv1.Cluster, machine *clusterv1.Machine, project string, metadata *[]machinesetup.MetadataItem) (map[string]string, error) {
	params := metadataParams{
		Cluster:     cluster,
		Machine:     machine,
		Project:     project,
		Metadata:    metadata,
		PodCIDR:     getSubnet(cluster.Spec.ClusterNetwork.Pods),
		ServiceCIDR: getSubnet(cluster.Spec.ClusterNetwork.Services),
	}

	masterMetadata := map[string]string{}
	var buf bytes.Buffer
	if err := masterEnvironmentVarsTemplate.Execute(&buf, params); err != nil {
		return nil, err
	}
	masterMetadata["startup-script"] = buf.String()

	for _, item := range *params.Metadata {
		buf.Reset()
		buf.WriteString(item.Value)
		masterMetadata[item.Name] += buf.String()
	}
	return masterMetadata, nil
}

func getEndpoint(apiEndpoint clusterv1.APIEndpoint) string {
	return fmt.Sprintf("%s:%d", apiEndpoint.Host, apiEndpoint.Port)
}

var (
	masterEnvironmentVarsTemplate *template.Template
	nodeEnvironmentVarsTemplate   *template.Template
)

func init() {
	masterEnvironmentVarsTemplate = template.Must(template.New("masterEnvironmentVars").Parse(masterEnvironmentVars))
	nodeEnvironmentVarsTemplate = template.Must(template.New("nodeEnvironmentVars").Parse(nodeEnvironmentVars))
}

// TODO(kcoronado): replace with actual network and node tag args when they are added into provider spec.
const masterEnvironmentVars = `
#!/bin/bash
KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}
VERSION=v${KUBELET_VERSION}
PORT=443
NAMESPACE={{ .Machine.ObjectMeta.Namespace }}
MACHINE=$NAMESPACE
MACHINE+="/"
MACHINE+={{ .Machine.ObjectMeta.Name }}
CONTROL_PLANE_VERSION={{ .Machine.Spec.Versions.ControlPlane }}
CLUSTER_DNS_DOMAIN={{ .Cluster.Spec.ClusterNetwork.ServiceDomain }}
POD_CIDR={{ .PodCIDR }}
SERVICE_CIDR={{ .ServiceCIDR }}
`

const nodeEnvironmentVars = `
#!/bin/bash
KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}
TOKEN={{ .Token }}
MASTER={{ .MasterEndpoint }}
NAMESPACE={{ .Machine.ObjectMeta.Namespace }}
MACHINE=$NAMESPACE
MACHINE+="/"
MACHINE+={{ .Machine.ObjectMeta.Name }}
CLUSTER_DNS_DOMAIN={{ .Cluster.Spec.ClusterNetwork.ServiceDomain }}
POD_CIDR={{ .PodCIDR }}
SERVICE_CIDR={{ .ServiceCIDR }}
`
