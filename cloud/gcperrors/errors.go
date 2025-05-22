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
	"net/http"
	"strings"

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

// IsAlreadyDeleted reports whether err is a Google API error indicating that the resource is already being deleted.
func IsAlreadyDeleted(err error) bool {
	if err == nil {
		return false
	}
	ae, _ := err.(*googleapi.Error)

	return strings.Contains(ae.Errors[0].Message, "Instance is already being deleted.")
}

// IsMemberNotFound reports whether err is a Google API error indicating that the member is not found.
func IsMemberNotFound(err error) bool {
	if err == nil {
		return false
	}
	ae, _ := err.(*googleapi.Error)

	return strings.Contains(ae.Errors[0].Message, "is not a member of")
}

// PrintGCPError returns the error message from a Google API error.
func PrintGCPError(err error) string {
	if err == nil {
		return ""
	}
	ae, _ := err.(*googleapi.Error)
	return ae.Message + " " + ae.Errors[0].Message + " " + ae.Errors[0].Reason
}
