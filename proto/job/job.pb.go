// Code generated by protoc-gen-go.
// source: job/job.proto
// DO NOT EDIT!

/*
Package job is a generated protocol buffer package.

It is generated from these files:
	job/job.proto

It has these top-level messages:
	SMCTask
	PreparePhase
	SessionPhase
*/
package job

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type DataOrigin int32

const (
	DataOrigin_TEMPERATURE  DataOrigin = 0
	DataOrigin_HUMIDITY     DataOrigin = 1
	DataOrigin_AMBIENT      DataOrigin = 2
	DataOrigin_AIR_PRESSURE DataOrigin = 3
	DataOrigin_PRESENCE     DataOrigin = 10
	// Dynamically assigned sensor types.
	DataOrigin_RESERVED_999            DataOrigin = 999
	DataOrigin_DYNAMIC_ASSIGNMENT_1000 DataOrigin = 1000
	// ...
	DataOrigin_DYNAMIC_ASSIGNMENT_99999 DataOrigin = 99999
	DataOrigin_RESERVED_100000          DataOrigin = 100000
)

var DataOrigin_name = map[int32]string{
	0:      "TEMPERATURE",
	1:      "HUMIDITY",
	2:      "AMBIENT",
	3:      "AIR_PRESSURE",
	10:     "PRESENCE",
	999:    "RESERVED_999",
	1000:   "DYNAMIC_ASSIGNMENT_1000",
	99999:  "DYNAMIC_ASSIGNMENT_99999",
	100000: "RESERVED_100000",
}
var DataOrigin_value = map[string]int32{
	"TEMPERATURE":              0,
	"HUMIDITY":                 1,
	"AMBIENT":                  2,
	"AIR_PRESSURE":             3,
	"PRESENCE":                 10,
	"RESERVED_999":             999,
	"DYNAMIC_ASSIGNMENT_1000":  1000,
	"DYNAMIC_ASSIGNMENT_99999": 99999,
	"RESERVED_100000":          100000,
}

func (x DataOrigin) String() string {
	return proto.EnumName(DataOrigin_name, int32(x))
}
func (DataOrigin) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type Aggregator int32

const (
	Aggregator_SUM           Aggregator = 0
	Aggregator_AVG           Aggregator = 1
	Aggregator_MEDIAN        Aggregator = 2
	Aggregator_STD_DEVIATION Aggregator = 3
)

var Aggregator_name = map[int32]string{
	0: "SUM",
	1: "AVG",
	2: "MEDIAN",
	3: "STD_DEVIATION",
}
var Aggregator_value = map[string]int32{
	"SUM":           0,
	"AVG":           1,
	"MEDIAN":        2,
	"STD_DEVIATION": 3,
}

func (x Aggregator) String() string {
	return proto.EnumName(Aggregator_name, int32(x))
}
func (Aggregator) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type SMCTask struct {
	Set    string     `protobuf:"bytes,1,opt,name=Set,json=set" json:"Set,omitempty"`
	Source DataOrigin `protobuf:"varint,2,opt,name=source,enum=job.DataOrigin" json:"source,omitempty"`
	// (Pre)Selectors
	Aggregator      Aggregator                 `protobuf:"varint,4,opt,name=aggregator,enum=job.Aggregator" json:"aggregator,omitempty"`
	TicketSignature string                     `protobuf:"bytes,7,opt,name=ticketSignature" json:"ticketSignature,omitempty"`
	Issued          *google_protobuf.Timestamp `protobuf:"bytes,8,opt,name=issued" json:"issued,omitempty"`
	QuerySignature  string                     `protobuf:"bytes,9,opt,name=querySignature" json:"querySignature,omitempty"`
}

func (m *SMCTask) Reset()                    { *m = SMCTask{} }
func (m *SMCTask) String() string            { return proto.CompactTextString(m) }
func (*SMCTask) ProtoMessage()               {}
func (*SMCTask) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *SMCTask) GetIssued() *google_protobuf.Timestamp {
	if m != nil {
		return m.Issued
	}
	return nil
}

type PreparePhase struct {
	Participants []*PreparePhase_Participant `protobuf:"bytes,1,rep,name=participants" json:"participants,omitempty"`
	SmcTask      *SMCTask                    `protobuf:"bytes,2,opt,name=smcTask" json:"smcTask,omitempty"`
}

func (m *PreparePhase) Reset()                    { *m = PreparePhase{} }
func (m *PreparePhase) String() string            { return proto.CompactTextString(m) }
func (*PreparePhase) ProtoMessage()               {}
func (*PreparePhase) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *PreparePhase) GetParticipants() []*PreparePhase_Participant {
	if m != nil {
		return m.Participants
	}
	return nil
}

