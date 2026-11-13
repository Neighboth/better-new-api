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
	Ids []int `json:"ids"`
}

type mergeFieldDiff struct {
	field string
	label string
	value string
}

// mergeChannelFieldMissing normalizes nil and empty values to "" so that a
// missing setting is not reported as a difference.
func mergeChannelFieldMissing(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func mergeHumanField(field string) string {
	labels := map[string]string{
		"type":                "Tür",
		"base_url":            "Temel URL",
		"models":              "Modeller",
		"group":               "Grup",
		"model_mapping":       "Model Eşleme",
		"status_code_mapping": "Durum Kodu Eşleme",
		"priority":            "Öncelik",
		"weight":              "Ağırlık",
		"auto_ban":            "Otomatik Yasak",
		"tag":                 "Etiket",
		"setting":             "Ayarlar (proxy vb.)",
		"other_settings":      "Diğer Ayarlar (azure vb.)",
		"param_override":      "Parametre Geçersiz Kılma",
		"header_override":     "Başlık Geçersiz Kılma",
		"model_ratio_override": "Model Oranı Geçersiz Kılma",
		"openai_organization": "Kuruluş",
		"test_model":          "Test Modeli",
		"remark":              "Not",
		"other":               "Diğer Bilgi",
		"other_info":          "Diğer Bilgi 2",
		"status":              "Durum",
	}
	if label, ok := labels[field]; ok {
		return label
	}
	return field
}

// normalizeListSetting tidies a comma separated list into a canonical form so
// ordering is ignored when comparing channels.
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
	return strings.Join(out, ",")
}

func normalizeJSONSetting(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var decoded any
	if err := common.UnmarshalJsonStr(raw, &decoded); err != nil {
		// Not JSON: compare it as-is (trimmed).
		return strings.TrimSpace(raw)
	}
	canonical, err := common.Marshal(decoded)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(canonical)
}

func channelsAreEqual(channels []*model.Channel, ignoreFields map[string]bool) []mergeFieldDiff {
	var diffs []mergeFieldDiff
	first := channels[0]
	for _, other := range channels[1:] {
		compare := func(field, label string, a, b string) {
			if ignoreFields[field] {
				return
			}
			if strings.TrimSpace(a) != strings.TrimSpace(b) {
				diffs = append(diffs, mergeFieldDiff{field: field, label: mergeHumanField(field), value: b})
			}
		}
		compare("type", "", fmt.Sprintf("%d", first.Type), fmt.Sprintf("%d", other.Type))
		compare("base_url", "", mergeChannelFieldMissing(first.BaseURL), mergeChannelFieldMissing(other.BaseURL))
		compare("models", "", normalizeListSetting(first.Models), normalizeListSetting(other.Models))
		compare("group", "", normalizeListSetting(first.Group), normalizeListSetting(other.Group))
		compare("model_mapping", "", normalizeJSONSetting(mergeChannelFieldMissing(first.ModelMapping)), normalizeJSONSetting(mergeChannelFieldMissing(other.ModelMapping)))
		compare("status_code_mapping", "", mergeChannelFieldMissing(first.StatusCodeMapping), mergeChannelFieldMissing(other.StatusCodeMapping))
		compare("priority", "", fmt.Sprintf("%d", derefInt64(first.Priority)), fmt.Sprintf("%d", derefInt64(other.Priority)))
		compare("weight", "", fmt.Sprintf("%d", derefUint(first.Weight)), fmt.Sprintf("%d", derefUint(other.Weight)))
		compare("auto_ban", "", fmt.Sprintf("%d", derefInt(first.AutoBan)), fmt.Sprintf("%d", derefInt(other.AutoBan)))
		compare("tag", "", mergeChannelFieldMissing(first.Tag), mergeChannelFieldMissing(other.Tag))
		compare("setting", "", normalizeJSONSetting(mergeChannelFieldMissing(first.Setting)), normalizeJSONSetting(mergeChannelFieldMissing(other.Setting)))
		compare("other_settings", "", normalizeJSONSetting(first.OtherSettings), normalizeJSONSetting(other.OtherSettings))
		compare("param_override", "", normalizeJSONSetting(mergeChannelFieldMissing(first.ParamOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.ParamOverride)))
		compare("header_override", "", normalizeJSONSetting(mergeChannelFieldMissing(first.HeaderOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.HeaderOverride)))
		compare("model_ratio_override", "", normalizeJSONSetting(mergeChannelFieldMissing(first.ModelRatioOverride)), normalizeJSONSetting(mergeChannelFieldMissing(other.ModelRatioOverride)))
		compare("openai_organization", "", mergeChannelFieldMissing(first.OpenAIOrganization), mergeChannelFieldMissing(other.OpenAIOrganization))
		compare("test_model", "", mergeChannelFieldMissing(first.TestModel), mergeChannelFieldMissing(other.TestModel))
		compare("remark", "", mergeChannelFieldMissing(first.Remark), mergeChannelFieldMissing(other.Remark))
		compare("other", "", strings.TrimSpace(first.Other), strings.TrimSpace(other.Other))
		compare("other_info", "", strings.TrimSpace(first.OtherInfo), strings.TrimSpace(other.OtherInfo))
		compare("status", "", fmt.Sprintf("%d", first.Status), fmt.Sprintf("%d", other.Status))
	}
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

// isVertexStyleChannel reports whether one channel carries a Vertex style
// credential that cannot be trivially joined into a single multi-key string.
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
