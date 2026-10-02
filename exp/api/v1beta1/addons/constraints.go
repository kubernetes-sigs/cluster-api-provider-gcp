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

package addons

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

// CPU architectures a GCE machine family can run.
const (
	ArchitectureAmd64 = "amd64"
	ArchitectureArm64 = "arm64"
)

// armMachineFamilies are the GCE machine families that run on Arm; every other family is x86. Like the
// version floors in Supported, this is a fact about GCP that has to be recorded when an add-on caring
// about it is onboarded, since no field on GCPManagedMachinePool reports a node's architecture.
var armMachineFamilies = []string{"t2a", "c4a"}

// RequiresWorkloadIdentity is shared by every add-on needing Workload Identity, since the requirement
// carries no per-add-on configuration.
var RequiresWorkloadIdentity Constraint = requiresWorkloadIdentity{}

type requiresWorkloadIdentity struct{}

func (requiresWorkloadIdentity) Describe() string {
	return "requires spec.clusterSecurity.workloadIdentityConfig to be set"
}

func (c requiresWorkloadIdentity) CheckSpec(intent Intent) error {
	security := intent.ControlPlane.Spec.ClusterSecurity
	if security == nil || security.WorkloadIdentityConfig == nil {
		return errors.New(c.Describe())
	}
	return nil
}

// DependsOnAddon requires another add-on to be enabled alongside this one. Only dependencies GKE itself
// enforces belong here: an add-on merely pointless without another one should say so in its description,
// so that CAPG doesn't reject a combination GKE accepts.
func DependsOnAddon(key string) Constraint { return dependsOnAddon{key: key} }

type dependsOnAddon struct{ key string }

func (c dependsOnAddon) Describe() string {
	return fmt.Sprintf("requires add-on %q to be enabled as well", c.key)
}

func (c dependsOnAddon) CheckSpec(intent Intent) error {
	if !intent.Config.Enabled(c.key) {
		return errors.New(c.Describe())
	}
	return nil
}

// ReleaseChannelIn requires the cluster to be on one of the given release channels.
func ReleaseChannelIn(channels ...infrav1exp.ReleaseChannel) Constraint {
	return releaseChannelIn{channels: channels}
}

type releaseChannelIn struct{ channels []infrav1exp.ReleaseChannel }

func (c releaseChannelIn) Describe() string {
	names := make([]string, len(c.channels))
	for i, channel := range c.channels {
		names[i] = string(channel)
	}
	return fmt.Sprintf("requires the %s release channel", strings.Join(names, " or "))
}

func (c releaseChannelIn) CheckSpec(intent Intent) error {
	channel := intent.ControlPlane.Spec.ReleaseChannel
	if channel == nil || !slices.Contains(c.channels, *channel) {
		return errors.New(c.Describe())
	}
	return nil
}

// MinimumVersion requires at least the given GKE version, written the way GKE's own documentation writes
// it ("1.28", "1.32.4-gke.1415000").
//
// The floor is parsed here rather than each time the constraint is checked, so that a floor nobody can
// read is a mistake that stops the package loading instead of one that quietly excuses the add-on from
// the requirement it was given.
func MinimumVersion(floor string) Constraint {
	parsed, err := version.ParseGeneric(floor)
	if err != nil {
		panic(fmt.Sprintf("addons: unreadable version floor %q: %v", floor, err))
	}

	build, hasBuild := gkeBuild(floor)
	return minimumVersion{floor: floor, version: parsed, build: build, hasBuild: hasBuild}
}

type minimumVersion struct {
	floor   string
	version *version.Version
	// build is the "-gke.N" the floor pins, if it pins one. ParseGeneric ignores it, and comparing
	// Kubernetes versions alone would make 1.32.4 outrank 1.32.4-gke.1415000.
	build    int
	hasBuild bool
}

func (c minimumVersion) Describe() string {
	return fmt.Sprintf("requires GKE version %s or later", c.floor)
}

func (c minimumVersion) CheckSpec(intent Intent) error {
	current := effectiveVersion(intent.ControlPlane)
	if current == "" || c.met(current) {
		return nil
	}
	return fmt.Errorf("%s, but the cluster is on %s", c.Describe(), current)
}

