/*
Copyright 2018 The Kubernetes Authors.

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

// Package gcperrors implements gcp errors types.
package gcperrors

import (
	"errors"
	"net/http"

	"google.golang.org/api/googleapi"
)

// IsNotFound reports whether err is a Google API error
// with http.StatusNotFround.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*googleapi.Error)

	return ok && ae.Code == http.StatusNotFound
}

// IgnoreNotFound ignore Google API not found error and return nil.
// Otherwise return the actual error.
func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}

	return err
}

func UnwrapGCPError(err error) error {
	// If the error is nil, return nil.
	if err == nil {
		return nil
	}

	// Check if the error is a Google API error.
	ae, ok := err.(*googleapi.Error)
	if !ok {
		return err
	}

	// Unwrap the error and add a prefix to it.
	unwrappedGCPError := "GCP error: " + ae.Error()

	return errors.New(unwrappedGCPError)
}
