// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.0
// source: internal/metamorph/metamorph_api/metamorph_api.proto

package metamorph_api

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	MetaMorphAPI_Health_FullMethodName               = "/metamorph_api.MetaMorphAPI/Health"
	MetaMorphAPI_PutTransaction_FullMethodName       = "/metamorph_api.MetaMorphAPI/PutTransaction"
	MetaMorphAPI_PutTransactions_FullMethodName      = "/metamorph_api.MetaMorphAPI/PutTransactions"
	MetaMorphAPI_GetTransaction_FullMethodName       = "/metamorph_api.MetaMorphAPI/GetTransaction"
	MetaMorphAPI_GetTransactions_FullMethodName      = "/metamorph_api.MetaMorphAPI/GetTransactions"
	MetaMorphAPI_GetTransactionStatus_FullMethodName = "/metamorph_api.MetaMorphAPI/GetTransactionStatus"
	MetaMorphAPI_SetUnlockedByName_FullMethodName    = "/metamorph_api.MetaMorphAPI/SetUnlockedByName"
	MetaMorphAPI_ClearData_FullMethodName            = "/metamorph_api.MetaMorphAPI/ClearData"
)

// MetaMorphAPIClient is the client API for MetaMorphAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MetaMorphAPIClient interface {
	Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error)
	PutTransaction(ctx context.Context, in *TransactionRequest, opts ...grpc.CallOption) (*TransactionStatus, error)
	PutTransactions(ctx context.Context, in *TransactionRequests, opts ...grpc.CallOption) (*TransactionStatuses, error)
	GetTransaction(ctx context.Context, in *TransactionStatusRequest, opts ...grpc.CallOption) (*Transaction, error)
	GetTransactions(ctx context.Context, in *TransactionsStatusRequest, opts ...grpc.CallOption) (*Transactions, error)
	GetTransactionStatus(ctx context.Context, in *TransactionStatusRequest, opts ...grpc.CallOption) (*TransactionStatus, error)
	SetUnlockedByName(ctx context.Context, in *SetUnlockedByNameRequest, opts ...grpc.CallOption) (*SetUnlockedByNameResponse, error)
	ClearData(ctx context.Context, in *ClearDataRequest, opts ...grpc.CallOption) (*ClearDataResponse, error)
}

type metaMorphAPIClient struct {
	cc grpc.ClientConnInterface
}

func NewMetaMorphAPIClient(cc grpc.ClientConnInterface) MetaMorphAPIClient {
	return &metaMorphAPIClient{cc}
}

func (c *metaMorphAPIClient) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, MetaMorphAPI_Health_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) PutTransaction(ctx context.Context, in *TransactionRequest, opts ...grpc.CallOption) (*TransactionStatus, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TransactionStatus)
	err := c.cc.Invoke(ctx, MetaMorphAPI_PutTransaction_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) PutTransactions(ctx context.Context, in *TransactionRequests, opts ...grpc.CallOption) (*TransactionStatuses, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TransactionStatuses)
	err := c.cc.Invoke(ctx, MetaMorphAPI_PutTransactions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) GetTransaction(ctx context.Context, in *TransactionStatusRequest, opts ...grpc.CallOption) (*Transaction, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Transaction)
	err := c.cc.Invoke(ctx, MetaMorphAPI_GetTransaction_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) GetTransactions(ctx context.Context, in *TransactionsStatusRequest, opts ...grpc.CallOption) (*Transactions, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Transactions)
	err := c.cc.Invoke(ctx, MetaMorphAPI_GetTransactions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) GetTransactionStatus(ctx context.Context, in *TransactionStatusRequest, opts ...grpc.CallOption) (*TransactionStatus, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TransactionStatus)
	err := c.cc.Invoke(ctx, MetaMorphAPI_GetTransactionStatus_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) SetUnlockedByName(ctx context.Context, in *SetUnlockedByNameRequest, opts ...grpc.CallOption) (*SetUnlockedByNameResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SetUnlockedByNameResponse)
	err := c.cc.Invoke(ctx, MetaMorphAPI_SetUnlockedByName_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *metaMorphAPIClient) ClearData(ctx context.Context, in *ClearDataRequest, opts ...grpc.CallOption) (*ClearDataResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClearDataResponse)
	err := c.cc.Invoke(ctx, MetaMorphAPI_ClearData_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MetaMorphAPIServer is the server API for MetaMorphAPI service.
// All implementations must embed UnimplementedMetaMorphAPIServer
// for forward compatibility.
type MetaMorphAPIServer interface {
	Health(context.Context, *emptypb.Empty) (*HealthResponse, error)
	PutTransaction(context.Context, *TransactionRequest) (*TransactionStatus, error)
	PutTransactions(context.Context, *TransactionRequests) (*TransactionStatuses, error)
	GetTransaction(context.Context, *TransactionStatusRequest) (*Transaction, error)
	GetTransactions(context.Context, *TransactionsStatusRequest) (*Transactions, error)
	GetTransactionStatus(context.Context, *TransactionStatusRequest) (*TransactionStatus, error)
	SetUnlockedByName(context.Context, *SetUnlockedByNameRequest) (*SetUnlockedByNameResponse, error)
	ClearData(context.Context, *ClearDataRequest) (*ClearDataResponse, error)
	mustEmbedUnimplementedMetaMorphAPIServer()
}

// UnimplementedMetaMorphAPIServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedMetaMorphAPIServer struct{}

func (UnimplementedMetaMorphAPIServer) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Health not implemented")
}
func (UnimplementedMetaMorphAPIServer) PutTransaction(context.Context, *TransactionRequest) (*TransactionStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PutTransaction not implemented")
}
func (UnimplementedMetaMorphAPIServer) PutTransactions(context.Context, *TransactionRequests) (*TransactionStatuses, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PutTransactions not implemented")
}
func (UnimplementedMetaMorphAPIServer) GetTransaction(context.Context, *TransactionStatusRequest) (*Transaction, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransaction not implemented")
}
func (UnimplementedMetaMorphAPIServer) GetTransactions(context.Context, *TransactionsStatusRequest) (*Transactions, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactions not implemented")
}
func (UnimplementedMetaMorphAPIServer) GetTransactionStatus(context.Context, *TransactionStatusRequest) (*TransactionStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionStatus not implemented")
}
func (UnimplementedMetaMorphAPIServer) SetUnlockedByName(context.Context, *SetUnlockedByNameRequest) (*SetUnlockedByNameResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUnlockedByName not implemented")
}
func (UnimplementedMetaMorphAPIServer) ClearData(context.Context, *ClearDataRequest) (*ClearDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ClearData not implemented")
}
func (UnimplementedMetaMorphAPIServer) mustEmbedUnimplementedMetaMorphAPIServer() {}
func (UnimplementedMetaMorphAPIServer) testEmbeddedByValue()                      {}

// UnsafeMetaMorphAPIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MetaMorphAPIServer will
// result in compilation errors.
type UnsafeMetaMorphAPIServer interface {
	mustEmbedUnimplementedMetaMorphAPIServer()
}

func RegisterMetaMorphAPIServer(s grpc.ServiceRegistrar, srv MetaMorphAPIServer) {
	// If the following call pancis, it indicates UnimplementedMetaMorphAPIServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&MetaMorphAPI_ServiceDesc, srv)
}

func _MetaMorphAPI_Health_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_Health_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).Health(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_PutTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).PutTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_PutTransaction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).PutTransaction(ctx, req.(*TransactionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_PutTransactions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionRequests)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).PutTransactions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_PutTransactions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).PutTransactions(ctx, req.(*TransactionRequests))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_GetTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).GetTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_GetTransaction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).GetTransaction(ctx, req.(*TransactionStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_GetTransactions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionsStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).GetTransactions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_GetTransactions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).GetTransactions(ctx, req.(*TransactionsStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_GetTransactionStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).GetTransactionStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_GetTransactionStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).GetTransactionStatus(ctx, req.(*TransactionStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_SetUnlockedByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetUnlockedByNameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).SetUnlockedByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_SetUnlockedByName_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).SetUnlockedByName(ctx, req.(*SetUnlockedByNameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MetaMorphAPI_ClearData_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearDataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetaMorphAPIServer).ClearData(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MetaMorphAPI_ClearData_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetaMorphAPIServer).ClearData(ctx, req.(*ClearDataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// MetaMorphAPI_ServiceDesc is the grpc.ServiceDesc for MetaMorphAPI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var MetaMorphAPI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "metamorph_api.MetaMorphAPI",
	HandlerType: (*MetaMorphAPIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Health",
			Handler:    _MetaMorphAPI_Health_Handler,
		},
		{
			MethodName: "PutTransaction",
			Handler:    _MetaMorphAPI_PutTransaction_Handler,
		},
		{
			MethodName: "PutTransactions",
			Handler:    _MetaMorphAPI_PutTransactions_Handler,
		},
		{
			MethodName: "GetTransaction",
			Handler:    _MetaMorphAPI_GetTransaction_Handler,
		},
		{
			MethodName: "GetTransactions",
			Handler:    _MetaMorphAPI_GetTransactions_Handler,
		},
		{
			MethodName: "GetTransactionStatus",
			Handler:    _MetaMorphAPI_GetTransactionStatus_Handler,
		},
		{
			MethodName: "SetUnlockedByName",
			Handler:    _MetaMorphAPI_SetUnlockedByName_Handler,
		},
		{
			MethodName: "ClearData",
			Handler:    _MetaMorphAPI_ClearData_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "internal/metamorph/metamorph_api/metamorph_api.proto",
}
