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
	IsClean              bool     `protobuf:"varint,19,opt,name=IsClean,proto3" json:"IsClean,omitempty"`
	SecurityStatus       string   `protobuf:"bytes,20,opt,name=SecurityStatus,proto3" json:"SecurityStatus,omitempty"`
	EncryptionCapable    string   `protobuf:"bytes,21,opt,name=EncryptionCapable,proto3" json:"EncryptionCapable,omitempty"`
	EncryptionProtocol   string   `protobuf:"bytes,22,opt,name=EncryptionProtocol,proto3" json:"EncryptionProtocol,omitempty"`
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

func (m *Drive) GetIsClean() bool {
	if m != nil {
		return m.IsClean
	}
	return false
}

func (m *Drive) GetSecurityStatus() string {
	if m != nil {
		return m.SecurityStatus
	}
	return ""
}

func (m *Drive) GetEncryptionCapable() string {
	if m != nil {
		return m.EncryptionCapable
	}
	return ""
}

func (m *Drive) GetEncryptionProtocol() string {
	if m != nil {
		return m.EncryptionProtocol
	}
	return ""
}

type Volume struct {
	Id                string   `protobuf:"bytes,1,opt,name=Id,proto3" json:"Id,omitempty"`
	Location          string   `protobuf:"bytes,2,opt,name=Location,proto3" json:"Location,omitempty"`
	LocationType      string   `protobuf:"bytes,3,opt,name=LocationType,proto3" json:"LocationType,omitempty"`
	StorageClass      string   `protobuf:"bytes,4,opt,name=StorageClass,proto3" json:"StorageClass,omitempty"`
	NodeId            string   `protobuf:"bytes,5,opt,name=NodeId,proto3" json:"NodeId,omitempty"`
	Owners            []string `protobuf:"bytes,6,rep,name=Owners,proto3" json:"Owners,omitempty"`
	Size              int64    `protobuf:"varint,7,opt,name=Size,proto3" json:"Size,omitempty"`
	Mode              string   `protobuf:"bytes,8,opt,name=Mode,proto3" json:"Mode,omitempty"`
	Type              string   `protobuf:"bytes,9,opt,name=Type,proto3" json:"Type,omitempty"`
	Health            string   `protobuf:"bytes,10,opt,name=Health,proto3" json:"Health,omitempty"`
	OperationalStatus string   `protobuf:"bytes,11,opt,name=OperationalStatus,proto3" json:"OperationalStatus,omitempty"`
	CSIStatus         string   `protobuf:"bytes,12,opt,name=CSIStatus,proto3" json:"CSIStatus,omitempty"`
	Usage             string   `protobuf:"bytes,13,opt,name=Usage,proto3" json:"Usage,omitempty"`
	// inline volumes are not support anymore. need to remove field in the next version
	Ephemeral            bool     `protobuf:"varint,14,opt,name=Ephemeral,proto3" json:"Ephemeral,omitempty"`
	StorageGroup         string   `protobuf:"bytes,15,opt,name=StorageGroup,proto3" json:"StorageGroup,omitempty"`
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

func (m *Volume) GetStorageGroup() string {
	if m != nil {
		return m.StorageGroup
	}
	return ""
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
	NodeRequests         *NodeRequests         `protobuf:"bytes,3,opt,name=NodeRequests,proto3" json:"NodeRequests,omitempty"`
	ReservationRequests  []*ReservationRequest `protobuf:"bytes,4,rep,name=ReservationRequests,proto3" json:"ReservationRequests,omitempty"`
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

func (m *AvailableCapacityReservation) GetNodeRequests() *NodeRequests {
	if m != nil {
		return m.NodeRequests
	}
	return nil
}

func (m *AvailableCapacityReservation) GetReservationRequests() []*ReservationRequest {
	if m != nil {
		return m.ReservationRequests
	}
	return nil
}

type NodeRequests struct {
	// requested - filled by scheduler/extender
	Requested []string `protobuf:"bytes,1,rep,name=Requested,proto3" json:"Requested,omitempty"`
	// reserved - filled by csi driver controller
	Reserved             []string `protobuf:"bytes,2,rep,name=Reserved,proto3" json:"Reserved,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeRequests) Reset()         { *m = NodeRequests{} }
func (m *NodeRequests) String() string { return proto.CompactTextString(m) }
func (*NodeRequests) ProtoMessage()    {}
func (*NodeRequests) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{4}
}

func (m *NodeRequests) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRequests.Unmarshal(m, b)
}
func (m *NodeRequests) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRequests.Marshal(b, m, deterministic)
}
func (m *NodeRequests) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRequests.Merge(m, src)
}
func (m *NodeRequests) XXX_Size() int {
	return xxx_messageInfo_NodeRequests.Size(m)
}
func (m *NodeRequests) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRequests.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRequests proto.InternalMessageInfo

func (m *NodeRequests) GetRequested() []string {
	if m != nil {
		return m.Requested
	}
	return nil
}

func (m *NodeRequests) GetReserved() []string {
	if m != nil {
		return m.Reserved
	}
	return nil
}

type ReservationRequest struct {
	// request per volume filled by scheduler/extender
	CapacityRequest *CapacityRequest `protobuf:"bytes,1,opt,name=CapacityRequest,proto3" json:"CapacityRequest,omitempty"`
	// reservation filled by csi driver controller
	Reservations         []string `protobuf:"bytes,2,rep,name=Reservations,proto3" json:"Reservations,omitempty"`
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

func (m *ReservationRequest) GetCapacityRequest() *CapacityRequest {
	if m != nil {
		return m.CapacityRequest
	}
	return nil
}

func (m *ReservationRequest) GetReservations() []string {
	if m != nil {
		return m.Reservations
	}
	return nil
}

type CapacityRequest struct {
	Name                 string   `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	StorageClass         string   `protobuf:"bytes,2,opt,name=StorageClass,proto3" json:"StorageClass,omitempty"`
	Size                 int64    `protobuf:"varint,3,opt,name=Size,proto3" json:"Size,omitempty"`
	StorageGroup         string   `protobuf:"bytes,4,opt,name=StorageGroup,proto3" json:"StorageGroup,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CapacityRequest) Reset()         { *m = CapacityRequest{} }
func (m *CapacityRequest) String() string { return proto.CompactTextString(m) }
func (*CapacityRequest) ProtoMessage()    {}
func (*CapacityRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{6}
}

func (m *CapacityRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CapacityRequest.Unmarshal(m, b)
}
func (m *CapacityRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CapacityRequest.Marshal(b, m, deterministic)
}
func (m *CapacityRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CapacityRequest.Merge(m, src)
}
func (m *CapacityRequest) XXX_Size() int {
	return xxx_messageInfo_CapacityRequest.Size(m)
}
func (m *CapacityRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CapacityRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CapacityRequest proto.InternalMessageInfo

func (m *CapacityRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *CapacityRequest) GetStorageClass() string {
	if m != nil {
		return m.StorageClass
	}
	return ""
}

func (m *CapacityRequest) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *CapacityRequest) GetStorageGroup() string {
	if m != nil {
		return m.StorageGroup
	}
	return ""
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
	return fileDescriptor_d938547f84707355, []int{7}
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
	return fileDescriptor_d938547f84707355, []int{8}
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
	proto.RegisterType((*NodeRequests)(nil), "v1api.NodeRequests")
	proto.RegisterType((*ReservationRequest)(nil), "v1api.ReservationRequest")
	proto.RegisterType((*CapacityRequest)(nil), "v1api.CapacityRequest")
	proto.RegisterType((*LogicalVolumeGroup)(nil), "v1api.LogicalVolumeGroup")
	proto.RegisterType((*Node)(nil), "v1api.Node")
	proto.RegisterMapType((map[string]string)(nil), "v1api.Node.AddressesEntry")
}

func init() { proto.RegisterFile("types.proto", fileDescriptor_d938547f84707355) }

var fileDescriptor_d938547f84707355 = []byte{
	// 864 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x96, 0xcd, 0x6e, 0xe3, 0x36,
	0x10, 0xc7, 0x21, 0x7f, 0x25, 0x1e, 0x67, 0xb3, 0x1b, 0x66, 0x1b, 0xb0, 0x41, 0x50, 0x18, 0x3a,
	0x14, 0x39, 0x2c, 0x0c, 0x34, 0x3d, 0x74, 0x51, 0xf4, 0xd0, 0x4d, 0x9c, 0x76, 0x85, 0x6e, 0xb3,
	0x81, 0xdc, 0xec, 0xa1, 0x37, 0x46, 0x9a, 0x26, 0x42, 0x65, 0x4b, 0x25, 0x25, 0x2f, 0x94, 0x4b,
	0xd1, 0x57, 0xe8, 0x33, 0xf4, 0x39, 0xfa, 0x12, 0xbd, 0xf5, 0x69, 0x16, 0x43, 0xd2, 0x12, 0x65,
	0xeb, 0x36, 0xf3, 0x9f, 0x21, 0x39, 0x9a, 0xf9, 0x91, 0x36, 0x4c, 0x8a, 0x2a, 0x47, 0x35, 0xcb,
	0x65, 0x56, 0x64, 0x6c, 0xb8, 0xfe, 0x4a, 0xe4, 0x89, 0xff, 0xdf, 0x00, 0x86, 0x73, 0x99, 0xac,
	0x91, 0x31, 0x18, 0xdc, 0xdd, 0x05, 0x73, 0xee, 0x4d, 0xbd, 0xf3, 0x71, 0xa8, 0x6d, 0xf6, 0x02,
	0xfa, 0x1f, 0x82, 0x39, 0xef, 0x69, 0x89, 0x4c, 0x52, 0x6e, 0x83, 0x39, 0xef, 0x1b, 0xe5, 0x36,
	0x98, 0x33, 0x1f, 0x0e, 0x16, 0x28, 0x13, 0x91, 0xde, 0x94, 0xcb, 0x7b, 0x94, 0x7c, 0xa0, 0x43,
	0x2d, 0x8d, 0x9d, 0xc0, 0xe8, 0x2d, 0x8a, 0xb4, 0x78, 0xe4, 0x43, 0x1d, 0xb5, 0x1e, 0x9d, 0xf9,
	0x4b, 0x95, 0x23, 0x1f, 0x99, 0x33, 0xc9, 0x26, 0x6d, 0x91, 0x3c, 0x21, 0xdf, 0x9b, 0x7a, 0xe7,
	0xfd, 0x50, 0xdb, 0xb4, 0x7e, 0x51, 0x88, 0xa2, 0x54, 0x7c, 0xdf, 0xac, 0x37, 0x1e, 0x7b, 0x09,
	0xc3, 0x3b, 0x25, 0x1e, 0x90, 0x8f, 0xb5, 0x6c, 0x1c, 0xca, 0xbe, 0xc9, 0x62, 0x0c, 0x62, 0x0e,
	0x26, 0xdb, 0x78, 0xb4, 0xf3, 0xad, 0x28, 0x1e, 0xf9, 0xc4, 0x9c, 0x46, 0x36, 0x3b, 0x83, 0xf1,
	0xf5, 0x2a, 0x4a, 0x33, 0x55, 0x4a, 0xe4, 0x07, 0x3a, 0xd0, 0x08, 0xba, 0x96, 0x34, 0x2b, 0xf8,
	0x33, 0xb3, 0x82, 0x6c, 0xea, 0xc0, 0xa5, 0xa8, 0xf8, 0xa1, 0xe9, 0xc0, 0xa5, 0xa8, 0xd8, 0x29,
	0xec, 0xff, 0x90, 0xc8, 0xe5, 0x47, 0x21, 0x91, 0x3f, 0xd7, 0x72, 0xed, 0x9b, 0xfd, 0xe3, 0x52,
	0x8a, 0x55, 0x84, 0xfc, 0x85, 0xfe, 0xa4, 0x46, 0xa0, 0x95, 0xef, 0xae, 0xe7, 0xf4, 0x31, 0xc8,
	0x8f, 0xcc, 0xca, 0x8d, 0x4f, 0xb1, 0x40, 0x2d, 0x2a, 0x55, 0xe0, 0x92, 0xb3, 0xa9, 0x77, 0xbe,
	0x1f, 0xd6, 0x3e, 0xe3, 0xb0, 0x17, 0xa8, 0xab, 0x14, 0xc5, 0x8a, 0x1f, 0xeb, 0xd0, 0xc6, 0x65,
	0x5f, 0xc2, 0xe1, 0x02, 0xa3, 0x52, 0x26, 0x45, 0x65, 0x3b, 0xf6, 0x52, 0xef, 0xbb, 0xa5, 0xb2,
	0x57, 0x70, 0x74, 0xbd, 0x8a, 0x64, 0x95, 0x17, 0x49, 0xb6, 0xba, 0x12, 0xb9, 0xb8, 0x4f, 0x91,
	0x7f, 0xa6, 0x53, 0x77, 0x03, 0x6c, 0x06, 0xac, 0x11, 0x6f, 0x89, 0x9f, 0x28, 0x4b, 0xf9, 0x89,
	0x4e, 0xef, 0x88, 0xf8, 0xff, 0xf4, 0x61, 0xf4, 0x21, 0x4b, 0xcb, 0x25, 0xb2, 0x43, 0xe8, 0x05,
	0xb1, 0x85, 0xaa, 0x17, 0xc4, 0xfa, 0x93, 0xb3, 0x48, 0x50, 0xba, 0xe5, 0xaa, 0xf6, 0x09, 0xa5,
	0x8d, 0xad, 0xb1, 0x30, 0x94, 0xb5, 0x34, 0x8d, 0x5b, 0x91, 0x49, 0xf1, 0x80, 0x57, 0xa9, 0x50,
	0xaa, 0xc6, 0xcd, 0xd1, 0x1c, 0x00, 0x86, 0x2d, 0x00, 0x4e, 0x60, 0xf4, 0xfe, 0xe3, 0x0a, 0xa5,
	0xe2, 0xa3, 0x69, 0x9f, 0x74, 0xe3, 0x75, 0x22, 0xc7, 0x60, 0xf0, 0x73, 0x16, 0xa3, 0x05, 0x4e,
	0xdb, 0x35, 0xae, 0x63, 0x07, 0xd7, 0x06, 0x6d, 0x68, 0xa1, 0xfd, 0x0a, 0x8e, 0xde, 0xe7, 0x28,
	0x75, 0xe1, 0x22, 0xb5, 0xb3, 0x30, 0xe4, 0xed, 0x06, 0x08, 0x93, 0xab, 0x45, 0x60, 0xb3, 0x2c,
	0x86, 0xb5, 0xd0, 0x60, 0xfe, 0xcc, 0xc5, 0x9c, 0xd0, 0xca, 0x1f, 0x71, 0x89, 0x52, 0xa4, 0x1a,
	0xc7, 0xfd, 0xb0, 0x11, 0x9c, 0x3e, 0xfd, 0x28, 0xb3, 0x32, 0xb7, 0x60, 0xb6, 0x34, 0xff, 0x4f,
	0x38, 0x7a, 0xb3, 0x16, 0x49, 0x4a, 0x33, 0xa6, 0x51, 0x47, 0x49, 0x51, 0xb5, 0x06, 0xe4, 0x6d,
	0x0d, 0xa8, 0x69, 0x6c, 0xaf, 0xd5, 0x58, 0x1f, 0x0e, 0x94, 0x3b, 0x14, 0x3b, 0x38, 0x57, 0xab,
	0x9b, 0x3c, 0x68, 0x9a, 0xec, 0xff, 0xef, 0xc1, 0xd9, 0x4e, 0x05, 0x21, 0x2a, 0x94, 0x6b, 0x73,
	0xe0, 0x19, 0x8c, 0x6f, 0xc4, 0x12, 0x55, 0x2e, 0x22, 0xb4, 0xd5, 0x34, 0x82, 0xf3, 0x2c, 0xf4,
	0x5a, 0xcf, 0xc2, 0x37, 0x70, 0x40, 0x85, 0x85, 0xf8, 0x47, 0x89, 0xaa, 0x30, 0xe5, 0x4c, 0x2e,
	0x8e, 0x67, 0xfa, 0xc9, 0x9b, 0xb9, 0xa1, 0xb0, 0x95, 0xc8, 0x7e, 0x82, 0x63, 0xe7, 0xf4, 0x7a,
	0xfd, 0x60, 0xda, 0x3f, 0x9f, 0x5c, 0x7c, 0x6e, 0xd7, 0xef, 0x66, 0x84, 0x5d, 0xab, 0xfc, 0xb7,
	0xed, 0x2a, 0xe8, 0x5b, 0xac, 0x8d, 0x74, 0x21, 0x08, 0xc0, 0x46, 0xa0, 0xb6, 0x9b, 0x4d, 0x90,
	0x9a, 0x4b, 0xc1, 0xda, 0xf7, 0x9f, 0x80, 0xed, 0x1e, 0xc0, 0xbe, 0x87, 0xe7, 0x4d, 0xcb, 0xb4,
	0xa4, 0x3b, 0x34, 0xb9, 0x38, 0xb1, 0x85, 0x6e, 0x45, 0xc3, 0xed, 0x74, 0x1a, 0x9b, 0xb3, 0xaf,
	0xb2, 0xe7, 0xb6, 0x34, 0xff, 0x2f, 0x6f, 0xe7, 0x18, 0x1a, 0x25, 0x0d, 0x61, 0xf3, 0x53, 0x41,
	0xf6, 0xce, 0xbd, 0xec, 0x75, 0xdc, 0xcb, 0x0d, 0x02, 0x7d, 0xe7, 0x9e, 0x6d, 0x73, 0x3a, 0xe8,
	0xe0, 0xf4, 0x5f, 0x0f, 0xd8, 0xbb, 0xec, 0x21, 0x89, 0x44, 0x6a, 0x5e, 0x15, 0x2d, 0x77, 0x96,
	0x41, 0x1a, 0x5d, 0xdb, 0x9e, 0xd5, 0xe8, 0xda, 0x9e, 0xc1, 0x78, 0x43, 0x30, 0xb1, 0xa0, 0x1b,
	0x5f, 0x0b, 0x5d, 0x5c, 0xb2, 0x2f, 0x00, 0xcc, 0x41, 0x21, 0xfe, 0xa6, 0xf8, 0x50, 0x2f, 0x71,
	0x14, 0x07, 0xbc, 0x51, 0x0b, 0xbc, 0xe6, 0x31, 0xd8, 0x73, 0x1f, 0x03, 0xff, 0x6f, 0xcf, 0x94,
	0xd5, 0xf9, 0x23, 0xfb, 0x1a, 0xc6, 0x6f, 0xe2, 0x58, 0xa2, 0x52, 0x68, 0x46, 0x30, 0xb9, 0x38,
	0x75, 0x50, 0x9d, 0xd5, 0xc1, 0xeb, 0x55, 0x21, 0xab, 0xb0, 0x49, 0x3e, 0xfd, 0x0e, 0x0e, 0xdb,
	0x41, 0xfa, 0x71, 0xfa, 0x1d, 0x2b, 0xbb, 0x3d, 0x99, 0xf4, 0x76, 0xac, 0x45, 0x5a, 0x6e, 0x3a,
	0x62, 0x9c, 0x6f, 0x7b, 0xaf, 0xbd, 0xcb, 0xbd, 0x5f, 0xcd, 0x7f, 0x80, 0xfb, 0x91, 0xfe, 0x47,
	0xf0, 0xf5, 0xa7, 0x00, 0x00, 0x00, 0xff, 0xff, 0x26, 0xe6, 0x81, 0x44, 0x20, 0x08, 0x00, 0x00,
}
