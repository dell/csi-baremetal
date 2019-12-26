package util

import (
	"os/exec"
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

var _ = Describe("Executor", func() {
	Context("Non zero return error", func() {
		It("Output should be empty, error shouldn't be a nil and should contain return code", func() {
			out, err := RunCmdAndCollectOutput(exec.Command("false"))
			Expect(out).Should(Equal(""))
			Expect(err).ShouldNot(Equal(nil))
			Expect(err.Error()).Should(Equal("exit status 1"))
		})
	})
})
