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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

// The keys GCPManagedControlPlane's addonsConfig accepts. Each is the name GKE's own API gives the
// add-on, which is what makes it enough on its own to find the add-on in a GKE AddonsConfig: see
// resolve in gke.go. They are constants so that a reference from one add-on to another fails to compile
// rather than silently never matching.
const (
	KeyAgentSandbox               = "agentSandboxConfig"
	KeyConfigConnector            = "configConnectorConfig"
	KeyDNSCache                   = "dnsCacheConfig"
	KeyGCEPersistentDiskCSIDriver = "gcePersistentDiskCsiDriverConfig"
	KeyGCPFilestoreCSIDriver      = "gcpFilestoreCsiDriverConfig"
	KeyGCSFuseCSIDriver           = "gcsFuseCsiDriverConfig"
	KeyGKEBackupAgent             = "gkeBackupAgentConfig"
	KeyHighScaleCheckpointing     = "highScaleCheckpointingConfig"
	KeyLustreCSIDriver            = "lustreCsiDriverConfig"
	KeyNodeReadiness              = "nodeReadinessConfig"
	KeyParallelstoreCSIDriver     = "parallelstoreCsiDriverConfig"
	KeyPodSnapshot                = "podSnapshotConfig"
	KeyRayOperator                = "rayOperatorConfig"
	KeySliceController            = "sliceControllerConfig"
	KeySlurmOperator              = "slurmOperatorConfig"
	KeyStatefulHA                 = "statefulHaConfig"

	// The keys an add-on's options are set under, within its own options map.
	OptionLustreDisableMultiNic = "disableMultiNic"
	OptionRayClusterLogging     = "rayClusterLoggingConfig"
	OptionRayClusterMonitoring  = "rayClusterMonitoringConfig"
)

// Addon is everything CAPG knows about one GKE add-on that GKE's own API doesn't already say: what it is
// for, what GKE requires before it can be enabled, and what else it can be configured with.
type Addon struct {
	// Key names the add-on, both as the addonsConfig map key and as the field to find in GKE's
	// AddonsConfig.
	Key string
	// Description says what the add-on does. A map's keys get no individual descriptions in a generated
	// CRD schema the way named struct fields would, so this is where that documentation lives.
	Description string

	// Constraints are the prerequisites GKE imposes before the add-on can be enabled. Facts GKE states
	// only in its documentation — version floors, machine families, which architectures an add-on runs
	// on — have to be recorded here when the add-on is onboarded, since nothing in the API reports them.
	Constraints []Constraint

	// Options are the settings this add-on exposes beyond being switched on. They belong to the add-on
	// rather than standing alongside it, so setting one without the add-on is a structural error Parse
	// catches, not a dependency someone has to remember to declare.
	Options []Option
}

// Option is one setting an add-on exposes beyond being switched on.
type Option struct {
	// Key names the option, both within its add-on's options map and within the add-on's GKE config.
	Key         string
	Description string
}

