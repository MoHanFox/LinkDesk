package db

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	gorm.Model
	Username string `gorm:"size:32;uniqueIndex;not null" json:"username"`
	Password string `gorm:"size:72;not null" json:"-"` // 只存哈希，永不出现在 JSON 里
}

// Link 短链接模型
type Link struct {
	gorm.Model
	UserID      uint   `gorm:"index;not null" json:"user_id"`
	OriginalURL string `gorm:"size:2048;not null" json:"original_url"`
	Code        string `gorm:"size:16;uniqueIndex;not null" json:"code"`
	ClickCount  int64  `gorm:"not null;default:0" json:"click_count"`
}

// Session 登录会话:一个用户可以同时存在多条(换设备、换浏览器各一条)
type Session struct {
	gorm.Model
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Token     string    `gorm:"size:64;uniqueIndex;not null" json:"-"` // 鉴权时按这一列反查
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
}
