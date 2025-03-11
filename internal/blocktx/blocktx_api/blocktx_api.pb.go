// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: internal/blocktx/blocktx_api/blocktx_api.proto

package blocktx_api

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Statuses have a difference between them in case
// it's necessary to add more values in between.
type Status int32

const (
	Status_UNKNOWN  Status = 0 // This value is unused for now, but it's required by protobuf to have enum field with value 0
	Status_LONGEST  Status = 10
	Status_STALE    Status = 20
	Status_ORPHANED Status = 30
)

// Enum value maps for Status.
var (
	Status_name = map[int32]string{
		0:  "UNKNOWN",
		10: "LONGEST",
		20: "STALE",
		30: "ORPHANED",
	}
	Status_value = map[string]int32{
		"UNKNOWN":  0,
		"LONGEST":  10,
		"STALE":    20,
		"ORPHANED": 30,
	}
)

func (x Status) Enum() *Status {
	p := new(Status)
	*p = x
	return p
}

func (x Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Status) Descriptor() protoreflect.EnumDescriptor {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_enumTypes[0].Descriptor()
}

func (Status) Type() protoreflect.EnumType {
	return &file_internal_blocktx_blocktx_api_blocktx_api_proto_enumTypes[0]
}

func (x Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Status.Descriptor instead.
func (Status) EnumDescriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{0}
}

// swagger:model HealthResponse
type HealthResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Ok            bool                   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Details       string                 `protobuf:"bytes,2,opt,name=details,proto3" json:"details,omitempty"`
	Timestamp     *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Nats          string                 `protobuf:"bytes,4,opt,name=nats,proto3" json:"nats,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthResponse) Reset() {
	*x = HealthResponse{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthResponse) ProtoMessage() {}

func (x *HealthResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthResponse.ProtoReflect.Descriptor instead.
func (*HealthResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{0}
}

func (x *HealthResponse) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}

func (x *HealthResponse) GetDetails() string {
	if x != nil {
		return x.Details
	}
	return ""
}

func (x *HealthResponse) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *HealthResponse) GetNats() string {
	if x != nil {
		return x.Nats
	}
	return ""
}

// swagger:model Block {
type Block struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Hash          []byte                 `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`                                     // Little endian
	PreviousHash  []byte                 `protobuf:"bytes,2,opt,name=previous_hash,json=previousHash,proto3" json:"previous_hash,omitempty"` // Little endian
	MerkleRoot    []byte                 `protobuf:"bytes,3,opt,name=merkle_root,json=merkleRoot,proto3" json:"merkle_root,omitempty"`       // Little endian
	Height        uint64                 `protobuf:"varint,4,opt,name=height,proto3" json:"height,omitempty"`
	Processed     bool                   `protobuf:"varint,5,opt,name=processed,proto3" json:"processed,omitempty"`
	Status        Status                 `protobuf:"varint,6,opt,name=status,proto3,enum=blocktx_api.Status" json:"status,omitempty"`
	Chainwork     string                 `protobuf:"bytes,7,opt,name=chainwork,proto3" json:"chainwork,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Block) Reset() {
	*x = Block{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
func (*Block) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{1}
}

func (x *Block) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Block) GetPreviousHash() []byte {
	if x != nil {
		return x.PreviousHash
	}
	return nil
}

func (x *Block) GetMerkleRoot() []byte {
	if x != nil {
		return x.MerkleRoot
	}
	return nil
}

func (x *Block) GetHeight() uint64 {
	if x != nil {
		return x.Height
	}
	return 0
}

func (x *Block) GetProcessed() bool {
	if x != nil {
		return x.Processed
	}
	return false
}

func (x *Block) GetStatus() Status {
	if x != nil {
		return x.Status
	}
	return Status_UNKNOWN
}

func (x *Block) GetChainwork() string {
	if x != nil {
		return x.Chainwork
	}
	return ""
}

