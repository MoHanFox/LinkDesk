package handler

import (
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"LinkDesk/config"
	"LinkDesk/db"
	"LinkDesk/middleware"
)

const (
	// 短码字符集和长度:只用字母和数字,不会产生需要转义的字符
	codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength   = 6
	maxURLLength = 2048
)

// quotaMu 把"查当天配额"和"写入新链接"这两步串起来,
// 否则并发请求可能同时通过配额检查,一起写进去超额。
// 它只对单个进程有效;以后部署多个实例时要换成数据库行锁。
var quotaMu sync.Mutex

type createLinkRequest struct {
	URL string `json:"url"`
}

// linkResponse 短链接对外的结构,创建、列表、详情三个接口共用同一套字段。
type linkResponse struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	OriginalURL string    `json:"original_url"`
	ShortURL    string    `json:"short_url"`
	ClickCount  int64     `json:"click_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateLink 创建短链接:校验 URL → 查配额 → 生成唯一短码 → 入库。
func CreateLink(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	var input createLinkRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "请求内容不是合法的 JSON"})
		return
	}

	originalURL := strings.TrimSpace(input.URL)
	if !isValidURL(originalURL) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_URL", "message": "URL 必须使用 http 或 https 协议"})
		return
	}

	quotaMu.Lock()
	defer quotaMu.Unlock()

	used, err := countTodayLinks(userID)
	if err != nil {
		internalError(c, "创建失败，请稍后重试")
		return
	}
	if used >= int64(config.Config.DailyLinkQuota) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "DAILY_QUOTA_EXCEEDED", "message": "今日创建次数已达上限"})
		return
	}

	link, err := createLinkWithUniqueCode(userID, originalURL)
	if err != nil {
		internalError(c, "创建失败，请稍后重试")
		return
	}

	c.JSON(http.StatusCreated, toLinkResponse(*link))
}

// ListLinks 返回当前用户的短链接,按创建时间倒序分页。
func ListLinks(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	page := parseQueryInt(c.Query("page"), 1, 1, 0)
	pageSize := parseQueryInt(c.Query("page_size"), 20, 1, 100)

	// 始终按当前用户的 user_id 过滤,这是数据隔离的落点
	var total int64
	if err := db.Db.Model(&db.Link{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		internalError(c, "读取列表失败，请稍后重试")
		return
	}

	var links []db.Link
	if err := db.Db.Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&links).Error; err != nil {
		internalError(c, "读取列表失败，请稍后重试")
		return
	}

	// 先初始化成空切片,没有数据时才能返回 [] 而不是 null
	items := make([]linkResponse, 0, len(links))
	for _, link := range links {
		items = append(items, toLinkResponse(link))
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetLink 返回单条短链接。别人的链接和不存在一样返回 404,不暴露它是否存在。
func GetLink(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		abortNotFound(c)
		return
	}

	var link db.Link
	if err := db.Db.Where("id = ? AND user_id = ?", id, userID).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			abortNotFound(c)
			return
		}
		internalError(c, "读取失败，请稍后重试")
		return
	}

	c.JSON(http.StatusOK, toLinkResponse(link))
}

// DeleteLink 删除自己的短链接。删除不会归还当天已经用掉的配额。
func DeleteLink(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "请先登录"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		abortNotFound(c)
		return
	}

	// 删除条件里带上 user_id,别人的链接删不到;影响行数为 0 就是不存在或不属于当前用户
	result := db.Db.Where("id = ? AND user_id = ?", id, userID).Delete(&db.Link{})
	if result.Error != nil {
		internalError(c, "删除失败，请稍后重试")
		return
	}
	if result.RowsAffected == 0 {
		abortNotFound(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// Redirect 处理访问短链接:查短码 → 点击数加一 → 302 跳转。该接口不需要登录。
func Redirect(c *gin.Context) {
	code := c.Param("code")

	var link db.Link
	if err := db.Db.Where("code = ?", code).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			abortNotFound(c)
			return
		}
		internalError(c, "跳转失败，请稍后重试")
		return
	}

	// 计数失败不能让用户被带到错的地方:记下错误,然后照常跳转。
	// 用 SQL 表达式自增,避免"读出来加一再写回去"在并发下丢计数。
	if err := db.Db.Model(&db.Link{}).Where("id = ?", link.ID).
		UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error; err != nil {
		log.Printf("[Link] 点击计数失败 code=%s: %v", code, err)
	}

	c.Redirect(http.StatusFound, link.OriginalURL)
}

// isValidURL 只放行 http 和 https,挡掉 javascript:、data:、file: 这些协议。
func isValidURL(raw string) bool {
	if raw == "" || len(raw) > maxURLLength {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

// countTodayLinks 统计用户当天已经创建了多少条。
// 用 Unscoped:软删除的链接同样占用当天配额(需求:删除不恢复配额)。
func countTodayLinks(userID uint) (int64, error) {
	start, end := todayRange()
	var count int64
	err := db.Db.Unscoped().Model(&db.Link{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).
		Count(&count).Error
	return count, err
}

// todayRange 按配置的时区算出"今天"的起止时刻。
func todayRange() (time.Time, time.Time) {
	location, err := time.LoadLocation(config.Config.Timezone)
	if err != nil {
		location = time.Local
	}
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1)
}

// createLinkWithUniqueCode 生成短码并入库,撞码就换一个重试,绝不覆盖已有记录。
func createLinkWithUniqueCode(userID uint, originalURL string) (*db.Link, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}

		link := db.Link{UserID: userID, OriginalURL: originalURL, Code: code}
		err = db.Db.Create(&link).Error
		if err == nil {
			return &link, nil
		}
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, err
		}
	}
	return nil, errors.New("短码连续冲突，生成失败")
}

// generateCode 用 crypto/rand 生成随机短码。
func generateCode() (string, error) {
	buf := make([]byte, codeLength)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		buf[i] = codeAlphabet[n.Int64()]
	}
	return string(buf), nil
}

func toLinkResponse(link db.Link) linkResponse {
	return linkResponse{
		ID:          link.ID,
		Code:        link.Code,
		OriginalURL: link.OriginalURL,
		ShortURL:    shortURL(link.Code),
		ClickCount:  link.ClickCount,
		// 时间统一按 UTC 返回(RFC3339),和需求里的示例格式一致
		CreatedAt: link.CreatedAt.UTC(),
	}
}

// shortURL 由后端按 PUBLIC_BASE_URL 拼出来,不让前端自己拼。
func shortURL(code string) string {
	return strings.TrimSuffix(config.Config.PublicBaseURL, "/") + "/r/" + code
}

// parseQueryInt 读分页参数:非法值回退到默认值,超过上限就截到上限。
func parseQueryInt(raw string, fallback, minimum, maximum int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum {
		return fallback
	}
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func abortNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "短链接不存在"})
}

func internalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": message})
}
