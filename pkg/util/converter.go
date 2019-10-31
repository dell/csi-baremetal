package util

import (
	"math"
)

func FormatCapacity(requiredBytes int64, unit string) int64 {
	switch unit {
	case "K":
		requiredBytes /= 1000
	case "M":
		requiredBytes /= int64(math.Pow(1000, 2))
	case "G":
		requiredBytes /= int64(math.Pow(1000, 3))
	case "T":
		requiredBytes /= int64(math.Pow(1000, 4))
	default:
		return requiredBytes
	}
	return requiredBytes
}
