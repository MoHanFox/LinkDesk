package middleware

import (
	"net/http"
	"strings"

	"LinkDesk/config"

	"github.com/gin-gonic/gin"
)

// CORS 按配置里的 cors_origins 放行前端跨域请求,并直接答复 OPTIONS 预检。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 只有来源在配置的白名单里才回跨域头;不回 "*",避免对所有人放开。
		if origin != "" && isAllowedOrigin(origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Add("Vary", "Origin")
		}

		// 预检请求:浏览器先问"这个跨域请求能不能发",这里直接答复,不进业务 handler。
		if c.Request.Method == http.MethodOptions {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isAllowedOrigin 判断来源是否在配置的白名单里(忽略大小写和结尾的斜杠)。
func isAllowedOrigin(origin string) bool {
	for _, allowed := range config.Config.CorsOrigins {
		if strings.EqualFold(strings.TrimSuffix(allowed, "/"), strings.TrimSuffix(origin, "/")) {
			return true
		}
	}
	return false
}
