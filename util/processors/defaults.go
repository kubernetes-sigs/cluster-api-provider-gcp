// Package processors provides a mapping of instancetype prefix to latest available processor.
package processors

import "strings"

// Processors is a map of instance type prefixes to the latest processor available in GCP (e2 cannot have a min cpu platform set, so it is not included here).
var Processors = map[string]string{
	"n1-":  "Intel Skylake",
	"n2-":  "Intel Ice Lake",
	"n2d-": "AMD Milan",
	"c3-":  "Intel Sapphire Rapids",
	"c2-":  "Intel Cascade Lake",
	"t2d-": "AMD Milan",
	"m1-":  "Intel Skylake",
}

// GetLatestProcessor returns the latest processor available for a given instance type.
func GetLatestProcessor(instanceType string) string {
	for machineType, processor := range Processors {
		if strings.HasPrefix(instanceType, machineType) {
			return processor
		}
	}
	// If the machine type is not recognized, return an empty string (This will hand off the processor selection to GCP).
	return ""
}
