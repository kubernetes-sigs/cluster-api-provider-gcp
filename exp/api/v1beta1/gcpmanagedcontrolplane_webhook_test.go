/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("GCPManagedControlPlaneWebhook", func() {

	var (
		controlPlane              *GCPManagedControlPlane
		TestControlPlaneName      = "test-managed-control-plane"
		TestControlPlaneNamespace = "test-namespace"
		TestProject               = "test-project"
		TestLocation              = "us-central1"
		TestCidrRangeName         = "test-range"
		TestCidr                  = "10.96.0.0/14"
	)

	BeforeEach(func() {
		controlPlane = &GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        TestControlPlaneName,
				Namespace:   TestControlPlaneNamespace,
			},
			Spec: GCPManagedControlPlaneSpec{
				Project:  TestProject,
				Location: TestLocation,
			},
		}
	})
	When("Defaulting GCPManagedControlPlane", func() {
		It("should add SystemComponents by default to LoggingConfig.EnableComponents if length > 0 and not specified", func() {
			controlPlane.Spec.LoggingConfig = &LoggingConfig{
				EnableComponents: []LoggingComponent{
					Scheduler,
					ControllerManager,
				},
			}
			controlPlane.Default()
			Expect(controlPlane.Spec.LoggingConfig.EnableComponents).To(HaveLen(3))
			Expect(controlPlane.Spec.LoggingConfig.EnableComponents[2]).To(Equal(SystemComponents))
		})
		It("should generate cluster name if left empty", func() {
			Expect(controlPlane.Spec.ClusterName).To(HaveLen(0))
			controlPlane.Default()
			name, err := generateGKEName(controlPlane.Name, controlPlane.Namespace, maxClusterNameLength)
			Expect(err).To(BeNil())
			Expect(controlPlane.Spec.ClusterName).To(Equal(name))
		})
	})

	When("Validating GCPManagedControlPlane", func() {
		It("should return an error for cluster name greater than max length", func() {
			controlPlane.Spec.ClusterName = strings.Repeat("a", maxClusterNameLength+1)
			errList := ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.ClusterName"))
		})
		It("should properly validate autopilot enabled", func() {
			controlPlane.Spec.EnableAutopilot = true
			errList := ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.ReleaseChannel"))

			rc := ReleaseChannel("rapid")
			controlPlane.Spec.ReleaseChannel = &rc
			errList = ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(0))
		})
		It("should properly validate IPAllocationPolicy configurations", func() {
			tests := []struct {
				policy               *IPAllocationPolicy
				expectFailure        bool
				expectedFailureField string
			}{
				{
					policy:               &IPAllocationPolicy{ClusterSecondaryRangeName: &TestCidrRangeName},
					expectFailure:        true,
					expectedFailureField: "spec.IPAllocationPolicy.ClusterSecondaryRangeName",
				},
				{
					policy:               &IPAllocationPolicy{ServicesSecondaryRangeName: &TestCidrRangeName},
					expectFailure:        true,
					expectedFailureField: "spec.IPAllocationPolicy.ServicesSecondaryRangeName",
				},
				{
					policy:               &IPAllocationPolicy{ServicesIpv4CidrBlock: &TestCidr},
					expectFailure:        true,
					expectedFailureField: "spec.IPAllocationPolicy.ServicesIpv4CidrBlock",
				},
				{
					policy:               &IPAllocationPolicy{ClusterIpv4CidrBlock: &TestCidr},
					expectFailure:        true,
					expectedFailureField: "spec.IPAllocationPolicy.ClusterIpv4CidrBlock",
				},
				{
					policy: &IPAllocationPolicy{
						UseIPAliases:              pointer.Bool(true),
						ClusterSecondaryRangeName: &TestCidrRangeName,
						ClusterIpv4CidrBlock:      &TestCidr,
					},
					expectFailure: false,
				},
			}

			for _, t := range tests {
				controlPlane.Spec.IPAllocationPolicy = t.policy
				errList := ValidateGcpManagedControlPlane(controlPlane)
				if t.expectFailure {
					Expect(errList).To(HaveLen(1))
					Expect(errList[0].Field).To(Equal(t.expectedFailureField))
				} else {
					Expect(errList).To(HaveLen(0))
				}
			}

			controlPlane.Spec.IPAllocationPolicy = &IPAllocationPolicy{
				UseIPAliases:         pointer.Bool(true),
				ClusterIpv4CidrBlock: &TestCidr,
			}
			controlPlane.Spec.ClusterIpv4Cidr = &TestCidr
			errList := ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.IPAllocationPolicy.ClusterIpv4CidrBlock"))
		})
		It("should return an error for DefaultMaxPodConstraint being set without useIPAliases=true", func() {
			controlPlane.Spec.DefaultMaxPodsConstraint = &MaxPodsConstraint{MaxPodsPerNode: 7}
			errList := ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.DefaultMaxPodsConstraint"))

			controlPlane.Spec.IPAllocationPolicy = &IPAllocationPolicy{UseIPAliases: pointer.Bool(true)}
			errList = ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(0))
		})
		It("should properly validate MaintenancePolicy configurations", func() {
			testStartTime := "2006-01-02T15:04:05Z"
			testEndTime := "2007-01-02T15:04:05Z"
			testNoMinorUpgrades := NoMinorUpgrades
			testNoUpgrades := NoUpgrades
			tests := []struct {
				maintenancePolicy    *MaintenancePolicy
				expectFailure        bool
				expectedFailureField string
			}{
				{
					maintenancePolicy: &MaintenancePolicy{
						DailyMaintenanceWindow:     &DailyMaintenanceWindow{},
						RecurringMaintenanceWindow: &RecurringMaintenanceWindow{},
					},
					expectFailure:        true,
					expectedFailureField: "spec.MaintenancePolicy.DailyMaintenanceWindow",
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						RecurringMaintenanceWindow: &RecurringMaintenanceWindow{
							Window: &TimeWindow{
								StartTime: testStartTime,
							},
						},
					},
					expectFailure:        true,
					expectedFailureField: "spec.MaintenancePolicy.RecurringMaintenanceWindow",
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						DailyMaintenanceWindow: &DailyMaintenanceWindow{StartTime: "12:05"},
						MaintenanceExclusions: map[string]*TimeWindow{
							"exclusion1": {
								StartTime: testStartTime,
							},
						},
					},
					expectFailure:        true,
					expectedFailureField: "spec.MaintenancePolicy.MaintenanceExclusions",
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						DailyMaintenanceWindow: &DailyMaintenanceWindow{StartTime: "12:05"},
					},
					expectFailure: false,
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						RecurringMaintenanceWindow: &RecurringMaintenanceWindow{
							Window: &TimeWindow{
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
						},
					},
					expectFailure: false,
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						MaintenanceExclusions: map[string]*TimeWindow{
							"exclusion1": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
							"exclusion2": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
							"exclusion3": {
								StartTime:                  testStartTime,
								EndTime:                    testEndTime,
								MaintenanceExclusionOption: &testNoMinorUpgrades,
							},
							"exclusion4": {
								StartTime:                  testStartTime,
								EndTime:                    testEndTime,
								MaintenanceExclusionOption: &testNoUpgrades,
							},
						},
					},
					expectFailure: false,
				},
				{
					maintenancePolicy: &MaintenancePolicy{
						MaintenanceExclusions: map[string]*TimeWindow{
							"exclusion1": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
							"exclusion2": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
							"exclusion3": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
							"exclusion4": {
								StartTime: testStartTime,
								EndTime:   testEndTime,
							},
						},
					},
					expectFailure:        true,
					expectedFailureField: "spec.MaintenancePolicy.MaintenanceExclusions",
				},
			}

			for _, t := range tests {
				controlPlane.Spec.MaintenancePolicy = t.maintenancePolicy
				errList := ValidateGcpManagedControlPlane(controlPlane)
				if t.expectFailure {
					Expect(errList).To(HaveLen(1))
					Expect(errList[0].Field).To(Equal(t.expectedFailureField))
				} else {
					Expect(errList).To(HaveLen(0))
				}
			}
		})
		It("should properly validate PrivateClusterConfig", func() {
			controlPlane.Spec.PrivateClusterConfig = &PrivateClusterConfig{
				EnablePrivateEndpoint: true,
			}
			errList := ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.PrivateClusterConfig.EnablePrivateEndpoint"))

			controlPlane.Spec.MasterAuthorizedNetworksConfig = &MasterAuthorizedNetworksConfig{
				GcpPublicCidrsAccessEnabled: pointer.Bool(true),
			}
			errList = ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Field).To(Equal("spec.PrivateClusterConfig.EnablePrivateEndpoint"))

			controlPlane.Spec.MasterAuthorizedNetworksConfig.GcpPublicCidrsAccessEnabled = pointer.Bool(false)
			errList = ValidateGcpManagedControlPlane(controlPlane)
			Expect(errList).To(HaveLen(0))
		})
		It("should disallow removing spec.NetworkConfig.DNSConfig if previously specified", func() {
			oldControlPlane := &GCPManagedControlPlane{
				ObjectMeta: controlPlane.ObjectMeta,
				Spec:       controlPlane.Spec,
			}
			oldControlPlane.Spec.NetworkConfig = &NetworkConfig{
				DNSConfig: &DNSConfig{},
			}
			warnings, err := controlPlane.ValidateUpdate(oldControlPlane)
			Expect(warnings).To(BeNil())
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(ContainSubstring("spec.NetworkConfig.DNSConfig"))
		})
	})
})