// swagger:model Transactions
type Transactions struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Transactions  []*Transaction         `protobuf:"bytes,1,rep,name=transactions,proto3" json:"transactions,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Transactions) Reset() {
	*x = Transactions{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transactions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transactions) ProtoMessage() {}

func (x *Transactions) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Transactions.ProtoReflect.Descriptor instead.
func (*Transactions) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{2}
}

func (x *Transactions) GetTransactions() []*Transaction {
	if x != nil {
		return x.Transactions
	}
	return nil
}

type TransactionBlock struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	BlockHash       []byte                 `protobuf:"bytes,1,opt,name=block_hash,json=blockHash,proto3" json:"block_hash,omitempty"` // Little endian
	BlockHeight     uint64                 `protobuf:"varint,2,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
	TransactionHash []byte                 `protobuf:"bytes,3,opt,name=transaction_hash,json=transactionHash,proto3" json:"transaction_hash,omitempty"` // Little endian
	MerklePath      string                 `protobuf:"bytes,4,opt,name=merklePath,proto3" json:"merklePath,omitempty"`
	BlockStatus     Status                 `protobuf:"varint,5,opt,name=block_status,json=blockStatus,proto3,enum=blocktx_api.Status" json:"block_status,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *TransactionBlock) Reset() {
	*x = TransactionBlock{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransactionBlock) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransactionBlock) ProtoMessage() {}

func (x *TransactionBlock) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransactionBlock.ProtoReflect.Descriptor instead.
func (*TransactionBlock) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{3}
}

func (x *TransactionBlock) GetBlockHash() []byte {
	if x != nil {
		return x.BlockHash
	}
	return nil
}

func (x *TransactionBlock) GetBlockHeight() uint64 {
	if x != nil {
		return x.BlockHeight
	}
	return 0
}

func (x *TransactionBlock) GetTransactionHash() []byte {
	if x != nil {
		return x.TransactionHash
	}
	return nil
}

func (x *TransactionBlock) GetMerklePath() string {
	if x != nil {
		return x.MerklePath
	}
	return ""
}

func (x *TransactionBlock) GetBlockStatus() Status {
	if x != nil {
		return x.BlockStatus
	}
	return Status_UNKNOWN
}

type TransactionBlocks struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	TransactionBlocks []*TransactionBlock    `protobuf:"bytes,1,rep,name=transaction_blocks,json=transactionBlocks,proto3" json:"transaction_blocks,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *TransactionBlocks) Reset() {
	*x = TransactionBlocks{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransactionBlocks) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransactionBlocks) ProtoMessage() {}

func (x *TransactionBlocks) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransactionBlocks.ProtoReflect.Descriptor instead.
func (*TransactionBlocks) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{4}
}

func (x *TransactionBlocks) GetTransactionBlocks() []*TransactionBlock {
	if x != nil {
		return x.TransactionBlocks
	}
	return nil
}

// swagger:model Transaction
type Transaction struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Hash          []byte                 `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"` // Little endian
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Transaction) Reset() {
	*x = Transaction{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transaction) ProtoMessage() {}

func (x *Transaction) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Transaction.ProtoReflect.Descriptor instead.
func (*Transaction) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{5}
}

func (x *Transaction) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

// swagger:model ClearData
type ClearData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RetentionDays int32                  `protobuf:"varint,1,opt,name=retentionDays,proto3" json:"retentionDays,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ClearData) Reset() {
	*x = ClearData{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClearData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClearData) ProtoMessage() {}

func (x *ClearData) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClearData.ProtoReflect.Descriptor instead.
func (*ClearData) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{6}
}

func (x *ClearData) GetRetentionDays() int32 {
	if x != nil {
		return x.RetentionDays
	}
	return 0
}

// swagger:model RowsAffectedResponse
type RowsAffectedResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Rows          int64                  `protobuf:"varint,1,opt,name=rows,proto3" json:"rows,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RowsAffectedResponse) Reset() {
	*x = RowsAffectedResponse{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RowsAffectedResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RowsAffectedResponse) ProtoMessage() {}

