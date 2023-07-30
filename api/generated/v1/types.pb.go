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

type StorageGroupSpec struct {
	DriveSelector        *DriveSelector `protobuf:"bytes,1,opt,name=driveSelector,proto3" json:"driveSelector,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *StorageGroupSpec) Reset()         { *m = StorageGroupSpec{} }
func (m *StorageGroupSpec) String() string { return proto.CompactTextString(m) }
func (*StorageGroupSpec) ProtoMessage()    {}
func (*StorageGroupSpec) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{9}
}

func (m *StorageGroupSpec) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StorageGroupSpec.Unmarshal(m, b)
}
func (m *StorageGroupSpec) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StorageGroupSpec.Marshal(b, m, deterministic)
}
func (m *StorageGroupSpec) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StorageGroupSpec.Merge(m, src)
}
func (m *StorageGroupSpec) XXX_Size() int {
	return xxx_messageInfo_StorageGroupSpec.Size(m)
}
func (m *StorageGroupSpec) XXX_DiscardUnknown() {
	xxx_messageInfo_StorageGroupSpec.DiscardUnknown(m)
}

var xxx_messageInfo_StorageGroupSpec proto.InternalMessageInfo

func (m *StorageGroupSpec) GetDriveSelector() *DriveSelector {
	if m != nil {
		return m.DriveSelector
	}
	return nil
}

type DriveSelector struct {
	NumberDrivesPerNode  int32             `protobuf:"varint,1,opt,name=numberDrivesPerNode,proto3" json:"numberDrivesPerNode,omitempty"`
	MatchFields          map[string]string `protobuf:"bytes,2,rep,name=matchFields,proto3" json:"matchFields,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *DriveSelector) Reset()         { *m = DriveSelector{} }
func (m *DriveSelector) String() string { return proto.CompactTextString(m) }
func (*DriveSelector) ProtoMessage()    {}
func (*DriveSelector) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{10}
}

func (m *DriveSelector) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DriveSelector.Unmarshal(m, b)
}
func (m *DriveSelector) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DriveSelector.Marshal(b, m, deterministic)
}
func (m *DriveSelector) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DriveSelector.Merge(m, src)
}
func (m *DriveSelector) XXX_Size() int {
	return xxx_messageInfo_DriveSelector.Size(m)
}
func (m *DriveSelector) XXX_DiscardUnknown() {
	xxx_messageInfo_DriveSelector.DiscardUnknown(m)
}

var xxx_messageInfo_DriveSelector proto.InternalMessageInfo

func (m *DriveSelector) GetNumberDrivesPerNode() int32 {
	if m != nil {
		return m.NumberDrivesPerNode
	}
	return 0
}

func (m *DriveSelector) GetMatchFields() map[string]string {
	if m != nil {
		return m.MatchFields
	}
	return nil
}

