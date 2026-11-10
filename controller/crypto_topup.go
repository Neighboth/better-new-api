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
package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type CryptoWalletItem struct {
	Id           string  `json:"id"`
	Network      string  `json:"network"`
	Address      string  `json:"address"`
	QrCode       string  `json:"qr_code,omitempty"`
	MinDeposit   float64 `json:"min_deposit,omitempty"`
	Instructions string  `json:"instructions,omitempty"`
}

func isCryptoTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return hasPayMethodSlug(operation_setting.PayMethods, model.PaymentMethodCrypto)
}

func getCryptoMinTopup() int {
	common.OptionMapRWMutex.RLock()
	val := common.OptionMap["CryptoMinTopup"]
	common.OptionMapRWMutex.RUnlock()
	if val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return operation_setting.MinTopUp
}

func GetCryptoWallets() []CryptoWalletItem {
	var wallets []CryptoWalletItem
	common.OptionMapRWMutex.RLock()
	jsonStr := common.OptionMap["CryptoWallets"]
	common.OptionMapRWMutex.RUnlock()
	if jsonStr != "" && jsonStr != "[]" {
		_ = json.Unmarshal([]byte(jsonStr), &wallets)
	}
	return wallets
}

type CreateCryptoDepositRequest struct {
	Amount   float64 `json:"amount"`
	WalletId string  `json:"wallet_id"`
	Network  string  `json:"network"`
}

func CreateCryptoDeposit(c *gin.Context) {
	if !isCryptoTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Kripto ile yükleme şu anda etkin değil",
		})
		return
	}

	var req CreateCryptoDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Geçersiz istek parametreleri",
		})
		return
	}

	minTopup := float64(getCryptoMinTopup())
	if req.Amount < minTopup {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Minimum yükleme tutarı $%.2f olmalıdır", minTopup),
		})
		return
	}

	userId := c.GetInt("id")
	username := c.GetString("username")
	if username == "" {
		u, _ := model.GetUserById(userId, false)
		if u != nil {
			username = u.Username
		}
	}

	wallets := GetCryptoWallets()
	var selectedWallet *CryptoWalletItem
	for _, w := range wallets {
		if w.Id == req.WalletId || strings.EqualFold(w.Network, req.Network) {
			selectedWallet = &w
			break
		}
	}

	if selectedWallet == nil && len(wallets) > 0 {
		selectedWallet = &wallets[0]
	}

	if selectedWallet == nil || strings.TrimSpace(selectedWallet.Address) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Geçerli bir kripto cüzdanı bulunamadı, lütfen sistem yöneticisine başvurun",
		})
		return
	}

	if selectedWallet.MinDeposit > 0 && req.Amount < selectedWallet.MinDeposit {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Bu ağ (%s) için minimum yükleme tutarı $%.2f olmalıdır", selectedWallet.Network, selectedWallet.MinDeposit),
		})
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	_ = group

	// Calculate quota from USD amount
	quotaToAdd, quotaErr := common.QuotaFromDecimalStrict(
		decimal.NewFromFloat(req.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	if quotaErr != nil || quotaToAdd <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Hesaplanan bakiye geçersiz",
		})
		return
	}

	deposit := &model.CryptoDeposit{
		UserId:   userId,
		Username: username,
		Network:  selectedWallet.Network,
		Address:  selectedWallet.Address,
		Amount:   req.Amount,
		Quota:    int64(quotaToAdd),
	}

	if err := model.CreateCryptoDeposit(deposit); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ödeme kaydı oluşturulamadı: " + err.Error(),
		})
		return
	}

	common.OptionMapRWMutex.RLock()
	instructions := common.OptionMap["CryptoInstructions"]
	common.OptionMapRWMutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ödeme referansı başarıyla oluşturuldu",
		"data": gin.H{
			"id":             deposit.Id,
			"reference_code": deposit.ReferenceCode,
			"trade_no":       deposit.TradeNo,
			"network":        deposit.Network,
			"address":        deposit.Address,
			"amount":         deposit.Amount,
			"quota":          deposit.Quota,
			"qr_code":        selectedWallet.QrCode,
			"instructions":   instructions,
		},
	})
}

type SubmitCryptoTxHashRequest struct {
	Id            int    `json:"id"`
	TxHash        string `json:"tx_hash"`
	SenderAddress string `json:"sender_address"`
}

func SubmitCryptoTxHash(c *gin.Context) {
	var req SubmitCryptoTxHashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Geçersiz istek parametreleri",
		})
		return
	}

	if req.Id <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ödeme ID belirtilmelidir",
		})
		return
	}

	userId := c.GetInt("id")
	deposit, err := model.SubmitCryptoTxHash(userId, req.Id, req.TxHash, req.SenderAddress)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "İşlem özeti (TxID) başarıyla kaydedildi. Ödemeniz incelendikten sonra bakiyeniz tanımlanacaktır.",
		"data":    deposit,
	})
}

func GetUserCryptoDeposits(c *gin.Context) {
	userId := c.GetInt("id")
	p, _ := strconv.Atoi(c.DefaultQuery("p", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}
	offset := p * pageSize

	deposits, total, err := model.GetUserCryptoDeposits(userId, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Kayıtlar getirilemedi: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": deposits,
			"total": total,
		},
	})
}

func GetAdminCryptoDeposits(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("p", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := p * pageSize
	status := c.DefaultQuery("status", "")
	keyword := c.DefaultQuery("keyword", "")

	deposits, total, err := model.GetAdminCryptoDeposits(status, keyword, offset, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Kayıtlar getirilemedi: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": deposits,
			"total": total,
		},
	})
}

func ApproveCryptoDeposit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Geçersiz ID",
		})
		return
	}

	adminUsername := c.GetString("username")
	callerIp := c.ClientIP()

	deposit, err := model.ApproveCryptoDeposit(id, adminUsername, callerIp)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Onaylama başarısız: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Kripto ödemesi başarıyla onaylandı ve %s kullanıcısına bakiye yüklendi.", deposit.Username),
		"data":    deposit,
	})
}

type RejectCryptoDepositRequest struct {
	Reason string `json:"reason"`
}

func RejectCryptoDeposit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Geçersiz ID",
		})
		return
	}

	var req RejectCryptoDepositRequest
	_ = c.ShouldBindJSON(&req)

	deposit, err := model.RejectCryptoDeposit(id, req.Reason)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Reddetme başarısız: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Kripto ödeme talebi reddedildi.",
		"data":    deposit,
	})
}