func (x *RowsAffectedResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RowsAffectedResponse.ProtoReflect.Descriptor instead.
func (*RowsAffectedResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{7}
}

func (x *RowsAffectedResponse) GetRows() int64 {
	if x != nil {
		return x.Rows
	}
	return 0
}

type DelUnfinishedBlockProcessingRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProcessedBy   string                 `protobuf:"bytes,1,opt,name=processed_by,json=processedBy,proto3" json:"processed_by,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DelUnfinishedBlockProcessingRequest) Reset() {
	*x = DelUnfinishedBlockProcessingRequest{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DelUnfinishedBlockProcessingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DelUnfinishedBlockProcessingRequest) ProtoMessage() {}

func (x *DelUnfinishedBlockProcessingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DelUnfinishedBlockProcessingRequest.ProtoReflect.Descriptor instead.
func (*DelUnfinishedBlockProcessingRequest) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{8}
}

func (x *DelUnfinishedBlockProcessingRequest) GetProcessedBy() string {
	if x != nil {
		return x.ProcessedBy
	}
	return ""
}

type MerkleRootVerificationRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	MerkleRoot    string                 `protobuf:"bytes,1,opt,name=merkle_root,json=merkleRoot,proto3" json:"merkle_root,omitempty"`
	BlockHeight   uint64                 `protobuf:"varint,2,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MerkleRootVerificationRequest) Reset() {
	*x = MerkleRootVerificationRequest{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootVerificationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootVerificationRequest) ProtoMessage() {}

func (x *MerkleRootVerificationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MerkleRootVerificationRequest.ProtoReflect.Descriptor instead.
func (*MerkleRootVerificationRequest) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{9}
}

func (x *MerkleRootVerificationRequest) GetMerkleRoot() string {
	if x != nil {
		return x.MerkleRoot
	}
	return ""
}

func (x *MerkleRootVerificationRequest) GetBlockHeight() uint64 {
	if x != nil {
		return x.BlockHeight
	}
	return 0
}

type MerkleRootsVerificationRequest struct {
	state         protoimpl.MessageState           `protogen:"open.v1"`
	MerkleRoots   []*MerkleRootVerificationRequest `protobuf:"bytes,1,rep,name=merkle_roots,json=merkleRoots,proto3" json:"merkle_roots,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *MerkleRootsVerificationRequest) Reset() {
	*x = MerkleRootsVerificationRequest{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootsVerificationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootsVerificationRequest) ProtoMessage() {}

func (x *MerkleRootsVerificationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MerkleRootsVerificationRequest.ProtoReflect.Descriptor instead.
func (*MerkleRootsVerificationRequest) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{10}
}

func (x *MerkleRootsVerificationRequest) GetMerkleRoots() []*MerkleRootVerificationRequest {
	if x != nil {
		return x.MerkleRoots
	}
	return nil
}

type MerkleRootVerificationResponse struct {
	state                  protoimpl.MessageState `protogen:"open.v1"`
	UnverifiedBlockHeights []uint64               `protobuf:"varint,1,rep,packed,name=unverified_block_heights,json=unverifiedBlockHeights,proto3" json:"unverified_block_heights,omitempty"`
	unknownFields          protoimpl.UnknownFields
	sizeCache              protoimpl.SizeCache
}

func (x *MerkleRootVerificationResponse) Reset() {
	*x = MerkleRootVerificationResponse{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootVerificationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootVerificationResponse) ProtoMessage() {}

func (x *MerkleRootVerificationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[11]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MerkleRootVerificationResponse.ProtoReflect.Descriptor instead.
func (*MerkleRootVerificationResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{11}
}

func (x *MerkleRootVerificationResponse) GetUnverifiedBlockHeights() []uint64 {
	if x != nil {
		return x.UnverifiedBlockHeights
	}
	return nil
}

var File_internal_blocktx_blocktx_api_blocktx_api_proto protoreflect.FileDescriptor

var file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc = string([]byte{
	0x0a, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x62, 0x6c, 0x6f, 0x63, 0x6b,
	0x74, 0x78, 0x2f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2f, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x1a, 0x1f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x88, 0x01, 0x0a, 0x0e,
	0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e,
	0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x02, 0x6f, 0x6b, 0x12, 0x18,
	0x0a, 0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x74, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x74, 0x73, 0x22, 0xe2, 0x01, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04,
	0x68, 0x61, 0x73, 0x68, 0x12, 0x23, 0x0a, 0x0d, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73,
	0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0c, 0x70, 0x72, 0x65,
	0x76, 0x69, 0x6f, 0x75, 0x73, 0x48, 0x61, 0x73, 0x68, 0x12, 0x1f, 0x0a, 0x0b, 0x6d, 0x65, 0x72,
	0x6b, 0x6c, 0x65, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0a,
	0x6d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x65,
	0x69, 0x67, 0x68, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x68, 0x65, 0x69, 0x67,
	0x68, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x65, 0x64, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x65, 0x64,
	0x12, 0x2b, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x13, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x53,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x1c, 0x0a,
	0x09, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x09, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x77, 0x6f, 0x72, 0x6b, 0x22, 0x4c, 0x0a, 0x0c, 0x54,
	0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x3c, 0x0a, 0x0c, 0x74,
	0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x18, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e,
	0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x74, 0x72, 0x61,
	0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0xd7, 0x01, 0x0a, 0x10, 0x54, 0x72,
	0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x1d,
	0x0a, 0x0a, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x09, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x12, 0x21, 0x0a,
	0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x04, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x12, 0x29, 0x0a, 0x10, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x5f,
	0x68, 0x61, 0x73, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0f, 0x74, 0x72, 0x61, 0x6e,
	0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x48, 0x61, 0x73, 0x68, 0x12, 0x1e, 0x0a, 0x0a, 0x6d,
	0x65, 0x72, 0x6b, 0x6c, 0x65, 0x50, 0x61, 0x74, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x6d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x50, 0x61, 0x74, 0x68, 0x12, 0x36, 0x0a, 0x0c, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x13, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e,
	0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x22, 0x61, 0x0a, 0x11, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x12, 0x4c, 0x0a, 0x12, 0x74, 0x72, 0x61, 0x6e,
	0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61,
	0x70, 0x69, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x6c,
	0x6f, 0x63, 0x6b, 0x52, 0x11, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x22, 0x21, 0x0a, 0x0b, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x22, 0x31, 0x0a, 0x09, 0x43, 0x6c, 0x65,
	0x61, 0x72, 0x44, 0x61, 0x74, 0x61, 0x12, 0x24, 0x0a, 0x0d, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74,
	0x69, 0x6f, 0x6e, 0x44, 0x61, 0x79, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0d, 0x72,
	0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x44, 0x61, 0x79, 0x73, 0x22, 0x2a, 0x0a, 0x14,
	0x52, 0x6f, 0x77, 0x73, 0x41, 0x66, 0x66, 0x65, 0x63, 0x74, 0x65, 0x64, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x6f, 0x77, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x04, 0x72, 0x6f, 0x77, 0x73, 0x22, 0x48, 0x0a, 0x23, 0x44, 0x65, 0x6c, 0x55,
	0x6e, 0x66, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x72,
	0x6f, 0x63, 0x65, 0x73, 0x73, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x21, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x65, 0x64, 0x5f, 0x62, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x65, 0x64,
	0x42, 0x79, 0x22, 0x63, 0x0a, 0x1d, 0x4d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74,
	0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x6d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x5f, 0x72, 0x6f,
	0x6f, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6d, 0x65, 0x72, 0x6b, 0x6c, 0x65,
	0x52, 0x6f, 0x6f, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65,
	0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x22, 0x6f, 0x0a, 0x1e, 0x4d, 0x65, 0x72, 0x6b, 0x6c,
	0x65, 0x52, 0x6f, 0x6f, 0x74, 0x73, 0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x4d, 0x0a, 0x0c, 0x6d, 0x65, 0x72,
	0x6b, 0x6c, 0x65, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x2a, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x4d, 0x65,
	0x72, 0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x0b, 0x6d, 0x65, 0x72,
	0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x73, 0x22, 0x5a, 0x0a, 0x1e, 0x4d, 0x65, 0x72, 0x6b,
	0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x38, 0x0a, 0x18, 0x75, 0x6e,
	0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x65, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68,
	0x65, 0x69, 0x67, 0x68, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x04, 0x52, 0x16, 0x75, 0x6e,
	0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69,
	0x67, 0x68, 0x74, 0x73, 0x2a, 0x3b, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x0b,
	0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x4c,
	0x4f, 0x4e, 0x47, 0x45, 0x53, 0x54, 0x10, 0x0a, 0x12, 0x09, 0x0a, 0x05, 0x53, 0x54, 0x41, 0x4c,
	0x45, 0x10, 0x14, 0x12, 0x0c, 0x0a, 0x08, 0x4f, 0x52, 0x50, 0x48, 0x41, 0x4e, 0x45, 0x44, 0x10,
	0x1e, 0x32, 0xb1, 0x03, 0x0a, 0x0a, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x54, 0x78, 0x41, 0x50, 0x49,
	0x12, 0x3f, 0x0a, 0x06, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x1a, 0x1b, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69,
	0x2e, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x4a, 0x0a, 0x0b, 0x43, 0x6c, 0x65, 0x61, 0x72, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x73,
	0x12, 0x16, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x43,
	0x6c, 0x65, 0x61, 0x72, 0x44, 0x61, 0x74, 0x61, 0x1a, 0x21, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b,
	0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x52, 0x6f, 0x77, 0x73, 0x41, 0x66, 0x66, 0x65, 0x63,
	0x74, 0x65, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x5a, 0x0a,
	0x1b, 0x43, 0x6c, 0x65, 0x61, 0x72, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x65, 0x64,
	0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x16, 0x2e, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x43, 0x6c, 0x65, 0x61, 0x72,
	0x44, 0x61, 0x74, 0x61, 0x1a, 0x21, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61,
	0x70, 0x69, 0x2e, 0x52, 0x6f, 0x77, 0x73, 0x41, 0x66, 0x66, 0x65, 0x63, 0x74, 0x65, 0x64, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x6f, 0x0a, 0x11, 0x56, 0x65, 0x72,
	0x69, 0x66, 0x79, 0x4d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x73, 0x12, 0x2b,
	0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x4d, 0x65, 0x72,
	0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x73, 0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e, 0x4d, 0x65, 0x72, 0x6b, 0x6c, 0x65,
	0x52, 0x6f, 0x6f, 0x74, 0x56, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x49, 0x0a, 0x13, 0x52, 0x65,
	0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x18, 0x2e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x2e,
	0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x16, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x22, 0x00, 0x42, 0x0f, 0x5a, 0x0d, 0x2e, 0x3b, 0x62, 0x6c, 0x6f, 0x63, 0x6b,
	0x74, 0x78, 0x5f, 0x61, 0x70, 0x69, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescOnce sync.Once
	file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescData []byte
)

func file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP() []byte {
	file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescOnce.Do(func() {
		file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc), len(file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc)))
	})
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescData
}

var file_internal_blocktx_blocktx_api_blocktx_api_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_internal_blocktx_blocktx_api_blocktx_api_proto_goTypes = []any{
	(Status)(0),                                 // 0: blocktx_api.Status
	(*HealthResponse)(nil),                      // 1: blocktx_api.HealthResponse
	(*Block)(nil),                               // 2: blocktx_api.Block
	(*Transactions)(nil),                        // 3: blocktx_api.Transactions
	(*TransactionBlock)(nil),                    // 4: blocktx_api.TransactionBlock
	(*TransactionBlocks)(nil),                   // 5: blocktx_api.TransactionBlocks
	(*Transaction)(nil),                         // 6: blocktx_api.Transaction
	(*ClearData)(nil),                           // 7: blocktx_api.ClearData
	(*RowsAffectedResponse)(nil),                // 8: blocktx_api.RowsAffectedResponse
	(*DelUnfinishedBlockProcessingRequest)(nil), // 9: blocktx_api.DelUnfinishedBlockProcessingRequest
	(*MerkleRootVerificationRequest)(nil),       // 10: blocktx_api.MerkleRootVerificationRequest
	(*MerkleRootsVerificationRequest)(nil),      // 11: blocktx_api.MerkleRootsVerificationRequest
	(*MerkleRootVerificationResponse)(nil),      // 12: blocktx_api.MerkleRootVerificationResponse
	(*timestamppb.Timestamp)(nil),               // 13: google.protobuf.Timestamp
	(*emptypb.Empty)(nil),                       // 14: google.protobuf.Empty
}
var file_internal_blocktx_blocktx_api_blocktx_api_proto_depIdxs = []int32{
	13, // 0: blocktx_api.HealthResponse.timestamp:type_name -> google.protobuf.Timestamp
	0,  // 1: blocktx_api.Block.status:type_name -> blocktx_api.Status
	6,  // 2: blocktx_api.Transactions.transactions:type_name -> blocktx_api.Transaction
	0,  // 3: blocktx_api.TransactionBlock.block_status:type_name -> blocktx_api.Status
	4,  // 4: blocktx_api.TransactionBlocks.transaction_blocks:type_name -> blocktx_api.TransactionBlock
	10, // 5: blocktx_api.MerkleRootsVerificationRequest.merkle_roots:type_name -> blocktx_api.MerkleRootVerificationRequest
	14, // 6: blocktx_api.BlockTxAPI.Health:input_type -> google.protobuf.Empty
	7,  // 7: blocktx_api.BlockTxAPI.ClearBlocks:input_type -> blocktx_api.ClearData
	7,  // 8: blocktx_api.BlockTxAPI.ClearRegisteredTransactions:input_type -> blocktx_api.ClearData
	11, // 9: blocktx_api.BlockTxAPI.VerifyMerkleRoots:input_type -> blocktx_api.MerkleRootsVerificationRequest
	6,  // 10: blocktx_api.BlockTxAPI.RegisterTransaction:input_type -> blocktx_api.Transaction
	1,  // 11: blocktx_api.BlockTxAPI.Health:output_type -> blocktx_api.HealthResponse
	8,  // 12: blocktx_api.BlockTxAPI.ClearBlocks:output_type -> blocktx_api.RowsAffectedResponse
	8,  // 13: blocktx_api.BlockTxAPI.ClearRegisteredTransactions:output_type -> blocktx_api.RowsAffectedResponse
	12, // 14: blocktx_api.BlockTxAPI.VerifyMerkleRoots:output_type -> blocktx_api.MerkleRootVerificationResponse
	14, // 15: blocktx_api.BlockTxAPI.RegisterTransaction:output_type -> google.protobuf.Empty
	11, // [11:16] is the sub-list for method output_type
	6,  // [6:11] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_internal_blocktx_blocktx_api_blocktx_api_proto_init() }
func file_internal_blocktx_blocktx_api_blocktx_api_proto_init() {
	if File_internal_blocktx_blocktx_api_blocktx_api_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc), len(file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_internal_blocktx_blocktx_api_blocktx_api_proto_goTypes,
		DependencyIndexes: file_internal_blocktx_blocktx_api_blocktx_api_proto_depIdxs,
		EnumInfos:         file_internal_blocktx_blocktx_api_blocktx_api_proto_enumTypes,
		MessageInfos:      file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes,
	}.Build()
	File_internal_blocktx_blocktx_api_blocktx_api_proto = out.File
	file_internal_blocktx_blocktx_api_blocktx_api_proto_goTypes = nil
	file_internal_blocktx_blocktx_api_blocktx_api_proto_depIdxs = nil
}
