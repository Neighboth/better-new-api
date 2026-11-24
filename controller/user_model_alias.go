package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type userModelAliasRequest struct {
	Alias       string `json:"alias"`
	TargetModel string `json:"target_model"`
	Description string `json:"description"`
}

func ListUserModelAliases(c *gin.Context) {
	userId := c.GetInt("id")
	items, err := model.ListUserModelAliases(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if items == nil {
		items = []model.UserModelAlias{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func CreateUserModelAlias(c *gin.Context) {
	userId := c.GetInt("id")
	var req userModelAliasRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	item, err := model.CreateUserModelAlias(userId, req.Alias, req.TargetModel, req.Description)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func UpdateUserModelAlias(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid alias ID"})
		return
	}
	var req userModelAliasRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	item, err := model.UpdateUserModelAlias(userId, id, req.Alias, req.TargetModel, req.Description)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func DeleteUserModelAlias(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid alias ID"})
		return
	}
	if err := model.DeleteUserModelAlias(userId, id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
