package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"LinkDesk/db"
)

// contextUserID 是当前登录用户 ID 在 gin.Context 里的键名
const contextUserID = "currentUserID"

// Auth 校验 Authorization: Bearer <token>,通过后把用户 ID 放进上下文,供后面的 handler 取用。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			abortUnauthorized(c)
			return
		}

		// token 是随机串,没有自带信息,所以每次都要查会话表确认它是否有效。
		var session db.Session
		err := db.Db.Where("token = ?", token).First(&session).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				abortUnauthorized(c)
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "服务异常，请稍后重试"})
			}
			return
		}

		if time.Now().After(session.ExpiresAt) {
			abortUnauthorized(c)
			return
		}

		c.Set(contextUserID, session.UserID)
		c.Next()
	}
}

// CurrentUserID 取出 Auth 中间件放进上下文的当前用户 ID。
// 业务 handler 只能用这个拿用户身份,绝不能从请求体或查询参数里取 user_id。
func CurrentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(contextUserID)
	if !exists {
		return 0, false
	}
	userID, ok := value.(uint)
	return userID, ok
}

// bearerToken 从 "Bearer xxx" 这样的请求头里取出 token。
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    "UNAUTHORIZED",
		"message": "请先登录",
	})
}
