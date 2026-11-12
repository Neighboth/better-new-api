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
package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// ReferralSetting configures the enhanced referral reward of the site.
//
// The classic site-wide pair of options (QuotaForInviter / QuotaForInvitee)
// grants plain wallet quota that is indistinguishable from topped-up money.
// The enhanced mode grants a *display* balance and a *real* balance instead:
// the wallet shows the display amount, and every settlement spends the real
// balance. A reward that is advertised as "100 USD" but costs the site one
// dollar is configured as InviteeReal = $1 and InviteeDisplay = $100.
//
// Both balances are tracked per user (extra columns of the users table) and
// they are always consumed before the plain wallet quota. Spending the reward
// never touches the money the user topped up, and topping up never touches the
// reward.
type ReferralSetting struct {
	Enabled bool `json:"enabled"`

	// InviteeReal / InviteeDisplay are the real and the displayed balance
	// (in quota units, i.e. dollars * QuotaPerUnit) credited to a user that
	// registered with an invitation code.
	InviteeReal    int `json:"invitee_real"`
	InviteeDisplay int `json:"invitee_display"`

	// InviterReal / InviterDisplay are the real and the displayed balance
	// credited to the inviter when the invitee registers.
	InviterReal    int `json:"inviter_real"`
	InviterDisplay int `json:"inviter_display"`

	// ExcludedChannelIds lists the channels that a request funded (even
	// partially) by the referral balance may never take. It is the same
	// semantics the per-user channel blocklist uses: while the referral
	// balance of the user is positive, these channels are never selected
	// for the request, not even as a fallback.
	ExcludedChannelIds []int `json:"excluded_channel_ids"`
}

var referralSetting = ReferralSetting{
	Enabled:            false,
	InviteeReal:        1 * 500000,   // $1 real
	InviteeDisplay:     100 * 500000, // shown as $100
	InviterReal:        1 * 500000,
	InviterDisplay:     100 * 500000,
	ExcludedChannelIds: []int{},
}

func init() {
	config.GlobalConfig.Register("referral_setting", &referralSetting)
}

func GetReferralSetting() *ReferralSetting {
	return &referralSetting
}
