// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.24.3
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
	BlockTxAPI_Health_FullMethodName                       = "/blocktx_api.BlockTxAPI/Health"
	BlockTxAPI_RegisterTransaction_FullMethodName          = "/blocktx_api.BlockTxAPI/RegisterTransaction"
	BlockTxAPI_LocateTransaction_FullMethodName            = "/blocktx_api.BlockTxAPI/LocateTransaction"
	BlockTxAPI_GetTransactionMerklePath_FullMethodName     = "/blocktx_api.BlockTxAPI/GetTransactionMerklePath"
	BlockTxAPI_GetBlockTransactions_FullMethodName         = "/blocktx_api.BlockTxAPI/GetBlockTransactions"
	BlockTxAPI_GetTransactionBlock_FullMethodName          = "/blocktx_api.BlockTxAPI/GetTransactionBlock"
	BlockTxAPI_GetTransactionBlocks_FullMethodName         = "/blocktx_api.BlockTxAPI/GetTransactionBlocks"
	BlockTxAPI_GetBlock_FullMethodName                     = "/blocktx_api.BlockTxAPI/GetBlock"
	BlockTxAPI_GetBlockForHeight_FullMethodName            = "/blocktx_api.BlockTxAPI/GetBlockForHeight"
	BlockTxAPI_GetLastProcessedBlock_FullMethodName        = "/blocktx_api.BlockTxAPI/GetLastProcessedBlock"
	BlockTxAPI_GetMinedTransactionsForBlock_FullMethodName = "/blocktx_api.BlockTxAPI/GetMinedTransactionsForBlock"
	BlockTxAPI_GetBlockNotificationStream_FullMethodName   = "/blocktx_api.BlockTxAPI/GetBlockNotificationStream"
)

