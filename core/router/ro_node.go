package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type NodeRouter struct{}

func (n *NodeRouter) InitRouter(Router *gin.RouterGroup) {
	baseApi := v2.ApiGroupApp.BaseApi
	nodeRouter := Router.Group("nodes").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	{
		nodeRouter.POST("/list", baseApi.ListNodes)
		nodeRouter.GET("/simple/all", baseApi.ListSimpleNodes)
		nodeRouter.POST("", baseApi.CreateNode)
		nodeRouter.POST("/check", baseApi.CheckNode)
		nodeRouter.POST("/update", baseApi.UpdateNode)
		nodeRouter.POST("/del", baseApi.DeleteNode)
	}

	xpackNodeRouter := Router.Group("xpack/nodes").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	{
		xpackNodeRouter.GET("/dashboard", baseApi.NodeDashboard)
		xpackNodeRouter.POST("/favorite", baseApi.FavoriteNode)
		xpackNodeRouter.POST("/sync", baseApi.SyncNodes)
		xpackNodeRouter.POST("/upgrade", baseApi.UpgradeNodes)
	}

	xpackSyncRouter := Router.Group("xpack/sync").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	{
		xpackSyncRouter.POST("/app/install", baseApi.InstallAppToNodes)
	}
}
