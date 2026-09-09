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

// Package controllers implements controller types.
package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	machineTemplateCapacityRequeueAfter = 30 * time.Second
	gcpMachineTemplateKind              = "GCPMachineTemplate"
	// GCP MachineType.Architecture values
	gcpArchitectureARM64 = "ARM64"
)

// GCPMachineTemplateReconciler reconciles GCPMachineTemplate objects.
//
// This controller populates template capacity to enable Cluster Autoscaler to
// scale a MachineDeployment from zero replicas.
type GCPMachineTemplateReconciler struct {
	client.Client
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machinedeployments;machinesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=gcpmachinetemplates/status,verbs=get;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *GCPMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.GCPMachineTemplate{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Watches(
			&clusterv1.MachineDeployment{},
			handler.EnqueueRequestsFromMapFunc(machineDeploymentToGCPMachineTemplate),
		).
		Watches(
			&clusterv1.MachineSet{},
			handler.EnqueueRequestsFromMapFunc(machineSetToGCPMachineTemplate),
		).
		Complete(r)
}

func machineDeploymentToGCPMachineTemplate(_ context.Context, object client.Object) []ctrl.Request {
	md, ok := object.(*clusterv1.MachineDeployment)
	if !ok || md.Spec.Template.Spec.InfrastructureRef.Kind != gcpMachineTemplateKind {
		return nil
	}
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Namespace: md.Namespace, Name: md.Spec.Template.Spec.InfrastructureRef.Name}}}
}

func machineSetToGCPMachineTemplate(_ context.Context, object client.Object) []ctrl.Request {
	ms, ok := object.(*clusterv1.MachineSet)
	if !ok || ms.Spec.Template.Spec.InfrastructureRef.Kind != gcpMachineTemplateKind {
		return nil
	}
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Namespace: ms.Namespace, Name: ms.Spec.Template.Spec.InfrastructureRef.Name}}}
}

// Reconcile populates capacity information for GCPMachineTemplate.
func (r *GCPMachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	logger := log.FromContext(ctx)
	template := &infrav1.GCPMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !template.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	instanceType := template.Spec.Template.Spec.InstanceType
	if instanceType == "" || (len(template.Status.Capacity) > 0 && template.Status.NodeInfo != nil) {
		return ctrl.Result{}, nil
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, template.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("GCPMachineTemplate does not yet have an owner Cluster")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, template) {
		logger.Info("GCPMachineTemplate or owner Cluster is paused")
		return ctrl.Result{}, nil
	}

	if cluster.Spec.InfrastructureRef.Kind != "GCPCluster" || cluster.Spec.InfrastructureRef.Name == "" {
		return ctrl.Result{}, nil
	}

	gcpCluster := &infrav1.GCPCluster{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, gcpCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: machineTemplateCapacityRequeueAfter}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get GCPCluster")
	}

	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:     r.Client,
		Cluster:    cluster,
		GCPCluster: gcpCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create cluster scope")
	}
	defer func() {
		if err := clusterScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Get zone - prefer failure domains, fallback to region-a
	// Machine type specs are identical across all zones in a region
	var zone string
	failureDomains := clusterScope.FailureDomains()
	if len(failureDomains) > 0 {
		sort.Strings(failureDomains)
		zone = failureDomains[0]
	} else {
		// Fallback: use first zone in region when failure domains not discovered yet
		// This mirrors CAPA approach which uses region-level API without zone dependency
		if gcpCluster.Spec.Region == "" {
			logger.Info("GCPCluster has no region specified")
			return ctrl.Result{RequeueAfter: machineTemplateCapacityRequeueAfter}, nil
		}
		zone = gcpCluster.Spec.Region + "-a"
		logger.Info("Using default zone from region as fallback", "region", gcpCluster.Spec.Region, "zone", zone)
	}

	machineType, err := getMachineType(ctx, clusterScope.Compute, clusterScope.Project(), zone, instanceType)
	if err != nil {
		return ctrl.Result{}, err
	}

	capacity, err := machineTypeCapacity(machineType)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "invalid capacity for machine type %q", instanceType)
	}

	nodeInfo, err := machineTypeNodeInfo(ctx, clusterScope.Compute, clusterScope.Project(), machineType, template)
	if err != nil {
		logger.Info("Failed to determine nodeInfo from image, using defaults", "error", err)
		// Use defaults: set only architecture from machine type, omit OS
		nodeInfo = &infrav1.NodeInfo{
			Architecture: getArchitectureFromMachineType(machineType),
		}
	}

	original := template.DeepCopy()
	template.Status.Capacity = capacity
	template.Status.NodeInfo = nodeInfo
	if err := r.Status().Patch(ctx, template, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch GCPMachineTemplate status")
	}

	logger.Info("Populated GCPMachineTemplate capacity and nodeInfo", "instanceType", instanceType, "zone", zone, "capacity", template.Status.Capacity, "nodeInfo", nodeInfo)
	return ctrl.Result{}, nil
}

func getMachineType(ctx context.Context, computeService *compute.Service, project, zone, instanceType string) (*compute.MachineType, error) {
	machineType, err := computeService.MachineTypes.Get(project, zone, instanceType).Context(ctx).Do()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get machine type %q in zone %q", instanceType, zone)
	}
	return machineType, nil
}

func machineTypeCapacity(machineType *compute.MachineType) (corev1.ResourceList, error) {
	if machineType.GuestCpus <= 0 || machineType.MemoryMb <= 0 {
		return nil, fmt.Errorf("cpu=%d memoryMb=%d", machineType.GuestCpus, machineType.MemoryMb)
	}

	return corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewQuantity(machineType.GuestCpus, resource.DecimalSI),
		corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", machineType.MemoryMb)),
	}, nil
}

