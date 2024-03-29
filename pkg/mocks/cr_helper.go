// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import (
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	context "context"

	drivecrd "github.com/dell/csi-baremetal/api/v1/drivecrd"

	k8s "github.com/dell/csi-baremetal/pkg/base/k8s"

	lvgcrd "github.com/dell/csi-baremetal/api/v1/lvgcrd"

	mock "github.com/stretchr/testify/mock"

	v1api "github.com/dell/csi-baremetal/api/generated/v1"

	volumecrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
)

// CRHelper is an autogenerated mock type for the CRHelper type
type CRHelper struct {
	mock.Mock
}

// DeleteACsByNodeID provides a mock function with given fields: nodeID
func (_m *CRHelper) DeleteACsByNodeID(nodeID string) error {
	ret := _m.Called(nodeID)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(nodeID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteObjectByName provides a mock function with given fields: ctx, name, namespace, obj
func (_m *CRHelper) DeleteObjectByName(ctx context.Context, name string, namespace string, obj client.Object) error {
	ret := _m.Called(ctx, name, namespace, obj)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, client.Object) error); ok {
		r0 = rf(ctx, name, namespace, obj)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetACByLocation provides a mock function with given fields: location
func (_m *CRHelper) GetACByLocation(location string) (*accrd.AvailableCapacity, error) {
	ret := _m.Called(location)

	var r0 *accrd.AvailableCapacity
	if rf, ok := ret.Get(0).(func(string) *accrd.AvailableCapacity); ok {
		r0 = rf(location)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*accrd.AvailableCapacity)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(location)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetACCRs provides a mock function with given fields: node
func (_m *CRHelper) GetACCRs(node ...string) ([]accrd.AvailableCapacity, error) {
	_va := make([]interface{}, len(node))
	for _i := range node {
		_va[_i] = node[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []accrd.AvailableCapacity
	if rf, ok := ret.Get(0).(func(...string) []accrd.AvailableCapacity); ok {
		r0 = rf(node...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]accrd.AvailableCapacity)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(...string) error); ok {
		r1 = rf(node...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDriveCRAndLVGCRByVolume provides a mock function with given fields: volume
func (_m *CRHelper) GetDriveCRAndLVGCRByVolume(volume *volumecrd.Volume) (*drivecrd.Drive, *lvgcrd.LogicalVolumeGroup, error) {
	ret := _m.Called(volume)

	var r0 *drivecrd.Drive
	if rf, ok := ret.Get(0).(func(*volumecrd.Volume) *drivecrd.Drive); ok {
		r0 = rf(volume)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*drivecrd.Drive)
		}
	}

	var r1 *lvgcrd.LogicalVolumeGroup
	if rf, ok := ret.Get(1).(func(*volumecrd.Volume) *lvgcrd.LogicalVolumeGroup); ok {
		r1 = rf(volume)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*lvgcrd.LogicalVolumeGroup)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*volumecrd.Volume) error); ok {
		r2 = rf(volume)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetDriveCRByVolume provides a mock function with given fields: volume
func (_m *CRHelper) GetDriveCRByVolume(volume *volumecrd.Volume) (*drivecrd.Drive, error) {
	ret := _m.Called(volume)

	var r0 *drivecrd.Drive
	if rf, ok := ret.Get(0).(func(*volumecrd.Volume) *drivecrd.Drive); ok {
		r0 = rf(volume)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*drivecrd.Drive)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*volumecrd.Volume) error); ok {
		r1 = rf(volume)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDriveCRs provides a mock function with given fields: node
func (_m *CRHelper) GetDriveCRs(node ...string) ([]drivecrd.Drive, error) {
	_va := make([]interface{}, len(node))
	for _i := range node {
		_va[_i] = node[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []drivecrd.Drive
	if rf, ok := ret.Get(0).(func(...string) []drivecrd.Drive); ok {
		r0 = rf(node...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]drivecrd.Drive)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(...string) error); ok {
		r1 = rf(node...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLVGByDrive provides a mock function with given fields: ctx, driveUUID
func (_m *CRHelper) GetLVGByDrive(ctx context.Context, driveUUID string) (*lvgcrd.LogicalVolumeGroup, error) {
	ret := _m.Called(ctx, driveUUID)

	var r0 *lvgcrd.LogicalVolumeGroup
	if rf, ok := ret.Get(0).(func(context.Context, string) *lvgcrd.LogicalVolumeGroup); ok {
		r0 = rf(ctx, driveUUID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*lvgcrd.LogicalVolumeGroup)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, driveUUID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLVGCRs provides a mock function with given fields: node
func (_m *CRHelper) GetLVGCRs(node ...string) ([]lvgcrd.LogicalVolumeGroup, error) {
	_va := make([]interface{}, len(node))
	for _i := range node {
		_va[_i] = node[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []lvgcrd.LogicalVolumeGroup
	if rf, ok := ret.Get(0).(func(...string) []lvgcrd.LogicalVolumeGroup); ok {
		r0 = rf(node...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]lvgcrd.LogicalVolumeGroup)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(...string) error); ok {
		r1 = rf(node...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVGNameByLVGCRName provides a mock function with given fields: lvgCRName
func (_m *CRHelper) GetVGNameByLVGCRName(lvgCRName string) (string, error) {
	ret := _m.Called(lvgCRName)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(lvgCRName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(lvgCRName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVolumeByID provides a mock function with given fields: volID
func (_m *CRHelper) GetVolumeByID(volID string) (*volumecrd.Volume, error) {
	ret := _m.Called(volID)

	var r0 *volumecrd.Volume
	if rf, ok := ret.Get(0).(func(string) *volumecrd.Volume); ok {
		r0 = rf(volID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*volumecrd.Volume)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(volID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVolumeCRs provides a mock function with given fields: node
func (_m *CRHelper) GetVolumeCRs(node ...string) ([]volumecrd.Volume, error) {
	_va := make([]interface{}, len(node))
	for _i := range node {
		_va[_i] = node[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []volumecrd.Volume
	if rf, ok := ret.Get(0).(func(...string) []volumecrd.Volume); ok {
		r0 = rf(node...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]volumecrd.Volume)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(...string) error); ok {
		r1 = rf(node...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetVolumesByLocation provides a mock function with given fields: ctx, location
func (_m *CRHelper) GetVolumesByLocation(ctx context.Context, location string) ([]*volumecrd.Volume, error) {
	ret := _m.Called(ctx, location)

	var r0 []*volumecrd.Volume
	if rf, ok := ret.Get(0).(func(context.Context, string) []*volumecrd.Volume); ok {
		r0 = rf(ctx, location)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*volumecrd.Volume)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, location)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetReader provides a mock function with given fields: reader
func (_m *CRHelper) SetReader(reader k8s.CRReader) k8s.CRHelper {
	ret := _m.Called(reader)

	var r0 k8s.CRHelper
	if rf, ok := ret.Get(0).(func(k8s.CRReader) k8s.CRHelper); ok {
		r0 = rf(reader)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(k8s.CRHelper)
		}
	}

	return r0
}

// UpdateDrivesStatusOnNode provides a mock function with given fields: nodeID, status
func (_m *CRHelper) UpdateDrivesStatusOnNode(nodeID string, status string) error {
	ret := _m.Called(nodeID, status)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(nodeID, status)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateVolumeCRSpec provides a mock function with given fields: volName, namespace, newSpec
func (_m *CRHelper) UpdateVolumeCRSpec(volName string, namespace string, newSpec v1api.Volume) error {
	ret := _m.Called(volName, namespace, newSpec)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, v1api.Volume) error); ok {
		r0 = rf(volName, namespace, newSpec)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateVolumeOpStatus provides a mock function with given fields: ctx, volume, opStatus
func (_m *CRHelper) UpdateVolumeOpStatus(ctx context.Context, volume *volumecrd.Volume, opStatus string) error {
	ret := _m.Called(ctx, volume, opStatus)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *volumecrd.Volume, string) error); ok {
		r0 = rf(ctx, volume, opStatus)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateVolumesOpStatusOnNode provides a mock function with given fields: nodeID, opStatus
func (_m *CRHelper) UpdateVolumesOpStatusOnNode(nodeID string, opStatus string) error {
	ret := _m.Called(nodeID, opStatus)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string) error); ok {
		r0 = rf(nodeID, opStatus)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
