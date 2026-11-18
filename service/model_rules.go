/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// EffectiveStudioModel is the resolved option set of a configured studio
// model after its conditional option rules are applied for a concrete
// (duration, quality, ratio) selection.
type EffectiveStudioModel struct {
	Durations        []int
	Qualities        []string
	Ratios           []string
	MaxReferenceImages int // 0 = reference images disallowed
	ReferencesAllowed  bool
}

func intersectInts(base []int, only []int) []int {
	if len(only) == 0 {
		return base
	}
	allowed := make(map[int]struct{}, len(only))
	for _, value := range only {
		allowed[value] = struct{}{}
	}
	filtered := make([]int, 0, len(base))
	for _, value := range base {
		if _, ok := allowed[value]; ok {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func intersectStringsLower(base []string, only []string) []string {
	if len(only) == 0 {
		return base
	}
	allowed := make(map[string]struct{}, len(only))
	for _, value := range only {
		allowed[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	filtered := make([]string, 0, len(base))
	for _, value := range base {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(value))]; ok {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func stringSetContains(list []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, value := range list {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

func intSetContains(list []int, want int) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

func ruleMatches(rule operation_setting.MediaModelConstraint, duration int, durationSet bool, quality string, ratio string) bool {
	if len(rule.Durations) > 0 {
		if !durationSet || !intSetContains(rule.Durations, duration) {
			return false
		}
	}
	if len(rule.Qualities) > 0 {
		if quality == "" || !stringSetContains(rule.Qualities, quality) {
			return false
		}
	}
	if len(rule.Ratios) > 0 {
		if ratio == "" || !stringSetContains(rule.Ratios, ratio) {
			return false
		}
	}
	return true
}

// ResolveEffectiveStudioModel applies the model's conditional option rules
// for the concrete (duration, quality, ratio) selection. The zero value of an
// effect field in a rule means "no effect"; max_reference_images uses
// 0 = no override, -1 = references disabled, >=1 = cap override.
func ResolveEffectiveStudioModel(model operation_setting.MediaModel, duration int, durationSet bool, quality string, ratio string) EffectiveStudioModel {
	durations := append([]int(nil), model.Durations...)
	qualities := append([]string(nil), model.Qualities...)
	ratios := append([]string(nil), model.Ratios...)
	maxRefs := 0
	if model.ReferenceImage {
		if model.MaxReferenceImages > 0 {
			maxRefs = model.MaxReferenceImages
		} else {
			maxRefs = 5
		}
	}
	for _, rule := range model.Constraints {
		if !ruleMatches(rule, duration, durationSet, quality, ratio) {
			continue
		}
		durations = intersectInts(durations, rule.DurationsOnly)
		qualities = intersectStringsLower(qualities, rule.QualitiesOnly)
		ratios = intersectStringsLower(ratios, rule.RatiosOnly)
		if rule.MaxReferenceImages == -1 {
			maxRefs = 0
		} else if rule.MaxReferenceImages >= 1 {
			maxRefs = rule.MaxReferenceImages
		}
	}
	if !model.ReferenceImage {
		maxRefs = 0
	}
	return EffectiveStudioModel{
		Durations:          durations,
		Qualities:          qualities,
		Ratios:             ratios,
		MaxReferenceImages: maxRefs,
		ReferencesAllowed:  maxRefs > 0,
	}
}

// FindMediaModel resolves a configured model by exact (case-insensitive)
// match on its name. It returns nil when the name is not a configured studio
// model of the given kind ("video" or "image").
func FindMediaModel(kind string, name string) *operation_setting.MediaModel {
	setting := operation_setting.GetPlaygroundSetting()
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var models []operation_setting.MediaModel
	if kind == "image" {
		models = setting.ImageModelOptions()
	} else {
		models = setting.VideoModelOptions()
	}
	for i := range models {
		if strings.EqualFold(strings.TrimSpace(models[i].Model), name) {
			return &models[i]
		}
	}
	return nil
}
