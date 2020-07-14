// Package drivemgr contains a code for managers of storage hardware such as drives
package drivemgr

import api "github.com/dell/csi-baremetal/api/generated/v1"

// DriveManager is the interface for managers that provide information about drives on a node
type DriveManager interface {
	// get list of drives
	GetDrivesList() ([]*api.Drive, error)
}
