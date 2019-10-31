package util

import (
	. "math"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Converter Spec")
}

var _ = Describe("Converter", func() {
	var requiredBytes int64 = int64(Pow(10, 9))
	Context("Required bytes", func() {
		It("Size must be 1000", func() {
			size := FormatCapacity(requiredBytes, "M")
			Expect(size == 1000)
		})

		It("Size should be 1", func() {
			size := FormatCapacity(requiredBytes, "G")
			Expect(size == 1)
		})
	})
})
