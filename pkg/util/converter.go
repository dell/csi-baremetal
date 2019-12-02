//Package util provide util function for CSI working
package util

//FormatCapacity format capacity of disk:
func FormatCapacity(size int64, unit string) int64 {
	switch unit {
	case "K":
		size *= 1024
	case "M":
		size *= 1024 * 1024
	case "G":
		size *= 1024 * 1024 * 1024
	case "T":
		size *= 1024 * 1024 * 1024 * 1024
	default:
		return size
	}

	return size
}
