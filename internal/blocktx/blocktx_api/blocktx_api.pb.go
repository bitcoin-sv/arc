// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
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

type IsMined struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Hash          []byte                 `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Mined         bool                   `protobuf:"varint,2,opt,name=mined,proto3" json:"mined,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *IsMined) Reset() {
	*x = IsMined{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IsMined) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IsMined) ProtoMessage() {}

func (x *IsMined) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use IsMined.ProtoReflect.Descriptor instead.
func (*IsMined) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{0}
}

func (x *IsMined) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *IsMined) GetMined() bool {
	if x != nil {
		return x.Mined
	}
	return false
}

type AnyTransactionsMinedResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Transactions  []*IsMined             `protobuf:"bytes,1,rep,name=transactions,proto3" json:"transactions,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AnyTransactionsMinedResponse) Reset() {
	*x = AnyTransactionsMinedResponse{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AnyTransactionsMinedResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnyTransactionsMinedResponse) ProtoMessage() {}

func (x *AnyTransactionsMinedResponse) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use AnyTransactionsMinedResponse.ProtoReflect.Descriptor instead.
func (*AnyTransactionsMinedResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{1}
}

func (x *AnyTransactionsMinedResponse) GetTransactions() []*IsMined {
	if x != nil {
		return x.Transactions
	}
	return nil
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthResponse) ProtoMessage() {}

func (x *HealthResponse) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use HealthResponse.ProtoReflect.Descriptor instead.
func (*HealthResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{2}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
func (*Block) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{3}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transactions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transactions) ProtoMessage() {}

func (x *Transactions) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Transactions.ProtoReflect.Descriptor instead.
func (*Transactions) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{4}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransactionBlock) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransactionBlock) ProtoMessage() {}

func (x *TransactionBlock) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use TransactionBlock.ProtoReflect.Descriptor instead.
func (*TransactionBlock) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{5}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TransactionBlocks) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransactionBlocks) ProtoMessage() {}

func (x *TransactionBlocks) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use TransactionBlocks.ProtoReflect.Descriptor instead.
func (*TransactionBlocks) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{6}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transaction) ProtoMessage() {}

func (x *Transaction) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Transaction.ProtoReflect.Descriptor instead.
func (*Transaction) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{7}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClearData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClearData) ProtoMessage() {}

func (x *ClearData) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use ClearData.ProtoReflect.Descriptor instead.
func (*ClearData) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{8}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RowsAffectedResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RowsAffectedResponse) ProtoMessage() {}

func (x *RowsAffectedResponse) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use RowsAffectedResponse.ProtoReflect.Descriptor instead.
func (*RowsAffectedResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{9}
}

func (x *RowsAffectedResponse) GetRows() int64 {
	if x != nil {
		return x.Rows
	}
	return 0
}

// swagger:model CurrentBlockHeightResponse
type CurrentBlockHeightResponse struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	CurrentBlockHeight uint64                 `protobuf:"varint,1,opt,name=currentBlockHeight,proto3" json:"currentBlockHeight,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *CurrentBlockHeightResponse) Reset() {
	*x = CurrentBlockHeightResponse{}
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CurrentBlockHeightResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CurrentBlockHeightResponse) ProtoMessage() {}

func (x *CurrentBlockHeightResponse) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use CurrentBlockHeightResponse.ProtoReflect.Descriptor instead.
func (*CurrentBlockHeightResponse) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{10}
}

