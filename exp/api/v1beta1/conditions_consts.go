/*
Copyright 2022 The Kubernetes Authors.

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

package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// GKEControlPlaneReadyCondition condition reports on the successful reconciliation of GKE control plane.
	GKEControlPlaneReadyCondition clusterv1.ConditionType = "GKEControlPlaneReady"
	// GKEControlPlaneCreatingCondition condition reports on whether the GKE control plane is creating.
	GKEControlPlaneCreatingCondition clusterv1.ConditionType = "GKEControlPlaneCreating"
	// GKEControlPlaneUpdatingCondition condition reports on whether the GKE control plane is updating.
	GKEControlPlaneUpdatingCondition clusterv1.ConditionType = "GKEControlPlaneUpdating"
	// GKEControlPlaneDeletingCondition condition reports on whether the GKE control plane is deleting.
	GKEControlPlaneDeletingCondition clusterv1.ConditionType = "GKEControlPlaneDeleting"

	// GKEControlPlaneCreatingReason used to report GKE control plane being created.
	GKEControlPlaneCreatingReason = "GKEControlPlaneCreating"
	// GKEControlPlaneCreatedReason used to report GKE control plane is created.
	GKEControlPlaneCreatedReason = "GKEControlPlaneCreating"
	// GKEControlPlaneDeletingReason used to report GKE control plane being deleted.
	GKEControlPlaneDeletingReason = "GKEControlPlaneDeleting"
	// GKEControlPlaneDeletedReason used to report GKE control plane is deleted.
	GKEControlPlaneDeletedReason = "GKEControlPlaneDeleting"
	// GKEControlPlaneErrorReason used to report GKE control plane is in error state.
	GKEControlPlaneErrorReason = "GKEControlPlaneError"
	// GKEControlPlaneFailedReason used to report failures while reconciling GKE control plane.
	GKEControlPlaneReconciliationFailedReason = "GKEControlPlaneReconciliationFailed"
)