// BlockTxAPIClient is the client API for BlockTxAPI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BlockTxAPIClient interface {
	// Health returns the health of the API.
	Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HealthResponse, error)
	// RegisterTransaction registers a transaction with the API.
	RegisterTransaction(ctx context.Context, in *TransactionAndSource, opts ...grpc.CallOption) (*RegisterTransactionResponse, error)
	// LocateTransaction returns the source of a transaction.
	LocateTransaction(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Source, error)
	// GetTransactionMerklePath returns the merkle path of a transaction.
	GetTransactionMerklePath(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*MerklePath, error)
	// GetBlockTransactions returns a list of transaction hashes for a given block.
	GetBlockTransactions(ctx context.Context, in *Block, opts ...grpc.CallOption) (*Transactions, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlock(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Block, error)
	GetTransactionBlocks(ctx context.Context, in *Transactions, opts ...grpc.CallOption) (*TransactionBlocks, error)
	// GetBlock returns the non-orphaned block for a given block hash.
	GetBlock(ctx context.Context, in *Hash, opts ...grpc.CallOption) (*Block, error)
	// GetBlockForHeight returns the non-orphaned block for a given block height.
	GetBlockForHeight(ctx context.Context, in *Height, opts ...grpc.CallOption) (*Block, error)
	// GetLastProcessedBlock returns the last processed block.
	GetLastProcessedBlock(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*Block, error)
	// GetMyTransactionsForBlock returns a list of transaction hashes for a given block that were registered by this API.
	GetMinedTransactionsForBlock(ctx context.Context, in *BlockAndSource, opts ...grpc.CallOption) (*MinedTransactions, error)
	// GetBlockNotificationStream returns a stream of mined blocks starting at a specific block height.
	// If Height is 0, the stream starts from the current best block.
	GetBlockNotificationStream(ctx context.Context, in *Height, opts ...grpc.CallOption) (BlockTxAPI_GetBlockNotificationStreamClient, error)
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

func (c *blockTxAPIClient) RegisterTransaction(ctx context.Context, in *TransactionAndSource, opts ...grpc.CallOption) (*RegisterTransactionResponse, error) {
	out := new(RegisterTransactionResponse)
	err := c.cc.Invoke(ctx, BlockTxAPI_RegisterTransaction_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) LocateTransaction(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Source, error) {
	out := new(Source)
	err := c.cc.Invoke(ctx, BlockTxAPI_LocateTransaction_FullMethodName, in, out, opts...)
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

func (c *blockTxAPIClient) GetBlockTransactions(ctx context.Context, in *Block, opts ...grpc.CallOption) (*Transactions, error) {
	out := new(Transactions)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetBlockTransactions_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetTransactionBlock(ctx context.Context, in *Transaction, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetTransactionBlock_FullMethodName, in, out, opts...)
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

func (c *blockTxAPIClient) GetBlock(ctx context.Context, in *Hash, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetBlock_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetBlockForHeight(ctx context.Context, in *Height, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetBlockForHeight_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetLastProcessedBlock(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*Block, error) {
	out := new(Block)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetLastProcessedBlock_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetMinedTransactionsForBlock(ctx context.Context, in *BlockAndSource, opts ...grpc.CallOption) (*MinedTransactions, error) {
	out := new(MinedTransactions)
	err := c.cc.Invoke(ctx, BlockTxAPI_GetMinedTransactionsForBlock_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blockTxAPIClient) GetBlockNotificationStream(ctx context.Context, in *Height, opts ...grpc.CallOption) (BlockTxAPI_GetBlockNotificationStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &BlockTxAPI_ServiceDesc.Streams[0], BlockTxAPI_GetBlockNotificationStream_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &blockTxAPIGetBlockNotificationStreamClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type BlockTxAPI_GetBlockNotificationStreamClient interface {
	Recv() (*Block, error)
	grpc.ClientStream
}

type blockTxAPIGetBlockNotificationStreamClient struct {
	grpc.ClientStream
}

func (x *blockTxAPIGetBlockNotificationStreamClient) Recv() (*Block, error) {
	m := new(Block)
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
	// RegisterTransaction registers a transaction with the API.
	RegisterTransaction(context.Context, *TransactionAndSource) (*RegisterTransactionResponse, error)
	// LocateTransaction returns the source of a transaction.
	LocateTransaction(context.Context, *Transaction) (*Source, error)
	// GetTransactionMerklePath returns the merkle path of a transaction.
	GetTransactionMerklePath(context.Context, *Transaction) (*MerklePath, error)
	// GetBlockTransactions returns a list of transaction hashes for a given block.
	GetBlockTransactions(context.Context, *Block) (*Transactions, error)
	// GetTransactionBlocks returns a list of block hashes (excluding orphaned) for a given transaction hash.
	GetTransactionBlock(context.Context, *Transaction) (*Block, error)
	GetTransactionBlocks(context.Context, *Transactions) (*TransactionBlocks, error)
	// GetBlock returns the non-orphaned block for a given block hash.
	GetBlock(context.Context, *Hash) (*Block, error)
	// GetBlockForHeight returns the non-orphaned block for a given block height.
	GetBlockForHeight(context.Context, *Height) (*Block, error)
	// GetLastProcessedBlock returns the last processed block.
	GetLastProcessedBlock(context.Context, *emptypb.Empty) (*Block, error)
	// GetMyTransactionsForBlock returns a list of transaction hashes for a given block that were registered by this API.
	GetMinedTransactionsForBlock(context.Context, *BlockAndSource) (*MinedTransactions, error)
	// GetBlockNotificationStream returns a stream of mined blocks starting at a specific block height.
	// If Height is 0, the stream starts from the current best block.
	GetBlockNotificationStream(*Height, BlockTxAPI_GetBlockNotificationStreamServer) error
	mustEmbedUnimplementedBlockTxAPIServer()
}

// UnimplementedBlockTxAPIServer must be embedded to have forward compatible implementations.
type UnimplementedBlockTxAPIServer struct {
}

func (UnimplementedBlockTxAPIServer) Health(context.Context, *emptypb.Empty) (*HealthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Health not implemented")
}
func (UnimplementedBlockTxAPIServer) RegisterTransaction(context.Context, *TransactionAndSource) (*RegisterTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterTransaction not implemented")
}
func (UnimplementedBlockTxAPIServer) LocateTransaction(context.Context, *Transaction) (*Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LocateTransaction not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionMerklePath(context.Context, *Transaction) (*MerklePath, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionMerklePath not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlockTransactions(context.Context, *Block) (*Transactions, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockTransactions not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionBlock(context.Context, *Transaction) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionBlock not implemented")
}
func (UnimplementedBlockTxAPIServer) GetTransactionBlocks(context.Context, *Transactions) (*TransactionBlocks, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransactionBlocks not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlock(context.Context, *Hash) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlock not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlockForHeight(context.Context, *Height) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockForHeight not implemented")
}
func (UnimplementedBlockTxAPIServer) GetLastProcessedBlock(context.Context, *emptypb.Empty) (*Block, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetLastProcessedBlock not implemented")
}
func (UnimplementedBlockTxAPIServer) GetMinedTransactionsForBlock(context.Context, *BlockAndSource) (*MinedTransactions, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMinedTransactionsForBlock not implemented")
}
func (UnimplementedBlockTxAPIServer) GetBlockNotificationStream(*Height, BlockTxAPI_GetBlockNotificationStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method GetBlockNotificationStream not implemented")
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

func _BlockTxAPI_LocateTransaction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Transaction)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).LocateTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_LocateTransaction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).LocateTransaction(ctx, req.(*Transaction))
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
		FullMethod: BlockTxAPI_GetBlockTransactions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetBlockTransactions(ctx, req.(*Block))
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
		FullMethod: BlockTxAPI_GetTransactionBlock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetTransactionBlock(ctx, req.(*Transaction))
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

func _BlockTxAPI_GetBlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Hash)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetBlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_GetBlock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetBlock(ctx, req.(*Hash))
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
		FullMethod: BlockTxAPI_GetBlockForHeight_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetBlockForHeight(ctx, req.(*Height))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetLastProcessedBlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetLastProcessedBlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_GetLastProcessedBlock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetLastProcessedBlock(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetMinedTransactionsForBlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BlockAndSource)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockTxAPIServer).GetMinedTransactionsForBlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BlockTxAPI_GetMinedTransactionsForBlock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockTxAPIServer).GetMinedTransactionsForBlock(ctx, req.(*BlockAndSource))
	}
	return interceptor(ctx, in, info, handler)
}

func _BlockTxAPI_GetBlockNotificationStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Height)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(BlockTxAPIServer).GetBlockNotificationStream(m, &blockTxAPIGetBlockNotificationStreamServer{stream})
}

type BlockTxAPI_GetBlockNotificationStreamServer interface {
	Send(*Block) error
	grpc.ServerStream
}

type blockTxAPIGetBlockNotificationStreamServer struct {
	grpc.ServerStream
}

func (x *blockTxAPIGetBlockNotificationStreamServer) Send(m *Block) error {
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
			MethodName: "RegisterTransaction",
			Handler:    _BlockTxAPI_RegisterTransaction_Handler,
		},
		{
			MethodName: "LocateTransaction",
			Handler:    _BlockTxAPI_LocateTransaction_Handler,
		},
		{
			MethodName: "GetTransactionMerklePath",
			Handler:    _BlockTxAPI_GetTransactionMerklePath_Handler,
		},
		{
			MethodName: "GetBlockTransactions",
			Handler:    _BlockTxAPI_GetBlockTransactions_Handler,
		},
		{
			MethodName: "GetTransactionBlock",
			Handler:    _BlockTxAPI_GetTransactionBlock_Handler,
		},
		{
			MethodName: "GetTransactionBlocks",
			Handler:    _BlockTxAPI_GetTransactionBlocks_Handler,
		},
		{
			MethodName: "GetBlock",
			Handler:    _BlockTxAPI_GetBlock_Handler,
		},
		{
			MethodName: "GetBlockForHeight",
			Handler:    _BlockTxAPI_GetBlockForHeight_Handler,
		},
		{
			MethodName: "GetLastProcessedBlock",
			Handler:    _BlockTxAPI_GetLastProcessedBlock_Handler,
		},
		{
			MethodName: "GetMinedTransactionsForBlock",
			Handler:    _BlockTxAPI_GetMinedTransactionsForBlock_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetBlockNotificationStream",
			Handler:       _BlockTxAPI_GetBlockNotificationStream_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "blocktx/blocktx_api/blocktx_api.proto",
}
