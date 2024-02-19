// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.1
// source: blocktx/blocktx_api/blocktx_api.proto

package blocktx_api

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	BlockTxAPI_Health_FullMethodName                    = "/blocktx_api.BlockTxAPI/Health"
	BlockTxAPI_RegisterTransaction_FullMethodName       = "/blocktx_api.BlockTxAPI/RegisterTransaction"
	BlockTxAPI_GetTransactionMerklePath_FullMethodName  = "/blocktx_api.BlockTxAPI/GetTransactionMerklePath"
	BlockTxAPI_GetTransactionBlocks_FullMethodName      = "/blocktx_api.BlockTxAPI/GetTransactionBlocks"
	BlockTxAPI_ClearTransactions_FullMethodName         = "/blocktx_api.BlockTxAPI/ClearTransactions"
	BlockTxAPI_ClearBlocks_FullMethodName               = "/blocktx_api.BlockTxAPI/ClearBlocks"
	BlockTxAPI_ClearBlockTransactionsMap_FullMethodName = "/blocktx_api.BlockTxAPI/ClearBlockTransactionsMap"
)

// BlockTxAPIClient is the client API for BlockTxAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BlockTxAPIClient interface {
	// Health returns the health of the API.
	Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error)
	// RegisterTransaction registers a transaction with the API.
	RegisterTransaction(ctx context.Context, in *TransactionAndSource, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// GetTransactionMerklePath returns the merkle path of a transaction.
	GetTransactionMerklePath(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*MerklePath, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlocks(ctx context.Context, in *Transactions, opts ...grpc.CallOption) (*TransactionBlocks, error)
	// ClearTransactions clears transaction data
	ClearTransactions(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error)
	// ClearBlocks clears block data
	ClearBlocks(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error)
	// ClearBlockTransactionsMap clears block-transaction-map data
	ClearBlockTransactionsMap(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error)
}

type blockTxAPIClient struct {
	cc grpc.ClientConnInterface
}

func NewBlockTxAPIClient(cc grpc.ClientConnInterface) BlockTxAPIClient {
	return &blockTxAPIClient{cc}
}

func (c *blockTxAPIClient) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, BlockTxAPI_Health_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) RegisterTransaction(ctx context.Context, in *TransactionAndSource, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, BlockTxAPI_RegisterTransaction_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetTransactionMerklePath(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*MerklePath, error) {
	out := new(MerklePath)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetTransactionMerklePath_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetTransactionBlocks(ctx context.Context, in *Transactions, opts ...grpc.CallOption) (*TransactionBlocks, error) {
	out := new(TransactionBlocks)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetTransactionBlocks_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) ClearTransactions(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error) {
	out := new(ClearDataResponse)
	err := c.cc.Invoke(ctx, BlockTxAPI_ClearTransactions_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) ClearBlocks(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error) {
	out := new(ClearDataResponse)
	err := c.cc.Invoke(ctx, BlockTxAPI_ClearBlocks_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) ClearBlockTransactionsMap(ctx context.Context, in *ClearData, opts ...grpc.CallOption) (*ClearDataResponse, error) {
	out := new(ClearDataResponse)
	err := c.cc.Invoke(ctx, BlockTxAPI_ClearBlockTransactionsMap_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BlockTxAPIServer is the server API for BlockTxAPI service.
// All implementations must embed UnimplementedBlockTxAPIServer
// for forward compatibility
type BlockTxAPIServer interface {
	// Health returns the health of the API.
	Health(context.Context, *emptypb.Empty) (*HealthResponse, error)
	// RegisterTransaction registers a transaction with the API.
	RegisterTransaction(context.Context, *TransactionAndSource) (*emptypb.Empty, error)
	// GetTransactionMerklePath returns the merkle path of a transaction.
	GetTransactionMerklePath(context.Context, *Transaction) (*MerklePath, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlocks(context.Context, *Transactions) (*TransactionBlocks, error)
	// ClearTransactions clears transaction data
	ClearTransactions(context.Context, *ClearData) (*ClearDataResponse, error)
	// ClearBlocks clears block data
	ClearBlocks(context.Context, *ClearData) (*ClearDataResponse, error)
	// ClearBlockTransactionsMap clears block-transaction-map data
	ClearBlockTransactionsMap(context.Context, *ClearData) (*ClearDataResponse, error)
	mustEmbedUnimplementedBlockTxAPIServer()
}

// UnimplementedBlockTxAPIServer must be embedded to have forward compatible implementations.
type UnimplementedBlockTxAPIServer struct {
}

func (UnimplementedBlockTxAPIServer) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Health not implemented")
}
func (UnimplementedBlockTxAPIServer) RegisterTransaction(context.Context, *TransactionAndSource) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterTransaction not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionMerklePath(context.Context, *Transaction) (*MerklePath, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionMerklePath not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionBlocks(context.Context, *Transactions) (*TransactionBlocks, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionBlocks not implemented")
}
func (UnimplementedBlockTxAPIServer) ClearTransactions(context.Context, *ClearData) (*ClearDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ClearTransactions not implemented")
}
func (UnimplementedBlockTxAPIServer) ClearBlocks(context.Context, *ClearData) (*ClearDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ClearBlocks not implemented")
}
func (UnimplementedBlockTxAPIServer) ClearBlockTransactionsMap(context.Context, *ClearData) (*ClearDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ClearBlockTransactionsMap not implemented")
}
func (UnimplementedBlockTxAPIServer) mustEmbedUnimplementedBlockTxAPIServer() {}

// UnsafeBlockTxAPIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BlockTxAPIServer will
// result in compilation errors.
type UnsafeBlockTxAPIServer interface {
	mustEmbedUnimplementedBlockTxAPIServer()
}

func RegisterBlockTxAPIServer(s grpc.ServiceRegistrar, srv BlockTxAPIServer) {
	s.RegisterService(&BlockTxAPI_ServiceDesc, srv)
}

func _BlockTxAPI_Health_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_Health_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).Health(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_RegisterTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TransactionAndSource)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).RegisterTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_RegisterTransaction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).RegisterTransaction(ctx, req.(*TransactionAndSource))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetTransactionMerklePath_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Transaction)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetTransactionMerklePath(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_GetTransactionMerklePath_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetTransactionMerklePath(ctx, req.(*Transaction))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetTransactionBlocks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Transactions)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetTransactionBlocks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_GetTransactionBlocks_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetTransactionBlocks(ctx, req.(*Transactions))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_ClearTransactions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearData)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).ClearTransactions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_ClearTransactions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).ClearTransactions(ctx, req.(*ClearData))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_ClearBlocks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearData)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).ClearBlocks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_ClearBlocks_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).ClearBlocks(ctx, req.(*ClearData))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_ClearBlockTransactionsMap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearData)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).ClearBlockTransactionsMap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_ClearBlockTransactionsMap_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).ClearBlockTransactionsMap(ctx, req.(*ClearData))
	}
	return interceptor(ctx, in, info, handler)
}

// BlockTxAPI_ServiceDesc is the grpc.ServiceDesc for BlockTxAPI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BlockTxAPI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "blocktx_api.BlockTxAPI",
	HandlerType: (*BlockTxAPIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Health",
			Handler:    _BlockTxAPI_Health_Handler,
		},
		{
			MethodName: "RegisterTransaction",
			Handler:    _BlockTxAPI_RegisterTransaction_Handler,
		},
		{
			MethodName: "GetTransactionMerklePath",
			Handler:    _BlockTxAPI_GetTransactionMerklePath_Handler,
		},
		{
			MethodName: "GetTransactionBlocks",
			Handler:    _BlockTxAPI_GetTransactionBlocks_Handler,
		},
		{
			MethodName: "ClearTransactions",
			Handler:    _BlockTxAPI_ClearTransactions_Handler,
		},
		{
			MethodName: "ClearBlocks",
			Handler:    _BlockTxAPI_ClearBlocks_Handler,
		},
		{
			MethodName: "ClearBlockTransactionsMap",
			Handler:    _BlockTxAPI_ClearBlockTransactionsMap_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "blocktx/blocktx_api/blocktx_api.proto",
}
