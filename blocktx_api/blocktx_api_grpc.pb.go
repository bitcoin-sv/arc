// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.9
// source: blocktx_api/blocktx_api.proto

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

// BlockTxAPIClient is the client API for BlockTxAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BlockTxAPIClient interface {
	// Health returns the health of the API.
	Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error)
	// GetBlockTransactions returns a list of transaction hashes for a given block.
	GetBlockTransactions(ctx context.Context, in *Block, opts ...grpc.CallOption) (*Transactions, error)
	// GetTransactionBlocks returns a list of block hashes (including orphaned) for a given transaction hash.
	GetTransactionBlocks(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Blocks, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlock(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Block, error)
	// GetBlockForHeight returns the non-orphaned block for a given block height.
	GetBlockForHeight(ctx context.Context, in *Height, opts ...grpc.CallOption) (*Block, error)
	// GetMinedBlockTransactions returns a stream of mined transactions starting at a specific block height.
	// If Height is 0, the stream starts from the current best block.
	GetMinedBlockTransactions(ctx context.Context, in *Height, opts ...grpc.CallOption) (BlockTxAPI_GetMinedBlockTransactionsClient, error)
}

type blockTxAPIClient struct {
	cc grpc.ClientConnInterface
}

func NewBlockTxAPIClient(cc grpc.ClientConnInterface) BlockTxAPIClient {
	return &blockTxAPIClient{cc}
}

func (c *blockTxAPIClient) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, "/blocktx_api.BlockTxAPI/Health", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetBlockTransactions(ctx context.Context, in *Block, opts ...grpc.CallOption) (*Transactions, error) {
	out := new(Transactions)
	err := c.cc.Invoke(ctx, "/blocktx_api.BlockTxAPI/GetBlockTransactions", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetTransactionBlocks(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Blocks, error) {
	out := new(Blocks)
	err := c.cc.Invoke(ctx, "/blocktx_api.BlockTxAPI/GetTransactionBlocks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetTransactionBlock(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, "/blocktx_api.BlockTxAPI/GetTransactionBlock", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetBlockForHeight(ctx context.Context, in *Height, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, "/blocktx_api.BlockTxAPI/GetBlockForHeight", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetMinedBlockTransactions(ctx context.Context, in *Height, opts ...grpc.CallOption) (BlockTxAPI_GetMinedBlockTransactionsClient, error) {
	stream, err := c.cc.NewStream(ctx, &BlockTxAPI_ServiceDesc.Streams[0], "/blocktx_api.BlockTxAPI/GetMinedBlockTransactions", opts...)
	if err != nil {
		return nil, err
	}
	x := &blockTxAPIGetMinedBlockTransactionsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type BlockTxAPI_GetMinedBlockTransactionsClient interface {
	Recv() (*MinedTransaction, error)
	grpc.ClientStream
}

type blockTxAPIGetMinedBlockTransactionsClient struct {
	grpc.ClientStream
}

func (x *blockTxAPIGetMinedBlockTransactionsClient) Recv() (*MinedTransaction, error) {
	m := new(MinedTransaction)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// BlockTxAPIServer is the server API for BlockTxAPI service.
// All implementations must embed UnimplementedBlockTxAPIServer
// for forward compatibility
type BlockTxAPIServer interface {
	// Health returns the health of the API.
	Health(context.Context, *emptypb.Empty) (*HealthResponse, error)
	// GetBlockTransactions returns a list of transaction hashes for a given block.
	GetBlockTransactions(context.Context, *Block) (*Transactions, error)
	// GetTransactionBlocks returns a list of block hashes (including orphaned) for a given transaction hash.
	GetTransactionBlocks(context.Context, *Transaction) (*Blocks, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlock(context.Context, *Transaction) (*Block, error)
	// GetBlockForHeight returns the non-orphaned block for a given block height.
	GetBlockForHeight(context.Context, *Height) (*Block, error)
	// GetMinedBlockTransactions returns a stream of mined transactions starting at a specific block height.
	// If Height is 0, the stream starts from the current best block.
	GetMinedBlockTransactions(*Height, BlockTxAPI_GetMinedBlockTransactionsServer) error
	mustEmbedUnimplementedBlockTxAPIServer()
}

// UnimplementedBlockTxAPIServer must be embedded to have forward compatible implementations.
type UnimplementedBlockTxAPIServer struct {
}

func (UnimplementedBlockTxAPIServer) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Health not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlockTransactions(context.Context, *Block) (*Transactions, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockTransactions not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionBlocks(context.Context, *Transaction) (*Blocks, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionBlocks not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionBlock(context.Context, *Transaction) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionBlock not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlockForHeight(context.Context, *Height) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockForHeight not implemented")
}
func (UnimplementedBlockTxAPIServer) GetMinedBlockTransactions(*Height, BlockTxAPI_GetMinedBlockTransactionsServer) error {
	return status.Errorf(codes.Unimplemented, "method GetMinedBlockTransactions not implemented")
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
		FullMethod: "/blocktx_api.BlockTxAPI/Health",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).Health(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetBlockTransactions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Block)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetBlockTransactions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blocktx_api.BlockTxAPI/GetBlockTransactions",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetBlockTransactions(ctx, req.(*Block))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetTransactionBlocks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Transaction)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetTransactionBlocks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blocktx_api.BlockTxAPI/GetTransactionBlocks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetTransactionBlocks(ctx, req.(*Transaction))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetTransactionBlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Transaction)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetTransactionBlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blocktx_api.BlockTxAPI/GetTransactionBlock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetTransactionBlock(ctx, req.(*Transaction))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetBlockForHeight_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Height)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetBlockForHeight(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blocktx_api.BlockTxAPI/GetBlockForHeight",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetBlockForHeight(ctx, req.(*Height))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetMinedBlockTransactions_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Height)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(BlockTxAPIServer).GetMinedBlockTransactions(m, &blockTxAPIGetMinedBlockTransactionsServer{stream})
}

type BlockTxAPI_GetMinedBlockTransactionsServer interface {
	Send(*MinedTransaction) error
	grpc.ServerStream
}

type blockTxAPIGetMinedBlockTransactionsServer struct {
	grpc.ServerStream
}

func (x *blockTxAPIGetMinedBlockTransactionsServer) Send(m *MinedTransaction) error {
	return x.ServerStream.SendMsg(m)
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
			MethodName: "GetBlockTransactions",
			Handler:    _BlockTxAPI_GetBlockTransactions_Handler,
		},
		{
			MethodName: "GetTransactionBlocks",
			Handler:    _BlockTxAPI_GetTransactionBlocks_Handler,
		},
		{
			MethodName: "GetTransactionBlock",
			Handler:    _BlockTxAPI_GetTransactionBlock_Handler,
		},
		{
			MethodName: "GetBlockForHeight",
			Handler:    _BlockTxAPI_GetBlockForHeight_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetMinedBlockTransactions",
			Handler:       _BlockTxAPI_GetMinedBlockTransactions_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "blocktx_api/blocktx_api.proto",
}