// Supported is every add-on CAPG understands, and the single source of truth for everything CAPG does
// with one: the validating webhook rejects keys that aren't here, and both it and the reconciler check
// Constraints before a cluster can reach GKE in a state GKE would reject.
//
// Only what GKE's API cannot be asked is written down here. Where each add-on lives in a GKE
// AddonsConfig is not, because an add-on's key is already the name GKE gives it, so the two cannot drift
// apart as the GKE SDK moves; TestSupportedResolve checks that every key here still names something.
//
// GKE's older add-ons — Cloud Run, HTTP load balancing, horizontal pod autoscaling, the Kubernetes
// dashboard and network policy — are absent because each is on by default, deprecated, or superseded, so
// there is little to ask for: network policy, for one, is enforced by Dataplane V2 with no add-on at all.
var Supported = []Addon{
	{
		Key:         KeyDNSCache,
		Description: "NodeLocal DNSCache, a DNS cache running on cluster nodes.",
		Constraints: []Constraint{
			MinimumVersion("1.15"),
			ExcludesNodeOS(corev1.Windows),
		},
	},
	{
		Key:         KeyGCEPersistentDiskCSIDriver,
		Description: "The Compute Engine persistent disk CSI driver.",
	},
	{
		Key:         KeyGCPFilestoreCSIDriver,
		Description: "The Filestore CSI driver.",
		Constraints: []Constraint{
			MinimumVersion("1.21"),
			ExcludesNodeOS(corev1.Windows),
		},
	},
	{
		Key:         KeyGKEBackupAgent,
		Description: "The Backup for GKE agent.",
		Constraints: []Constraint{
			RequiresWorkloadIdentity,
			MinimumVersion("1.19"),
		},
	},
	{
		Key:         KeyConfigConnector,
		Description: "Config Connector, a Kubernetes extension for managing hosted Google Cloud services through the Kubernetes API.",
		Constraints: []Constraint{
			RequiresWorkloadIdentity,
			MinimumVersion("1.15.11-gke.5"),
		},
	},
	{
		Key:         KeyStatefulHA,
		Description: "The Stateful HA add-on.",
		Constraints: []Constraint{
			DependsOnAddon(KeyGCEPersistentDiskCSIDriver),
			MinimumVersion("1.28"),
			RequiresMachineFamily("e2", "n1", "n2", "n2d"),
		},
	},
	{
		Key:         KeyGCSFuseCSIDriver,
		Description: "The Cloud Storage FUSE CSI driver.",
		Constraints: []Constraint{RequiresWorkloadIdentity},
	},
	{
		Key:         KeyParallelstoreCSIDriver,
		Description: "The Cloud Storage Parallelstore CSI driver.",
		Constraints: []Constraint{
			MinimumVersion("1.31.1-gke.1729000"),
			RequiresArchitecture(ArchitectureAmd64),
		},
	},
	{
		Key:         KeyLustreCSIDriver,
		Description: "The Managed Lustre CSI driver.",
		Options: []Option{
			{
				Key: OptionLustreDisableMultiNic,
				// GKE's own flag is the negative one, and keeping its sense avoids an option whose
				// false means two different things: setting it false is not the same as leaving
				// multi-NIC support alone.
				Description: "Turns off multi-NIC support, which the driver otherwise enables.",
			},
		},
	},
	{
		Key:         KeyRayOperator,
		Description: "The Ray Operator, which manages Ray clusters.",
		Options: []Option{
			{Key: OptionRayClusterLogging, Description: "Collects logs from Ray clusters."},
			{Key: OptionRayClusterMonitoring, Description: "Collects metrics from Ray clusters."},
		},
	},
	{
		Key:         KeyHighScaleCheckpointing,
		Description: "The High Scale Checkpointing add-on.",
		Constraints: []Constraint{
			RequiresWorkloadIdentity,
			DependsOnAddon(KeyGCSFuseCSIDriver),
			MinimumVersion("1.32.4-gke.1415000"),
		},
	},
	{
		Key:         KeySliceController,
		Description: "The Slice Controller add-on.",
		Constraints: []Constraint{
			ReleaseChannelIn(infrav1exp.Rapid),
			RequiresAccelerator("TPU7x"),
		},
	},
	{
		Key:         KeyAgentSandbox,
		Description: "The AgentSandbox add-on.",
		Constraints: []Constraint{
			MinimumVersion("1.35.2-gke.1269000"),
			RequiresNodePoolSandbox(sandboxTypeGVisor),
			RequiresProjectAPI("artifactregistry.googleapis.com"),
		},
	},
	{
		Key: KeyNodeReadiness,
		// The Node Readiness Controller clears a taint the Compute Engine persistent disk CSI driver
		// sets, so it does nothing useful without that add-on. That isn't a DependsOnAddon constraint
		// because GKE accepts the combination, and rejecting it would block a cluster GKE is happy to
		// create.
		Description: "The GKE Node Readiness Controller. Only useful alongside " + KeyGCEPersistentDiskCSIDriver + ".",
	},
	{
		Key:         KeyPodSnapshot,
		Description: "The Pod Snapshots feature.",
		Constraints: []Constraint{
			RequiresWorkloadIdentity,
			MinimumVersion("1.35.3-gke.1234000"),
			RequiresNodePoolSandbox(sandboxTypeGVisor),
			ExcludesMachineFamily("e2"),
		},
	},
	{
		Key:         KeySlurmOperator,
		Description: "The Slurm Operator, which manages the compute pods for a Slurm cluster.",
		Constraints: []Constraint{
			ReleaseChannelIn(infrav1exp.Rapid),
			MinimumVersion("1.35.2-gke.1842000"),
		},
	},
}

// supportedKeys is every key Supported recognizes, so that judging a cluster's add-on names doesn't
// scan it.
var supportedKeys = func() sets.Set[string] {
	keys := sets.New[string]()
	for _, addon := range Supported {
		keys.Insert(addon.Key)
	}
	return keys
}()

// Keys returns every supported addonsConfig key, sorted, for listing the valid values in an error.
func Keys() []string {
	return sets.List(supportedKeys)
}

// option returns the named option of this add-on.
func (a Addon) option(key string) (Option, bool) {
	for _, option := range a.Options {
		if option.Key == key {
			return option, true
		}
	}
	return Option{}, false
}

// optionKeys returns this add-on's option keys, for listing the valid values in an error.
func (a Addon) optionKeys() []string {
	keys := make([]string, len(a.Options))
	for i, option := range a.Options {
		keys[i] = option.Key
	}
	return keys
}
