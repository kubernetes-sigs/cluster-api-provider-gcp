package processors_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProcessors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Processors Suite")
}
