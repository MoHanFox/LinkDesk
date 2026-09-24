package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"LinkDesk/middleware"
)

// RegisterRoutes 注册前端约定的全部路由
func RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// 这两个接口不需要登录
		api.POST("/auth/register", Register)
		api.POST("/auth/login", Login)

		// 以下接口都要求 Authorization: Bearer <token>,
		// 校验统一交给 Auth 中间件,handler 里不用再重复写。
		authed := api.Group("")
		authed.Use(middleware.Auth())
		{
			authed.GET("/me", Me)
			authed.GET("/links", ListLinks)
			authed.POST("/links", CreateLink)
			authed.GET("/links/:id", GetLink)
			authed.DELETE("/links/:id", DeleteLink)
		}
	}

	// 访问短链接不需要登录,直接 302 跳转
	r.GET("/r/:code", Redirect)
}

// notImplemented 占位 handler，等对应的业务实现写好后逐个替换
func notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"code":    "NOT_IMPLEMENTED",
		"message": "接口尚未实现",
	})
}
