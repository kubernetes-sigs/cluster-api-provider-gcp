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

import (
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestIsNonDefaultUniverseDomain(t *testing.T) {
	tests := []struct {
		name           string
		universeDomain string
		want           bool
	}{
		{
			name:           "default googleapis.com",
			universeDomain: "googleapis.com",
			want:           false,
		},
		{
			name:           "empty string",
			universeDomain: "",
			want:           false,
		},
		{
			name:           "sovereign cloud domain",
			universeDomain: "googleapis.example.com",
			want:           true,
		},
		{
			name:           "custom universe domain",
			universeDomain: "custom.universe.domain",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNonDefaultUniverseDomain(tt.universeDomain)
			if got != tt.want {
				t.Errorf("IsNonDefaultUniverseDomain(%q) = %v, want %v", tt.universeDomain, got, tt.want)
			}
		})
	}
}

func TestLogUniverseDomain(t *testing.T) {
	tests := []struct {
		name           string
		universeDomain string
		endpoints      *infrav1.ServiceEndpoints
		wantLogged     []string
		wantNotLogged  []string
	}{
		{
			name:           "default universe domain logs nothing",
			universeDomain: DefaultUniverseDomain,
			wantNotLogged:  []string{"universe domain"},
		},
		{
			name:          "empty universe domain logs nothing",
			wantNotLogged: []string{"universe domain"},
		},
		{
			name:           "non-default universe domain without service endpoints warns",
			universeDomain: "gcd.example.com",
			wantLogged: []string{
				"Operating in a non-default universe domain",
				"No ServiceEndpoints are configured",
				"gcd.example.com",
			},
		},
		{
			name:           "non-default universe domain with service endpoints does not warn",
			universeDomain: "gcd.example.com",
			endpoints:      &infrav1.ServiceEndpoints{ComputeServiceEndpoint: "https://compute.gcd.example.com"},
			wantLogged:     []string{"Operating in a non-default universe domain"},
			wantNotLogged:  []string{"No ServiceEndpoints are configured"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logged strings.Builder
			logger := funcr.New(func(_, args string) { logged.WriteString(args + "\n") }, funcr.Options{})
			ctx := log.IntoContext(t.Context(), logger)

			logUniverseDomain(ctx, tt.universeDomain, tt.endpoints)

			for _, want := range tt.wantLogged {
				assert.Contains(t, logged.String(), want)
			}
			for _, notWant := range tt.wantNotLogged {
				assert.NotContains(t, logged.String(), notWant)
			}
		})
	}
}
