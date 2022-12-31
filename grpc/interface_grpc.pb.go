// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.11.4
// source: interface.proto

package RA

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

// RAClient is the client API for RA service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RAClient interface {
	Request(ctx context.Context, in *Info, opts ...grpc.CallOption) (*Empty, error)
	Reply(ctx context.Context, in *Id, opts ...grpc.CallOption) (*Empty, error)
}

type rAClient struct {
	cc grpc.ClientConnInterface
}

func NewRAClient(cc grpc.ClientConnInterface) RAClient {
	return &rAClient{cc}
}

func (c *rAClient) Request(ctx context.Context, in *Info, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/proto.RA/Request", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rAClient) Reply(ctx context.Context, in *Id, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/proto.RA/Reply", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RAServer is the server API for RA service.
// All implementations must embed UnimplementedRAServer
// for forward compatibility
type RAServer interface {
	Request(context.Context, *Info) (*Empty, error)
	Reply(context.Context, *Id) (*Empty, error)
	mustEmbedUnimplementedRAServer()
}

// UnimplementedRAServer must be embedded to have forward compatible implementations.
type UnimplementedRAServer struct {
}

func (UnimplementedRAServer) Request(context.Context, *Info) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Request not implemented")
}
func (UnimplementedRAServer) Reply(context.Context, *Id) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Reply not implemented")
}
func (UnimplementedRAServer) mustEmbedUnimplementedRAServer() {}

// UnsafeRAServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RAServer will
// result in compilation errors.
type UnsafeRAServer interface {
	mustEmbedUnimplementedRAServer()
}

func RegisterRAServer(s grpc.ServiceRegistrar, srv RAServer) {
	s.RegisterService(&RA_ServiceDesc, srv)
}

func _RA_Request_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Info)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RAServer).Request(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.RA/Request",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RAServer).Request(ctx, req.(*Info))
	}
	return interceptor(ctx, in, info, handler)
}

func _RA_Reply_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RAServer).Reply(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.RA/Reply",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RAServer).Reply(ctx, req.(*Id))
	}
	return interceptor(ctx, in, info, handler)
}

// RA_ServiceDesc is the grpc.ServiceDesc for RA service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var RA_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "proto.RA",
	HandlerType: (*RAServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Request",
			Handler:    _RA_Request_Handler,
		},
		{
			MethodName: "Reply",
			Handler:    _RA_Reply_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "interface.proto",
}
