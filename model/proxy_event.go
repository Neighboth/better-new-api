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

package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ProxyPoolEvent is the audit log of the proxy pool, scoped per channel:
// assignments, 429 cooldowns, proxy switches during 429 retries, auto-test
// results and enable/disable transitions. Written fire-and-forget — a log
// failure must never break a relay.
type ProxyPoolEvent struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	ChannelId int    `json:"channel_id" gorm:"index:idx_proxy_event_channel_time,priority:1;not null"`
	KeyIndex  int    `json:"key_index" gorm:"index:idx_proxy_event_channel_time,priority:2"`
	ProxyID   int    `json:"proxy_id" gorm:"index"`
	ProxyURL  string `json:"proxy_url" gorm:"size:512"`
	Kind      string `json:"kind" gorm:"size:32;index"`
	Message   string `json:"message" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"index:idx_proxy_event_channel_time,priority:3;not null"`
}

func (ProxyPoolEvent) TableName() string { return "proxy_pool_events" }

// RecordProxyPoolEvent writes one audit row. Never fails the caller.
func RecordProxyPoolEvent(channelID int, keyIndex int, proxyID int, proxyURL string, kind string, message string) {
	if DB == nil {
		return
	}
	url := strings.TrimSpace(proxyURL)
	if len(url) > 512 {
		url = url[:512]
	}
	msg := strings.TrimSpace(message)
	if len(msg) > 4000 {
		msg = msg[:4000]
	}
	event := &ProxyPoolEvent{
		ChannelId: channelID,
		KeyIndex:  keyIndex,
		ProxyID:   proxyID,
		ProxyURL:  url,
		Kind:      kind,
		Message:   msg,
		CreatedAt: common.GetTimestamp(),
	}
	if err := DB.Create(event).Error; err != nil {
		common.SysError("proxy pool: failed to record event: " + err.Error())
	}
}

// ListProxyPoolEvents returns the newest events for one channel.
func ListProxyPoolEvents(channelID int, limit int) ([]ProxyPoolEvent, error) {
	if DB == nil {
		return []ProxyPoolEvent{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var events []ProxyPoolEvent
	err := DB.Where("channel_id = ?", channelID).Order("id DESC").Limit(limit).Find(&events).Error
	return events, err
}
