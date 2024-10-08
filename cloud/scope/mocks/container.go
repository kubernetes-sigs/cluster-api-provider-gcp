// Code generated by MockGen. DO NOT EDIT.
// Source: sigs.k8s.io/cluster-api-provider-gcp/cloud (interfaces: Container)
//
// Generated by this command:
//
//	mockgen -destination=cloud/scope/mocks/container.go -package=mocks sigs.k8s.io/cluster-api-provider-gcp/cloud Container
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	gax "github.com/googleapis/gax-go/v2"
	gomock "go.uber.org/mock/gomock"
)

// MockContainer is a mock of Container interface.
type MockContainer struct {
	ctrl     *gomock.Controller
	recorder *MockContainerMockRecorder
}

// MockContainerMockRecorder is the mock recorder for MockContainer.
type MockContainerMockRecorder struct {
	mock *MockContainer
}

// NewMockContainer creates a new mock instance.
func NewMockContainer(ctrl *gomock.Controller) *MockContainer {
	mock := &MockContainer{ctrl: ctrl}
	mock.recorder = &MockContainerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockContainer) EXPECT() *MockContainerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockContainer) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockContainerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockContainer)(nil).Close))
}

// CreateCluster mocks base method.
func (m *MockContainer) CreateCluster(arg0 context.Context, arg1 *containerpb.CreateClusterRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateCluster", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateCluster indicates an expected call of CreateCluster.
func (mr *MockContainerMockRecorder) CreateCluster(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCluster", reflect.TypeOf((*MockContainer)(nil).CreateCluster), varargs...)
}

// CreateNodePool mocks base method.
func (m *MockContainer) CreateNodePool(arg0 context.Context, arg1 *containerpb.CreateNodePoolRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CreateNodePool", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateNodePool indicates an expected call of CreateNodePool.
func (mr *MockContainerMockRecorder) CreateNodePool(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNodePool", reflect.TypeOf((*MockContainer)(nil).CreateNodePool), varargs...)
}

// DeleteCluster mocks base method.
func (m *MockContainer) DeleteCluster(arg0 context.Context, arg1 *containerpb.DeleteClusterRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteCluster", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteCluster indicates an expected call of DeleteCluster.
func (mr *MockContainerMockRecorder) DeleteCluster(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCluster", reflect.TypeOf((*MockContainer)(nil).DeleteCluster), varargs...)
}

// DeleteNodePool mocks base method.
func (m *MockContainer) DeleteNodePool(arg0 context.Context, arg1 *containerpb.DeleteNodePoolRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteNodePool", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteNodePool indicates an expected call of DeleteNodePool.
func (mr *MockContainerMockRecorder) DeleteNodePool(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNodePool", reflect.TypeOf((*MockContainer)(nil).DeleteNodePool), varargs...)
}

// GetCluster mocks base method.
func (m *MockContainer) GetCluster(arg0 context.Context, arg1 *containerpb.GetClusterRequest, arg2 ...gax.CallOption) (*containerpb.Cluster, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetCluster", varargs...)
	ret0, _ := ret[0].(*containerpb.Cluster)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCluster indicates an expected call of GetCluster.
func (mr *MockContainerMockRecorder) GetCluster(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCluster", reflect.TypeOf((*MockContainer)(nil).GetCluster), varargs...)
}

// GetNodePool mocks base method.
func (m *MockContainer) GetNodePool(arg0 context.Context, arg1 *containerpb.GetNodePoolRequest, arg2 ...gax.CallOption) (*containerpb.NodePool, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetNodePool", varargs...)
	ret0, _ := ret[0].(*containerpb.NodePool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNodePool indicates an expected call of GetNodePool.
func (mr *MockContainerMockRecorder) GetNodePool(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNodePool", reflect.TypeOf((*MockContainer)(nil).GetNodePool), varargs...)
}

// ListNodePools mocks base method.
func (m *MockContainer) ListNodePools(arg0 context.Context, arg1 *containerpb.ListNodePoolsRequest, arg2 ...gax.CallOption) (*containerpb.ListNodePoolsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListNodePools", varargs...)
	ret0, _ := ret[0].(*containerpb.ListNodePoolsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListNodePools indicates an expected call of ListNodePools.
func (mr *MockContainerMockRecorder) ListNodePools(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListNodePools", reflect.TypeOf((*MockContainer)(nil).ListNodePools), varargs...)
}

// SetNodePoolAutoscaling mocks base method.
func (m *MockContainer) SetNodePoolAutoscaling(arg0 context.Context, arg1 *containerpb.SetNodePoolAutoscalingRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetNodePoolAutoscaling", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetNodePoolAutoscaling indicates an expected call of SetNodePoolAutoscaling.
func (mr *MockContainerMockRecorder) SetNodePoolAutoscaling(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNodePoolAutoscaling", reflect.TypeOf((*MockContainer)(nil).SetNodePoolAutoscaling), varargs...)
}

// SetNodePoolSize mocks base method.
func (m *MockContainer) SetNodePoolSize(arg0 context.Context, arg1 *containerpb.SetNodePoolSizeRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetNodePoolSize", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetNodePoolSize indicates an expected call of SetNodePoolSize.
func (mr *MockContainerMockRecorder) SetNodePoolSize(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetNodePoolSize", reflect.TypeOf((*MockContainer)(nil).SetNodePoolSize), varargs...)
}

// UpdateCluster mocks base method.
func (m *MockContainer) UpdateCluster(arg0 context.Context, arg1 *containerpb.UpdateClusterRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateCluster", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateCluster indicates an expected call of UpdateCluster.
func (mr *MockContainerMockRecorder) UpdateCluster(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCluster", reflect.TypeOf((*MockContainer)(nil).UpdateCluster), varargs...)
}

// UpdateNodePool mocks base method.
func (m *MockContainer) UpdateNodePool(arg0 context.Context, arg1 *containerpb.UpdateNodePoolRequest, arg2 ...gax.CallOption) (*containerpb.Operation, error) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateNodePool", varargs...)
	ret0, _ := ret[0].(*containerpb.Operation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateNodePool indicates an expected call of UpdateNodePool.
func (mr *MockContainerMockRecorder) UpdateNodePool(arg0, arg1 any, arg2 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateNodePool", reflect.TypeOf((*MockContainer)(nil).UpdateNodePool), varargs...)
}
