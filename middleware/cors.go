package middleware

import (
	"net/http"
	"strings"
	"time"

	"LinkDesk/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func GinCORSMiddleware() gin.HandlerFunc {
	c := cors.Config{
		AllowOrigins:     config.Config.CorsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	return cors.New(c)
}

// Gin有自己的模块，如果需要自定义Cors，可位于main.go使用:
// r.Use(middleware.CORS())

// CustomCORS 用于定制CORS
func CustomCORS() gin.HandlerFunc {
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