// met reports whether the GKE version current is at least this floor. A version GKE reports that CAPG
// can't read is treated as meeting it, since refusing to act on a cluster over a version string is worse
// than letting GKE have the final say.
//
// A floor pinning a GKE build, against a version naming none, is also treated as met: that only arises
// from a partially written spec.version, since the version GKE reports always carries its build.
func (c minimumVersion) met(current string) bool {
	currentVersion, err := version.ParseGeneric(current)
	if err != nil {
		return true
	}
	if !currentVersion.EqualTo(c.version) {
		return currentVersion.AtLeast(c.version)
	}

	currentBuild, hasBuild := gkeBuild(current)
	if !hasBuild || !c.hasBuild {
		return true
	}
	return currentBuild >= c.build
}

// effectiveVersion is the version to judge a floor against: the version the user pinned, else the version
// GKE actually runs. The latter is the only source for a release-channel-managed cluster, which has no
// spec.version at all, and is empty until the cluster exists — so a floor goes unchecked on such a
// cluster's first reconcile and is enforced from the next one onwards.
func effectiveVersion(cp *infrav1exp.GCPManagedControlPlane) string {
	if cp.Spec.Version != nil && *cp.Spec.Version != "" {
		return *cp.Spec.Version
	}
	return ptr.Deref(cp.Status.Version, "")
}

// RequiresMachineFamily requires every node pool to use one of the given GCE machine families.
func RequiresMachineFamily(families ...string) Constraint {
	return requiresMachineFamily{families: families}
}

type requiresMachineFamily struct{ families []string }

func (c requiresMachineFamily) Describe() string {
	return fmt.Sprintf("requires node pools to use one of the %s machine families", strings.Join(c.families, ", "))
}

func (c requiresMachineFamily) CheckCluster(snapshot Snapshot) error {
	return eachKnownFamily(snapshot.NodePools, func(pool NodePoolState, family string) error {
		if !slices.Contains(c.families, family) {
			return fmt.Errorf("%s, but node pool %q uses machine type %q", c.Describe(), pool.Name, pool.MachineType)
		}
		return nil
	})
}

// ExcludesMachineFamily rejects node pools using any of the given GCE machine families.
func ExcludesMachineFamily(families ...string) Constraint {
	return excludesMachineFamily{families: families}
}

type excludesMachineFamily struct{ families []string }

func (c excludesMachineFamily) Describe() string {
	return fmt.Sprintf("is not supported on the %s machine families", strings.Join(c.families, ", "))
}

func (c excludesMachineFamily) CheckCluster(snapshot Snapshot) error {
	return eachKnownFamily(snapshot.NodePools, func(pool NodePoolState, family string) error {
		if slices.Contains(c.families, family) {
			return fmt.Errorf("%s, but node pool %q uses machine type %q", c.Describe(), pool.Name, pool.MachineType)
		}
		return nil
	})
}

// RequiresArchitecture requires every node pool to run the given CPU architecture.
func RequiresArchitecture(arch string) Constraint { return requiresArchitecture{arch: arch} }

type requiresArchitecture struct{ arch string }

func (c requiresArchitecture) Describe() string {
	return fmt.Sprintf("requires %s nodes", c.arch)
}

func (c requiresArchitecture) CheckCluster(snapshot Snapshot) error {
	return eachKnownFamily(snapshot.NodePools, func(pool NodePoolState, family string) error {
		if architecture(family) != c.arch {
			return fmt.Errorf("%s, but node pool %q uses machine type %q", c.Describe(), pool.Name, pool.MachineType)
		}
		return nil
	})
}

// ExcludesNodeOS rejects node pools running the given operating system.
func ExcludesNodeOS(os corev1.OSName) Constraint { return excludesNodeOS{os: os} }

type excludesNodeOS struct{ os corev1.OSName }

func (c excludesNodeOS) Describe() string {
	return fmt.Sprintf("is not supported on %s node pools", c.os)
}

func (c excludesNodeOS) CheckCluster(snapshot Snapshot) error {
	for _, pool := range snapshot.NodePools {
		if pool.ImageType == "" {
			continue
		}
		if nodeOS(pool.ImageType) == c.os {
			return fmt.Errorf("%s, but node pool %q uses image type %q", c.Describe(), pool.Name, pool.ImageType)
		}
	}
	return nil
}

// RequiresNodePoolSandbox requires at least one node pool to run the given sandbox type.
func RequiresNodePoolSandbox(sandboxType string) Constraint {
	return requiresNodePoolSandbox{sandboxType: sandboxType}
}

