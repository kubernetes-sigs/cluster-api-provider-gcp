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

// Package log provides a log collector for machine and cluster logs.
package log

import (
	"context"
	"io"
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
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineLogCollector implements the ClusterLogCollector interface.
type MachineLogCollector struct{}

var _ framework.ClusterLogCollector = (*MachineLogCollector)(nil)

// CollectMachinePoolLog collects log from machine pools.
func (c *MachineLogCollector) CollectMachinePoolLog(_ context.Context, _ client.Client, _ *clusterv1.MachinePool, _ string) error {
	return nil
}

// CollectMachineLog collects log from machines.
func (c *MachineLogCollector) CollectMachineLog(ctx context.Context, ctrlClient client.Client, m *clusterv1.Machine, outputPath string) error {
	gcpMachine := &infrav1.GCPMachine{}
	if err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}, gcpMachine); err != nil {
		return errors.Wrapf(err, "getting GCPMachine %s", klog.KObj(m))
	}

	cluster, err := util.GetClusterFromMetadata(ctx, ctrlClient, m.ObjectMeta)
	if err != nil {
		return errors.Wrap(err, "failed to get cluster from metadata")
	}

	gcpCluster := &infrav1.GCPCluster{}
	if err := ctrlClient.Get(ctx, client.ObjectKey{Namespace: gcpMachine.Namespace, Name: cluster.Spec.InfrastructureRef.Name}, gcpCluster); err != nil {
		return errors.Wrapf(err, "getting GCPCluster %s", klog.KObj(gcpCluster))
	}

	zone := zoneWithFallback(m, gcpCluster.Status.FailureDomains)

	captureLogs := func(hostFileName, command string) func() error {
		return func() error {
			f, err := createOutputFile(filepath.Join(outputPath, hostFileName))
			if err != nil {
				return err
			}
			defer f.Close()

			if err := executeRemoteCommand(f, gcpMachine.Name, zone, command); err != nil {
				return errors.Wrapf(err, "failed to run command %s for machine %s", command, klog.KObj(m))
			}

			return nil
		}
	}

	return aggregateConcurrent(
		captureLogs("kubelet.log",
			"sudo journalctl --no-pager --output=short-precise -u kubelet.service"),
		captureLogs("containerd.log",
			"sudo journalctl --no-pager --output=short-precise -u containerd.service"),
		captureLogs("cloud-init.log",
			"sudo cat /var/log/cloud-init.log"),
		captureLogs("cloud-init-output.log",
			"sudo cat /var/log/cloud-init-output.log"),
		captureLogs("kubeadm-service.log",
			"sudo cat /var/log/kubeadm-service.log"),
		captureLogs("pod-logs.tar.gz",
			"sudo tar -czf - -C /var/log pods"),
	)
}

// CollectInfrastructureLogs collects log from the infrastructure.
func (c *MachineLogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

func createOutputFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}

	return os.Create(filepath.Clean(path))
}

func executeRemoteCommand(f io.StringWriter, instanceName, zone, command string) error {
	cmd := exec.Command("gcloud", "compute", "ssh", "--zone", zone, "--command", command, instanceName)

	commandString := cmd.Path + " " + strings.Join(cmd.Args, " ")

	cmd.Env = os.Environ()

	errs := []error{}
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		errs = append(errs, err)
	}

	if outputBytes != nil {
		// Always write the output to the file, if any (so we get the error message)
		if _, err := f.WriteString(string(outputBytes)); err != nil {
			errs = append(errs, err)
		}
	}

	if err := kerrors.NewAggregate(errs); err != nil {
		return errors.Wrapf(err, "running command %q", commandString)
	}

	return nil
}

// aggregateConcurrent runs fns concurrently, returning aggregated errors.
func aggregateConcurrent(funcs ...func() error) error {
	// run all fns concurrently
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
	// collect up and return errors
	errs := []error{}
	for err := range ch {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

func zoneWithFallback(machine *clusterv1.Machine, gcpClusterFailureDomains clusterv1beta1.FailureDomains) string {
	if machine.Spec.FailureDomain == "" {
		fd := []string{}
		for failureDomainName := range gcpClusterFailureDomains {
			fd = append(fd, failureDomainName)
		}
		if len(fd) == 0 {
			return ""
		}
		sort.Strings(fd)
		return fd[0]
	}
	return machine.Spec.FailureDomain
}
