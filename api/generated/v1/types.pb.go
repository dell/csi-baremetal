// Code generated by protoc-gen-go. DO NOT EDIT.
// source: types.proto

package v1api

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Drive struct {
	UUID         string `protobuf:"bytes,1,opt,name=UUID,proto3" json:"UUID,omitempty"`
	VID          string `protobuf:"bytes,2,opt,name=VID,proto3" json:"VID,omitempty"`
	PID          string `protobuf:"bytes,3,opt,name=PID,proto3" json:"PID,omitempty"`
	SerialNumber string `protobuf:"bytes,4,opt,name=SerialNumber,proto3" json:"SerialNumber,omitempty"`
	Health       string `protobuf:"bytes,5,opt,name=Health,proto3" json:"Health,omitempty"`
	Type         string `protobuf:"bytes,6,opt,name=Type,proto3" json:"Type,omitempty"`
	// size in bytes
	Size   int64  `protobuf:"varint,7,opt,name=Size,proto3" json:"Size,omitempty"`
	Status string `protobuf:"bytes,8,opt,name=Status,proto3" json:"Status,omitempty"`
	Usage  string `protobuf:"bytes,9,opt,name=Usage,proto3" json:"Usage,omitempty"`
	NodeId string `protobuf:"bytes,10,opt,name=NodeId,proto3" json:"NodeId,omitempty"`
	// path to the device. may not be set by drivemgr.
	Path                 string   `protobuf:"bytes,11,opt,name=Path,proto3" json:"Path,omitempty"`
	Enclosure            string   `protobuf:"bytes,12,opt,name=Enclosure,proto3" json:"Enclosure,omitempty"`
	Slot                 string   `protobuf:"bytes,13,opt,name=Slot,proto3" json:"Slot,omitempty"`
	Bay                  string   `protobuf:"bytes,14,opt,name=Bay,proto3" json:"Bay,omitempty"`
	Firmware             string   `protobuf:"bytes,15,opt,name=Firmware,proto3" json:"Firmware,omitempty"`
	Endurance            int64    `protobuf:"varint,16,opt,name=Endurance,proto3" json:"Endurance,omitempty"`
	LEDState             string   `protobuf:"bytes,17,opt,name=LEDState,proto3" json:"LEDState,omitempty"`
	IsSystem             bool     `protobuf:"varint,18,opt,name=IsSystem,proto3" json:"IsSystem,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Drive) Reset()         { *m = Drive{} }
func (m *Drive) String() string { return proto.CompactTextString(m) }
func (*Drive) ProtoMessage()    {}
func (*Drive) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{0}
}

func (m *Drive) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Drive.Unmarshal(m, b)
}
func (m *Drive) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Drive.Marshal(b, m, deterministic)
}
func (m *Drive) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Drive.Merge(m, src)
}
func (m *Drive) XXX_Size() int {
	return xxx_messageInfo_Drive.Size(m)
}
func (m *Drive) XXX_DiscardUnknown() {
	xxx_messageInfo_Drive.DiscardUnknown(m)
}

var xxx_messageInfo_Drive proto.InternalMessageInfo

func (m *Drive) GetUUID() string {
	if m != nil {
		return m.UUID
	}
	return ""
}

func (m *Drive) GetVID() string {
	if m != nil {
		return m.VID
	}
	return ""
}

func (m *Drive) GetPID() string {
	if m != nil {
		return m.PID
	}
	return ""
}

func (m *Drive) GetSerialNumber() string {
	if m != nil {
		return m.SerialNumber
	}
	return ""
}

func (m *Drive) GetHealth() string {
	if m != nil {
		return m.Health
	}
	return ""
}

func (m *Drive) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *Drive) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *Drive) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *Drive) GetUsage() string {
	if m != nil {
		return m.Usage
	}
	return ""
}

func (m *Drive) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *Drive) GetPath() string {
	if m != nil {
		return m.Path
	}
	return ""
}

func (m *Drive) GetEnclosure() string {
	if m != nil {
		return m.Enclosure
	}
	return ""
}

func (m *Drive) GetSlot() string {
	if m != nil {
		return m.Slot
	}
	return ""
}

func (m *Drive) GetBay() string {
	if m != nil {
		return m.Bay
	}
	return ""
}

func (m *Drive) GetFirmware() string {
	if m != nil {
		return m.Firmware
	}
	return ""
}

func (m *Drive) GetEndurance() int64 {
	if m != nil {
		return m.Endurance
	}
	return 0
}

func (m *Drive) GetLEDState() string {
	if m != nil {
		return m.LEDState
	}
	return ""
}

func (m *Drive) GetIsSystem() bool {
	if m != nil {
		return m.IsSystem
	}
	return false
}

type Volume struct {
	Id                   string   `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Location             string   `protobuf:"bytes,2,opt,name=Location,proto3" json:"Location,omitempty"`
	LocationType         string   `protobuf:"bytes,3,opt,name=LocationType,proto3" json:"LocationType,omitempty"`
	StorageClass         string   `protobuf:"bytes,4,opt,name=StorageClass,proto3" json:"StorageClass,omitempty"`
	NodeId               string   `protobuf:"bytes,5,opt,name=NodeId,proto3" json:"NodeId,omitempty"`
	Owners               []string `protobuf:"bytes,6,rep,name=Owners,proto3" json:"Owners,omitempty"`
	Size                 int64    `protobuf:"varint,7,opt,name=Size,proto3" json:"Size,omitempty"`
	Mode                 string   `protobuf:"bytes,8,opt,name=Mode,proto3" json:"Mode,omitempty"`
	Type                 string   `protobuf:"bytes,9,opt,name=Type,proto3" json:"Type,omitempty"`
	Health               string   `protobuf:"bytes,10,opt,name=Health,proto3" json:"Health,omitempty"`
	OperationalStatus    string   `protobuf:"bytes,11,opt,name=OperationalStatus,proto3" json:"OperationalStatus,omitempty"`
	CSIStatus            string   `protobuf:"bytes,12,opt,name=CSIStatus,proto3" json:"CSIStatus,omitempty"`
	Usage                string   `protobuf:"bytes,13,opt,name=Usage,proto3" json:"Usage,omitempty"`
	Ephemeral            bool     `protobuf:"varint,14,opt,name=Ephemeral,proto3" json:"Ephemeral,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Volume) Reset()         { *m = Volume{} }
func (m *Volume) String() string { return proto.CompactTextString(m) }
func (*Volume) ProtoMessage()    {}
func (*Volume) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{1}
}

func (m *Volume) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Volume.Unmarshal(m, b)
}
func (m *Volume) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Volume.Marshal(b, m, deterministic)
}
func (m *Volume) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Volume.Merge(m, src)
}
func (m *Volume) XXX_Size() int {
	return xxx_messageInfo_Volume.Size(m)
}
func (m *Volume) XXX_DiscardUnknown() {
	xxx_messageInfo_Volume.DiscardUnknown(m)
}

var xxx_messageInfo_Volume proto.InternalMessageInfo

func (m *Volume) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *Volume) GetLocation() string {
	if m != nil {
		return m.Location
	}
	return ""
}

func (m *Volume) GetLocationType() string {
	if m != nil {
		return m.LocationType
	}
	return ""
}

func (m *Volume) GetStorageClass() string {
	if m != nil {
		return m.StorageClass
	}
	return ""
}

func (m *Volume) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *Volume) GetOwners() []string {
	if m != nil {
		return m.Owners
	}
	return nil
}

func (m *Volume) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *Volume) GetMode() string {
	if m != nil {
		return m.Mode
	}
	return ""
}

func (m *Volume) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *Volume) GetHealth() string {
	if m != nil {
		return m.Health
	}
	return ""
}

func (m *Volume) GetOperationalStatus() string {
	if m != nil {
		return m.OperationalStatus
	}
	return ""
}

func (m *Volume) GetCSIStatus() string {
	if m != nil {
		return m.CSIStatus
	}
	return ""
}

func (m *Volume) GetUsage() string {
	if m != nil {
		return m.Usage
	}
	return ""
}

func (m *Volume) GetEphemeral() bool {
	if m != nil {
		return m.Ephemeral
	}
	return false
}

type AvailableCapacity struct {
	Location             string   `protobuf:"bytes,1,opt,name=Location,proto3" json:"Location,omitempty"`
	NodeId               string   `protobuf:"bytes,2,opt,name=NodeId,proto3" json:"NodeId,omitempty"`
	StorageClass         string   `protobuf:"bytes,3,opt,name=storageClass,proto3" json:"storageClass,omitempty"`
	Size                 int64    `protobuf:"varint,4,opt,name=Size,proto3" json:"Size,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AvailableCapacity) Reset()         { *m = AvailableCapacity{} }
func (m *AvailableCapacity) String() string { return proto.CompactTextString(m) }
func (*AvailableCapacity) ProtoMessage()    {}
func (*AvailableCapacity) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{2}
}

