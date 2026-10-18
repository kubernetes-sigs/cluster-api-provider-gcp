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
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
)

// Config is the add-on configuration a cluster has asked for, resolved out of the CR into CAPG's own
// terms. Holding one is proof it makes sense on its own: every add-on and option it names exists, and no
// option is set on an add-on that is switched off. Whether GKE will accept it is a separate question, and
// the business of Constraints.
type Config struct {
	addons []AddonConfig
}

// AddonConfig is one add-on's resolved configuration.
type AddonConfig struct {
	Addon   Addon
	Enabled bool
	// Options are the add-on's own settings, in the order the add-on declares them.
	Options []OptionValue
}

// OptionValue is one of an add-on's options, and what the cluster set it to.
type OptionValue struct {
	Option Option
	Value  bool
}

// Addons returns the add-ons the cluster configured, in Supported order so that anything reported about
// them comes out in a stable order rather than a map-iteration one.
func (c Config) Addons() []AddonConfig {
	return c.addons
}

// EnabledKeys returns the keys of the add-ons the cluster has switched on, for naming them in a log line
// or a message without spelling out the whole configuration.
func (c Config) EnabledKeys() []string {
	keys := make([]string, 0, len(c.addons))
	for _, addon := range c.addons {
		if addon.Enabled {
			keys = append(keys, addon.Addon.Key)
		}
	}
	return keys
}

// Enabled reports whether the named add-on is switched on.
func (c Config) Enabled(key string) bool {
	for _, addon := range c.addons {
		if addon.Addon.Key == key {
			return addon.Enabled
		}
	}
	return false
}

// Parse resolves what a cluster asks for into CAPG's own terms, reporting whatever doesn't make sense:
// an add-on or option CAPG doesn't know, or an option set on an add-on that is switched off. It says
// nothing about whether GKE would accept the result — that is what Constraints are for — so a Config
// still has to be validated before it is acted on.
func Parse(config infrav1exp.AddonsConfig, path *field.Path) (Config, field.ErrorList) {
	var errs field.ErrorList

	resolved := make([]AddonConfig, 0, len(config))
	for _, addon := range Supported {
		settings, configured := config[addon.Key]
		if !configured {
			continue
		}

		options, optionErrs := parseOptions(addon, settings, path.Key(addon.Key).Child("options"))
		errs = append(errs, optionErrs...)
		if len(options) > 0 && !settings.Enabled {
			errs = append(errs, field.Invalid(path.Key(addon.Key), settings.Options,
				"options can only be set on an add-on that is enabled"))
		}

		resolved = append(resolved, AddonConfig{Addon: addon, Enabled: settings.Enabled, Options: options})
	}

	for _, key := range unknownKeys(config) {
		errs = append(errs, field.NotSupported(path.Key(key), key, Keys()))
	}

	return Config{addons: resolved}, errs
}

// parseOptions resolves one add-on's options, reporting any the add-on doesn't have.
func parseOptions(addon Addon, settings infrav1exp.AddonSettings, path *field.Path) ([]OptionValue, field.ErrorList) {
	values := make([]OptionValue, 0, len(settings.Options))
	for _, option := range addon.Options {
		value, configured := settings.Options[option.Key]
		if !configured {
			continue
		}
		values = append(values, OptionValue{Option: option, Value: value})
	}

	unknown := make([]string, 0, len(settings.Options))
	for key := range settings.Options {
		if _, ok := addon.option(key); !ok {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	errs := make(field.ErrorList, 0, len(unknown))
	for _, key := range unknown {
		errs = append(errs, field.NotSupported(path.Key(key), key, addon.optionKeys()))
	}

	return values, errs
}

// unknownKeys returns the add-on names config uses that CAPG doesn't know, sorted so that a cluster
// naming several is reported the same way every time.
func unknownKeys(config infrav1exp.AddonsConfig) []string {
	var unknown []string
	for key := range config {
		if !supportedKeys.Has(key) {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	return unknown
}
