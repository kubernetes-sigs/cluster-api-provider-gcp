/*
Copyright 2021 The Kubernetes Authors.

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

package cloud

import (
	"context"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/googleapis/gax-go/v2"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

// Cloud alias for cloud.Cloud interface.
type Cloud = cloud.Cloud

// Reconciler is a generic interface used by components offering a type of service.
type Reconciler interface {
	Reconcile(ctx context.Context) error
	Delete(ctx context.Context) error
}

// ReconcilerWithResult is a generic interface used by components offering a type of service.
type ReconcilerWithResult interface {
	Reconcile(ctx context.Context) (ctrl.Result, error)
	Delete(ctx context.Context) (ctrl.Result, error)
}

// Client is an interface which can get cloud client.
type Client interface {
	Cloud() Cloud
	NetworkCloud() Cloud
}

// Container is an interface which implements bits of internalClusterManagerClient from since there is no Public Interface for it.
// ref: https://github.com/googleapis/google-cloud-go/blob/a187451a912835703078e5b6a339c514edebe5de/container/apiv1/cluster_manager_client.go#L468
type Container interface {
	Close() error
	CreateCluster(context.Context, *containerpb.CreateClusterRequest, ...gax.CallOption) (*containerpb.Operation, error)
	UpdateCluster(context.Context, *containerpb.UpdateClusterRequest, ...gax.CallOption) (*containerpb.Operation, error)
	DeleteCluster(context.Context, *containerpb.DeleteClusterRequest, ...gax.CallOption) (*containerpb.Operation, error)
	GetCluster(context.Context, *containerpb.GetClusterRequest, ...gax.CallOption) (*containerpb.Cluster, error)
	ListNodePools(context.Context, *containerpb.ListNodePoolsRequest, ...gax.CallOption) (*containerpb.ListNodePoolsResponse, error)
	GetNodePool(context.Context, *containerpb.GetNodePoolRequest, ...gax.CallOption) (*containerpb.NodePool, error)
	CreateNodePool(context.Context, *containerpb.CreateNodePoolRequest, ...gax.CallOption) (*containerpb.Operation, error)
	DeleteNodePool(context.Context, *containerpb.DeleteNodePoolRequest, ...gax.CallOption) (*containerpb.Operation, error)
	SetNodePoolSize(context.Context, *containerpb.SetNodePoolSizeRequest, ...gax.CallOption) (*containerpb.Operation, error)
	SetNodePoolAutoscaling(context.Context, *containerpb.SetNodePoolAutoscalingRequest, ...gax.CallOption) (*containerpb.Operation, error)
	UpdateNodePool(context.Context, *containerpb.UpdateNodePoolRequest, ...gax.CallOption) (*containerpb.Operation, error)
}

// ClusterGetter is an interface which can get cluster information.
type ClusterGetter interface {
	Client
	Project() string
	Region() string
	Name() string
	Namespace() string
	NetworkName() string
	NetworkProject() string
	IsSharedVpc() bool
	Network() *infrav1.Network
	AdditionalLabels() infrav1.Labels
	FailureDomains() clusterv1.FailureDomains
	ControlPlaneEndpoint() clusterv1.APIEndpoint
	ResourceManagerTags() infrav1.ResourceManagerTags
	LoadBalancer() infrav1.LoadBalancerSpec
}

// ClusterSetter is an interface which can set cluster information.
type ClusterSetter interface {
	SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint)
}

// Cluster is an interface which can get and set cluster information.
type Cluster interface {
	ClusterGetter
	ClusterSetter
}

// MachineGetter is an interface which can get machine information.
type MachineGetter interface {
	Client
	Name() string
	Namespace() string
	Zone() string
	Project() string
	Role() string
	IsControlPlane() bool
	ControlPlaneGroupName() string
	GetInstanceID() *string
	GetProviderID() string
	GetBootstrapData() (string, error)
	GetInstanceStatus() *infrav1.InstanceStatus
}

// MachineSetter is an interface which can set machine information.
type MachineSetter interface {
	SetProviderID()
	SetInstanceStatus(v infrav1.InstanceStatus)
	SetFailureMessage(v error)
	SetFailureReason(v capierrors.MachineStatusError)
	SetAnnotation(key, value string)
	SetAddresses(addressList []corev1.NodeAddress)
}

// Machine is an interface which can get and set machine information.
type Machine interface {
	MachineGetter
	MachineSetter
}