func (m *AvailableCapacity) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AvailableCapacity.Unmarshal(m, b)
}
func (m *AvailableCapacity) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AvailableCapacity.Marshal(b, m, deterministic)
}
func (m *AvailableCapacity) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AvailableCapacity.Merge(m, src)
}
func (m *AvailableCapacity) XXX_Size() int {
	return xxx_messageInfo_AvailableCapacity.Size(m)
}
func (m *AvailableCapacity) XXX_DiscardUnknown() {
	xxx_messageInfo_AvailableCapacity.DiscardUnknown(m)
}

var xxx_messageInfo_AvailableCapacity proto.InternalMessageInfo

func (m *AvailableCapacity) GetLocation() string {
	if m != nil {
		return m.Location
	}
	return ""
}

func (m *AvailableCapacity) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *AvailableCapacity) GetStorageClass() string {
	if m != nil {
		return m.StorageClass
	}
	return ""
}

func (m *AvailableCapacity) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

type AvailableCapacityReservation struct {
	Namespace            string                `protobuf:"bytes,1,opt,name=Namespace,proto3" json:"Namespace,omitempty"`
	Status               string                `protobuf:"bytes,2,opt,name=Status,proto3" json:"Status,omitempty"`
	Nodes                []*NodeRequest        `protobuf:"bytes,3,rep,name=Nodes,proto3" json:"Nodes,omitempty"`
	Requests             []*ReservationRequest `protobuf:"bytes,4,rep,name=Requests,proto3" json:"Requests,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *AvailableCapacityReservation) Reset()         { *m = AvailableCapacityReservation{} }
func (m *AvailableCapacityReservation) String() string { return proto.CompactTextString(m) }
func (*AvailableCapacityReservation) ProtoMessage()    {}
func (*AvailableCapacityReservation) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{3}
}

func (m *AvailableCapacityReservation) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AvailableCapacityReservation.Unmarshal(m, b)
}
func (m *AvailableCapacityReservation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AvailableCapacityReservation.Marshal(b, m, deterministic)
}
func (m *AvailableCapacityReservation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AvailableCapacityReservation.Merge(m, src)
}
func (m *AvailableCapacityReservation) XXX_Size() int {
	return xxx_messageInfo_AvailableCapacityReservation.Size(m)
}
func (m *AvailableCapacityReservation) XXX_DiscardUnknown() {
	xxx_messageInfo_AvailableCapacityReservation.DiscardUnknown(m)
}

var xxx_messageInfo_AvailableCapacityReservation proto.InternalMessageInfo

func (m *AvailableCapacityReservation) GetNamespace() string {
	if m != nil {
		return m.Namespace
	}
	return ""
}

func (m *AvailableCapacityReservation) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *AvailableCapacityReservation) GetNodes() []*NodeRequest {
	if m != nil {
		return m.Nodes
	}
	return nil
}

func (m *AvailableCapacityReservation) GetRequests() []*ReservationRequest {
	if m != nil {
		return m.Requests
	}
	return nil
}

type NodeRequest struct {
	Id                   string   `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Score                int64    `protobuf:"varint,2,opt,name=Score,proto3" json:"Score,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeRequest) Reset()         { *m = NodeRequest{} }
func (m *NodeRequest) String() string { return proto.CompactTextString(m) }
func (*NodeRequest) ProtoMessage()    {}
func (*NodeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{4}
}

func (m *NodeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRequest.Unmarshal(m, b)
}
func (m *NodeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRequest.Marshal(b, m, deterministic)
}
func (m *NodeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRequest.Merge(m, src)
}
func (m *NodeRequest) XXX_Size() int {
	return xxx_messageInfo_NodeRequest.Size(m)
}
func (m *NodeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRequest proto.InternalMessageInfo

func (m *NodeRequest) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *NodeRequest) GetScore() int64 {
	if m != nil {
		return m.Score
	}
	return 0
}

type ReservationRequest struct {
	Name                 string   `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	StorageClass         string   `protobuf:"bytes,2,opt,name=StorageClass,proto3" json:"StorageClass,omitempty"`
	Size                 int64    `protobuf:"varint,3,opt,name=Size,proto3" json:"Size,omitempty"`
	Reservations         []string `protobuf:"bytes,4,rep,name=Reservations,proto3" json:"Reservations,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReservationRequest) Reset()         { *m = ReservationRequest{} }
func (m *ReservationRequest) String() string { return proto.CompactTextString(m) }
func (*ReservationRequest) ProtoMessage()    {}
func (*ReservationRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{5}
}

func (m *ReservationRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReservationRequest.Unmarshal(m, b)
}
func (m *ReservationRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReservationRequest.Marshal(b, m, deterministic)
}
func (m *ReservationRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReservationRequest.Merge(m, src)
}
func (m *ReservationRequest) XXX_Size() int {
	return xxx_messageInfo_ReservationRequest.Size(m)
}
func (m *ReservationRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReservationRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReservationRequest proto.InternalMessageInfo

func (m *ReservationRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ReservationRequest) GetStorageClass() string {
	if m != nil {
		return m.StorageClass
	}
	return ""
}

func (m *ReservationRequest) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *ReservationRequest) GetReservations() []string {
	if m != nil {
		return m.Reservations
	}
	return nil
}

type LogicalVolumeGroup struct {
	Name                 string   `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	Node                 string   `protobuf:"bytes,2,opt,name=Node,proto3" json:"Node,omitempty"`
	Locations            []string `protobuf:"bytes,3,rep,name=Locations,proto3" json:"Locations,omitempty"`
	Size                 int64    `protobuf:"varint,4,opt,name=Size,proto3" json:"Size,omitempty"`
	VolumeRefs           []string `protobuf:"bytes,5,rep,name=VolumeRefs,proto3" json:"VolumeRefs,omitempty"`
	Status               string   `protobuf:"bytes,6,opt,name=Status,proto3" json:"Status,omitempty"`
	Health               string   `protobuf:"bytes,7,opt,name=Health,proto3" json:"Health,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LogicalVolumeGroup) Reset()         { *m = LogicalVolumeGroup{} }
func (m *LogicalVolumeGroup) String() string { return proto.CompactTextString(m) }
func (*LogicalVolumeGroup) ProtoMessage()    {}
func (*LogicalVolumeGroup) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{6}
}

func (m *LogicalVolumeGroup) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LogicalVolumeGroup.Unmarshal(m, b)
}
func (m *LogicalVolumeGroup) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LogicalVolumeGroup.Marshal(b, m, deterministic)
}
func (m *LogicalVolumeGroup) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LogicalVolumeGroup.Merge(m, src)
}
func (m *LogicalVolumeGroup) XXX_Size() int {
	return xxx_messageInfo_LogicalVolumeGroup.Size(m)
}
func (m *LogicalVolumeGroup) XXX_DiscardUnknown() {
	xxx_messageInfo_LogicalVolumeGroup.DiscardUnknown(m)
}

var xxx_messageInfo_LogicalVolumeGroup proto.InternalMessageInfo

func (m *LogicalVolumeGroup) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *LogicalVolumeGroup) GetNode() string {
	if m != nil {
		return m.Node
	}
	return ""
}

func (m *LogicalVolumeGroup) GetLocations() []string {
	if m != nil {
		return m.Locations
	}
	return nil
}

func (m *LogicalVolumeGroup) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *LogicalVolumeGroup) GetVolumeRefs() []string {
	if m != nil {
		return m.VolumeRefs
	}
	return nil
}

func (m *LogicalVolumeGroup) GetStatus() string {
	if m != nil {
		return m.Status
	}
	return ""
}

func (m *LogicalVolumeGroup) GetHealth() string {
	if m != nil {
		return m.Health
	}
	return ""
}

type Node struct {
	UUID string `protobuf:"bytes,1,opt,name=UUID,proto3" json:"UUID,omitempty"`
	// key - address type, value - address, align with NodeAddress struct from k8s.io/api/core/v1
	Addresses            map[string]string `protobuf:"bytes,2,rep,name=Addresses,proto3" json:"Addresses,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *Node) Reset()         { *m = Node{} }
func (m *Node) String() string { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()    {}
func (*Node) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{7}
}

func (m *Node) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Node.Unmarshal(m, b)
}
func (m *Node) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Node.Marshal(b, m, deterministic)
}
func (m *Node) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Node.Merge(m, src)
}
func (m *Node) XXX_Size() int {
	return xxx_messageInfo_Node.Size(m)
}
func (m *Node) XXX_DiscardUnknown() {
	xxx_messageInfo_Node.DiscardUnknown(m)
}