func (m *PreparePhase) GetSmcTask() *SMCTask {
	if m != nil {
		return m.SmcTask
	}
	return nil
}

type PreparePhase_Participant struct {
	Identity string `protobuf:"bytes,1,opt,name=identity" json:"identity,omitempty"`
	Addr     string `protobuf:"bytes,2,opt,name=addr" json:"addr,omitempty"`
}

func (m *PreparePhase_Participant) Reset()                    { *m = PreparePhase_Participant{} }
func (m *PreparePhase_Participant) String() string            { return proto.CompactTextString(m) }
func (*PreparePhase_Participant) ProtoMessage()               {}
func (*PreparePhase_Participant) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1, 0} }

type SessionPhase struct {
}

func (m *SessionPhase) Reset()                    { *m = SessionPhase{} }
func (m *SessionPhase) String() string            { return proto.CompactTextString(m) }
func (*SessionPhase) ProtoMessage()               {}
func (*SessionPhase) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func init() {
	proto.RegisterType((*SMCTask)(nil), "job.SMCTask")
	proto.RegisterType((*PreparePhase)(nil), "job.PreparePhase")
	proto.RegisterType((*PreparePhase_Participant)(nil), "job.PreparePhase.Participant")
	proto.RegisterType((*SessionPhase)(nil), "job.SessionPhase")
	proto.RegisterEnum("job.DataOrigin", DataOrigin_name, DataOrigin_value)
	proto.RegisterEnum("job.Aggregator", Aggregator_name, Aggregator_value)
}