type StorageGroupStatus struct {
	Phase                string   `protobuf:"bytes,1,opt,name=phase,proto3" json:"phase,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StorageGroupStatus) Reset()         { *m = StorageGroupStatus{} }
func (m *StorageGroupStatus) String() string { return proto.CompactTextString(m) }
func (*StorageGroupStatus) ProtoMessage()    {}
func (*StorageGroupStatus) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{11}
}

func (m *StorageGroupStatus) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StorageGroupStatus.Unmarshal(m, b)
}
func (m *StorageGroupStatus) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StorageGroupStatus.Marshal(b, m, deterministic)
}
func (m *StorageGroupStatus) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StorageGroupStatus.Merge(m, src)
}
func (m *StorageGroupStatus) XXX_Size() int {
	return xxx_messageInfo_StorageGroupStatus.Size(m)
}
func (m *StorageGroupStatus) XXX_DiscardUnknown() {
	xxx_messageInfo_StorageGroupStatus.DiscardUnknown(m)
}

var xxx_messageInfo_StorageGroupStatus proto.InternalMessageInfo

func (m *StorageGroupStatus) GetPhase() string {
	if m != nil {
		return m.Phase
	}
	return ""
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
	proto.RegisterType((*StorageGroupSpec)(nil), "v1api.StorageGroupSpec")
	proto.RegisterType((*DriveSelector)(nil), "v1api.DriveSelector")
	proto.RegisterMapType((map[string]string)(nil), "v1api.DriveSelector.MatchFieldsEntry")
	proto.RegisterType((*StorageGroupStatus)(nil), "v1api.StorageGroupStatus")
}

func init() { proto.RegisterFile("types.proto", fileDescriptor_d938547f84707355) }

var fileDescriptor_d938547f84707355 = []byte{
	// 968 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x56, 0xcd, 0x6e, 0xdb, 0x46,
	0x10, 0x06, 0xf5, 0x67, 0x6b, 0x64, 0x3b, 0xf6, 0xda, 0x35, 0xb6, 0x86, 0x51, 0x08, 0x04, 0x5a,
	0x18, 0x45, 0x20, 0xb4, 0xee, 0xa1, 0x41, 0x50, 0x14, 0x8d, 0x2d, 0x27, 0x21, 0x9a, 0x38, 0x02,
	0x55, 0xe7, 0xd0, 0xdb, 0x9a, 0x9c, 0x5a, 0x44, 0x29, 0x91, 0xdd, 0xa5, 0x14, 0xd0, 0x97, 0xa2,
	0xaf, 0xd0, 0x67, 0xe8, 0x73, 0xf4, 0x01, 0x7a, 0xed, 0xad, 0x4f, 0x53, 0xcc, 0x2e, 0x45, 0x2e,
	0x25, 0x5e, 0x72, 0x9b, 0xf9, 0x66, 0x66, 0x67, 0xf8, 0xcd, 0xc7, 0x25, 0x61, 0x90, 0xe5, 0x29,
	0xaa, 0x51, 0x2a, 0x93, 0x2c, 0x61, 0xdd, 0xd5, 0xd7, 0x22, 0x8d, 0xdc, 0x7f, 0x3b, 0xd0, 0x1d,
	0xcb, 0x68, 0x85, 0x8c, 0x41, 0xe7, 0xee, 0xce, 0x1b, 0x73, 0x67, 0xe8, 0x5c, 0xf4, 0x7d, 0x6d,
	0xb3, 0x43, 0x68, 0xbf, 0xf7, 0xc6, 0xbc, 0xa5, 0x21, 0x32, 0x09, 0x99, 0x78, 0x63, 0xde, 0x36,
	0xc8, 0xc4, 0x1b, 0x33, 0x17, 0xf6, 0xa6, 0x28, 0x23, 0x11, 0xdf, 0x2e, 0xe7, 0xf7, 0x28, 0x79,
	0x47, 0x87, 0x6a, 0x18, 0x3b, 0x85, 0xde, 0x6b, 0x14, 0x71, 0x36, 0xe3, 0x5d, 0x1d, 0x2d, 0x3c,
	0xea, 0xf9, 0x53, 0x9e, 0x22, 0xef, 0x99, 0x9e, 0x64, 0x13, 0x36, 0x8d, 0x1e, 0x91, 0xef, 0x0c,
	0x9d, 0x8b, 0xb6, 0xaf, 0x6d, 0xaa, 0x9f, 0x66, 0x22, 0x5b, 0x2a, 0xbe, 0x6b, 0xea, 0x8d, 0xc7,
	0x4e, 0xa0, 0x7b, 0xa7, 0xc4, 0x03, 0xf2, 0xbe, 0x86, 0x8d, 0x43, 0xd9, 0xb7, 0x49, 0x88, 0x5e,
	0xc8, 0xc1, 0x64, 0x1b, 0x8f, 0x4e, 0x9e, 0x88, 0x6c, 0xc6, 0x07, 0xa6, 0x1b, 0xd9, 0xec, 0x1c,
	0xfa, 0x37, 0x8b, 0x20, 0x4e, 0xd4, 0x52, 0x22, 0xdf, 0xd3, 0x81, 0x0a, 0xd0, 0xb3, 0xc4, 0x49,
	0xc6, 0xf7, 0x4d, 0x05, 0xd9, 0xc4, 0xc0, 0x95, 0xc8, 0xf9, 0x81, 0x61, 0xe0, 0x4a, 0xe4, 0xec,
	0x0c, 0x76, 0x5f, 0x46, 0x72, 0xfe, 0x41, 0x48, 0xe4, 0x4f, 0x34, 0x5c, 0xfa, 0xe6, 0xfc, 0x70,
	0x29, 0xc5, 0x22, 0x40, 0x7e, 0xa8, 0x1f, 0xa9, 0x02, 0xa8, 0xf2, 0xcd, 0xcd, 0x98, 0x1e, 0x06,
	0xf9, 0x91, 0xa9, 0x5c, 0xfb, 0x14, 0xf3, 0xd4, 0x34, 0x57, 0x19, 0xce, 0x39, 0x1b, 0x3a, 0x17,
	0xbb, 0x7e, 0xe9, 0x33, 0x0e, 0x3b, 0x9e, 0xba, 0x8e, 0x51, 0x2c, 0xf8, 0xb1, 0x0e, 0xad, 0x5d,
	0xf6, 0x05, 0x1c, 0x4c, 0x31, 0x58, 0xca, 0x28, 0xcb, 0x0b, 0xc6, 0x4e, 0xf4, 0xb9, 0x1b, 0x28,
	0x7b, 0x0a, 0x47, 0x37, 0x8b, 0x40, 0xe6, 0x69, 0x16, 0x25, 0x8b, 0x6b, 0x91, 0x8a, 0xfb, 0x18,
	0xf9, 0x27, 0x3a, 0x75, 0x3b, 0xc0, 0x46, 0xc0, 0x2a, 0x70, 0x42, 0xfa, 0x09, 0x92, 0x98, 0x9f,
	0xea, 0xf4, 0x86, 0x88, 0xfb, 0x57, 0x1b, 0x7a, 0xef, 0x93, 0x78, 0x39, 0x47, 0x76, 0x00, 0x2d,
	0x2f, 0x2c, 0x44, 0xd5, 0xf2, 0x42, 0xfd, 0xc8, 0x49, 0x20, 0x28, 0xbd, 0xd0, 0x55, 0xe9, 0x93,
	0x94, 0xd6, 0xb6, 0x96, 0x85, 0x51, 0x59, 0x0d, 0xd3, 0x72, 0xcb, 0x12, 0x29, 0x1e, 0xf0, 0x3a,
	0x16, 0x4a, 0x95, 0x72, 0xb3, 0x30, 0x4b, 0x00, 0xdd, 0x9a, 0x00, 0x4e, 0xa1, 0xf7, 0xee, 0xc3,
	0x02, 0xa5, 0xe2, 0xbd, 0x61, 0x9b, 0x70, 0xe3, 0x35, 0x4a, 0x8e, 0x41, 0xe7, 0x6d, 0x12, 0x62,
	0x21, 0x38, 0x6d, 0x97, 0x72, 0xed, 0x5b, 0x72, 0xad, 0xa4, 0x0d, 0x35, 0x69, 0x3f, 0x85, 0xa3,
	0x77, 0x29, 0x4a, 0x3d, 0xb8, 0x88, 0x8b, 0x5d, 0x18, 0xe5, 0x6d, 0x07, 0x48, 0x26, 0xd7, 0x53,
	0xaf, 0xc8, 0x2a, 0x64, 0x58, 0x02, 0x95, 0xcc, 0xf7, 0x6d, 0x99, 0x93, 0xb4, 0xd2, 0x19, 0xce,
	0x51, 0x8a, 0x58, 0xcb, 0x71, 0xd7, 0xaf, 0x00, 0x8b, 0xa7, 0x57, 0x32, 0x59, 0xa6, 0x85, 0x30,
	0x6b, 0x98, 0xfb, 0x3b, 0x1c, 0xbd, 0x58, 0x89, 0x28, 0xa6, 0x1d, 0xd3, 0xaa, 0x83, 0x28, 0xcb,
	0x6b, 0x0b, 0x72, 0x36, 0x16, 0x54, 0x11, 0xdb, 0xaa, 0x11, 0xeb, 0xc2, 0x9e, 0xb2, 0x97, 0x52,
	0x2c, 0xce, 0xc6, 0x4a, 0x92, 0x3b, 0x15, 0xc9, 0xee, 0x7f, 0x0e, 0x9c, 0x6f, 0x4d, 0xe0, 0xa3,
	0x42, 0xb9, 0x32, 0x0d, 0xcf, 0xa1, 0x7f, 0x2b, 0xe6, 0xa8, 0x52, 0x11, 0x60, 0x31, 0x4d, 0x05,
	0x58, 0xd7, 0x42, 0xab, 0x76, 0x2d, 0x7c, 0x0b, 0x7b, 0x34, 0x98, 0x8f, 0xbf, 0x2d, 0x51, 0x65,
	0x66, 0x9c, 0xc1, 0xe5, 0xf1, 0x48, 0x5f, 0x79, 0x23, 0x3b, 0xe4, 0xd7, 0x12, 0xd9, 0x8f, 0x70,
	0x6c, 0x75, 0x2f, 0xeb, 0x3b, 0xc3, 0xf6, 0xc5, 0xe0, 0xf2, 0xd3, 0xa2, 0x7e, 0x3b, 0xc3, 0x6f,
	0xaa, 0x72, 0x5f, 0xd7, 0xa7, 0xa0, 0x67, 0x29, 0x6c, 0xa4, 0x17, 0x82, 0x04, 0x58, 0x01, 0x44,
	0xbb, 0x39, 0x04, 0x89, 0x5c, 0x0a, 0x96, 0xbe, 0xfb, 0x08, 0x6c, 0xbb, 0x01, 0xfb, 0x01, 0x9e,
	0x54, 0x94, 0x69, 0x48, 0x33, 0x34, 0xb8, 0x3c, 0x2d, 0x06, 0xdd, 0x88, 0xfa, 0x9b, 0xe9, 0xb4,
	0x36, 0xeb, 0x5c, 0x55, 0xf4, 0xad, 0x61, 0xee, 0x1f, 0xce, 0x56, 0x1b, 0x5a, 0x25, 0x2d, 0x61,
	0xfd, 0xa9, 0x20, 0x7b, 0xeb, 0xbd, 0x6c, 0x35, 0xbc, 0x97, 0x6b, 0x09, 0xb4, 0xad, 0xf7, 0x6c,
	0x53, 0xa7, 0x9d, 0x06, 0x9d, 0xfe, 0xed, 0x00, 0x7b, 0x93, 0x3c, 0x44, 0x81, 0x88, 0xcd, 0xad,
	0xa2, 0xe1, 0xc6, 0x31, 0x08, 0xa3, 0xd7, 0xb6, 0x55, 0x60, 0xf4, 0xda, 0x9e, 0x43, 0x7f, 0xad,
	0x60, 0xd2, 0x82, 0x26, 0xbe, 0x04, 0x9a, 0x74, 0xc9, 0x3e, 0x03, 0x30, 0x8d, 0x7c, 0xfc, 0x45,
	0xf1, 0xae, 0x2e, 0xb1, 0x10, 0x4b, 0x78, 0xbd, 0x9a, 0xf0, 0xaa, 0xcb, 0x60, 0xc7, 0xbe, 0x0c,
	0xdc, 0x3f, 0x1d, 0x33, 0x56, 0xe3, 0x47, 0xf6, 0x19, 0xf4, 0x5f, 0x84, 0xa1, 0x44, 0xa5, 0xd0,
	0xac, 0x60, 0x70, 0x79, 0x66, 0x49, 0x75, 0x54, 0x06, 0x6f, 0x16, 0x99, 0xcc, 0xfd, 0x2a, 0xf9,
	0xec, 0x3b, 0x38, 0xa8, 0x07, 0xe9, 0xe3, 0xf4, 0x2b, 0xe6, 0xc5, 0xf1, 0x64, 0xd2, 0xdd, 0xb1,
	0x12, 0xf1, 0x72, 0xcd, 0x88, 0x71, 0x9e, 0xb7, 0x9e, 0x39, 0xee, 0x2d, 0x1c, 0xda, 0x2c, 0x4f,
	0x53, 0x0c, 0xd8, 0x73, 0xd8, 0x0f, 0xe9, 0x6f, 0x60, 0x8a, 0x31, 0x06, 0x59, 0x22, 0x0b, 0x45,
	0x9d, 0x14, 0xf3, 0x8c, 0xed, 0x98, 0x5f, 0x4f, 0x75, 0xff, 0x71, 0x60, 0xbf, 0x96, 0xc0, 0xbe,
	0x82, 0xe3, 0x85, 0xfe, 0x01, 0xd0, 0xb0, 0x9a, 0xa0, 0xd4, 0xbb, 0xa1, 0x33, 0xbb, 0x7e, 0x53,
	0x88, 0xbd, 0x82, 0xc1, 0x5c, 0x64, 0xc1, 0xec, 0x65, 0x84, 0x71, 0xb8, 0x66, 0xe3, 0xf3, 0xa6,
	0xee, 0xa3, 0xb7, 0x55, 0x9e, 0x21, 0xc6, 0xae, 0x3c, 0xfb, 0x1e, 0x0e, 0x37, 0x13, 0x3e, 0x8a,
	0x9c, 0x2f, 0x81, 0xd5, 0xc8, 0x29, 0x2f, 0xe2, 0x74, 0x26, 0xd4, 0x5a, 0x72, 0xc6, 0xb9, 0xda,
	0xf9, 0xd9, 0xfc, 0x4c, 0xdd, 0xf7, 0xf4, 0xaf, 0xd5, 0x37, 0xff, 0x07, 0x00, 0x00, 0xff, 0xff,
	0x9b, 0x5f, 0xec, 0x44, 0x69, 0x09, 0x00, 0x00,
}
