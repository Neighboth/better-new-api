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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
)

type CryptoDeposit struct {
	Id            int     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int     `json:"user_id" gorm:"index"`
	Username      string  `json:"username" gorm:"type:varchar(100);index"`
	Network       string  `json:"network" gorm:"type:varchar(50);index"`
	Address       string  `json:"address" gorm:"type:varchar(255)"`
	Amount        float64 `json:"amount"` // USD money amount
	Quota         int64   `json:"quota"`  // Quota to add
	ReferenceCode string  `json:"reference_code" gorm:"unique;type:varchar(64);index"`
	TradeNo       string  `json:"trade_no" gorm:"unique;type:varchar(100);index"`
	TxHash        string  `json:"tx_hash" gorm:"type:varchar(255);index"`
	SenderAddress string  `json:"sender_address" gorm:"type:varchar(255)"`
	Status        string  `json:"status" gorm:"type:varchar(20);index;default:'pending'"` // pending, approved, rejected
	RejectReason  string  `json:"reject_reason" gorm:"type:text"`
	CreateTime    int64   `json:"create_time"`
	CompleteTime  int64   `json:"complete_time"`
}

const (
	CryptoDepositStatusPending  = "pending"
	CryptoDepositStatusApproved = "approved"
	CryptoDepositStatusRejected = "rejected"
)

func GenerateCryptoReferenceCode() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("CR-%d", time.Now().Unix()%1000000)
	}
	return fmt.Sprintf("CR-%s", strings.ToUpper(hex.EncodeToString(bytes)))
}

func (deposit *CryptoDeposit) Insert() error {
	var err error
	err = DB.Create(deposit).Error
	return err
}

func CreateCryptoDeposit(deposit *CryptoDeposit) error {
	if deposit.UserId <= 0 {
		return errors.New("invalid user id")
	}
	if deposit.ReferenceCode == "" {
		deposit.ReferenceCode = GenerateCryptoReferenceCode()
	}
	if deposit.TradeNo == "" {
		deposit.TradeNo = fmt.Sprintf("CRYPTO-USR%d-%s-%d", deposit.UserId, common.GetRandomString(6), time.Now().Unix())
	}
	deposit.Status = CryptoDepositStatusPending
	deposit.CreateTime = time.Now().Unix()
	return deposit.Insert()
}

func SubmitCryptoTxHash(userId int, depositId int, txHash string, senderAddress string) (*CryptoDeposit, error) {
	txHash = strings.TrimSpace(txHash)
	senderAddress = strings.TrimSpace(senderAddress)
	if txHash == "" {
		return nil, errors.New("İşlem Özeti (TxID / Hash) boş bırakılamaz")
	}

	var duplicateCount int64
	// Check if this txHash is already used by another non-rejected deposit
	err := DB.Model(&CryptoDeposit{}).
		Where("tx_hash = ? AND status != ? AND id != ?", txHash, CryptoDepositStatusRejected, depositId).
		Count(&duplicateCount).Error
	if err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		return nil, errors.New("bu TxID / İşlem Özeti zaten başka bir ödemede kullanılmış veya onay bekliyor")
	}

	deposit := &CryptoDeposit{}
	err = DB.Where("id = ? AND user_id = ?", depositId, userId).First(deposit).Error
	if err != nil {
		return nil, errors.New("ödeme kaydı bulunamadı")
	}

	if deposit.Status != CryptoDepositStatusPending {
		return nil, errors.New("bu ödeme talebi zaten sonuçlandırılmış")
	}

	deposit.TxHash = txHash
	if senderAddress != "" {
		deposit.SenderAddress = senderAddress
	}

	err = DB.Save(deposit).Error
	return deposit, err
}

func GetUserCryptoDeposits(userId int, offset int, limit int) ([]*CryptoDeposit, int64, error) {
	var deposits []*CryptoDeposit
	var total int64
	err := DB.Model(&CryptoDeposit{}).Where("user_id = ?", userId).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = DB.Where("user_id = ?", userId).
		Order("id desc").
		Offset(offset).
		Limit(limit).
		Find(&deposits).Error
	return deposits, total, err
}

func GetAdminCryptoDeposits(status string, keyword string, offset int, limit int) ([]*CryptoDeposit, int64, error) {
	var deposits []*CryptoDeposit
	var total int64
	query := DB.Model(&CryptoDeposit{})

	status = strings.TrimSpace(status)
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("username LIKE ? OR reference_code LIKE ? OR tx_hash LIKE ? OR sender_address LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("id desc").
		Offset(offset).
		Limit(limit).
		Find(&deposits).Error
	return deposits, total, err
}

func ApproveCryptoDeposit(depositId int, adminUsername string, callerIp string) (*CryptoDeposit, error) {
	deposit := &CryptoDeposit{}
	var quotaToAdd int

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", depositId).First(deposit).Error; err != nil {
			return errors.New("kripto ödeme kaydı bulunamadı")
		}

		if deposit.Status != CryptoDepositStatusPending {
			return errors.New("bu ödeme zaten sonuçlandırılmış")
		}

		if deposit.Quota <= 0 {
			return errors.New("geçersiz bakiye miktarı")
		}

		quotaToAdd = int(deposit.Quota)
		deposit.Status = CryptoDepositStatusApproved
		deposit.CompleteTime = time.Now().Unix()

		if err := tx.Save(deposit).Error; err != nil {
			return err
		}

		// Also record TopUp row for unified billing history
		topUp := &TopUp{
			UserId:          deposit.UserId,
			Amount:          int64(deposit.Amount),
			Money:           deposit.Amount,
			TradeNo:         deposit.TradeNo,
			PaymentMethod:   PaymentMethodCrypto,
			PaymentProvider: PaymentProviderCrypto,
			CreateTime:      deposit.CreateTime,
			CompleteTime:    deposit.CompleteTime,
			Status:          common.TopUpStatusSuccess,
		}
		_ = tx.Save(topUp).Error

		return creditTopUpQuota(tx, deposit.UserId, quotaToAdd, nil)
	})

	if err != nil {
		return nil, err
	}

	syncCreditUserQuotaCache(deposit.UserId, quotaToAdd, "crypto topup")
	RecordTopupLog(
		deposit.UserId,
		fmt.Sprintf("Kripto (%s) ödemesi onaylandı. Tutar: $%.2f, Bakiye: %s, TxID: %s, Referans: %s (Onaylayan: %s)",
			deposit.Network, deposit.Amount, logger.LogQuota(quotaToAdd), deposit.TxHash, deposit.ReferenceCode, adminUsername),
		callerIp,
		PaymentMethodCrypto,
		PaymentProviderCrypto,
	)

	return deposit, nil
}

func RejectCryptoDeposit(depositId int, reason string) (*CryptoDeposit, error) {
	deposit := &CryptoDeposit{}
	err := DB.Where("id = ?", depositId).First(deposit).Error
	if err != nil {
		return nil, errors.New("kripto ödeme kaydı bulunamadı")
	}

	if deposit.Status != CryptoDepositStatusPending {
		return nil, errors.New("bu ödeme zaten sonuçlandırılmış")
	}

	deposit.Status = CryptoDepositStatusRejected
	deposit.RejectReason = strings.TrimSpace(reason)
	deposit.CompleteTime = time.Now().Unix()

	err = DB.Save(deposit).Error
	return deposit, err
}
