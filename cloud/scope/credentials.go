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
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConfigFilePath is the path to GCP credential config.
	ConfigFilePath = "/home/.gcp/credentials"
)

// Credential is a struct to hold GCP credential data.
type Credential struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	ClientID    string `json:"client_id"`
}

func getCredentialDataFromRef(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client) ([]byte, error) {
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

	return rawData, nil
}

func getCredentialDataFromMount() ([]byte, error) {
	byteValue, err := os.ReadFile(ConfigFilePath)
	if err != nil {
		return nil, err
	}
	return byteValue, nil
}

func parseCredential(rawData []byte) (*Credential, error) {
	var credential Credential
	err := json.Unmarshal(rawData, &credential)
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

func createComputeService(ctx context.Context, credentialsRef *infrav1.ObjectReference, crClient client.Client) (*compute.Service, error) {
	var computeSvc *compute.Service
	var err error
	if credentialsRef == nil {
		computeSvc, err = compute.NewService(ctx)
	} else {
		var rawData []byte
		rawData, err = getCredentialDataFromRef(ctx, credentialsRef, crClient)
		if err == nil {
			computeSvc, err = compute.NewService(ctx, option.WithCredentialsJSON(rawData))
		}
	}
	if err != nil {
		return nil, errors.Errorf("failed to create gcp compute client: %v", err)
	}
	return computeSvc, nil
}
