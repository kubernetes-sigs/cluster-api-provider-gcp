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

var numberFifty int32 = 50

var _ = Describe("NodePoolUpgradeSettings CEL validation", func() {
	DescribeTable("CEL Validates Correctly", func(npus *v1beta1.NodePoolUpgradeSettings, errorField string) {
		Expect(k8sClient).NotTo(BeNil())
		mmp := v1beta1.GCPManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    "default",
				GenerateName: "my-node-pool-",
			},
			Spec: v1beta1.GCPManagedMachinePoolSpec{
				GCPManagedMachinePoolClassSpec: v1beta1.GCPManagedMachinePoolClassSpec{
					NodePoolName:    "my-node-pool",
					UpgradeSettings: npus,
				},
			},
		}
		err := k8sClient.Create(context.Background(), &mmp)
		if errorField != "" {
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "actual error was '%v'", err)
			var causeErr *apierrors.StatusError
			Expect(errors.As(err, &causeErr)).To(BeTrue())
			Expect(causeErr.ErrStatus.Details.Causes).To(ContainElement(HaveField("Field", errorField)))
			return
		}
		Expect(err).NotTo(HaveOccurred())
	},
		Entry("SURGE is allowed on its own", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("SURGE"),
		}, ""),
		Entry("SURGE is allowed with maxSurge", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("SURGE"),
			MaxSurge: &numberFifty,
		}, ""),
		Entry("SURGE is allowed with maxSurge and maxUnavailable", &v1beta1.NodePoolUpgradeSettings{
			Strategy:       new("SURGE"),
			MaxSurge:       &numberFifty,
			MaxUnavailable: &numberFifty,
		}, ""),
		Entry("BLUE_GREEN is allowed with blueGreenSettings & standardRolloutPolicy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
		}, ""),
		Entry("BLUE_GREEN fails without standardRolloutPolicy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				NodePoolSoakDuration: &metav1.Duration{
					Duration: 20 * time.Minute,
				},
			},
		}, "spec.upgradeSettings"),
		Entry("BLUE_GREEN fails without blueGreenSettings", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
		}, "spec.upgradeSettings"),
		Entry("blueGreenSettings can only be set with a BLUE_GREEN strategy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("SURGE"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
		}, "spec.upgradeSettings"),
		Entry("blueGreenSettings can only be set with a BLUE_GREEN strategy (empty STRATEGY)", &v1beta1.NodePoolUpgradeSettings{
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
		}, "spec.upgradeSettings"),
		Entry("cannot set maxSurge with a BLUE_GREEN strategy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
			MaxSurge: &numberFifty,
		}, "spec.upgradeSettings"),
		Entry("cannot set maxUnavailable with a BLUE_GREEN strategy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
			MaxUnavailable: &numberFifty,
		}, "spec.upgradeSettings"),
		Entry("cannot set maxUnavailable and maxSurge with a BLUE_GREEN strategy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
				},
			},
			MaxUnavailable: &numberFifty,
			MaxSurge:       &numberFifty,
		}, "spec.upgradeSettings"),
		Entry("cannot set both batchPercentage and batchNodeCount on a StandardRolloutPolicy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchPercentage: &numberFifty,
					BatchNodeCount:  &numberFifty,
				},
			},
		}, "spec.upgradeSettings.blueGreenSettings.standardRolloutPolicy"),
		Entry("one of mutually exclusive pair is satisfied on StandardRolloutPolicy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchNodeCount: &numberFifty,
				},
			},
		}, ""),
		Entry("neither batch field is rejected on StandardRolloutPolicy", &v1beta1.NodePoolUpgradeSettings{
			Strategy: new("BLUE_GREEN"),
			BlueGreenSettings: &v1beta1.BlueGreenSettings{
				StandardRolloutPolicy: &v1beta1.StandardRolloutPolicy{
					BatchSoakDuration: &metav1.Duration{
						Duration: 60 * time.Minute,
					},
				},
			},
		}, "spec.upgradeSettings.blueGreenSettings.standardRolloutPolicy"),
		Entry("no strategy but maxSurge is ok", &v1beta1.NodePoolUpgradeSettings{
			MaxSurge: &numberFifty,
		}, ""),
	)
})
