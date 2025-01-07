// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package inbound

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// VLESSAPIClient is the client API for VLESSAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type VLESSAPIClient interface {
	AddUser(ctx context.Context, in *AddUserRequest, opts ...grpc.CallOption) (*AddUserResponse, error)
	RemoveUser(ctx context.Context, in *RemoveUserRequest, opts ...grpc.CallOption) (*RemoveUserResponse, error)
	GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error)
	GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error)
}

type vLESSAPIClient struct {
	cc grpc.ClientConnInterface
}

func NewVLESSAPIClient(cc grpc.ClientConnInterface) VLESSAPIClient {
	return &vLESSAPIClient{cc}
}

func (c *vLESSAPIClient) AddUser(ctx context.Context, in *AddUserRequest, opts ...grpc.CallOption) (*AddUserResponse, error) {
	out := new(AddUserResponse)
	err := c.cc.Invoke(ctx, "/xray.proxy.vless.inbound.VLESSAPI/AddUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vLESSAPIClient) RemoveUser(ctx context.Context, in *RemoveUserRequest, opts ...grpc.CallOption) (*RemoveUserResponse, error) {
	out := new(RemoveUserResponse)
	err := c.cc.Invoke(ctx, "/xray.proxy.vless.inbound.VLESSAPI/RemoveUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vLESSAPIClient) GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error) {
	out := new(GetUserResponse)
	err := c.cc.Invoke(ctx, "/xray.proxy.vless.inbound.VLESSAPI/GetUser", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vLESSAPIClient) GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error) {
	out := new(GetUsersResponse)
	err := c.cc.Invoke(ctx, "/xray.proxy.vless.inbound.VLESSAPI/GetUsers", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VLESSAPIServer is the server API for VLESSAPI service.
// All implementations must embed UnimplementedVLESSAPIServer
// for forward compatibility
type VLESSAPIServer interface {
	AddUser(context.Context, *AddUserRequest) (*AddUserResponse, error)
	RemoveUser(context.Context, *RemoveUserRequest) (*RemoveUserResponse, error)
	GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error)
	GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error)
	mustEmbedUnimplementedVLESSAPIServer()
}

// UnimplementedVLESSAPIServer must be embedded to have forward compatible implementations.
type UnimplementedVLESSAPIServer struct {
}

func (UnimplementedVLESSAPIServer) AddUser(context.Context, *AddUserRequest) (*AddUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddUser not implemented")
}
func (UnimplementedVLESSAPIServer) RemoveUser(context.Context, *RemoveUserRequest) (*RemoveUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveUser not implemented")
}
func (UnimplementedVLESSAPIServer) GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUser not implemented")
}
func (UnimplementedVLESSAPIServer) GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUsers not implemented")
}
func (UnimplementedVLESSAPIServer) mustEmbedUnimplementedVLESSAPIServer() {}

// UnsafeVLESSAPIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to VLESSAPIServer will
// result in compilation errors.
type UnsafeVLESSAPIServer interface {
	mustEmbedUnimplementedVLESSAPIServer()
}

func RegisterVLESSAPIServer(s grpc.ServiceRegistrar, srv VLESSAPIServer) {
	s.RegisterService(&VLESSAPI_ServiceDesc, srv)
}

func _VLESSAPI_AddUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VLESSAPIServer).AddUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/xray.proxy.vless.inbound.VLESSAPI/AddUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VLESSAPIServer).AddUser(ctx, req.(*AddUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VLESSAPI_RemoveUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VLESSAPIServer).RemoveUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/xray.proxy.vless.inbound.VLESSAPI/RemoveUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VLESSAPIServer).RemoveUser(ctx, req.(*RemoveUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VLESSAPI_GetUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VLESSAPIServer).GetUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/xray.proxy.vless.inbound.VLESSAPI/GetUser",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VLESSAPIServer).GetUser(ctx, req.(*GetUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VLESSAPI_GetUsers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUsersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VLESSAPIServer).GetUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/xray.proxy.vless.inbound.VLESSAPI/GetUsers",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VLESSAPIServer).GetUsers(ctx, req.(*GetUsersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// VLESSAPI_ServiceDesc is the grpc.ServiceDesc for VLESSAPI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var VLESSAPI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "xray.proxy.vless.inbound.VLESSAPI",
	HandlerType: (*VLESSAPIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AddUser",
			Handler:    _VLESSAPI_AddUser_Handler,
		},
		{
			MethodName: "RemoveUser",
			Handler:    _VLESSAPI_RemoveUser_Handler,
		},
		{
			MethodName: "GetUser",
			Handler:    _VLESSAPI_GetUser_Handler,
		},
		{
			MethodName: "GetUsers",
			Handler:    _VLESSAPI_GetUsers_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proxy/vless/inbound/config.proto",
}
