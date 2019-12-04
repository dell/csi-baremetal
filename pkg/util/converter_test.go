package util

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Converter Spec")
}

var _ = Describe("Converter", func() {
	var requiredBytes = 1.5
	Context("Required bytes", func() {
		It("Size must be 1K", func() {
			size := FormatCapacity(requiredBytes, "K")
			Expect(size).Should(BeNumerically("==", int64(requiredBytes*1024)))
		})

		It("Size must be 1M", func() {
			size := FormatCapacity(requiredBytes, "M")
			Expect(size).Should(BeNumerically("==", int64(requiredBytes*1024*1024)))
		})

		It("Size should be 1G", func() {
			size := FormatCapacity(requiredBytes, "G")
			Expect(size).Should(BeNumerically("==", int64(requiredBytes*1024*1024*1024)))
		})

		It("Size should be 1T", func() {
			size := FormatCapacity(requiredBytes, "T")
			Expect(size).Should(BeNumerically("==", int64(requiredBytes*1024*1024*1024*1024)))
		})
	})
})
