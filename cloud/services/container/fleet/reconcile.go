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

package fleet

import (
	"context"
	"fmt"

	"cloud.google.com/go/gkehub/apiv1beta1/gkehubpb"
	"github.com/go-logr/logr"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-gcp/util/reconciler"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reconciles the GKE Fleet membership for the cluster.
func (s *Service) Reconcile(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("service", "container.fleet")
	log.Info("Reconciling fleet membership resources")

	desiredProject := ""
	if s.scope.GCPManagedControlPlane.Spec.Fleet != nil {
		if !s.scope.GCPManagedControlPlane.Status.Ready {
			log.Info("GKE cluster not ready yet, retry later")
			return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
		}
		desiredProject = s.scope.FleetProject()
	}

	return s.ensureMembership(ctx, &log, desiredProject)
}

// Delete deregisters the fleet membership, if any.
func (s *Service) Delete(ctx context.Context) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("service", "container.fleet")
	log.Info("Deleting fleet membership resources")

	return s.ensureMembership(ctx, &log, "")
}

// ensureMembership drives the fleet Membership towards desiredProject, using
// Status.FleetMembership (rather than the current spec alone) to find any
// previously-registered Membership that needs cleaning up first. An empty
// desiredProject means no Membership should exist.
func (s *Service) ensureMembership(ctx context.Context, log *logr.Logger, desiredProject string) (ctrl.Result, error) {
	existingProject := ""
	if fm := s.scope.GCPManagedControlPlane.Status.FleetMembership; fm != nil {
		existingProject = fm.Project
	}

	if existingProject != "" && existingProject != desiredProject {
		done, err := s.deleteMembershipIfPresent(ctx, log, existingProject)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !done {
			return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
		}
		s.scope.GCPManagedControlPlane.Status.FleetMembership = nil
	}

	if desiredProject == "" {
		v1beta1conditions.Delete(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition)
		return ctrl.Result{}, nil
	}

	membership, err := s.describeMembership(ctx, log, desiredProject)
	if err != nil {
		v1beta1conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition, infrav1exp.GKEControlPlaneFleetRegistrationFailedReason, clusterv1beta1.ConditionSeverityError, "describing fleet membership: %v", err)
		return ctrl.Result{}, err
	}
	if membership == nil {
		if err := s.createMembership(ctx, desiredProject); err != nil {
			v1beta1conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition, infrav1exp.GKEControlPlaneFleetRegistrationFailedReason, clusterv1beta1.ConditionSeverityError, "creating fleet membership: %v", err)
			return ctrl.Result{}, err
		}
		log.Info("Fleet membership creation in progress", "project", desiredProject)
		return ctrl.Result{RequeueAfter: reconciler.DefaultRetryTime}, nil
	}

	s.scope.GCPManagedControlPlane.Status.FleetMembership = &infrav1exp.FleetMembershipStatus{Project: desiredProject}
	v1beta1conditions.MarkTrue(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition)
	return ctrl.Result{}, nil
}

// deleteMembershipIfPresent requests deletion of the Membership in project if it
// still exists. It returns done=true once the Membership is confirmed gone.
func (s *Service) deleteMembershipIfPresent(ctx context.Context, log *logr.Logger, project string) (bool, error) {
	membership, err := s.describeMembership(ctx, log, project)
	if err != nil {
		v1beta1conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition, infrav1exp.GKEControlPlaneFleetRegistrationFailedReason, clusterv1beta1.ConditionSeverityError, "describing fleet membership: %v", err)
		return false, err
	}
	if membership == nil {
		return true, nil
	}

	if _, err := s.scope.GkeHubMembershipClient().DeleteMembership(ctx, &gkehubpb.DeleteMembershipRequest{
		Name: s.membershipName(project),
	}); err != nil {
		v1beta1conditions.MarkFalse(s.scope.ConditionSetter(), infrav1exp.GKEControlPlaneFleetRegisteredCondition, infrav1exp.GKEControlPlaneFleetRegistrationFailedReason, clusterv1beta1.ConditionSeverityError, "deleting fleet membership: %v", err)
		return false, err
	}
	log.Info("Fleet membership deletion in progress", "project", project)
	return false, nil
}

func (s *Service) describeMembership(ctx context.Context, log *logr.Logger, project string) (*gkehubpb.Membership, error) {
	membership, err := s.scope.GkeHubMembershipClient().GetMembership(ctx, &gkehubpb.GetMembershipRequest{
		Name: s.membershipName(project),
	})
	if err != nil {
		var e *apierror.APIError
		if ok := errors.As(err, &e); ok {
			if e.GRPCStatus().Code() == codes.NotFound {
				return nil, nil
			}
		}
		log.Error(err, "Error getting fleet membership", "project", project)
		return nil, err
	}

	return membership, nil
}

func (s *Service) createMembership(ctx context.Context, project string) error {
	_, err := s.scope.GkeHubMembershipClient().CreateMembership(ctx, s.buildCreateMembershipRequest(project))
	return err
}

// buildCreateMembershipRequest builds the request to register the cluster as a
// Membership in project, pointing at the cluster via its GKE resource link.
func (s *Service) buildCreateMembershipRequest(project string) *gkehubpb.CreateMembershipRequest {
	return &gkehubpb.CreateMembershipRequest{
		Parent:       s.parent(project),
		MembershipId: s.scope.ClusterName(),
		Resource: &gkehubpb.Membership{
			Type: &gkehubpb.Membership_Endpoint{
				Endpoint: &gkehubpb.MembershipEndpoint{
					Type: &gkehubpb.MembershipEndpoint_GkeCluster{
						GkeCluster: &gkehubpb.GkeCluster{
							ResourceLink: "//container.googleapis.com/" + s.scope.ClusterFullName(),
						},
					},
				},
			},
		},
	}
}

// parent returns the `projects/*/locations/*` path Memberships in project live under.
func (s *Service) parent(project string) string {
	return fmt.Sprintf("projects/%s/locations/%s", project, s.scope.GCPManagedControlPlane.Spec.Location)
}

// membershipName returns the full `projects/*/locations/*/memberships/*` name of
// the Membership for this cluster in project.
func (s *Service) membershipName(project string) string {
	return fmt.Sprintf("%s/memberships/%s", s.parent(project), s.scope.ClusterName())
}
