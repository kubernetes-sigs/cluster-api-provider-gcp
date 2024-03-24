package processors_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-gcp/util/processors"
)

var _ = Describe("Processors", func() {

	var n2InstTypes = []string{"n2-slow-8", "n2-test-8"}
	var n2dInstTypes = []string{"n2d-medium-4", "n2d-fast-23"}
	var c2InstTypes = []string{"c2-medium-4", "c2-fast-23"}
	var t2dInstTypes = []string{"t2d-medium-4", "t2d-fast-23"}

	Describe("Mapping Instance Types", func() {
		Context("n2 instance types", func() {
			It("should match n2 processor", func() {
				for _, instType := range n2InstTypes {
					Expect(processors.GetLatestProcessor(instType)).To(Equal(processors.Processors["n2-"]))
				}
			})
		})

		Context("c2 intstance types", func() {
			It("should match c2 processor", func() {
				for _, instType := range c2InstTypes {
					Expect(processors.GetLatestProcessor(instType)).To(Equal(processors.Processors["c2-"]))
				}
			})
		})

		Context("n2d instance types", func() {
			It("should match n2d processor", func() {
				for _, instType := range n2dInstTypes {
					Expect(processors.GetLatestProcessor(instType)).To(Equal(processors.Processors["n2d-"]))
				}
			})
		})

		Context("n2d instance types", func() {
			It("should not match n2 processor", func() {
				for _, instType := range n2dInstTypes {
					Expect(processors.GetLatestProcessor(instType)).NotTo(Equal(processors.Processors["n2-"]))
				}
			})
		})

		Context("t2d instance types", func() {
			It("should match t2d processor", func() {
				for _, instType := range t2dInstTypes {
					Expect(processors.GetLatestProcessor(instType)).To(Equal(processors.Processors["t2d-"]))
				}
			})
		})
	})
})
