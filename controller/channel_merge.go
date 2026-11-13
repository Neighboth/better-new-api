/*
Copyright (C) 2023-2026 QuantumNous
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/

package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

type mergeChannelsRequest struct {
	Ids    []int `json:"ids"`
	DryRun bool  `json:"dry_run"`
}

type mergeFieldDiff struct {
	field string
	label string
	value string
}

// ignoredOtherSettingsKeys are runtime-produced keys inside the other-settings
// JSON that change by themselves (upstream auto-detect sweeps). Comparing
// them would report differences for two identical channels.
var ignoredOtherSettingsKeys = map[string]bool{
	"upstream_model_update_last_check_time":     true,
	"upstream_model_update_last_detected_models": true,
	"upstream_model_update_last_removed_models":  true,
	"upstream_model_update_ignored_models":       true,
}

// ignoredOtherSettingsPrefixes matches any other-settings key that is produced
// by the upstream model-sync sweeper. Two identical channels would falsely
// differ on those keys, so they are skipped entirely.
var ignoredOtherSettingsPrefixes = []string{
	"upstream_model_update_",
}

// isIgnoredRuntimeKey reports whether a lower-cased leaf JSON key is runtime
// metadata that must not block a merge of two otherwise-identical channels.
func isIgnoredRuntimeKey(lower string) bool {
	if ignoredOtherSettingsKeys[lower] {
		return true
	}
	for _, prefix := range ignoredOtherSettingsPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// otherSettingFriendlyLabels maps raw JSON keys (as they appear in the
// channel form JSON) to short human Turkish labels. The raw key is still
// available on the field slot for audit purposes.
var otherSettingFriendlyLabels = map[string]string{
	"azure_responses_version":                 "Azure yanıt sürümü",
	"vertex_key_type":                         "Vertex anahtar tipi",
	"claude_beta_query":                       "Claude beta sorgu",
	"allow_service_tier":                      "Servis katmanı izni",
	"allow_inference_geo":                     "Çıkarım konumu izni",
	"allow_speed":                             "Hız ayarı izni",
	"allow_safety_identifier":                 "Güvenlik kimliği izni",
	"disable_store":                           "store parametresi kapalı",
	"allow_include_obfuscation":               "Obfuscation izni",
	"disable_task_polling_sleep":              "Görev bekleme kapalı",
	"aws_key_type":                            "AWS anahtar tipi",
	"upstream_model_update_check_enabled":     "Model güncelleme kontrolü",
	"upstream_model_update_auto_sync_enabled": "Model güncelleme otomatik",
	"force_format":                            "Biçim zorlama",
	"thinking_to_content":                     "Düşünceyi içeriğe dönüştür",
	"proxy":                                   "Proxy",
	"pass_through_body_enabled":               "Ham gövde geçir",
	"system_prompt":                           "Sistem prompt",
	"system_prompt_override":                  "Sistem promptu üstüne yaz",
	"http_protocol":                           "HTTP protokolü",
	"http2_connection_shards":                 "HTTP/2 bağlantı parçalama",
	"advanced_custom":                         "Özel istek dönüştürücü",
}

func mergeChannelFieldMissing(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func mergeHumanField(field string) string {
	if label, ok := otherSettingFriendlyLabels[field]; ok {
		return label
	}
	labels := map[string]string{
		"type":                 "Tür",
		"base_url":             "Temel URL",
		"models":               "Modeller",
		"group":                "Grup",
		"model_mapping":        "Model eşleme",
		"status_code_mapping":  "Durum kodu eşleme",
		"priority":             "Öncelik",
		"weight":               "Ağırlık",
		"auto_ban":             "Otomatik yasaklama",
		"tag":                  "Etiket",
		"setting":              "Ayarlar",
		"other_settings":       "Diğer ayarlar",
		"param_override":       "İstek parametresi geçersiz kılma",
		"header_override":      "İstek başlığı geçersiz kılma",
		"model_ratio_override": "Model oranı geçersiz kılma",
		"openai_organization":  "Kuruluş",
		"test_model":           "Test modeli",
		"remark":               "Not",
		"other":                "Diğer bilgi",
		"status":               "Durum",
		"multi_key_mode":       "Çok anahtarlı mod",
		"key_mode":             "Anahtar modu",
	}
	if label, ok := labels[field]; ok {
		return label
	}
	return field
}

// normalizeListSetting tidies a comma separated list into a canonical form so
// that ordering is ignored when comparing channels.
func normalizeListSetting(raw string) string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, strings.ToLower(part))
	}
	sort.Strings(out)
	for i, part := range out {
		out[i] = strings.Join(strings.Fields(part), " ")
	}
	return strings.Join(out, ",")
}

func normalizeJSONSetting(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var decoded any
	if err := common.UnmarshalJsonStr(trimmed, &decoded); err != nil {
		// Not JSON: compare literally (trimmed).
		return strings.TrimSpace(raw)
	}
	canonical, err := common.Marshal(decoded)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(canonical)
}

// flattenJSON reduces a nested document into dotted leaf paths so that each
// differing field points at the exact setting that differs.
func flattenJSON(prefix string, doc any) map[string]any {
	out := map[string]any{}
	switch value := doc.(type) {
	case map[string]any:
		if len(value) == 0 {
			out[prefix] = map[string]any{}
			return out
		}
		for key, child := range value {
			childPrefix := prefix + "." + key
			childMap := flattenJSON(childPrefix, child)
			for k, v := range childMap {
				out[k] = v
			}
		}
	case []any:
		out[prefix] = canonicalJSONValue(value)
	default:
		out[prefix] = value
	}
	return out
}

func canonicalJSONValue(value any) string {
	switch v := value.(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, canonicalJSONValue(item))
		}
		sort.Strings(parts)
		return "[" + strings.Join(parts, "|") + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+canonicalJSONValue(v[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case string:
		return v
	case bool:
		return fmt.Sprint(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// jsonDifferingPaths flattens the nested document under the dotted paths and
// reports the differing leaves. parent is the human name of the section, used
// to build a readable breadcrumb label (e.g. "Ayarlar › Sistem prompt").
func jsonDifferingPaths(aDoc any, bDoc any, parent string) []mergeFieldDiff {
	var diffs []mergeFieldDiff
	aMap := map[string]any{}
	bMap := map[string]any{}
	if am, ok := aDoc.(map[string]any); ok {
		aMap = flattenJSON("other", am)
	} else if aDoc != nil {
		aMap["other"] = aDoc
	}
	if bm, ok := bDoc.(map[string]any); ok {
		bMap = flattenJSON("other", bm)
	} else if bDoc != nil {
		bMap["other"] = bDoc
	}

	keys := map[string]bool{}
	for k := range aMap {
		keys[k] = true
	}
	for k := range bMap {
		keys[k] = true
	}
	for k := range keys {
		// strip the generic parent prefix to use the JSON key as the field id
		leaf := strings.TrimPrefix(k, "other.")
		short := leaf
		if idx := strings.LastIndex(leaf, "."); idx >= 0 {
			short = leaf[idx+1:]
		}
		if isIgnoredRuntimeKey(strings.ToLower(short)) {
			continue
		}
		aVal, aOk := aMap[k]
		bVal, bOk := bMap[k]
		aFlat, bFlat := "", ""
		if aOk {
			aFlat = canonicalJSONValue(aVal)
		}
		if bOk {
			bFlat = canonicalJSONValue(bVal)
		}
		if aFlat == bFlat {
			continue
		}
		diffs = append(diffs, mergeFieldDiff{
			field: leaf,
			label: parent + " › " + mergeHumanField(short),
			value: bFlat,
		})
	}
	return diffs
}

func channelsAreEqual(channels []*model.Channel, ignoreFields map[string]bool) []mergeFieldDiff {
	var diffs []mergeFieldDiff
	first := channels[0]
	for _, other := range channels[1:] {
		compare := func(field string, a, b string) {
			if ignoreFields[field] {
				return
			}
			if strings.TrimSpace(a) != strings.TrimSpace(b) {
				diffs = append(diffs, mergeFieldDiff{field: field, label: mergeHumanField(field), value: b})
			}
		}

		compare("type", fmt.Sprintf("%d", first.Type), fmt.Sprintf("%d", other.Type))
		compare("base_url", mergeChannelFieldMissing(first.BaseURL), mergeChannelFieldMissing(other.BaseURL))
		compare("models", normalizeListSetting(first.Models), normalizeListSetting(other.Models))
		compare("group", normalizeListSetting(first.Group), normalizeListSetting(other.Group))
		compare("model_mapping", normalizeJSONSetting(mergeChannelFieldMissing(first.ModelMapping)), normalizeJSONSetting(mergeChannelFieldMissing(other.ModelMapping)))
		compare("status_code_mapping", mergeChannelFieldMissing(first.StatusCodeMapping), mergeChannelFieldMissing(other.StatusCodeMapping))
		compare("priority", fmt.Sprintf("%d", derefInt64(first.Priority)), fmt.Sprintf("%d", derefInt64(other.Priority)))
		compare("weight", fmt.Sprintf("%d", derefUint(first.Weight)), fmt.Sprintf("%d", derefUint(other.Weight)))
		compare("auto_ban", fmt.Sprintf("%d", derefInt(first.AutoBan)), fmt.Sprintf("%d", derefInt(other.AutoBan)))
		compare("tag", mergeChannelFieldMissing(first.Tag), mergeChannelFieldMissing(other.Tag))
		compare("setting", normalizeJSONSetting(mergeChannelFieldMissing(first.Setting)), normalizeJSONSetting(mergeChannelFieldMissing(other.Setting)))
		compare("param_override", normalizeJSONSetting(mergeChannelFieldMissing(first.ParamOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.ParamOverride)))
		compare("header_override", normalizeJSONSetting(mergeChannelFieldMissing(first.HeaderOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.HeaderOverride)))
		compare("model_ratio_override", normalizeJSONSetting(mergeChannelFieldMissing(first.ModelRatioOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.ModelRatioOverride)))
		compare("openai_organization", mergeChannelFieldMissing(first.OpenAIOrganization), mergeChannelFieldMissing(other.OpenAIOrganization))
		compare("test_model", mergeChannelFieldMissing(first.TestModel), mergeChannelFieldMissing(other.TestModel))
		compare("remark", mergeChannelFieldMissing(first.Remark), mergeChannelFieldMissing(other.Remark))
		compare("other", strings.TrimSpace(first.Other), strings.TrimSpace(other.Other))
		// other_info is runtime metadata produced by channel test pings. It
		// changes by itself and must not block a merge of identical channels.
		compare("status", fmt.Sprintf("%d", first.Status), fmt.Sprintf("%d", other.Status))

		var aSettingDoc, bSettingDoc any
		_ = common.UnmarshalJsonStr(mergeChannelFieldMissing(first.Setting), &aSettingDoc)
		_ = common.UnmarshalJsonStr(mergeChannelFieldMissing(other.Setting), &bSettingDoc)
		for _, diff := range jsonDifferingPaths(aSettingDoc, bSettingDoc, mergeHumanField("setting")) {
			diff.field = "setting." + diff.field
			diffs = append(diffs, diff)
		}

		var aOtherDoc, bOtherDoc any
		_ = common.UnmarshalJsonStr(first.OtherSettings, &aOtherDoc)
		_ = common.UnmarshalJsonStr(other.OtherSettings, &bOtherDoc)
		for _, diff := range jsonDifferingPaths(aOtherDoc, bOtherDoc, mergeHumanField("other_settings")) {
			diff.field = "other_settings." + diff.field
			diffs = append(diffs, diff)
		}
	}

	// Deduplicate labels in order of first appearance.
	seen := map[string]bool{}
	unique := make([]mergeFieldDiff, 0, len(diffs))
	for _, diff := range diffs {
		if seen[diff.field] {
			continue
		}
		seen[diff.field] = true
		unique = append(unique, diff)
	}
	diffs = unique
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].field < diffs[j].field })
	return diffs
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func derefUint(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func isVertexStyleChannel(ch *model.Channel) bool {
	return ch.Type == constant.ChannelTypeVertexAi
}

// MergeChannels merges several single-key channels into one multi-key
// channel. Every setting except the API key must be identical; otherwise the
// merge is refused and the differing fields are reported.
func MergeChannels(c *gin.Context) {
	req := mergeChannelsRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.Ids) < 2 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Birleştirmek için en az 2 kanal seçin"})
		return
	}
	seen := map[int]bool{}
	ids := make([]int, 0, len(req.Ids))
	for _, id := range req.Ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	channels := make([]*model.Channel, 0, len(ids))
	for _, id := range ids {
		ch, err := model.GetChannelById(id, true)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Kanal %d okunamadı: %v", id, err)})
			return
		}
		channels = append(channels, ch)
	}

	for _, ch := range channels {
		if ch.ChannelInfo.IsMultiKey {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Kanal #%d zaten çok anahtarlı. Çok anahtarlı kanallar birleştirilemez.", ch.Id)})
			return
		}
		if isVertexStyleChannel(ch) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Kanal #%d Vertex AI. Bu tip anahtarlar virgülle birleştirilemez; tek tek düzenleyin.", ch.Id)})
			return
		}
	}

	diffs := channelsAreEqual(channels, map[string]bool{})
	if req.DryRun {
		labels := make([]string, 0, len(diffs))
		for _, diff := range diffs {
			labels = append(labels, diff.label)
		}
		sort.Strings(labels)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"dry_run":          true,
				"identical":        len(labels) == 0,
				"differing_fields": labels,
				"differing_count":  len(labels),
			},
		})
		return
	}
	if len(diffs) > 0 {
		labels := make([]string, 0, len(diffs))
		for _, diff := range diffs {
			labels = append(labels, diff.label)
		}
		sort.Strings(labels)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Kanallar birleştirilemez: farklı olan ayarlar: %s", strings.Join(labels, ", ")),
			"data":    gin.H{"differing_fields": labels},
		})
		return
	}

	// Combine keys in id order and deduplicate exact duplicates.
	keySet := map[string]struct{}{}
	keys := make([]string, 0)
	for _, ch := range channels {
		for _, key := range strings.Split(ch.Key, "\n") {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if _, ok := keySet[key]; ok {
				continue
			}
			keySet[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Birleştirilecek geçerli anahtar bulunamadı"})
		return
	}

	first := channels[0]
	merged := *first
	merged.Id = 0
	merged.CreatedTime = common.GetTimestamp()
	merged.TestTime = 0
	merged.ResponseTime = 0
	merged.Balance = 0
	merged.BalanceUpdatedTime = 0
	var totalUsed int64
	for _, ch := range channels {
		totalUsed += ch.UsedQuota
	}
	merged.UsedQuota = totalUsed
	merged.Key = strings.Join(keys, "\n")
	merged.ChannelInfo = model.ChannelInfo{
		IsMultiKey:   true,
		MultiKeySize: len(keys),
		MultiKeyMode: constant.MultiKeyModeRandom,
	}

	// Create abilities for the merged channel first, then drop the originals
	// (which also drops their abilities). If insertion fails nothing was
	// deleted yet.
	combined := []model.Channel{merged}
	if err := model.BatchInsertChannels(combined); err != nil {
		common.ApiError(c, err)
		return
	}
	createdId := combined[0].Id

	idsToDelete := make([]int, 0, len(channels))
	for _, ch := range channels {
		idsToDelete = append(idsToDelete, ch.Id)
	}
	deleted, err := model.BatchDeleteChannels(idsToDelete)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Birleşik kanal oluşturuldu ancak eski kanallar silinemedi: %v", err)})
		return
	}

	recordManageAudit(c, "channel.merge", map[string]interface{}{
		"name":     merged.Name,
		"keys":     len(keys),
		"deleted":  deleted,
		"original": idsToDelete,
		"created":  createdId,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":        createdId,
			"name":      merged.Name,
			"key_count": len(keys),
			"merged":    deleted,
		},
	})
}
