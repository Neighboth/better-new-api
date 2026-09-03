package controller

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllVendors 获取供应商列表（分页）
func GetAllVendors(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	vendors, err := model.GetAllVendors(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var total int64
	model.DB.Model(&model.Vendor{}).Count(&total)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(vendors)
	common.ApiSuccess(c, pageInfo)
}

// SearchVendors 搜索供应商
func SearchVendors(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	vendors, total, err := model.SearchVendors(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(vendors)
	common.ApiSuccess(c, pageInfo)
}

// GetVendorMeta 根据 ID 获取供应商
func GetVendorMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	v, err := model.GetVendorByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, v)
}

// CreateVendorMeta 新建供应商
func CreateVendorMeta(c *gin.Context) {
	var v model.Vendor
	if err := c.ShouldBindJSON(&v); err != nil {
		common.ApiError(c, err)
		return
	}
	if v.Name == "" {
		common.ApiErrorMsg(c, "供应商名称不能为空")
		return
	}
	// 创建前先检查名称
	if dup, err := model.IsVendorNameDuplicated(0, v.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "供应商名称已存在")
		return
	}

	if err := v.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &v)
}

// UpdateVendorMeta 更新供应商
func UpdateVendorMeta(c *gin.Context) {
	var v model.Vendor
	if err := c.ShouldBindJSON(&v); err != nil {
		common.ApiError(c, err)
		return
	}
	if v.Id == 0 {
		common.ApiErrorMsg(c, "缺少供应商 ID")
		return
	}

	existing, err := model.GetVendorByID(v.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if existing.IsBuiltin {
		v.Name = existing.Name
		v.IsBuiltin = true
	} else {
		if dup, err := model.IsVendorNameDuplicated(v.Id, v.Name); err != nil {
			common.ApiError(c, err)
			return
		} else if dup {
			common.ApiErrorMsg(c, "供应商名称已存在")
			return
		}
	}

	if err := v.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &v)
}

// DeleteVendorMeta 删除供应商
func DeleteVendorMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	v, err := model.GetVendorByID(id)
	if err == nil && v.IsBuiltin {
		common.ApiErrorMsg(c, "Sisteme gömülü (Builtin) vendorlar silinemez.")
		return
	}
	if err := model.DB.Delete(&model.Vendor{}, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UploadVendorLogo handles uploading a vendor logo image file
func UploadVendorLogo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "No file uploaded: " + err.Error()})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".svg" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Unsupported file format"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to read file: " + err.Error()})
		return
	}
	defer f.Close()

	fileBytes := make([]byte, file.Size)
	_, _ = f.Read(fileBytes)

	fileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), common.GetRandomString(6), ext)
	relPath := "managed_files/vendors/" + fileName

	now := common.GetTimestamp()
	mf := model.ManagedFile{
		Path:      relPath,
		Name:      fileName,
		IsDir:     false,
		Size:      file.Size,
		Content:   fileBytes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.DB.Create(&mf)

	url := "/" + relPath
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    url,
	})
}