func init() { proto.RegisterFile("job/job.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 551 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x6c, 0x52, 0xcb, 0x6e, 0x9b, 0x4c,
	0x14, 0xfe, 0x09, 0x11, 0x24, 0xc7, 0x24, 0x26, 0x23, 0xfd, 0x2a, 0xb2, 0x7a, 0x53, 0x16, 0x4d,
	0x94, 0x05, 0x4e, 0xdd, 0x95, 0x17, 0x59, 0xe0, 0x80, 0x52, 0x22, 0x81, 0xd1, 0x80, 0x2d, 0x65,
	0x65, 0x8d, 0xcd, 0x84, 0x52, 0xc7, 0xe0, 0xce, 0x8c, 0xa5, 0xe6, 0x21, 0xf2, 0x0c, 0xad, 0xfa,
	0x14, 0x5d, 0xf6, 0x6d, 0xda, 0x77, 0xe8, 0xa6, 0x03, 0xf1, 0xad, 0x51, 0x91, 0x90, 0xe6, 0x9c,
	0xf3, 0x9d, 0xef, 0x7c, 0xe7, 0x02, 0x07, 0x1f, 0xcb, 0x71, 0x5b, 0xfe, 0xf6, 0x9c, 0x95, 0xa2,
	0x44, 0xaa, 0x7c, 0xb6, 0x5e, 0x65, 0x65, 0x99, 0xdd, 0xd1, 0x76, 0xed, 0x1a, 0x2f, 0x6e, 0xdb,
	0x22, 0x9f, 0x51, 0x2e, 0xc8, 0x6c, 0xfe, 0x88, 0x3a, 0xfe, 0xad, 0x80, 0x1e, 0x07, 0x97, 0x09,
	0xe1, 0x53, 0x64, 0x82, 0x1a, 0x53, 0x61, 0x29, 0xaf, 0x95, 0xd3, 0x7d, 0xac, 0x72, 0x2a, 0xd0,
	0x09, 0x68, 0xbc, 0x5c, 0xb0, 0x09, 0xb5, 0x76, 0xa4, 0xf3, 0xb0, 0xd3, 0xb4, 0x2b, 0x7e, 0x97,
	0x08, 0xd2, 0x67, 0x79, 0x96, 0x17, 0x78, 0x19, 0x46, 0x6d, 0x00, 0x92, 0x65, 0x8c, 0x66, 0x44,
	0x94, 0xcc, 0xda, 0xdd, 0x02, 0x3b, 0x6b, 0x37, 0xde, 0x82, 0xa0, 0x53, 0x68, 0x8a, 0x7c, 0x32,
	0xa5, 0x22, 0xce, 0xb3, 0x82, 0x88, 0x05, 0xa3, 0x96, 0x5e, 0xd7, 0x7d, 0xea, 0x46, 0x1d, 0xd0,
	0x72, 0xce, 0x17, 0x34, 0xb5, 0xf6, 0x24, 0xa0, 0xd1, 0x69, 0xd9, 0x8f, 0x3d, 0xd9, 0xab, 0x9e,
	0xec, 0x64, 0xd5, 0x13, 0x5e, 0x22, 0xd1, 0x1b, 0x38, 0xfc, 0xb4, 0xa0, 0xec, 0x7e, 0x43, 0xbe,
	0x5f, 0x93, 0x3f, 0xf1, 0x1e, 0x7f, 0x57, 0xc0, 0x88, 0x18, 0x9d, 0x13, 0x46, 0xa3, 0x0f, 0x84,
	0x53, 0xe4, 0x80, 0x21, 0x0d, 0x29, 0x21, 0x9f, 0x93, 0x42, 0x70, 0x39, 0x0b, 0x55, 0x96, 0x7c,
	0x51, 0x77, 0xb2, 0x0d, 0xb4, 0xa3, 0x0d, 0x0a, 0xff, 0x95, 0x22, 0x6b, 0xeb, 0x7c, 0x36, 0xa9,
	0x06, 0x5a, 0x0f, 0xad, 0xd1, 0x31, 0xea, 0xec, 0xe5, 0x90, 0xf1, 0x2a, 0xd8, 0xba, 0x80, 0xc6,
	0x16, 0x09, 0x6a, 0xc1, 0x5e, 0x9e, 0xd2, 0x42, 0xe4, 0xe2, 0x7e, 0xb9, 0x81, 0xb5, 0x8d, 0x10,
	0xec, 0x92, 0x34, 0x65, 0x35, 0xdf, 0x3e, 0xae, 0xdf, 0xc7, 0x87, 0x60, 0xc4, 0x94, 0xf3, 0xbc,
	0x2c, 0x6a, 0x41, 0x67, 0x3f, 0x14, 0x80, 0xcd, 0x62, 0x50, 0x13, 0x1a, 0x89, 0x17, 0x44, 0x1e,
	0x76, 0x92, 0x01, 0xf6, 0xcc, 0xff, 0x90, 0x01, 0x7b, 0xef, 0x07, 0x81, 0xef, 0xfa, 0xc9, 0x8d,
	0xa9, 0xa0, 0x06, 0xe8, 0x4e, 0xd0, 0xf3, 0xbd, 0x30, 0x31, 0x77, 0xe4, 0xde, 0x0d, 0xc7, 0xc7,
	0xa3, 0x08, 0x7b, 0x71, 0x5c, 0x81, 0xd5, 0x0a, 0x5c, 0x59, 0x5e, 0x78, 0xe9, 0x99, 0x80, 0x8e,
	0xc0, 0xa8, 0x0c, 0x3c, 0xf4, 0xdc, 0x51, 0xb7, 0xdb, 0x35, 0x7f, 0xea, 0xe8, 0x39, 0x3c, 0x73,
	0x6f, 0x42, 0x27, 0xf0, 0x2f, 0x47, 0x4e, 0x1c, 0xfb, 0x57, 0x61, 0x20, 0xa9, 0x46, 0x6f, 0xcf,
	0xcf, 0xcf, 0xcd, 0x5f, 0x3a, 0x7a, 0x09, 0xd6, 0x3f, 0xa2, 0xdd, 0xea, 0x33, 0xbf, 0x3c, 0x68,
	0xe8, 0x7f, 0x68, 0xae, 0x09, 0xab, 0x1c, 0x99, 0xf5, 0xf5, 0x41, 0x3b, 0xbb, 0x00, 0xd8, 0x5c,
	0x0b, 0xd2, 0xe5, 0x35, 0x0e, 0x02, 0xa9, 0x5c, 0x3e, 0x9c, 0xe1, 0x95, 0x14, 0x0d, 0xa0, 0x05,
	0x9e, 0xeb, 0x3b, 0xa1, 0xd4, 0x7c, 0x04, 0x07, 0x71, 0xe2, 0x8e, 0x5c, 0x6f, 0xe8, 0x3b, 0x89,
	0xdf, 0x0f, 0x4d, 0xb5, 0x77, 0x02, 0x56, 0x3a, 0xb5, 0xc9, 0x1d, 0xfd, 0x4c, 0x8a, 0x94, 0x11,
	0xfb, 0x96, 0x51, 0x3e, 0x29, 0xed, 0x94, 0xce, 0xca, 0x9e, 0x7a, 0x5d, 0x8e, 0x23, 0xe5, 0xdb,
	0x8e, 0x7a, 0xdd, 0xef, 0x8d, 0xb5, 0xfa, 0x72, 0xde, 0xfd, 0x09, 0x00, 0x00, 0xff, 0xff, 0x37,
	0x73, 0xb0, 0xfc, 0x31, 0x03, 0x00, 0x00,
}
