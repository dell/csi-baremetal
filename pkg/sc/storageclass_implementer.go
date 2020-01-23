package sc

// StorageClassImplementer is an interface with different methods for a volume creation depending on Storage Class
// It should be used across node level operations
type StorageClassImplementer interface {
	CreateFileSystem(fsType FileSystem, device string) (bool, error)
	DeleteFileSystem(device string) (bool, error)

	CreateTargetPath(path string) (bool, error)
	DeleteTargetPath(path string) (bool, error)

	IsMounted(device, targetPath string) (bool, error)
	Mount(device, dir string) (bool, error)
}

// FileSystem defines Linux filesystem
type FileSystem string

// Filesystem which can be used for CSI
const (
	XFS FileSystem = "xfs"
)
