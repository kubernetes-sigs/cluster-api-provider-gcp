/*
Copyright 2023 The Kubernetes Authors.

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

package scope

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	kubedrain "k8s.io/kubectl/pkg/drain"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capierrors "sigs.k8s.io/cluster-api/errors"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// MachinePoolMachineScopeName is the sourceName, or more specifically the UserAgent, of client used in cordon and drain.
	MachinePoolMachineScopeName = "gcpmachinepoolmachine-scope"
)

type (
	// MachinePoolMachineScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolMachineScopeParams struct {
		Client                client.Client
		ClusterGetter         cloud.ClusterGetter
		MachinePool           *clusterv1exp.MachinePool
		GCPMachinePool        *infrav1exp.GCPMachinePool
		GCPMachinePoolMachine *infrav1exp.GCPMachinePoolMachine
	}
	// MachinePoolMachineScope defines a scope defined around a machine pool and its cluster.
	MachinePoolMachineScope struct {
		Client                     client.Client
		PatchHelper                *patch.Helper
		CapiMachinePoolPatchHelper *patch.Helper

		ClusterGetter         cloud.ClusterGetter
		MachinePool           *clusterv1exp.MachinePool
		GCPMachinePool        *infrav1exp.GCPMachinePool
		GCPMachinePoolMachine *infrav1exp.GCPMachinePoolMachine
	}
)

// PatchObject persists the machine pool configuration and status.
func (m *MachinePoolMachineScope) PatchObject(ctx context.Context) error {
	return m.PatchHelper.Patch(ctx, m.GCPMachinePoolMachine)
}

// SetReady sets the GCPMachinePoolMachine Ready Status to true.
func (m *MachinePoolMachineScope) SetReady() {
	m.GCPMachinePoolMachine.Status.Ready = true
}

// SetNotReady sets the GCPMachinePoolMachine Ready Status to false.
func (m *MachinePoolMachineScope) SetNotReady() {
	m.GCPMachinePoolMachine.Status.Ready = false
}

// SetFailureMessage sets the GCPMachinePoolMachine status failure message.
func (m *MachinePoolMachineScope) SetFailureMessage(v error) {
	m.GCPMachinePoolMachine.Status.FailureMessage = ptr.To(v.Error())
}

// SetFailureReason sets the GCPMachinePoolMachine status failure reason.
func (m *MachinePoolMachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.GCPMachinePoolMachine.Status.FailureReason = &v
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *MachinePoolMachineScope) Close(ctx context.Context) error {
	if err := m.PatchObject(ctx); err != nil {
		return err
	}
	return nil
}

// NewMachinePoolMachineScope creates a new MachinePoolScope from the supplied parameters.
func NewMachinePoolMachineScope(params MachinePoolMachineScopeParams) (*MachinePoolMachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	if params.GCPMachinePool == nil {
		return nil, errors.New("gcp machine pool is required when creating a MachinePoolScope")
	}
	if params.GCPMachinePoolMachine == nil {
		return nil, errors.New("gcp machine pool machine is required when creating a MachinePoolScope")
	}

	helper, err := patch.NewHelper(params.GCPMachinePoolMachine, params.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", params.GCPMachinePoolMachine.GroupVersionKind(), params.GCPMachinePoolMachine.Namespace, params.GCPMachinePoolMachine.Name)
	}

	capiMachinePoolPatchHelper, err := patch.NewHelper(params.MachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", params.MachinePool.GroupVersionKind(), params.MachinePool.Namespace, params.MachinePool.Name)
	}

	return &MachinePoolMachineScope{
		Client:                     params.Client,
		ClusterGetter:              params.ClusterGetter,
		MachinePool:                params.MachinePool,
		GCPMachinePool:             params.GCPMachinePool,
		GCPMachinePoolMachine:      params.GCPMachinePoolMachine,
		PatchHelper:                helper,
		CapiMachinePoolPatchHelper: capiMachinePoolPatchHelper,
	}, nil
}

// UpdateNodeStatus updates the GCPMachinePoolMachine conditions and ready status. It will also update the node ref and the Kubernetes version.
func (m *MachinePoolMachineScope) UpdateNodeStatus(ctx context.Context) (bool, error) {
	var node *corev1.Node
	nodeRef := m.GCPMachinePoolMachine.Status.NodeRef

	// See if we can fetch a node using either the providerID or the nodeRef
	node, found, err := m.GetNode(ctx)
	switch {
	case err != nil && apierrors.IsNotFound(err) && nodeRef != nil && nodeRef.Name != "":
		// Node was not found due to 404 when finding by ObjectReference.
		conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.MachineNodeHealthyCondition, clusterv1.NodeNotFoundReason, clusterv1.ConditionSeverityError, "")
		return false, nil
	case err != nil:
		// Failed due to an unexpected error
		return false, err
	case !found && m.ProviderID() == "":
		// Node was not found due to not having a providerID set
		conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.MachineNodeHealthyCondition, clusterv1.WaitingForNodeRefReason, clusterv1.ConditionSeverityInfo, "")
		return false, nil
	case !found && m.ProviderID() != "":
		// Node was not found due to not finding a matching node by providerID
		conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.MachineNodeHealthyCondition, clusterv1.NodeProvisioningReason, clusterv1.ConditionSeverityInfo, "")
		return false, nil
	default:
		// Node was found. Check if it is ready.
		nodeReady := noderefutil.IsNodeReady(node)
		m.GCPMachinePoolMachine.Status.Ready = nodeReady
		if nodeReady {
			conditions.MarkTrue(m.GCPMachinePoolMachine, clusterv1.MachineNodeHealthyCondition)
		} else {
			conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.MachineNodeHealthyCondition, clusterv1.NodeConditionsFailedReason, clusterv1.ConditionSeverityWarning, "")
		}

		m.GCPMachinePoolMachine.Status.NodeRef = &corev1.ObjectReference{
			Kind:       node.Kind,
			Namespace:  node.Namespace,
			Name:       node.Name,
			UID:        node.UID,
			APIVersion: node.APIVersion,
		}

		m.GCPMachinePoolMachine.Status.Version = node.Status.NodeInfo.KubeletVersion
	}

	return true, nil
}

// GetNode returns the node for the GCPMachinePoolMachine. If the node is not found, it returns false.
func (m *MachinePoolMachineScope) GetNode(ctx context.Context) (*corev1.Node, bool, error) {
	var (
		nodeRef = m.GCPMachinePoolMachine.Status.NodeRef
		node    *corev1.Node
		err     error
	)

	if nodeRef == nil || nodeRef.Name == "" {
		node, err = m.GetNodeByProviderID(ctx, m.ProviderID())
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to get node by providerID")
		}
	} else {
		node, err = m.GetNodeByObjectReference(ctx, *nodeRef)
		if err != nil {
			return nil, false, errors.Wrap(err, "failed to get node by object reference")
		}
	}

	if node == nil {
		return nil, false, nil
	}

	return node, true, nil
}

// GetNodeByObjectReference will fetch a *corev1.Node via a node object reference.
func (m *MachinePoolMachineScope) GetNodeByObjectReference(ctx context.Context, nodeRef corev1.ObjectReference) (*corev1.Node, error) {
	// get remote client
	workloadClient, err := remote.NewClusterClient(ctx, MachinePoolMachineScopeName, m.Client, client.ObjectKey{
		Name:      m.ClusterGetter.Name(),
		Namespace: m.GCPMachinePoolMachine.Namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	var node corev1.Node
	err = workloadClient.Get(ctx, client.ObjectKey{
		Namespace: nodeRef.Namespace,
		Name:      nodeRef.Name,
	}, &node)

	return &node, err
}

// GetNodeByProviderID returns a node by its providerID. If the node is not found, it returns nil.
func (m *MachinePoolMachineScope) GetNodeByProviderID(ctx context.Context, providerID string) (*corev1.Node, error) {
	// get remote client
	workloadClient, err := remote.NewClusterClient(ctx, MachinePoolMachineScopeName, m.Client, client.ObjectKey{
		Name:      m.ClusterGetter.Name(),
		Namespace: m.GCPMachinePoolMachine.Namespace,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	nodeList := corev1.NodeList{}
	for {
		if err := workloadClient.List(ctx, &nodeList, client.Continue(nodeList.Continue)); err != nil {
			return nil, errors.Wrapf(err, "failed to List nodes")
		}

		for _, node := range nodeList.Items {
			if node.Spec.ProviderID == providerID {
				return &node, nil
			}
		}

		if nodeList.Continue == "" {
			break
		}
	}

	return nil, nil
}

// GetGCPClientCredentials returns the GCP client credentials.
func (m *MachinePoolMachineScope) GetGCPClientCredentials() ([]byte, error) {
	credsPath := os.Getenv(ConfigFileEnvVar)
	if credsPath == "" {
		return nil, fmt.Errorf("no ADC environment variable found for credentials (expect %s)", ConfigFileEnvVar)
	}

	byteValue, err := os.ReadFile(credsPath) //nolint:gosec // We need to read a file here
	if err != nil {
		return nil, fmt.Errorf("reading credentials from file %s: %w", credsPath, err)
	}
	return byteValue, nil
}

// Zone returns the zone for the GCPMachinePoolMachine.
func (m *MachinePoolMachineScope) Zone() string {
	return m.GCPMachinePool.Spec.Zone
}

// Project return the project for the GCPMachinePoolMachine cluster.
func (m *MachinePoolMachineScope) Project() string {
	return m.ClusterGetter.Project()
}

// Name returns the GCPMachinePoolMachine name.
func (m *MachinePoolMachineScope) Name() string {
	return m.GCPMachinePoolMachine.GetName()
}

// ProviderID returns the provider ID for the GCPMachinePoolMachine.
func (m *MachinePoolMachineScope) ProviderID() string {
	return fmt.Sprintf("gce://%s/%s/%s", m.Project(), m.GCPMachinePool.Spec.Zone, m.Name())
}

// HasLatestModelApplied checks if the latest model is applied to the GCPMachinePoolMachine.
func (m *MachinePoolMachineScope) HasLatestModelApplied(_ context.Context, instance *compute.Disk) (bool, error) {
	image := ""

	if m.GCPMachinePool.Spec.Image == nil {
		version := ""
		if m.MachinePool.Spec.Template.Spec.Version != nil {
			version = *m.MachinePool.Spec.Template.Spec.Version
		}
		image = cloud.ClusterAPIImagePrefix + strings.ReplaceAll(semver.MajorMinor(version), ".", "-")
	} else {
		image = *m.GCPMachinePool.Spec.Image
	}

	// Get the image from the disk URL path to compare with the latest image name
	diskImage, err := url.Parse(instance.SourceImage)
	if err != nil {
		return false, err
	}
	instanceImage := path.Base(diskImage.Path)

	// Check if the image is the latest
	if image == instanceImage {
		return true, nil
	}

	return false, nil
}

// CordonAndDrainNode cordon and drain the node for the GCPMachinePoolMachine.
func (m *MachinePoolMachineScope) CordonAndDrainNode(ctx context.Context) error {
	log := log.FromContext(ctx)

	// See if we can fetch a node using either the providerID or the nodeRef
	node, found, err := m.GetNode(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		// failed due to an unexpected error
		return errors.Wrap(err, "failed to get node")
	} else if !found {
		// node was not found due to not finding a nodes with the ProviderID
		return nil
	}

	// Drain node before deletion and issue a patch in order to make this operation visible to the users.
	if m.isNodeDrainAllowed() {
		patchHelper, err := patch.NewHelper(m.GCPMachinePoolMachine, m.Client)
		if err != nil {
			return errors.Wrap(err, "failed to build a patchHelper when draining node")
		}

		log.Info("Draining node before deletion", "node", node.Name)
		// The DrainingSucceededCondition never exists before the node is drained for the first time,
		// so its transition time can be used to record the first time draining.
		// This `if` condition prevents the transition time to be changed more than once.
		if conditions.Get(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
			conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition, clusterv1.DrainingReason, clusterv1.ConditionSeverityInfo, "Draining the node before deletion")
		}

		if err := patchHelper.Patch(ctx, m.GCPMachinePoolMachine); err != nil {
			return errors.Wrap(err, "failed to patch GCPMachinePoolMachine")
		}

		if err := m.drainNode(ctx, node); err != nil {
			// Check for condition existence. If the condition exists, it may have a different severity or message, which
			// would cause the last transition time to be updated. The last transition time is used to determine how
			// long to wait to timeout the node drain operation. If we were to keep updating the last transition time,
			// a drain operation may never timeout.
			if conditions.Get(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
				conditions.MarkFalse(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition, clusterv1.DrainingFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
			}
			return err
		}

		conditions.MarkTrue(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition)
	}

	return nil
}

// isNodeDrainAllowed checks to see the node is excluded from draining or if the NodeDrainTimeout has expired.
func (m *MachinePoolMachineScope) isNodeDrainAllowed() bool {
	if _, exists := m.GCPMachinePoolMachine.ObjectMeta.Annotations[clusterv1.ExcludeNodeDrainingAnnotation]; exists {
		return false
	}

	if m.nodeDrainTimeoutExceeded() {
		return false
	}

	return true
}

// nodeDrainTimeoutExceeded will check to see if the GCPMachinePool's NodeDrainTimeout is exceeded for the
// GCPMachinePoolMachine.
func (m *MachinePoolMachineScope) nodeDrainTimeoutExceeded() bool {
	// if the NodeDrainTineout type is not set by user
	pool := m.GCPMachinePool
	if pool == nil || pool.Spec.NodeDrainTimeout == nil || pool.Spec.NodeDrainTimeout.Seconds() <= 0 {
		return false
	}

	// if the draining succeeded condition does not exist
	if conditions.Get(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition) == nil {
		return false
	}

	now := time.Now()
	firstTimeDrain := conditions.GetLastTransitionTime(m.GCPMachinePoolMachine, clusterv1.DrainingSucceededCondition)
	diff := now.Sub(firstTimeDrain.Time)
	return diff.Seconds() >= m.GCPMachinePool.Spec.NodeDrainTimeout.Seconds()
}

func (m *MachinePoolMachineScope) drainNode(ctx context.Context, node *corev1.Node) error {
	log := log.FromContext(ctx)

	restConfig, err := remote.RESTConfig(ctx, MachinePoolMachineScopeName, m.Client, client.ObjectKey{
		Name:      m.ClusterGetter.Name(),
		Namespace: m.GCPMachinePoolMachine.Namespace,
	})

	if err != nil {
		log.Error(err, "Error creating a remote client while deleting Machine, won't retry")
		return nil
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Error(err, "Error creating a remote client while deleting Machine, won't retry")
		return nil
	}

	drainer := &kubedrain.Helper{
		Client:              kubeClient,
		Ctx:                 ctx,
		Force:               true,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1,
		// If a pod is not evicted in 20 seconds, retry the eviction next time the
		// machine gets reconciled again (to allow other machines to be reconciled).
		Timeout: 20 * time.Second,
		OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
			verbStr := "Deleted"
			if usingEviction {
				verbStr = "Evicted"
			}
			log.Info("Pod", verbStr, "from node", "pod", pod.Name, "node", node.Name)
		},
		Out:    &writerInfo{logFunc: log.Info},
		ErrOut: &writerError{logFunc: log.Error},
	}

	if noderefutil.IsNodeUnreachable(node) {
		// When the node is unreachable and some pods are not evicted for as long as this timeout, we ignore them.
		drainer.SkipWaitForDeleteTimeoutSeconds = 60 * 5 // 5 minutes
	}

	if err := kubedrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		// Machine will be re-reconciled after a cordon failure.
		return fmt.Errorf("cordoning failed, retry in 20s: %v", err)
	}

	if err := kubedrain.RunNodeDrain(drainer, node.Name); err != nil {
		// Machine will be re-reconciled after a drain failure.
		return fmt.Errorf("draining failed, retry in 20s: %v", err)
	}

	log.Info("Node drained successfully", "node", node.Name)
	return nil
}

type writerInfo struct {
	logFunc func(msg string, keysAndValues ...any)
}

func (w *writerInfo) Write(p []byte) (n int, err error) {
	w.logFunc(string(p))
	return len(p), nil
}

type writerError struct {
	logFunc func(err error, msg string, keysAndValues ...any)
}

func (w *writerError) Write(p []byte) (n int, err error) {
	w.logFunc(errors.New(string(p)), "")
	return len(p), nil
}
