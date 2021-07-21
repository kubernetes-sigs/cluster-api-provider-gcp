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

package compute

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/record"

	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/gcperrors"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/wait"
)

// InstanceIfExists returns the existing instance or nothing if it doesn't exist.
func (s *Service) InstanceIfExists(scope *scope.MachineScope) (*compute.Instance, error) {
	log := s.scope.Logger.WithValues("instance-name", scope.Name())
	log.V(2).Info("Looking for instance by name")

	res, err := s.instances.Get(s.scope.Project(), scope.Zone(), scope.Name()).Do()
	switch {
	case gcperrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, errors.Wrapf(err, "failed to describe instance: %q", scope.Name())
	}

	return res, nil
}

func diskTypeURL(zone string, dt infrav1.DiskType) string {
	return fmt.Sprintf("zones/%s/diskTypes/%s", zone, dt)
}

// CreateInstance runs a GCE instance.
func (s *Service) CreateInstance(scope *scope.MachineScope) (*compute.Instance, error) {
	log := s.scope.Logger.WithValues("machine-role", scope.Role())
	log.V(2).Info("Creating an instance")

	bootstrapData, err := scope.GetBootstrapData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	sourceImage, err := s.rootDiskImage(scope)
	if err != nil {
		return nil, err
	}

	input := &compute.Instance{
		Name:         scope.Name(),
		Zone:         scope.Zone(),
		MachineType:  fmt.Sprintf("zones/%s/machineTypes/%s", scope.Zone(), scope.GCPMachine.Spec.InstanceType),
		CanIpForward: true,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Network: s.scope.NetworkSelfLink(),
		}},
		Tags: &compute.Tags{
			Items: append(
				scope.GCPMachine.Spec.AdditionalNetworkTags,
				fmt.Sprintf("%s-%s", scope.Cluster.Name, scope.Role()),
				scope.Cluster.Name,
			),
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskSizeGb:  30,
					DiskType:    diskTypeURL(scope.Zone(), infrav1.PdStandardDiskType),
					SourceImage: sourceImage,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "user-data",
					Value: pointer.StringPtr(bootstrapData),
				},
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.CloudPlatformScope,
				},
			},
		},
	}

	if scope.GCPMachine.Spec.ServiceAccount != nil {
		serviceAccount := scope.GCPMachine.Spec.ServiceAccount
		input.ServiceAccounts = []*compute.ServiceAccount{
			{
				Email:  serviceAccount.Email,
				Scopes: serviceAccount.Scopes,
			},
		}
	}

	input.Labels = infrav1.Build(infrav1.BuildParams{
		ClusterName: s.scope.Name(),
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Role:        pointer.StringPtr(scope.Role()),
		// TODO(vincepri): Check what needs to be added for the cloud provider label.
		Additional: s.scope.
			GCPCluster.Spec.
			AdditionalLabels.
			AddLabels(scope.GCPMachine.Spec.AdditionalLabels),
	})

	if scope.GCPMachine.Spec.PublicIP != nil && *scope.GCPMachine.Spec.PublicIP {
		input.NetworkInterfaces[0].AccessConfigs = []*compute.AccessConfig{
			{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		}
	}

	if scope.GCPMachine.Spec.RootDeviceSize > 0 {
		input.Disks[0].InitializeParams.DiskSizeGb = scope.GCPMachine.Spec.RootDeviceSize
	}
	if scope.GCPMachine.Spec.RootDeviceType != nil {
		input.Disks[0].InitializeParams.DiskType = diskTypeURL(scope.Zone(), *scope.GCPMachine.Spec.RootDeviceType)
	}

	if scope.GCPMachine.Spec.Subnet != nil {
		input.NetworkInterfaces[0].Subnetwork = fmt.Sprintf("regions/%s/subnetworks/%s",
			scope.Region(), *scope.GCPMachine.Spec.Subnet)
	}

	if s.scope.Network().APIServerAddress == nil {
		return nil, errors.New("failed to run controlplane, APIServer address not available")
	}

	log.Info("Running instance")
	out, err := s.runInstance(input)
	if err != nil {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance: %v", err)

		return nil, err
	}

	record.Eventf(scope.Machine, "SuccessfulCreate", "Created new %s instance with name %q", scope.Role(), out.Name)

	return out, nil
}

func (s *Service) runInstance(input *compute.Instance) (*compute.Instance, error) {
	op, err := s.instances.Insert(s.scope.Project(), input.Zone, input).Do()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gcp instance")
	}

	if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
		return nil, errors.Wrap(err, "failed to create gcp instance")
	}

	return s.instances.Get(s.scope.Project(), input.Zone, input.Name).Do()
}

func (s *Service) TerminateInstanceAndWait(scope *scope.MachineScope) error {
	op, err := s.instances.Delete(s.scope.Project(), scope.Zone(), scope.Name()).Do()
	if err != nil {
		return errors.Wrap(err, "failed to terminate gcp instance")
	}

	if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
		return errors.Wrap(err, "failed to terminate gcp instance")
	}

	return nil
}

// rootDiskImage computes the GCE disk image to use as the boot disk.
func (s *Service) rootDiskImage(scope *scope.MachineScope) (string, error) {
	if scope.GCPMachine.Spec.Image != nil {
		return *scope.GCPMachine.Spec.Image, nil
	} else if scope.GCPMachine.Spec.ImageFamily != nil {
		return *scope.GCPMachine.Spec.ImageFamily, nil
	}

	if scope.Machine.Spec.Version == nil {
		return "", errors.Errorf("missing required Spec.Version on Machine %q in namespace %q",
			scope.Name(), scope.Namespace())
	}

	version, err := semver.ParseTolerant(*scope.Machine.Spec.Version)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing Spec.Version on Machine %q in namespace %q, expected valid SemVer string",
			scope.Name(), scope.Namespace())
	}

	image := fmt.Sprintf(
		"projects/%s/global/images/family/capi-ubuntu-1804-k8s-v%d-%d",
		s.scope.Project(), version.Major, 19)

	return image, nil
}
