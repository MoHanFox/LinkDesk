package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"LinkDesk/config"
	"LinkDesk/db"
	"LinkDesk/middleware"
)

// registerRequest 注册接口的请求体。
// 不能直接绑定 model.User:它的 Password 标了 json:"-",请求里的密码根本读不进来。
type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Register(c *gin.Context) {
	var input registerRequest
	// 把请求体里的 JSON 解析进 input。解析失败说明客户端发来的数据有问题。
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "请求内容不是合法的 JSON"})
		return
	}

	username := strings.TrimSpace(input.Username)
	password := input.Password

	if count := utf8.RuneCountInString(username); count < 3 || count > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "用户名长度需为 3-32 个字符"})
		return
	}
	if len(password) < 6 || len(password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "密码长度需为 6-72 个字符"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "注册失败，请稍后重试"})
		return
	}

	user := db.User{Username: username, Password: string(hashed)}
	if err := db.Db.Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"code": "USERNAME_EXISTS", "message": "用户名已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "注册失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": "OK", "message": "注册成功"})
}

func Login(c *gin.Context) {
	var input registerRequest
	// 把请求体里的 JSON 解析进 input。解析失败说明客户端发来的数据有问题。
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "请求内容不是合法的 JSON"})
		return
	}

	username := strings.TrimSpace(input.Username)
	password := input.Password

	// 按用户名查用户。查不到时不要单独提示,和密码错误返回同样的结果,
	// 否则别人可以用登录接口试出哪些用户名已经注册。
	var user db.User
	err := db.Db.Where("username = ?", username).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "登录失败，请稍后重试"})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_CREDENTIALS", "message": "用户名或密码错误"})
		return
	}

	// 用入库时的哈希和这次提交的密码比对,不一致就是密码错。
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "INVALID_CREDENTIALS", "message": "用户名或密码错误"})
		return
	}

	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "登录失败，请稍后重试"})
		return
	}

	// 有效期由配置项 token_ttl 决定,这里存一条会话记录,之后鉴权就靠查这张表。
	expiresAt := time.Now().Add(config.Config.TokenTTL)
	session := db.Session{UserID: user.ID, Token: token, ExpiresAt: expiresAt}
	if err := db.Db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "登录失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt.UTC(),
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// Me 返回当前登录用户。前端刷新页面后靠它确认本地 token 是否还有效。
func Me(c *gin.Context) {
	// 用户身份只从鉴权中间件放进上下文的值里取,不信任请求里的任何 user_id。
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	var user db.User
	if err := db.Db.First(&user, userID).Error; err != nil {
		// 会话还在但用户已经不在了,同样按未登录处理
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
	})
}

// generateToken 生成登录 token:32 字节随机数,编码成不含特殊字符的字符串。
// 必须用 crypto/rand,math/rand 的序列可以被推算出来,等于没有安全性。
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
