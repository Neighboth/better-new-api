package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
			perfMetricsRoute.DELETE("", middleware.RootAuth(), controller.ResetPerfMetrics)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.GET("/captcha/image", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), controller.GetImageCaptcha)
		apiRouter.GET("/legal", controller.GetLegalContent)

		// Public blog (read) + member interactions + admin management
		blogRoute := apiRouter.Group("/blog")
		{
			blogRoute.GET("/posts", controller.GetBlogPosts)
			blogRoute.GET("/ads", controller.GetAdsStatus)
			blogRoute.POST("/ads/impressions", controller.TrackAdImpression)
			blogRoute.GET("/posts/:id", middleware.TryUserAuth(), controller.GetBlogPost)
			blogUserRoute := blogRoute.Group("/")
			blogUserRoute.Use(middleware.UserAuth())
			{
				blogUserRoute.POST("/posts/:id/comments", controller.CreateBlogComment)
				blogUserRoute.DELETE("/posts/:id/comments/:commentId", controller.DeleteBlogComment)
				blogUserRoute.POST("/posts/:id/reactions", controller.ReactBlog)
				blogUserRoute.POST("/posts/:id/comments/:commentId/reactions", controller.ReactBlog)
			}
			blogAdminRoute := blogRoute.Group("/manage")
			blogAdminRoute.Use(middleware.AdminAuth())
			{
				blogAdminRoute.GET("/settings", middleware.RequirePermission(authz.BlogRead), controller.GetBlogSettings)
				blogAdminRoute.PUT("/settings", middleware.RequirePermission(authz.BlogWrite), controller.UpdateBlogSettings)
				blogAdminRoute.GET("/posts", middleware.RequirePermission(authz.BlogRead), controller.GetAllBlogPosts)
				blogAdminRoute.POST("/posts", middleware.RequirePermission(authz.BlogWrite), controller.CreateBlogPost)
				blogAdminRoute.PUT("/posts/:id", middleware.RequirePermission(authz.BlogWrite), controller.UpdateBlogPost)
				blogAdminRoute.DELETE("/posts/:id", middleware.RequirePermission(authz.BlogWrite), controller.DeleteBlogPost)
				blogAdminRoute.GET("/ads/stats", middleware.RequirePermission(authz.BlogRead), controller.GetAdImpressionStats)
				blogAdminRoute.DELETE("/ads/stats/clear", middleware.RequirePermission(authz.BlogWrite), controller.ClearAdImpressionStats)
				blogAdminRoute.GET("/ads/impressions.csv", middleware.RequirePermission(authz.BlogRead), controller.DownloadAdImpressionsCSV)
				blogAdminRoute.POST("/ads/upload", middleware.RequirePermission(authz.BlogWrite), controller.UploadAdImage)
			}
		}
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.POST("/oauth/state", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), anonymousRequestBodyLimit, controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLogin)
		apiRouter.POST("/oauth/telegram/bind/start", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramBindStart)
		apiRouter.GET("/oauth/telegram/bind/:flow_token", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		// :env separates test vs prod URLs so the operator can register each
		// in Pancake's matching webhook slot; handler enforces env match.
		apiRouter.POST("/waffo-pancake/webhook/:env", anonymousRequestBodyLimit, controller.WaffoPancakeWebhook)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/auth/refresh", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.RefreshAuth)
			userRoute.POST("/auth/logout", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AuthLogout)
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.POST("/shopier/notify", anonymousRequestBodyLimit, controller.ShopierNotify)
			userRoute.GET("/shopier/notify", controller.ShopierNotify)
			userRoute.POST("/paytr/notify", anonymousRequestBodyLimit, controller.PayTRNotify)
			userRoute.GET("/paypal/capture", controller.PayPalCapture)
			userRoute.POST("/paypal/capture", anonymousRequestBodyLimit, controller.PayPalCapture)
			userRoute.POST("/iyzico/callback", anonymousRequestBodyLimit, controller.IyzicoCallback)
			userRoute.GET("/iyzico/callback", controller.IyzicoCallback)
			userRoute.POST("/shopify/webhook", anonymousRequestBodyLimit, controller.ShopifyWebhook)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/sessions", middleware.DisableCache(), controller.GetLoginSessions)
				selfRoute.DELETE("/sessions/:sid", middleware.DisableCache(), controller.DeleteLoginSession)
				selfRoute.POST("/sessions/revoke-others", middleware.DisableCache(), controller.RevokeOtherLoginSessions)
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.UpdateSelf)
				selfRoute.POST("/avatar/upload", controller.UploadAvatar)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), middleware.DisableCache(), controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", middleware.DisableCache(), controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", middleware.DisableCache(), controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", middleware.DisableCache(), controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", middleware.DisableCache(), controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", middleware.DisableCache(), controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/shopier/pay", middleware.CriticalRateLimit(), controller.RequestShopier)
				selfRoute.POST("/paytr/pay", middleware.CriticalRateLimit(), controller.RequestPayTR)
				selfRoute.POST("/paypal/pay", middleware.CriticalRateLimit(), controller.RequestPayPal)
				selfRoute.POST("/iyzico/pay", middleware.CriticalRateLimit(), controller.RequestIyzico)
				selfRoute.POST("/shopify/pay", middleware.CriticalRateLimit(), controller.RequestShopify)
				selfRoute.POST("/billing_priority", controller.UpdateBillingPriority)
				selfRoute.PUT("/billing_priority", controller.UpdateBillingPriority)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				selfRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", middleware.UserCriticalRateLimit("aff-transfer"), controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", middleware.DisableCache(), controller.Setup2FA)
				selfRoute.POST("/2fa/enable", middleware.DisableCache(), controller.Enable2FA)
				selfRoute.POST("/2fa/disable", middleware.DisableCache(), controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", middleware.DisableCache(), controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", middleware.RequirePermission(authz.UserRead), controller.GetAllUsers)
				adminRoute.GET("/topup", middleware.RequirePermission(authz.UserRead), controller.GetAllTopUps)
				adminRoute.POST("/topup/complete", middleware.RequirePermission(authz.UserWrite), controller.AdminCompleteTopUp)
				adminRoute.GET("/search", middleware.RequirePermission(authz.UserRead), controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", middleware.RequirePermission(authz.UserRead), controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", middleware.RequirePermission(authz.UserWrite), controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", middleware.RequirePermission(authz.UserWrite), controller.AdminClearUserBinding)
				adminRoute.GET("/:id", middleware.RequirePermission(authz.UserRead), controller.GetUser)
				adminRoute.POST("/", middleware.RequirePermission(authz.UserWrite), controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", middleware.RequirePermission(authz.UserWrite), controller.UpdateUser)
				adminRoute.DELETE("/:id", middleware.RequirePermission(authz.UserDelete), controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", middleware.RequirePermission(authz.UserWrite), controller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", middleware.RequirePermission(authz.UserRead), controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", middleware.RequirePermission(authz.UserWrite), controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.PUT("/self/priority", controller.UpdateBillingPriority)
			subscriptionRoute.POST("/balance/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestBalancePay)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestWaffoPancakePay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", middleware.RequirePermission(authz.SubscriptionRead), controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.DELETE("/plans/:id", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminDeleteSubscriptionPlan)
			subscriptionAdminRoute.POST("/bind", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminBindSubscription)
			subscriptionAdminRoute.POST("/plans/:id/subscriptions/reset", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminResetPlanSubscriptions)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", middleware.RequirePermission(authz.SubscriptionRead), controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/users/:id/subscriptions/reset", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminResetUserSubscriptionsByPlan)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", middleware.RequirePermission(authz.SubscriptionWrite), controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.AdminAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.POST("/payment_compliance", middleware.RootAuth(), controller.ConfirmPaymentCompliance)
			optionRoute.GET("/channel_affinity_cache", middleware.RootAuth(), controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", middleware.RootAuth(), controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", middleware.RootAuth(), controller.ResetModelRatio)
			optionRoute.GET("/waffo-pancake/catalog", middleware.RootAuth(), controller.ListWaffoPancakeCatalog)
			optionRoute.POST("/waffo-pancake/pair", middleware.RootAuth(), controller.CreateWaffoPancakePair)
			optionRoute.POST("/waffo-pancake/save", middleware.RootAuth(), controller.SaveWaffoPancake)
			optionRoute.POST("/waffo-pancake/subscription-product", middleware.RootAuth(), controller.CreateWaffoPancakeSubscriptionProduct)
			optionRoute.GET("/waffo-pancake/subscription-product-options", middleware.RootAuth(), controller.ListWaffoPancakeSubscriptionProductOptions)
			optionRoute.POST("/test_email", middleware.AdminAuth(), controller.SendTestEmail)
			optionRoute.GET("/email_default_templates", middleware.AdminAuth(), controller.GetEmailDefaultTemplates)
		}

		// Managed Files (admin only)
		managedFilesRoute := apiRouter.Group("/files")
		managedFilesRoute.Use(middleware.AdminAuth())
		{
			managedFilesRoute.GET("/", controller.ListManagedFiles)
			managedFilesRoute.POST("/upload", controller.UploadManagedFile)
			managedFilesRoute.GET("/content", controller.GetManagedFileContent)
			managedFilesRoute.POST("/content", controller.SaveManagedFileContent)
			managedFilesRoute.POST("/settings", controller.UpdateManagedFileSettings)
			managedFilesRoute.DELETE("/", controller.DeleteManagedFile)
			managedFilesRoute.POST("/rename", controller.RenameManagedFile)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		registerChannelRoutes(apiRouter)
		registerAuthzRoutes(apiRouter)
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/auto-groups", controller.GetTokenAutoGroups)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", middleware.RequirePermission(authz.RedemptionRead), controller.GetAllRedemptions)
			redemptionRoute.GET("/search", middleware.RequirePermission(authz.RedemptionRead), controller.SearchRedemptions)
			redemptionRoute.GET("/:id", middleware.RequirePermission(authz.RedemptionRead), controller.GetRedemption)
			redemptionRoute.POST("/", middleware.RequirePermission(authz.RedemptionWrite), controller.AddRedemption)
			redemptionRoute.PUT("/", middleware.RequirePermission(authz.RedemptionWrite), controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", middleware.RequirePermission(authz.RedemptionWrite), controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", middleware.RequirePermission(authz.RedemptionWrite), controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		systemTaskRoute := apiRouter.Group("/system-task")
		systemTaskRoute.Use(middleware.RootAuth())
		{
			systemTaskRoute.POST("/log-cleanup", controller.CreateLogCleanupSystemTask)
			systemTaskRoute.GET("/list", controller.ListSystemTasks)
			systemTaskRoute.GET("/current", controller.GetCurrentSystemTask)
			systemTaskRoute.GET("/:task_id", controller.GetSystemTask)
		}
		systemInfoRoute := apiRouter.Group("/system-info")
		systemInfoRoute.Use(middleware.RootAuth())
		{
			systemInfoRoute.GET("/instances", controller.ListSystemInstances)
			systemInfoRoute.DELETE("/stale-instances", controller.DeleteStaleSystemInstances)
			systemInfoRoute.DELETE("/instances/:node_name", controller.DeleteStaleSystemInstance)
		}

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)
		dataRoute.GET("/flow", middleware.AdminAuth(), controller.GetAllFlowQuotaDates)
		dataRoute.GET("/flow/self", middleware.UserAuth(), controller.GetUserFlowQuotaDates)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)
		}

		// Ticket and Live Support routes
		ticketPublicRoute := apiRouter.Group("/ticket")
		{
			ticketPublicRoute.GET("/config", controller.GetTicketConfig)
			ticketPublicRoute.GET("/ws", controller.TicketWebSocket)
			ticketPublicRoute.GET("/", middleware.TryUserAuth(), controller.GetUserTickets)
		}

		ticketRoute := apiRouter.Group("/ticket")
		ticketRoute.Use(middleware.UserAuth())
		{
			ticketRoute.POST("/", controller.CreateTicket)
			ticketRoute.GET("/:id", controller.GetTicketDetail)
			ticketRoute.POST("/:id/message", controller.AddTicketMessage)
			ticketRoute.PUT("/:id/close", controller.CloseTicket)
		}

		adminTicketRoute := apiRouter.Group("/admin/ticket")
		adminTicketRoute.Use(middleware.AdminAuth())
		{
			adminTicketRoute.GET("/", controller.AdminGetAllTickets)
			adminTicketRoute.PUT("/:id", controller.AdminUpdateTicket)
			adminTicketRoute.DELETE("/:id", controller.AdminDeleteTicket)
			adminTicketRoute.POST("/:id/message", controller.AddTicketMessage)
		}


		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
			vendorRoute.POST("/upload", controller.UploadVendorLogo)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", middleware.RequirePermission(authz.ModelRead), controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", middleware.RequirePermission(authz.ModelWrite), controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", middleware.RequirePermission(authz.ModelRead), controller.GetMissingModels)
			modelsRoute.GET("/", middleware.RequirePermission(authz.ModelRead), controller.GetAllModelsMeta)
			modelsRoute.GET("/search", middleware.RequirePermission(authz.ModelRead), controller.SearchModelsMeta)
			modelsRoute.GET("/:id", middleware.RequirePermission(authz.ModelRead), controller.GetModelMeta)
			modelsRoute.POST("/", middleware.RequirePermission(authz.ModelWrite), controller.CreateModelMeta)
			modelsRoute.PUT("/", middleware.RequirePermission(authz.ModelWrite), controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", middleware.RequirePermission(authz.ModelWrite), controller.DeleteModelMeta)
		}

		// Reseller admin and self-service routes
		resellerAdminRoute := apiRouter.Group("/admin/reseller")
		resellerAdminRoute.Use(middleware.AdminAuth())
		{
			resellerAdminRoute.GET("/:userId", middleware.RequirePermission(authz.UserWrite), controller.AdminGetResellerConfig)
			resellerAdminRoute.PUT("/:userId", middleware.RequirePermission(authz.UserWrite), controller.AdminUpdateResellerConfig)
		}

		resellerRoute := apiRouter.Group("/reseller")
		resellerRoute.Use(middleware.ResellerAuth())
		{
			resellerRoute.GET("/self", controller.ResellerGetSelfConfig)
			resellerRoute.PUT("/self", controller.ResellerUpdateSelfConfig)
			resellerRoute.GET("/config", controller.ResellerGetSelfConfig)
			resellerRoute.PUT("/config", controller.ResellerUpdateSelfConfig)
			resellerRoute.GET("/summary", controller.ResellerGetSummary)
			resellerRoute.GET("/redemptions", controller.ResellerGetRedemptions)
			resellerRoute.POST("/redemptions", controller.ResellerCreateRedemption)
			resellerRoute.DELETE("/redemptions/:id", controller.ResellerDeleteRedemption)
			resellerRoute.GET("/users/:userId/logs", controller.ResellerGetUserLogs)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
