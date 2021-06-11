// Code generated by mockery v2.4.0-beta. DO NOT EDIT.

// Re-generate by running 'make mock'

package topologyupdater

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"
)

// MockNodeTopologyClient is an autogenerated mock type for the NodeTopologyClient type
type MockNodeTopologyClient struct {
	mock.Mock
}

// UpdateNodeTopology provides a mock function with given fields: ctx, in, opts
func (_m *MockNodeTopologyClient) UpdateNodeTopology(ctx context.Context, in *NodeTopologyRequest, opts ...grpc.CallOption) (*NodeTopologyResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *NodeTopologyResponse
	if rf, ok := ret.Get(0).(func(context.Context, *NodeTopologyRequest, ...grpc.CallOption) *NodeTopologyResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*NodeTopologyResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *NodeTopologyRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