// machineTypeNodeInfo populates NodeInfo by querying GCP APIs.
// Architecture: from MachineType API (already fetched for capacity).
// OS: from Images API (queries boot disk image metadata).
func machineTypeNodeInfo(ctx context.Context, computeService *compute.Service, project string, machineType *compute.MachineType, template *infrav1.GCPMachineTemplate) (*infrav1.NodeInfo, error) {
	// Get architecture from machine type (no additional API call)
	arch := getArchitectureFromMachineType(machineType)

	// Get OS from image metadata (requires Images API call)
	os, err := getOperatingSystemFromImage(ctx, computeService, project, template)
	if err != nil {
		return nil, err
	}

	return &infrav1.NodeInfo{
		Architecture:    arch,
		OperatingSystem: os,
	}, nil
}

// getArchitectureFromMachineType extracts architecture from MachineType API response.
func getArchitectureFromMachineType(machineType *compute.MachineType) infrav1.Architecture {
	// GCP MachineType.Architecture values:
	// - "ARM64" for ARM instances (t2a-*, c4a-*)
	// - "X86_64" for x86 instances (all others)
	// - empty/unset for older API responses (treat as x86_64)
	if machineType.Architecture == gcpArchitectureARM64 {
		return infrav1.ArchitectureArm64
	}
	return infrav1.ArchitectureAmd64
}

// getOperatingSystemFromImage determines OS by querying GCP Images API.
// Mirrors AWS approach: query image metadata to detect Windows vs Linux.
func getOperatingSystemFromImage(ctx context.Context, computeService *compute.Service, defaultProject string, template *infrav1.GCPMachineTemplate) (infrav1.OperatingSystem, error) {
	// Strategy 1: Explicit image reference (takes precedence)
	if template.Spec.Template.Spec.Image != nil {
		return queryImageOS(ctx, computeService, defaultProject, *template.Spec.Template.Spec.Image, false)
	}

	// Strategy 2: Image family reference
	if template.Spec.Template.Spec.ImageFamily != nil {
		return queryImageOS(ctx, computeService, defaultProject, *template.Spec.Template.Spec.ImageFamily, true)
	}

	// Strategy 3: No image specified - default to Linux (safe assumption for CAPG)
	// All current CAPG deployments use Linux, Windows support not yet implemented
	return infrav1.OperatingSystemLinux, nil
}

// queryImageOS queries GCP Images API and checks guestOsFeatures for Windows.
func queryImageOS(ctx context.Context, computeService *compute.Service, defaultProject, imageRef string, isFamily bool) (infrav1.OperatingSystem, error) {
	// Parse image reference to extract project and image/family name
	project, name, err := parseImageReference(imageRef, defaultProject)
	if err != nil {
		return "", err
	}

	// Query Images API
	var image *compute.Image
	if isFamily {
		// Get latest image from family
		image, err = computeService.Images.GetFromFamily(project, name).Context(ctx).Do()
		if err != nil {
			return "", errors.Wrapf(err, "failed to get image from family %q in project %q", name, project)
		}
	} else {
		// Get specific image
		image, err = computeService.Images.Get(project, name).Context(ctx).Do()
		if err != nil {
			return "", errors.Wrapf(err, "failed to get image %q in project %q", name, project)
		}
	}

	// Check guestOsFeatures for WINDOWS marker
	// GCP marks Windows images with "WINDOWS" feature in guestOsFeatures array
	for _, feature := range image.GuestOsFeatures {
		if feature.Type == "WINDOWS" {
			return infrav1.OperatingSystemWindows, nil
		}
	}

	// No WINDOWS feature = Linux
	return infrav1.OperatingSystemLinux, nil
}

// parseImageReference parses GCP image reference into project and name.
// Handles multiple formats:
// - "projects/PROJECT/global/images/IMAGE"
// - "projects/PROJECT/global/images/family/FAMILY"
// - "global/images/IMAGE"
// - "IMAGE" (short form, uses defaultProject)
// - "family/FAMILY" (short form, uses defaultProject)
func parseImageReference(ref, defaultProject string) (project, name string, err error) {
	// Full path with project prefix
	if strings.HasPrefix(ref, "projects/") {
		parts := strings.Split(ref, "/")
		// projects/PROJECT/global/images/IMAGE = 5 parts
		// projects/PROJECT/global/images/family/FAMILY = 6 parts
		if len(parts) < 5 {
			return "", "", errors.Errorf("invalid image reference format: %s", ref)
		}
		project = parts[1]
		if len(parts) >= 6 && parts[4] == "family" {
			name = parts[5]
		} else {
			name = parts[4]
		}
		return project, name, nil
	}

	// Global path without project prefix
	if strings.HasPrefix(ref, "global/images/") {
		parts := strings.Split(ref, "/")
		// global/images/IMAGE = 3 parts
		// global/images/family/FAMILY = 4 parts
		switch {
		case len(parts) >= 4 && parts[2] == "family":
			name = parts[3]
		case len(parts) >= 3:
			name = parts[2]
		default:
			return "", "", errors.Errorf("invalid global image reference: %s", ref)
		}
		return defaultProject, name, nil
	}

	// Short form: "family/FAMILY" or "IMAGE"
	if strings.HasPrefix(ref, "family/") {
		name = strings.TrimPrefix(ref, "family/")
	} else {
		name = ref
	}
	return defaultProject, name, nil
}