type requiresNodePoolSandbox struct{ sandboxType string }

func (c requiresNodePoolSandbox) Describe() string {
	return fmt.Sprintf("requires a node pool with spec.nodeSecurity.sandboxType %q", c.sandboxType)
}

func (c requiresNodePoolSandbox) CheckCluster(snapshot Snapshot) error {
	for _, pool := range snapshot.NodePools {
		if strings.EqualFold(pool.SandboxType, c.sandboxType) {
			return nil
		}
	}
	return errors.New(c.Describe())
}

// RequiresProjectAPI requires a GCP service to be enabled on the cluster's project.
func RequiresProjectAPI(api string) Constraint { return requiresProjectAPI{api: api} }

type requiresProjectAPI struct{ api string }

func (c requiresProjectAPI) Describe() string {
	return fmt.Sprintf("requires the %s API to be enabled on the project", c.api)
}

// RequiredAPI reports the service this constraint needs looked up. See CollectRequiredProjectAPIs.
func (c requiresProjectAPI) RequiredAPI() string { return c.api }

func (c requiresProjectAPI) CheckCluster(snapshot Snapshot) error {
	// An absent entry means the lookup didn't happen or didn't succeed, which is not evidence that the
	// API is disabled, so say nothing rather than blocking the cluster on it.
	if isEnabled, known := snapshot.ProjectAPIsEnabled[c.api]; known && !isEnabled {
		return errors.New(c.Describe())
	}
	return nil
}

// RequiresAccelerator requires node pools to carry a particular accelerator.
//
// It deliberately implements neither SpecConstraint nor ClusterConstraint: GCPManagedMachinePool has no
// accelerator field, so nothing in the API reports what a node pool runs. Declaring the requirement anyway
// means it still reaches users through admission warnings and the generated documentation. Give this type
// a CheckCluster method if GCPManagedMachinePool ever grows the field.
func RequiresAccelerator(accelerator string) Constraint {
	return requiresAccelerator{accelerator: accelerator}
}

type requiresAccelerator struct{ accelerator string }

func (c requiresAccelerator) Describe() string {
	return fmt.Sprintf("requires node pools with %s accelerators", c.accelerator)
}

// eachKnownFamily calls fn for every node pool whose machine family can be determined. A pool whose
// machine type is set neither on the GCPManagedMachinePool nor by GKE is skipped rather than guessed at.
func eachKnownFamily(pools []NodePoolState, fn func(pool NodePoolState, family string) error) error {
	for _, pool := range pools {
		family := machineFamily(pool.MachineType)
		if family == "" {
			continue
		}
		if err := fn(pool, family); err != nil {
			return err
		}
	}
	return nil
}

// machineFamily returns the GCE machine family a machine type belongs to: the part before the first "-",
// so both "e2-standard-4" and "e2-custom-4-8192" are "e2". Legacy custom machine types ("custom-4-8192")
// carry no family prefix and are N1. Returns "" when the family can't be determined.
func machineFamily(machineType string) string {
	machineType = strings.ToLower(strings.TrimSpace(machineType))
	if strings.HasPrefix(machineType, "custom-") {
		return "n1"
	}
	family, _, found := strings.Cut(machineType, "-")
	if !found {
		return ""
	}
	return family
}

// architecture reports the CPU architecture a GCE machine family runs.
func architecture(family string) string {
	if slices.Contains(armMachineFamilies, family) {
		return ArchitectureArm64
	}
	return ArchitectureAmd64
}

// nodeOS reports the operating system a GKE node image type runs. GKE's Windows images are all prefixed
// "windows" ("windows_ltsc_containerd", "windows_sac_containerd"); everything else is Linux.
func nodeOS(imageType string) corev1.OSName {
	if strings.HasPrefix(strings.ToLower(imageType), string(corev1.Windows)) {
		return corev1.Windows
	}
	return corev1.Linux
}

// gkeBuild returns the build number from a GKE version's "-gke.N" suffix, if it carries one.
func gkeBuild(gkeVersion string) (int, bool) {
	_, suffix, found := strings.Cut(gkeVersion, "-gke.")
	if !found {
		return 0, false
	}
	build, err := strconv.Atoi(suffix)
	return build, err == nil
}
