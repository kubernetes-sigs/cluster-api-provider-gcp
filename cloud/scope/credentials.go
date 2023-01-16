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

package scope

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createComputeService(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client) (*compute.Service, error) {
	if credentialsRef == nil {
		computeSvc, err := compute.NewService(ctx)
		if err != nil {
			return nil, errors.Errorf("failed to create gcp compute client: %v", err)
		}

		return computeSvc, nil
	}

	secretRefName := types.NamespacedName{
		Name:      credentialsRef.Name,
		Namespace: credentialsRef.Namespace,
	}

	credSecret := &corev1.Secret{}
	if err := crClient.Get(ctx, secretRefName, credSecret); err != nil {
		return nil, fmt.Errorf("getting credentials secret %s\\%s: %w", secretRefName.Namespace, secretRefName.Name, err)
	}

	rawData, ok := credSecret.Data["credentials"]
	if !ok {
		return nil, errors.New("no credentials key in secret")
	}

	computeSvc, err := compute.NewService(ctx, option.WithCredentialsJSON(rawData))
	if err != nil {
		return nil, errors.Errorf("failed to create gcp compute client with credentials secret: %v", err)
	}

	return computeSvc, nil
}
