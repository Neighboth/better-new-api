package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

type proxyPoolRoute struct {
	method     string
	path       string
	permission authz.Permission
	handler    func(*gin.Context)
}

var proxyPoolRoutes = []proxyPoolRoute{
	{http.MethodGet, "/", authz.ProxyPoolRead, controller.ListProxyPoolProxies},
	{http.MethodPost, "/", authz.ProxyPoolWrite, controller.CreateProxyPoolProxy},
	{http.MethodPut, "/:id", authz.ProxyPoolWrite, controller.UpdateProxyPoolProxy},
	{http.MethodDelete, "/:id", authz.ProxyPoolWrite, controller.DeleteProxyPoolProxy},
	{http.MethodPost, "/:id/test", authz.ProxyPoolWrite, controller.TestProxyPoolProxy},
	{http.MethodGet, "/:id/detail", authz.ProxyPoolRead, controller.GetProxyPoolProxyDetail},
	{http.MethodPost, "/:id/unassign", authz.ProxyPoolWrite, controller.UnassignProxyPoolProxy},
	{http.MethodGet, "/channels", authz.ProxyPoolRead, controller.ListProxyPoolChannels},
	{http.MethodGet, "/assignments", authz.ProxyPoolRead, controller.ListProxyPoolAssignments},
	{http.MethodPost, "/assign", authz.ProxyPoolWrite, controller.AutoAssignProxyPool},
	{http.MethodPost, "/rebalance", authz.ProxyPoolWrite, controller.RebalanceAllProxyPoolChannels},
	{http.MethodDelete, "/assignments", authz.ProxyPoolWrite, controller.ClearProxyPoolAssignments},
	{http.MethodGet, "/usage-stats", authz.ProxyPoolRead, controller.GetProxyPoolUsageStats},
}

// registerProxyPoolRoutes wires the admin-only Proxy Pool API. Every route
// sits behind the admin dashboard session check and a per-resource permission
// so delegated admins can be granted or denied it independently.
func registerProxyPoolRoutes(apiRouter *gin.RouterGroup) {
	proxyPoolRoute := apiRouter.Group("/proxy-pool")
	proxyPoolRoute.Use(middleware.AdminAuth())
	for _, route := range proxyPoolRoutes {
		proxyPoolRoute.Handle(
			route.method,
			route.path,
			middleware.RequirePermission(route.permission),
			route.handler,
		)
	}
}
