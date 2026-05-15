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
	"sync"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GCPLogCollector collects logs from GCP VM instances using gcloud CLI.
// It collects serial port output (always works via GCP API) and optionally
// attempts SSH-based collection for richer logs (kubelet, containerd, cloud-init).
type GCPLogCollector struct{}

func (c GCPLogCollector) CollectMachineLog(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	instanceName, zone, project, err := resolveInstanceInfo(ctx, managementClusterClient, m)
	if err != nil {
		return err
	}

	serialErr := collectSerialLog(ctx, project, zone, instanceName, outputPath)

	sshErr := collectSSHLogs(ctx, project, zone, instanceName, outputPath)
	if sshErr != nil {
		klog.Warningf("SSH-based log collection failed for %s (serial logs may still be available): %v", instanceName, sshErr)
	}

	return serialErr
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

type sshLogSpec struct {
	fileName string
	command  string
}

func collectSSHLogs(ctx context.Context, project, zone, instanceName, outputPath string) error {
	sshKeyFile, err := prepareSSHKeyPair()
	if err != nil {
		return err
	}

	specs := []sshLogSpec{
		{"kubelet.log", "sudo journalctl --no-pager --output=short-precise -u kubelet.service"},
		{"containerd.log", "sudo journalctl --no-pager --output=short-precise -u containerd.service"},
		{"cloud-init.log", "sudo cat /var/log/cloud-init.log"},
		{"cloud-init-output.log", "sudo cat /var/log/cloud-init-output.log"},
	}

	funcs := make([]func() error, 0, len(specs))
	for _, s := range specs {
		funcs = append(funcs, func() error {
			return executeSSHCommand(ctx, project, zone, instanceName, outputPath, sshKeyFile, s.fileName, s.command)
		})
	}

	return aggregateConcurrent(funcs...)
}

func executeSSHCommand(ctx context.Context, project, zone, instanceName, outputPath, sshKeyFile, fileName, command string) error {
	if err := os.MkdirAll(outputPath, 0o750); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(outputPath, fileName)) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.CommandContext(ctx, //nolint:gosec
		"gcloud", "compute", "ssh", "--quiet",
		instanceName,
		"--zone", zone,
		"--project", project,
		"--ssh-key-file", sshKeyFile,
		"--command", command,
		"--",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
	)

	output, runErr := cmd.CombinedOutput()
	if len(output) > 0 {
		if _, writeErr := f.Write(output); writeErr != nil {
			return kerrors.NewAggregate([]error{runErr, writeErr})
		}
	}

	return runErr
}

// prepareSSHKeyPair ensures gcloud compute ssh can find a matching key pair.
// gcloud expects <key-file>.pub alongside the private key, but the test-infra
// preset-k8s-ssh mounts them at separate paths in a read-only secret volume.
// Copy both keys to a writable temp directory so gcloud finds the pair.
func prepareSSHKeyPair() (string, error) {
	privKeyFile := os.Getenv("GCE_SSH_PRIVATE_KEY_FILE")
	if privKeyFile == "" {
		return "", fmt.Errorf("GCE_SSH_PRIVATE_KEY_FILE not set, skipping SSH log collection")
	}

	pubKeyFile := os.Getenv("GCE_SSH_PUBLIC_KEY_FILE")
	if pubKeyFile == "" {
		return "", fmt.Errorf("GCE_SSH_PUBLIC_KEY_FILE not set, cannot create key pair for gcloud")
	}

	privKeyBytes, err := os.ReadFile(privKeyFile) //nolint:gosec
	if err != nil {
		return "", errors.Wrapf(err, "reading private key from %s", privKeyFile)
	}

	pubKeyBytes, err := os.ReadFile(pubKeyFile) //nolint:gosec
	if err != nil {
		return "", errors.Wrapf(err, "reading public key from %s", pubKeyFile)
	}

	tmpDir, err := os.MkdirTemp("", "capg-ssh-*")
	if err != nil {
		return "", errors.Wrap(err, "creating temp directory for SSH keys")
	}

	dst := filepath.Join(tmpDir, "ssh-key")
	if err := os.WriteFile(dst, privKeyBytes, 0o600); err != nil {
		return "", errors.Wrapf(err, "writing private key to %s", dst)
	}
	if err := os.WriteFile(dst+".pub", pubKeyBytes, 0o600); err != nil {
		return "", errors.Wrapf(err, "writing public key to %s", dst+".pub")
	}

	klog.Infof("Prepared SSH key pair at %s for gcloud compatibility", dst)
	return dst, nil
}

func aggregateConcurrent(funcs ...func() error) error {
	ch := make(chan error, len(funcs))
	var wg sync.WaitGroup
	for _, f := range funcs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- f()
		}()
	}
	wg.Wait()
	close(ch)

	var errs []error
	for err := range ch {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}
