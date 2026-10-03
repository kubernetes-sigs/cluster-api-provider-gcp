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

package addons

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// enabledFieldName is the flag GKE gives an add-on whose configuration is a message of its own.
const enabledFieldName = "enabled"

// sandboxTypeGVisor is the GCPManagedMachinePool spec.nodeSecurity.sandboxType value that turns on GKE
// Sandbox for a node pool, taken from GKE's own name for it so that the two can't disagree.
var sandboxTypeGVisor = containerpb.SandboxConfig_GVISOR.String()

// ToGKE renders the configuration as GKE's own AddonsConfig. Only what the cluster configured is set, so
// an add-on it never mentioned is left for GKE to default rather than being switched off.
func (c Config) ToGKE() *containerpb.AddonsConfig {
	if len(c.addons) == 0 {
		return nil
	}

	addonsConfig := &containerpb.AddonsConfig{}
	for _, flag := range c.flags() {
		message, field := flag.resolve(addonsConfig.ProtoReflect(), true)
		if field == nil {
			continue
		}
		message.Set(field, protoreflect.ValueOfBool(flag.value))
	}
	return addonsConfig
}

// DiffGKE returns what has to change for an existing GKE cluster's add-ons to match the configuration,
// and whether anything does.
//
// Only what the cluster configured is compared: GKE reports every add-on's real state, while the cluster
// speaks only about the ones it cares about, so comparing the messages whole would see a difference for
// every add-on it never mentioned. GKE treats an add-on update as a merge — the semantics
// `gcloud container clusters update --update-addons` relies on — so leaving an add-on out of the result
// leaves it as it is rather than switching it off.
func (c Config) DiffGKE(existing *containerpb.AddonsConfig) (bool, *containerpb.AddonsConfig) {
	if len(c.addons) == 0 {
		return false, nil
	}

	changed := &containerpb.AddonsConfig{}
	anyChanged := false
	for _, flag := range c.flags() {
		current, currentField := flag.resolve(existing.ProtoReflect(), false)
		if currentField != nil && current.Get(currentField).Bool() == flag.value {
			continue
		}

		message, field := flag.resolve(changed.ProtoReflect(), true)
		if field == nil {
			continue
		}
		message.Set(field, protoreflect.ValueOfBool(flag.value))
		anyChanged = true
	}

	if !anyChanged {
		return false, nil
	}
	return true, changed
}

// flag is one boolean the configuration sets: either an add-on's own switch, or one of its options.
type flag struct {
	addonKey string
	// optionKey is empty when the flag is the add-on's own switch.
	optionKey string
	value     bool
}

// flags returns every boolean the configuration sets, each add-on's own switch ahead of its options so
// that an add-on's message exists before anything reaches inside it.
func (c Config) flags() []flag {
	var flags []flag
	for _, addon := range c.addons {
		flags = append(flags, flag{addonKey: addon.Addon.Key, value: addon.Enabled})
		for _, option := range addon.Options {
			flags = append(flags, flag{addonKey: addon.Addon.Key, optionKey: option.Option.Key, value: option.Value})
		}
	}
	return flags
}

// resolve finds the boolean this flag sets within a GKE AddonsConfig, by name: an add-on's key and an
// option's key are both the name GKE's own API gives them, so neither has to be written down twice.
func (f flag) resolve(addonsConfig protoreflect.Message, mutate bool) (protoreflect.Message, protoreflect.FieldDescriptor) {
	addon, enabled := resolveFlag(addonsConfig, f.addonKey, mutate)
	if f.optionKey == "" || enabled == nil {
		return addon, enabled
	}
	return resolveFlag(addon, f.optionKey, mutate)
}

// resolveFlag finds the boolean key names within message: either a boolean field of its own, which is how
// GKE expresses an add-on's extra settings, or the enabled flag of a message field, which is how it
// expresses an add-on and any setting rich enough to have been given a message of its own.
//
// Reading allocates nothing, so anything unset reads as false all the way down. Writing allocates each
// message on the way, and writes only the boolean itself, so setting one of an add-on's options never
// quietly enables the add-on holding it.
//
// A nil descriptor means key doesn't name a boolean. TestSupportedResolve asserts that never happens for
// anything in Supported, and the check is here so that a GKE SDK bump that moved something degrades to a
// no-op rather than panicking inside a reconciler.
func resolveFlag(message protoreflect.Message, key string, mutate bool) (protoreflect.Message, protoreflect.FieldDescriptor) {
	field := message.Descriptor().Fields().ByJSONName(key)
	if field == nil {
		return nil, nil
	}
	if field.Kind() == protoreflect.BoolKind {
		return message, field
	}
	if field.Message() == nil {
		return nil, nil
	}

	addon := message.Get(field).Message()
	if mutate {
		addon = message.Mutable(field).Message()
	}

	enabled := addon.Descriptor().Fields().ByJSONName(enabledFieldName)
	if enabled == nil || enabled.Kind() != protoreflect.BoolKind {
		return nil, nil
	}
	return addon, enabled
}
