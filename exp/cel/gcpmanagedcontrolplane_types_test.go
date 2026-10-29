/*
 Copyright 2026 The Kubernetes Authors.

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

package cel

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

var (
	endOfSupport          = v1beta1.UntilEndOfSupport
	endTime               = metav1.NewTime(time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC))
	noMinorOrNodeUpgrades = v1beta1.NoMinorOrNodeUpgrades
	noMinorUpgrades       = v1beta1.NoMinorUpgrades
	noUpgrades            = v1beta1.NoUpgrades
)

var _ = Describe("SecretManagerConfig CEL Validation", func() {
	DescribeTable("SecretManagerConfig Validates Correctly", func(smc v1beta1.SecretManagerConfig, errorField string) {
		Expect(k8sClient).NotTo(BeNil())
		mcp := v1beta1.GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault},
			Spec: v1beta1.GCPManagedControlPlaneSpec{
				GCPManagedControlPlaneClassSpec: v1beta1.GCPManagedControlPlaneClassSpec{
					SecretManagerConfig: &smc,
				},
			},
		}
		err := k8sClient.Create(context.Background(), &mcp)
		if errorField != "" {
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "actual error was '%v'", err)
			var causeErr *apierrors.StatusError
			Expect(errors.As(err, &causeErr)).To(BeTrue())
			Expect(causeErr.ErrStatus.Details.Causes).To(ContainElement(HaveField("Field", errorField)))
			return
		}
		Expect(err).NotTo(HaveOccurred())
	},
		Entry("enabled false with rotationConfig is rejected", v1beta1.SecretManagerConfig{
			Enabled: false,
			RotationConfig: &v1beta1.SecretManagerRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 75},
			},
		}, "spec.secretManagerConfig"),
		Entry("enabled false within rotationConfig is rejected if rotationInterval is also provided", v1beta1.SecretManagerConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretManagerRotationConfig{
				Enabled:          false,
				RotationInterval: &metav1.Duration{Duration: time.Second * 75},
			},
		}, "spec.secretManagerConfig.rotationConfig"),
		Entry("rotationInterval rejected if lower than 60 seconds", v1beta1.SecretManagerConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretManagerRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 45},
			},
		}, "spec.secretManagerConfig.rotationConfig.rotationInterval"),
		Entry("valid no rotationConfig", v1beta1.SecretManagerConfig{
			Enabled: true,
		}, ""),
		Entry("valid with no rotationInterval", v1beta1.SecretManagerConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretManagerRotationConfig{
				Enabled: true,
			},
		}, ""),
		Entry("valid full config", v1beta1.SecretManagerConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretManagerRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 300},
			},
		}, ""),
	)
})

var _ = Describe("SecretSyncConfig CEL Validation", func() {
	DescribeTable("SecretSyncConfig Validates Correctly", func(ssc v1beta1.SecretSyncConfig, errorField string) {
		Expect(k8sClient).NotTo(BeNil())
		mcp := v1beta1.GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault},
			Spec: v1beta1.GCPManagedControlPlaneSpec{
				GCPManagedControlPlaneClassSpec: v1beta1.GCPManagedControlPlaneClassSpec{
					SecretSyncConfig: &ssc,
				},
			},
		}
		err := k8sClient.Create(context.Background(), &mcp)
		if errorField != "" {
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "actual error was '%v'", err)
			var causeErr *apierrors.StatusError
			Expect(errors.As(err, &causeErr)).To(BeTrue())
			Expect(causeErr.ErrStatus.Details.Causes).To(ContainElement(HaveField("Field", errorField)))
			return
		}
		Expect(err).NotTo(HaveOccurred())
	},
		Entry("enabled false with rotationConfig is rejected", v1beta1.SecretSyncConfig{
			Enabled: false,
			RotationConfig: &v1beta1.SecretSyncRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 75},
			},
		}, "spec.secretSyncConfig"),
		Entry("enabled false within rotationConfig is rejected if rotationInterval is also provided", v1beta1.SecretSyncConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretSyncRotationConfig{
				Enabled:          false,
				RotationInterval: &metav1.Duration{Duration: time.Second * 75},
			},
		}, "spec.secretSyncConfig.rotationConfig"),
		Entry("rotationInterval rejected if lower than 60 seconds", v1beta1.SecretSyncConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretSyncRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 45},
			},
		}, "spec.secretSyncConfig.rotationConfig.rotationInterval"),
		Entry("valid enabled true and no rotationConfig", v1beta1.SecretSyncConfig{
			Enabled: true,
		}, ""),
		Entry("valid enabled true and rotationConfig with no interval", v1beta1.SecretSyncConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretSyncRotationConfig{
				Enabled: true,
			},
		}, ""),
		Entry("valid full config", v1beta1.SecretSyncConfig{
			Enabled: true,
			RotationConfig: &v1beta1.SecretSyncRotationConfig{
				Enabled:          true,
				RotationInterval: &metav1.Duration{Duration: time.Second * 300},
			},
		}, ""),
	)
})

var _ = Describe("TimeWindow CEL Validation", func() {
	DescribeTable("TimeWindow Validates Correctly", func(tw v1beta1.TimeWindow, errorField string) {
		Expect(k8sClient).NotTo(BeNil())
		mcp := v1beta1.GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault},
			Spec: v1beta1.GCPManagedControlPlaneSpec{
				GCPManagedControlPlaneClassSpec: v1beta1.GCPManagedControlPlaneClassSpec{
					MaintenancePolicy: &v1beta1.MaintenancePolicy{
						MaintenanceExclusions: map[string]*v1beta1.TimeWindow{
							"foo": &tw,
						},
					},
				},
			},
		}
		err := k8sClient.Create(context.Background(), &mcp)
		if errorField != "" {
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "actual error was '%v'", err)
			var causeErr *apierrors.StatusError
			Expect(errors.As(err, &causeErr)).To(BeTrue())
			Expect(causeErr.ErrStatus.Details.Causes).To(ContainElement(HaveField("Field", errorField)))
			return
		}
		Expect(err).NotTo(HaveOccurred())
	},
		Entry("fixed endTime is valid", v1beta1.TimeWindow{
			StartTime: metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTime:   &endTime,
		}, ""),
		Entry("tracking with no endTime is valid (no-minor-upgrades)", v1beta1.TimeWindow{
			StartTime:                  metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTimeBehavior:            &endOfSupport,
			MaintenanceExclusionOption: &noMinorUpgrades,
		}, ""),
		Entry("tracking with no endTime is valid (no-minor-or-node-upgrades)", v1beta1.TimeWindow{
			StartTime:                  metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTimeBehavior:            &endOfSupport,
			MaintenanceExclusionOption: &noMinorOrNodeUpgrades,
		}, ""),
		Entry("tracking with no-upgrades is rejected", v1beta1.TimeWindow{
			StartTime:                  metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTimeBehavior:            &endOfSupport,
			MaintenanceExclusionOption: &noUpgrades,
		}, "spec.maintenancePolicy.maintenanceExclusions[foo]"),
		Entry("tracking with no scope", v1beta1.TimeWindow{
			StartTime:       metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTimeBehavior: &endOfSupport,
		}, "spec.maintenancePolicy.maintenanceExclusions[foo]"),
		Entry("no endTime and no behaviour is rejected", v1beta1.TimeWindow{
			StartTime: metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
		}, "spec.maintenancePolicy.maintenanceExclusions[foo]"),
		Entry("endTime after startTime is rejected", v1beta1.TimeWindow{
			StartTime: metav1.NewTime(time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTime:   &endTime,
		}, "spec.maintenancePolicy.maintenanceExclusions[foo]"),
	)
})

var _ = Describe("RecurringMaintenanceWindow Types CEL Validation", func() {
	DescribeTable("RecurringMaintenanceWindow Validates Correctly", func(tw *v1beta1.TimeWindow, errorField string) {
		Expect(k8sClient).NotTo(BeNil())
		mcp := v1beta1.GCPManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: metav1.NamespaceDefault},
			Spec: v1beta1.GCPManagedControlPlaneSpec{
				GCPManagedControlPlaneClassSpec: v1beta1.GCPManagedControlPlaneClassSpec{
					MaintenancePolicy: &v1beta1.MaintenancePolicy{
						RecurringMaintenanceWindow: &v1beta1.RecurringMaintenanceWindow{
							Window: tw,
						},
					},
				},
			},
		}
		err := k8sClient.Create(context.Background(), &mcp)
		if errorField != "" {
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "actual error was '%v'", err)
			var causeErr *apierrors.StatusError
			Expect(errors.As(err, &causeErr)).To(BeTrue())
			Expect(causeErr.ErrStatus.Details.Causes).To(ContainElement(HaveField("Field", errorField)))
			return
		}
		Expect(err).NotTo(HaveOccurred())
	},
		Entry("recurring window with no behaviour is valid", &v1beta1.TimeWindow{
			StartTime: metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTime:   &endTime,
		}, ""),
		Entry("nil window is valid", nil, ""),
		Entry("tracking on a recurring window is rejected", &v1beta1.TimeWindow{
			StartTime:                  metav1.NewTime(time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)),
			EndTimeBehavior:            &endOfSupport,
			MaintenanceExclusionOption: &noMinorUpgrades,
		}, "spec.maintenancePolicy.recurringMaintenanceWindow"),
	)
})
