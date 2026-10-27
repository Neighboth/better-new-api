package controller

import (
	"fmt"
	"net/http"
	"os"
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
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("GetAllVendors panic recovered: %v", r))
			c.JSON(200, gin.H{
				"success": true,
				"data": gin.H{
					"items":     []*model.Vendor{},
					"total":     0,
					"page":      1,
					"page_size": 10,
				},
			})
		}
	}()

	pageInfo := common.GetPageQuery(c)
	vendors, err := model.GetAllVendors(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.SysError(fmt.Sprintf("GetAllVendors error: %v", err))
		pageInfo.SetTotal(0)
		pageInfo.SetItems([]*model.Vendor{})
		common.ApiSuccess(c, pageInfo)
		return
	}
	if vendors == nil {
		vendors = []*model.Vendor{}
	}
	var total int64
	if model.DB != nil {
		_ = model.DB.Model(&model.Vendor{}).Count(&total).Error
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(vendors)
	common.ApiSuccess(c, pageInfo)
}

// SearchVendors 搜索供应商
func SearchVendors(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("SearchVendors panic recovered: %v", r))
			c.JSON(200, gin.H{
				"success": true,
				"data": gin.H{
					"items":     []*model.Vendor{},
					"total":     0,
					"page":      1,
					"page_size": 10,
				},
			})
		}
	}()

	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	vendors, total, err := model.SearchVendors(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.SysError(fmt.Sprintf("SearchVendors error: %v", err))
		pageInfo.SetTotal(0)
		pageInfo.SetItems([]*model.Vendor{})
		common.ApiSuccess(c, pageInfo)
		return
	}
	if vendors == nil {
		vendors = []*model.Vendor{}
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
	model.RefreshPricing()
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
	model.RefreshPricing()
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
	model.RefreshPricing()
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

	// Create vendors uploads directory if it doesn't exist
	uploadDir := filepath.Join("uploads", "vendors")
	if err := os.MkdirAll(uploadDir, 0777); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to create directory: " + err.Error()})
		return
	}

	filePath := filepath.Join(uploadDir, fileName)
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to save file: " + err.Error()})
		return
	}

	url := "/uploads/vendors/" + fileName
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    url,
	})
}
