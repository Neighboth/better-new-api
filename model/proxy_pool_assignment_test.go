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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteAllProxyPoolAssignmentsRemovesEverything(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM proxy_pool_assignments")
	DB.Exec("DELETE FROM proxy_pool_proxies")

	// Two channels, two proxies, assignments scattered across both.
	now := int64(1000)
	proxies := []ProxyPoolProxy{
		{Name: "p1", URL: "socks5://p1.example:3128", ProxyType: "standard", Enabled: true},
		{Name: "p2", URL: "socks5://p2.example:3128", ProxyType: "standard", Enabled: true},
		{Name: "p3", URL: "socks5://p3.example:3128", ProxyType: "standard", Enabled: true},
	}
	for i := range proxies {
		require.NoError(t, DB.Create(&proxies[i]).Error)
	}
	assignments := []ProxyPoolAssignment{
		{ChannelId: 10, KeyIndex: 0, ProxyId: proxies[0].Id, CreatedAt: now, UpdatedAt: now},
		{ChannelId: 10, KeyIndex: 1, ProxyId: proxies[1].Id, CreatedAt: now, UpdatedAt: now},
		{ChannelId: 20, KeyIndex: 0, ProxyId: proxies[2].Id, CreatedAt: now, UpdatedAt: now},
		{ChannelId: 30, KeyIndex: 0, ProxyId: proxies[0].Id, CreatedAt: now, UpdatedAt: now},
	}
	for i := range assignments {
		require.NoError(t, DB.Create(&assignments[i]).Error)
	}

	require.NoError(t, DeleteAllProxyPoolAssignments())

	var remaining int64
	require.NoError(t, DB.Model(&ProxyPoolAssignment{}).Count(&remaining).Error)
	require.Equal(t, int64(0), remaining)
}

func TestDeleteProxyPoolAssignmentsChannelScoped(t *testing.T) {
	truncateTables(t)
	DB.Exec("DELETE FROM proxy_pool_assignments")
	DB.Exec("DELETE FROM proxy_pool_proxies")

	now := int64(1000)
	proxies := []ProxyPoolProxy{
		{Name: "p1", URL: "socks5://p1.example:3128", ProxyType: "standard", Enabled: true},
		{Name: "p2", URL: "socks5://p2.example:3128", ProxyType: "standard", Enabled: true},
	}
	for i := range proxies {
		require.NoError(t, DB.Create(&proxies[i]).Error)
	}
	assignments := []ProxyPoolAssignment{
		{ChannelId: 10, KeyIndex: 0, ProxyId: proxies[0].Id, CreatedAt: now, UpdatedAt: now},
		{ChannelId: 10, KeyIndex: 1, ProxyId: proxies[1].Id, CreatedAt: now, UpdatedAt: now},
		{ChannelId: 20, KeyIndex: 0, ProxyId: proxies[0].Id, CreatedAt: now, UpdatedAt: now},
	}
	for i := range assignments {
		require.NoError(t, DB.Create(&assignments[i]).Error)
	}

	require.NoError(t, DeleteProxyPoolAssignments(10))

	var remaining []ProxyPoolAssignment
	require.NoError(t, DB.Order("id ASC").Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, 20, remaining[0].ChannelId)
}
