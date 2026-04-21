//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GCPLogCollector collects serial port output from GCP VM instances using gcloud CLI.
type GCPLogCollector struct{}

func (c GCPLogCollector) CollectMachineLog(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	instanceName, zone, project, err := resolveInstanceInfo(ctx, managementClusterClient, m)
	if err != nil {
		return err
	}

	return collectSerialLog(ctx, project, zone, instanceName, outputPath)
}

func (c GCPLogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *clusterv1.MachinePool, _ string) error {
	return nil
}

func (c GCPLogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

// resolveInstanceInfo extracts the GCP instance name, zone, and project for a Machine.
// It tries the ProviderID first, then falls back to looking up GCPMachine/GCPCluster resources.
func resolveInstanceInfo(ctx context.Context, ctrlClient client.Client, m *clusterv1.Machine) (instanceName, zone, project string, err error) {
	if m.Spec.ProviderID != "" {
		p, z, n, parseErr := parseProviderID(m.Spec.ProviderID)
		if parseErr == nil {
			return n, z, p, nil
		}
		klog.Warningf("Failed to parse provider ID %q: %v, falling back to GCPMachine/GCPCluster lookup", m.Spec.ProviderID, parseErr)
	}

	gcpMachine := &infrav1.GCPMachine{}
	key := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
	if getErr := ctrlClient.Get(ctx, key, gcpMachine); getErr != nil {
		return "", "", "", errors.Wrapf(getErr, "getting GCPMachine for machine %s", klog.KObj(m))
	}

	cluster, clusterErr := capiutil.GetClusterFromMetadata(ctx, ctrlClient, m.ObjectMeta)
	if clusterErr != nil {
		return "", "", "", errors.Wrap(clusterErr, "getting cluster from metadata")
	}

	gcpCluster := &infrav1.GCPCluster{}
	clusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}
	if getErr := ctrlClient.Get(ctx, clusterKey, gcpCluster); getErr != nil {
		return "", "", "", errors.Wrapf(getErr, "getting GCPCluster for cluster %s", cluster.Name)
	}

	return gcpMachine.Name, zoneForMachine(m, gcpCluster), gcpCluster.Spec.Project, nil
}

func parseProviderID(providerID string) (project, zone, name string, err error) {
	const prefix = "gce://"
	if !strings.HasPrefix(providerID, prefix) {
		return "", "", "", fmt.Errorf("expected prefix %q, got %q", prefix, providerID)
	}
	parts := strings.Split(strings.TrimPrefix(providerID, prefix), "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("expected format gce://project/zone/name, got %q", providerID)
	}
	return parts[0], parts[1], parts[2], nil
}

// zoneForMachine returns the zone for a machine, falling back to the first
// failure domain from the GCPCluster if the machine doesn't specify one.
func zoneForMachine(m *clusterv1.Machine, gcpCluster *infrav1.GCPCluster) string {
	if m.Spec.FailureDomain != "" {
		return m.Spec.FailureDomain
	}
	zones := make([]string, 0, len(gcpCluster.Status.FailureDomains))
	for name := range gcpCluster.Status.FailureDomains {
		zones = append(zones, name)
	}
	if len(zones) == 0 {
		return ""
	}
	sort.Strings(zones)
	return zones[0]
}

func collectSerialLog(ctx context.Context, project, zone, instanceName, outputPath string) error {
	if err := os.MkdirAll(outputPath, 0o750); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(outputPath, "serial-port.log")) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.CommandContext(ctx, //nolint:gosec
		"gcloud", "compute", "instances", "get-serial-port-output",
		instanceName,
		"--zone", zone,
		"--project", project,
		"--port", "1",
	)
	cmd.Stdout = f
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
