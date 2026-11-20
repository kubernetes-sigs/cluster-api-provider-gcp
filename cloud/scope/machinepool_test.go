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

package scope

import "testing"

func Test_checkResourcePolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies []string
		region   string
		project  string
		wantErr  bool
	}{
		{
			name:     "unparsable",
			policies: []string{"foo", "bar"},
			wantErr:  true,
		},
		{
			name:     "parsable and unparsable",
			policies: []string{"regions/us-central1/resourcePolicies/spread", "bar"},
			wantErr:  true,
		},
		{
			name:     "parsable with wrong region",
			policies: []string{"regions/us-central1/resourcePolicies/spread"},
			region:   "us-west1",
			wantErr:  true,
		},
		{
			name:     "parsable with wrong project",
			policies: []string{"projects/my-project/regions/us-central1/resourcePolicies/spread"},
			region:   "us-central1",
			project:  "another-project",
			wantErr:  true,
		},
		{
			name:     "parsable without project succeeds",
			policies: []string{"regions/us-central1/resourcePolicies/spread"},
			region:   "us-central1",
			project:  "my-project",
		},
		{
			name:     "parsable with project succeeds",
			policies: []string{"projects/my-project/regions/us-central1/resourcePolicies/spread"},
			region:   "us-central1",
			project:  "my-project",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkResourcePolicies(tt.policies, tt.region, tt.project); (err != nil) != tt.wantErr {
				t.Errorf("checkResourcePolicies() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
