package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func registerScopedAPIRoutes(apiRouter *gin.RouterGroup) {
	// Admin API key management for Blog API
	blogKeysRoute := apiRouter.Group("/blog/manage/api-keys")
	blogKeysRoute.Use(middleware.AdminAuth())
	{
		blogKeysRoute.GET("", middleware.RequirePermission(authz.BlogRead), controller.ListBlogAPIKeys)
		blogKeysRoute.GET("/", middleware.RequirePermission(authz.BlogRead), controller.ListBlogAPIKeys)
		blogKeysRoute.POST("", middleware.RequirePermission(authz.BlogWrite), controller.CreateBlogAPIKey)
		blogKeysRoute.POST("/", middleware.RequirePermission(authz.BlogWrite), controller.CreateBlogAPIKey)
		blogKeysRoute.DELETE("/:id", middleware.RequirePermission(authz.BlogWrite), controller.DeleteBlogAPIKey)
	}

	// Admin API key management for File Manager API
	fileKeysRoute := apiRouter.Group("/files/api-keys")
	fileKeysRoute.Use(middleware.AdminAuth())
	{
		fileKeysRoute.GET("", controller.ListFileManagerAPIKeys)
		fileKeysRoute.GET("/", controller.ListFileManagerAPIKeys)
		fileKeysRoute.POST("", controller.CreateFileManagerAPIKey)
		fileKeysRoute.POST("/", controller.CreateFileManagerAPIKey)
		fileKeysRoute.DELETE("/:id", controller.DeleteFileManagerAPIKey)
	}

	// External Blog Management API (only active with a valid Blog API key)
	blogExtAPI := apiRouter.Group("/v1/blog")
	blogExtAPI.Use(middleware.CORS(), controller.ScopedAPIKeyAuth(model.APIKeyScopeBlog))
	{
		blogExtAPI.GET("/posts", controller.BlogAPIListPosts)
		blogExtAPI.GET("/posts/:id", controller.BlogAPIGetPost)
		blogExtAPI.POST("/posts", controller.BlogAPICreatePost)
		blogExtAPI.PUT("/posts/:id", controller.BlogAPIUpdatePost)
		blogExtAPI.PATCH("/posts/:id", controller.BlogAPIUpdatePost)
		blogExtAPI.DELETE("/posts/:id", controller.BlogAPIDeletePost)
		blogExtAPI.POST("/upload", controller.BlogAPIUploadMedia)
	}

	// External File Manager API (only active with a valid File Manager API key)
	filesExtAPI := apiRouter.Group("/v1/files")
	filesExtAPI.Use(middleware.CORS(), controller.ScopedAPIKeyAuth(model.APIKeyScopeFileManager))
	{
		filesExtAPI.GET("", controller.FileManagerAPIList)
		filesExtAPI.GET("/", controller.FileManagerAPIList)
		filesExtAPI.GET("/info", controller.FileManagerAPIInfo)
		filesExtAPI.GET("/download", controller.FileManagerAPIDownload)
		filesExtAPI.GET("/content", controller.FileManagerAPIReadContent)
		filesExtAPI.PUT("/content", controller.FileManagerAPIWriteContent)
		filesExtAPI.POST("/content", controller.FileManagerAPIWriteContent)
		filesExtAPI.POST("/upload", controller.FileManagerAPIUpload)
		filesExtAPI.POST("/mkdir", controller.FileManagerAPIMkdir)
		filesExtAPI.POST("/rename", controller.FileManagerAPIRename)
		filesExtAPI.DELETE("", controller.FileManagerAPIDelete)
		filesExtAPI.DELETE("/", controller.FileManagerAPIDelete)
	}
}
