// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: pkg/message_queue/nats/client/test_api/test_api.proto

package test_api

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
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

type TestMessage struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Ok            bool                   `protobuf:"varint,1,opt,name=ok,proto3" json:"ok,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TestMessage) Reset() {
	*x = TestMessage{}
	mi := &file_pkg_message_queue_nats_client_test_api_test_api_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TestMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestMessage) ProtoMessage() {}

func (x *TestMessage) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_message_queue_nats_client_test_api_test_api_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestMessage.ProtoReflect.Descriptor instead.
func (*TestMessage) Descriptor() ([]byte, []int) {
	return file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescGZIP(), []int{0}
}

func (x *TestMessage) GetOk() bool {
	if x != nil {
		return x.Ok
	}
	return false
}

var File_pkg_message_queue_nats_client_test_api_test_api_proto protoreflect.FileDescriptor

var file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDesc = string([]byte{
	0x0a, 0x35, 0x70, 0x6b, 0x67, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x5f, 0x71, 0x75,
	0x65, 0x75, 0x65, 0x2f, 0x6e, 0x61, 0x74, 0x73, 0x2f, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x2f,
	0x74, 0x65, 0x73, 0x74, 0x5f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x61, 0x70,
	0x69, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x61, 0x70,
	0x69, 0x22, 0x1d, 0x0a, 0x0b, 0x54, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x02, 0x6f, 0x6b,
	0x42, 0x0c, 0x5a, 0x0a, 0x2e, 0x3b, 0x74, 0x65, 0x73, 0x74, 0x5f, 0x61, 0x70, 0x69, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescOnce sync.Once
	file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescData []byte
)

func file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescGZIP() []byte {
	file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescOnce.Do(func() {
		file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDesc), len(file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDesc)))
	})
	return file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDescData
}

var file_pkg_message_queue_nats_client_test_api_test_api_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_message_queue_nats_client_test_api_test_api_proto_goTypes = []any{
	(*TestMessage)(nil), // 0: test_api.TestMessage
}
var file_pkg_message_queue_nats_client_test_api_test_api_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pkg_message_queue_nats_client_test_api_test_api_proto_init() }
func file_pkg_message_queue_nats_client_test_api_test_api_proto_init() {
	if File_pkg_message_queue_nats_client_test_api_test_api_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDesc), len(file_pkg_message_queue_nats_client_test_api_test_api_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_message_queue_nats_client_test_api_test_api_proto_goTypes,
		DependencyIndexes: file_pkg_message_queue_nats_client_test_api_test_api_proto_depIdxs,
		MessageInfos:      file_pkg_message_queue_nats_client_test_api_test_api_proto_msgTypes,
	}.Build()
	File_pkg_message_queue_nats_client_test_api_test_api_proto = out.File
	file_pkg_message_queue_nats_client_test_api_test_api_proto_goTypes = nil
	file_pkg_message_queue_nats_client_test_api_test_api_proto_depIdxs = nil
}