func (x *CurrentBlockHeightResponse) GetCurrentBlockHeight() uint64 {
	if x != nil {
		return x.CurrentBlockHeight
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[11]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DelUnfinishedBlockProcessingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DelUnfinishedBlockProcessingRequest) ProtoMessage() {}

func (x *DelUnfinishedBlockProcessingRequest) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use DelUnfinishedBlockProcessingRequest.ProtoReflect.Descriptor instead.
func (*DelUnfinishedBlockProcessingRequest) Descriptor() ([]byte, []int) {
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{11}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[12]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootVerificationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootVerificationRequest) ProtoMessage() {}

func (x *MerkleRootVerificationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[12]
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
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{12}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[13]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootsVerificationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootsVerificationRequest) ProtoMessage() {}

func (x *MerkleRootsVerificationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[13]
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
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{13}
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
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[14]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *MerkleRootVerificationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MerkleRootVerificationResponse) ProtoMessage() {}

func (x *MerkleRootVerificationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes[14]
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
	return file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDescGZIP(), []int{14}
}

func (x *MerkleRootVerificationResponse) GetUnverifiedBlockHeights() []uint64 {
	if x != nil {
		return x.UnverifiedBlockHeights
	}
	return nil
}

var File_internal_blocktx_blocktx_api_blocktx_api_proto protoreflect.FileDescriptor

const file_internal_blocktx_blocktx_api_blocktx_api_proto_rawDesc = "" +
	"\n" +
	".internal/blocktx/blocktx_api/blocktx_api.proto\x12\vblocktx_api\x1a\x1fgoogle/protobuf/timestamp.proto\x1a\x1bgoogle/protobuf/empty.proto\"3\n" +
	"\aIsMined\x12\x12\n" +
	"\x04hash\x18\x01 \x01(\fR\x04hash\x12\x14\n" +
	"\x05mined\x18\x02 \x01(\bR\x05mined\"X\n" +
	"\x1cAnyTransactionsMinedResponse\x128\n" +
	"\ftransactions\x18\x01 \x03(\v2\x14.blocktx_api.IsMinedR\ftransactions\"\x88\x01\n" +
	"\x0eHealthResponse\x12\x0e\n" +
	"\x02ok\x18\x01 \x01(\bR\x02ok\x12\x18\n" +
	"\adetails\x18\x02 \x01(\tR\adetails\x128\n" +
	"\ttimestamp\x18\x03 \x01(\v2\x1a.google.protobuf.TimestampR\ttimestamp\x12\x12\n" +
	"\x04nats\x18\x04 \x01(\tR\x04nats\"\xe2\x01\n" +
	"\x05Block\x12\x12\n" +
	"\x04hash\x18\x01 \x01(\fR\x04hash\x12#\n" +
	"\rprevious_hash\x18\x02 \x01(\fR\fpreviousHash\x12\x1f\n" +
	"\vmerkle_root\x18\x03 \x01(\fR\n" +
	"merkleRoot\x12\x16\n" +
	"\x06height\x18\x04 \x01(\x04R\x06height\x12\x1c\n" +
	"\tprocessed\x18\x05 \x01(\bR\tprocessed\x12+\n" +
	"\x06status\x18\x06 \x01(\x0e2\x13.blocktx_api.StatusR\x06status\x12\x1c\n" +
	"\tchainwork\x18\a \x01(\tR\tchainwork\"L\n" +
	"\fTransactions\x12<\n" +
	"\ftransactions\x18\x01 \x03(\v2\x18.blocktx_api.TransactionR\ftransactions\"\xd7\x01\n" +
	"\x10TransactionBlock\x12\x1d\n" +
	"\n" +
	"block_hash\x18\x01 \x01(\fR\tblockHash\x12!\n" +
	"\fblock_height\x18\x02 \x01(\x04R\vblockHeight\x12)\n" +
	"\x10transaction_hash\x18\x03 \x01(\fR\x0ftransactionHash\x12\x1e\n" +
	"\n" +
	"merklePath\x18\x04 \x01(\tR\n" +
	"merklePath\x126\n" +
	"\fblock_status\x18\x05 \x01(\x0e2\x13.blocktx_api.StatusR\vblockStatus\"a\n" +
	"\x11TransactionBlocks\x12L\n" +
	"\x12transaction_blocks\x18\x01 \x03(\v2\x1d.blocktx_api.TransactionBlockR\x11transactionBlocks\"!\n" +
	"\vTransaction\x12\x12\n" +
	"\x04hash\x18\x01 \x01(\fR\x04hash\"1\n" +
	"\tClearData\x12$\n" +
	"\rretentionDays\x18\x01 \x01(\x05R\rretentionDays\"*\n" +
	"\x14RowsAffectedResponse\x12\x12\n" +
	"\x04rows\x18\x01 \x01(\x03R\x04rows\"L\n" +
	"\x1aCurrentBlockHeightResponse\x12.\n" +
	"\x12currentBlockHeight\x18\x01 \x01(\x04R\x12currentBlockHeight\"H\n" +
	"#DelUnfinishedBlockProcessingRequest\x12!\n" +
	"\fprocessed_by\x18\x01 \x01(\tR\vprocessedBy\"c\n" +
	"\x1dMerkleRootVerificationRequest\x12\x1f\n" +
	"\vmerkle_root\x18\x01 \x01(\tR\n" +
	"merkleRoot\x12!\n" +
	"\fblock_height\x18\x02 \x01(\x04R\vblockHeight\"o\n" +
	"\x1eMerkleRootsVerificationRequest\x12M\n" +
	"\fmerkle_roots\x18\x01 \x03(\v2*.blocktx_api.MerkleRootVerificationRequestR\vmerkleRoots\"Z\n" +
	"\x1eMerkleRootVerificationResponse\x128\n" +
	"\x18unverified_block_heights\x18\x01 \x03(\x04R\x16unverifiedBlockHeights*;\n" +
	"\x06Status\x12\v\n" +
	"\aUNKNOWN\x10\x00\x12\v\n" +
	"\aLONGEST\x10\n" +
	"\x12\t\n" +
	"\x05STALE\x10\x14\x12\f\n" +
	"\bORPHANED\x10\x1e2\xb7\x05\n" +
	"\n" +
	"BlockTxAPI\x12?\n" +
	"\x06Health\x12\x16.google.protobuf.Empty\x1a\x1b.blocktx_api.HealthResponse\"\x00\x12J\n" +
	"\vClearBlocks\x12\x16.blocktx_api.ClearData\x1a!.blocktx_api.RowsAffectedResponse\"\x00\x12Z\n" +
	"\x1bClearRegisteredTransactions\x12\x16.blocktx_api.ClearData\x1a!.blocktx_api.RowsAffectedResponse\"\x00\x12o\n" +
	"\x11VerifyMerkleRoots\x12+.blocktx_api.MerkleRootsVerificationRequest\x1a+.blocktx_api.MerkleRootVerificationResponse\"\x00\x12I\n" +
	"\x13RegisterTransaction\x12\x18.blocktx_api.Transaction\x1a\x16.google.protobuf.Empty\"\x00\x12K\n" +
	"\x14RegisterTransactions\x12\x19.blocktx_api.Transactions\x1a\x16.google.protobuf.Empty\"\x00\x12W\n" +
	"\x12CurrentBlockHeight\x12\x16.google.protobuf.Empty\x1a'.blocktx_api.CurrentBlockHeightResponse\"\x00\x12^\n" +
	"\x14AnyTransactionsMined\x12\x19.blocktx_api.Transactions\x1a).blocktx_api.AnyTransactionsMinedResponse\"\x00B\x0fZ\r.;blocktx_apib\x06proto3"

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
var file_internal_blocktx_blocktx_api_blocktx_api_proto_msgTypes = make([]protoimpl.MessageInfo, 15)
var file_internal_blocktx_blocktx_api_blocktx_api_proto_goTypes = []any{
	(Status)(0),                                 // 0: blocktx_api.Status
	(*IsMined)(nil),                             // 1: blocktx_api.IsMined
	(*AnyTransactionsMinedResponse)(nil),        // 2: blocktx_api.AnyTransactionsMinedResponse
	(*HealthResponse)(nil),                      // 3: blocktx_api.HealthResponse
	(*Block)(nil),                               // 4: blocktx_api.Block
	(*Transactions)(nil),                        // 5: blocktx_api.Transactions
	(*TransactionBlock)(nil),                    // 6: blocktx_api.TransactionBlock
	(*TransactionBlocks)(nil),                   // 7: blocktx_api.TransactionBlocks
	(*Transaction)(nil),                         // 8: blocktx_api.Transaction
	(*ClearData)(nil),                           // 9: blocktx_api.ClearData
	(*RowsAffectedResponse)(nil),                // 10: blocktx_api.RowsAffectedResponse
	(*CurrentBlockHeightResponse)(nil),          // 11: blocktx_api.CurrentBlockHeightResponse
	(*DelUnfinishedBlockProcessingRequest)(nil), // 12: blocktx_api.DelUnfinishedBlockProcessingRequest
	(*MerkleRootVerificationRequest)(nil),       // 13: blocktx_api.MerkleRootVerificationRequest
	(*MerkleRootsVerificationRequest)(nil),      // 14: blocktx_api.MerkleRootsVerificationRequest
	(*MerkleRootVerificationResponse)(nil),      // 15: blocktx_api.MerkleRootVerificationResponse
	(*timestamppb.Timestamp)(nil),               // 16: google.protobuf.Timestamp
	(*emptypb.Empty)(nil),                       // 17: google.protobuf.Empty
}
var file_internal_blocktx_blocktx_api_blocktx_api_proto_depIdxs = []int32{
	1,  // 0: blocktx_api.AnyTransactionsMinedResponse.transactions:type_name -> blocktx_api.IsMined
	16, // 1: blocktx_api.HealthResponse.timestamp:type_name -> google.protobuf.Timestamp
	0,  // 2: blocktx_api.Block.status:type_name -> blocktx_api.Status
	8,  // 3: blocktx_api.Transactions.transactions:type_name -> blocktx_api.Transaction
	0,  // 4: blocktx_api.TransactionBlock.block_status:type_name -> blocktx_api.Status
	6,  // 5: blocktx_api.TransactionBlocks.transaction_blocks:type_name -> blocktx_api.TransactionBlock
	13, // 6: blocktx_api.MerkleRootsVerificationRequest.merkle_roots:type_name -> blocktx_api.MerkleRootVerificationRequest
	17, // 7: blocktx_api.BlockTxAPI.Health:input_type -> google.protobuf.Empty
	9,  // 8: blocktx_api.BlockTxAPI.ClearBlocks:input_type -> blocktx_api.ClearData
	9,  // 9: blocktx_api.BlockTxAPI.ClearRegisteredTransactions:input_type -> blocktx_api.ClearData
	14, // 10: blocktx_api.BlockTxAPI.VerifyMerkleRoots:input_type -> blocktx_api.MerkleRootsVerificationRequest
	8,  // 11: blocktx_api.BlockTxAPI.RegisterTransaction:input_type -> blocktx_api.Transaction
	5,  // 12: blocktx_api.BlockTxAPI.RegisterTransactions:input_type -> blocktx_api.Transactions
	17, // 13: blocktx_api.BlockTxAPI.CurrentBlockHeight:input_type -> google.protobuf.Empty
	5,  // 14: blocktx_api.BlockTxAPI.AnyTransactionsMined:input_type -> blocktx_api.Transactions
	3,  // 15: blocktx_api.BlockTxAPI.Health:output_type -> blocktx_api.HealthResponse
	10, // 16: blocktx_api.BlockTxAPI.ClearBlocks:output_type -> blocktx_api.RowsAffectedResponse
	10, // 17: blocktx_api.BlockTxAPI.ClearRegisteredTransactions:output_type -> blocktx_api.RowsAffectedResponse
	15, // 18: blocktx_api.BlockTxAPI.VerifyMerkleRoots:output_type -> blocktx_api.MerkleRootVerificationResponse
	17, // 19: blocktx_api.BlockTxAPI.RegisterTransaction:output_type -> google.protobuf.Empty
	17, // 20: blocktx_api.BlockTxAPI.RegisterTransactions:output_type -> google.protobuf.Empty
	11, // 21: blocktx_api.BlockTxAPI.CurrentBlockHeight:output_type -> blocktx_api.CurrentBlockHeightResponse
	2,  // 22: blocktx_api.BlockTxAPI.AnyTransactionsMined:output_type -> blocktx_api.AnyTransactionsMinedResponse
	15, // [15:23] is the sub-list for method output_type
	7,  // [7:15] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
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
			NumMessages:   15,
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