var xxx_messageInfo_Node proto.InternalMessageInfo

func (m *Node) GetUUID() string {
	if m != nil {
		return m.UUID
	}
	return ""
}

func (m *Node) GetAddresses() map[string]string {
	if m != nil {
		return m.Addresses
	}
	return nil
}

func init() {
	proto.RegisterType((*Drive)(nil), "v1api.Drive")
	proto.RegisterType((*Volume)(nil), "v1api.Volume")
	proto.RegisterType((*AvailableCapacity)(nil), "v1api.AvailableCapacity")
	proto.RegisterType((*AvailableCapacityReservation)(nil), "v1api.AvailableCapacityReservation")
	proto.RegisterType((*NodeRequest)(nil), "v1api.NodeRequest")
	proto.RegisterType((*ReservationRequest)(nil), "v1api.ReservationRequest")
	proto.RegisterType((*LogicalVolumeGroup)(nil), "v1api.LogicalVolumeGroup")
	proto.RegisterType((*Node)(nil), "v1api.Node")
	proto.RegisterMapType((map[string]string)(nil), "v1api.Node.AddressesEntry")
}

func init() {
	proto.RegisterFile("types.proto", fileDescriptor_d938547f84707355)
}

var fileDescriptor_d938547f84707355 = []byte{
	// 750 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x55, 0xcf, 0x6f, 0xeb, 0x44,
	0x10, 0x96, 0xed, 0x38, 0x8d, 0x27, 0x69, 0x69, 0x57, 0x55, 0xb5, 0x54, 0x11, 0x8a, 0x7c, 0xca,
	0x01, 0x45, 0x82, 0x0a, 0xa9, 0x42, 0x5c, 0xda, 0xa6, 0x80, 0xa5, 0xd2, 0x56, 0x0e, 0xed, 0x81,
	0xdb, 0x36, 0x1e, 0x5a, 0x0b, 0x27, 0x36, 0xbb, 0x76, 0x2a, 0x73, 0x81, 0x03, 0x7f, 0x01, 0xff,
	0x0a, 0x7a, 0xd7, 0xf7, 0xb7, 0x3d, 0xcd, 0xae, 0xe3, 0x1f, 0x2f, 0xb9, 0xcd, 0x7c, 0x3b, 0xb3,
	0x3b, 0xfe, 0xbe, 0x6f, 0xd7, 0x30, 0xcc, 0xcb, 0x0c, 0xd5, 0x2c, 0x93, 0x69, 0x9e, 0x32, 0x77,
	0xf3, 0x8d, 0xc8, 0x62, 0xff, 0x7f, 0x07, 0xdc, 0xb9, 0x8c, 0x37, 0xc8, 0x18, 0xf4, 0x9e, 0x9e,
	0x82, 0x39, 0xb7, 0x26, 0xd6, 0xd4, 0x0b, 0x75, 0xcc, 0x8e, 0xc1, 0x79, 0x0e, 0xe6, 0xdc, 0xd6,
	0x10, 0x85, 0x84, 0x3c, 0x06, 0x73, 0xee, 0x18, 0xe4, 0x31, 0x98, 0x33, 0x1f, 0x46, 0x0b, 0x94,
	0xb1, 0x48, 0xee, 0x8b, 0xd5, 0x0b, 0x4a, 0xde, 0xd3, 0x4b, 0x1d, 0x8c, 0x9d, 0x41, 0xff, 0x67,
	0x14, 0x49, 0xfe, 0xc6, 0x5d, 0xbd, 0x5a, 0x65, 0x74, 0xe6, 0xaf, 0x65, 0x86, 0xbc, 0x6f, 0xce,
	0xa4, 0x98, 0xb0, 0x45, 0xfc, 0x17, 0xf2, 0x83, 0x89, 0x35, 0x75, 0x42, 0x1d, 0x53, 0xff, 0x22,
	0x17, 0x79, 0xa1, 0xf8, 0xc0, 0xf4, 0x9b, 0x8c, 0x9d, 0x82, 0xfb, 0xa4, 0xc4, 0x2b, 0x72, 0x4f,
	0xc3, 0x26, 0xa1, 0xea, 0xfb, 0x34, 0xc2, 0x20, 0xe2, 0x60, 0xaa, 0x4d, 0x46, 0x3b, 0x3f, 0x8a,
	0xfc, 0x8d, 0x0f, 0xcd, 0x69, 0x14, 0xb3, 0x31, 0x78, 0xb7, 0xeb, 0x65, 0x92, 0xaa, 0x42, 0x22,
	0x1f, 0xe9, 0x85, 0x06, 0xd0, 0xb3, 0x24, 0x69, 0xce, 0x0f, 0x4d, 0x07, 0xc5, 0xc4, 0xc0, 0xb5,
	0x28, 0xf9, 0x91, 0x61, 0xe0, 0x5a, 0x94, 0xec, 0x1c, 0x06, 0x3f, 0xc6, 0x72, 0xf5, 0x2e, 0x24,
	0xf2, 0x2f, 0x34, 0x5c, 0xe7, 0x66, 0xff, 0xa8, 0x90, 0x62, 0xbd, 0x44, 0x7e, 0xac, 0x3f, 0xa9,
	0x01, 0xa8, 0xf3, 0xee, 0x76, 0x4e, 0x1f, 0x83, 0xfc, 0xc4, 0x74, 0x6e, 0x73, 0x5a, 0x0b, 0xd4,
	0xa2, 0x54, 0x39, 0xae, 0x38, 0x9b, 0x58, 0xd3, 0x41, 0x58, 0xe7, 0xfe, 0x3f, 0x0e, 0xf4, 0x9f,
	0xd3, 0xa4, 0x58, 0x21, 0x3b, 0x02, 0x3b, 0x88, 0x2a, 0xd1, 0xec, 0x20, 0xd2, 0x5b, 0xa6, 0x4b,
	0x91, 0xc7, 0xe9, 0xba, 0xd2, 0xad, 0xce, 0x49, 0xaa, 0x6d, 0xac, 0x69, 0x37, 0x2a, 0x76, 0x30,
	0x2d, 0x67, 0x9e, 0x4a, 0xf1, 0x8a, 0x37, 0x89, 0x50, 0xaa, 0x96, 0xb3, 0x85, 0xb5, 0x08, 0x76,
	0x3b, 0x04, 0x9f, 0x41, 0xff, 0xe1, 0x7d, 0x8d, 0x52, 0xf1, 0xfe, 0xc4, 0x21, 0xdc, 0x64, 0x7b,
	0x25, 0x65, 0xd0, 0xfb, 0x25, 0x8d, 0xb0, 0x12, 0x54, 0xc7, 0xb5, 0x1d, 0xbc, 0x96, 0x1d, 0x1a,
	0xeb, 0x40, 0xc7, 0x3a, 0x5f, 0xc3, 0xc9, 0x43, 0x86, 0x52, 0x0f, 0x2e, 0x92, 0xca, 0x1d, 0x46,
	0xd9, 0xdd, 0x05, 0x92, 0xe1, 0x66, 0x11, 0x54, 0x55, 0x95, 0xcc, 0x35, 0xd0, 0xd8, 0xe8, 0xb0,
	0x6d, 0x23, 0x92, 0x2e, 0x7b, 0xc3, 0x15, 0x4a, 0x91, 0x68, 0xb9, 0x07, 0x61, 0x03, 0xf8, 0x7f,
	0xc3, 0xc9, 0xd5, 0x46, 0xc4, 0x89, 0x78, 0x49, 0xf0, 0x46, 0x64, 0x62, 0x19, 0xe7, 0x65, 0x87,
	0x7c, 0xeb, 0x33, 0xf2, 0x1b, 0xd2, 0xec, 0x0e, 0x69, 0x3e, 0x8c, 0x54, 0x9b, 0xf0, 0x4a, 0x94,
	0x36, 0x56, 0x13, 0xd8, 0x6b, 0x08, 0xf4, 0x3f, 0x58, 0x30, 0xde, 0x99, 0x20, 0x44, 0x85, 0x72,
	0x63, 0x0e, 0x1c, 0x83, 0x77, 0x2f, 0x56, 0xa8, 0x32, 0xb1, 0xc4, 0x6a, 0x9a, 0x06, 0x68, 0x5d,
	0x29, 0xbb, 0x73, 0xa5, 0xa6, 0xe0, 0xd2, 0x60, 0x34, 0x87, 0x33, 0x1d, 0x7e, 0xcb, 0x66, 0xfa,
	0x9d, 0x98, 0x11, 0x16, 0xe2, 0x9f, 0x05, 0xaa, 0x3c, 0x34, 0x05, 0xec, 0x3b, 0x18, 0x54, 0x08,
	0xb9, 0x84, 0x8a, 0xbf, 0xac, 0x8a, 0x5b, 0x53, 0x6c, 0x7b, 0xea, 0x52, 0xff, 0x02, 0x86, 0xad,
	0xcd, 0x76, 0xfc, 0x7b, 0x0a, 0xee, 0x62, 0x99, 0x4a, 0xd4, 0x63, 0x39, 0xa1, 0x49, 0xfc, 0x7f,
	0x2d, 0x60, 0xbb, 0xbb, 0x12, 0x2f, 0xf4, 0x45, 0xdb, 0x37, 0x8b, 0xe2, 0x1d, 0x03, 0xdb, 0x7b,
	0x0c, 0xbc, 0xe5, 0xd3, 0x69, 0x19, 0xd2, 0x87, 0x51, 0xeb, 0x04, 0xf3, 0x49, 0x5e, 0xd8, 0xc1,
	0xfc, 0x8f, 0x16, 0xb0, 0xbb, 0xf4, 0x35, 0x5e, 0x8a, 0xc4, 0x5c, 0xbf, 0x9f, 0x64, 0x5a, 0x64,
	0x7b, 0xc7, 0x20, 0x8c, 0xfc, 0x6d, 0x57, 0x18, 0xf9, 0x7b, 0x0c, 0xde, 0xd6, 0x0e, 0x86, 0x5f,
	0x2f, 0x6c, 0x80, 0x7d, 0x22, 0xb3, 0xaf, 0x00, 0xcc, 0x41, 0x21, 0xfe, 0xae, 0xb8, 0xab, 0x5b,
	0x5a, 0x48, 0x4b, 0xc5, 0x7e, 0x47, 0xc5, 0xe6, 0xd6, 0x1c, 0xb4, 0x6f, 0x8d, 0xff, 0x9f, 0x65,
	0xc6, 0xda, 0xfb, 0xda, 0x5f, 0x82, 0x77, 0x15, 0x45, 0x12, 0x95, 0x42, 0xa2, 0x8d, 0x14, 0x3d,
	0x6f, 0xc9, 0x3f, 0xab, 0x17, 0x6f, 0xd7, 0xb9, 0x2c, 0xc3, 0xa6, 0xf8, 0xfc, 0x07, 0x38, 0xea,
	0x2e, 0xd2, 0x2b, 0xf9, 0x07, 0x96, 0xd5, 0xf6, 0x14, 0x92, 0xb0, 0x1b, 0x91, 0x14, 0x5b, 0x46,
	0x4c, 0xf2, 0xbd, 0x7d, 0x69, 0x5d, 0x1f, 0xfc, 0x66, 0x7e, 0x46, 0x2f, 0x7d, 0xfd, 0x6b, 0xba,
	0xf8, 0x14, 0x00, 0x00, 0xff, 0xff, 0x2f, 0x4a, 0x01, 0x32, 0xa9, 0x06, 0x00, 0x00,
}
