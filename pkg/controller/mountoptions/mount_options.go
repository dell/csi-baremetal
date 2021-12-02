package mountoptions

/*
	In this package placed supported mount options pass from SCs
	Example:
		apiVersion: storage.k8s.io/v1
		kind: StorageClass
		metadata:
		  name: sc1
		mountOptions:
		  - noatime
*/

// MountOptionType type to clarify option purpose
type MountOptionType string

const (
	// PublishCmdOpt are options for mount func on NodePublishRequest
	// Example: mount -o <PublishCmdOpt> /src /dst
	PublishCmdOpt = MountOptionType("cmdOpt")
)

// MountOption describes mount option
type MountOption struct {
	arg       string
	mountType MountOptionType
}

const (
	noatimeOpt = "noatime"
)

var (
	// supportedMountOption contains all supported options
	// map[optName]{optArg, optType}
	// optName - passed from SC
	// optArg - cmd string
	supportedMountOption = map[string]MountOption{
		noatimeOpt: {
			arg:       noatimeOpt,
			mountType: PublishCmdOpt,
		},
	}
)

// IsOptionSupported returns true if option in supportedMountOption
func IsOptionSupported(option string) bool {
	_, ok := supportedMountOption[option]
	return ok
}

// IsOptionsSupported returns true if all options in supportedMountOption
func IsOptionsSupported(options []string) bool {
	for _, option := range options {
		if !IsOptionSupported(option) {
			return false
		}
	}

	return true
}

// FilterWithType returns all option from list with passed type
func FilterWithType(mountType MountOptionType, options []string) (filteredOptions []string) {
	for _, option := range options {
		if val, ok := supportedMountOption[option]; ok {
			if mountType == val.mountType {
				filteredOptions = append(filteredOptions, val.arg)
			}
		}
	}

	return
}
