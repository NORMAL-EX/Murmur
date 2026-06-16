package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"murmur/auth"
	"murmur/middleware"
	"murmur/models"
	"murmur/settings"
	"murmur/view"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

// reservedUsernames cannot be registered by normal users.
var reservedUsernames = map[string]bool{"bot": true, "system": true}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

func (h *H) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)

	if !h.St.GetBool(settings.RegistrationOpen) {
		fail(c, http.StatusForbidden, "closed", "注册已关闭")
		return
	}
	if !usernameRe.MatchString(req.Username) {
		fail(c, http.StatusBadRequest, "bad_username", "用户名需为 3-32 位字母、数字或下划线")
		return
	}
	if reservedUsernames[strings.ToLower(req.Username)] {
		fail(c, http.StatusBadRequest, "reserved", "该用户名被保留")
		return
	}
	if len(req.Password) < 6 {
		fail(c, http.StatusBadRequest, "weak_password", "密码至少 6 位")
		return
	}

	var count int64
	h.DB.Model(&models.User{}).Where("LOWER(username) = ?", strings.ToLower(req.Username)).Count(&count)
	if count > 0 {
		fail(c, http.StatusConflict, "exists", "用户名已被占用")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "hash", "服务器错误")
		return
	}
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}
	status := models.StatusActive
	if h.St.GetBool(settings.RegistrationRev) {
		status = models.StatusPending
	}
	u := models.User{
		Username:        req.Username,
		PasswordHash:    hash,
		Nickname:        nickname,
		Role:            models.RoleUser,
		Status:          status,
		RateLimitPerMin: models.RateInherit,
	}
	if err := h.DB.Create(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, "db", "注册失败")
		return
	}

	if status == models.StatusPending {
		c.JSON(http.StatusOK, gin.H{"pending": true})
		return
	}
	token, _ := auth.GenerateToken(h.Cfg.JWTSecret, u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": view.PublicUser(u)})
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *H) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	var u models.User
	err := h.DB.Where("LOWER(username) = ?", strings.ToLower(strings.TrimSpace(req.Username))).First(&u).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			fail(c, http.StatusUnauthorized, "invalid", "用户名或密码错误")
			return
		}
		fail(c, http.StatusInternalServerError, "db", "服务器错误")
		return
	}
	if u.Role == models.RoleBot {
		fail(c, http.StatusForbidden, "bot", "机器人账号不可登录")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		fail(c, http.StatusUnauthorized, "invalid", "用户名或密码错误")
		return
	}
	if u.Status == models.StatusBanned {
		fail(c, http.StatusForbidden, "banned", "账号已被封禁")
		return
	}
	if u.Status == models.StatusPending {
		fail(c, http.StatusForbidden, "pending", "账号待审核,请等待管理员通过")
		return
	}
	token, _ := auth.GenerateToken(h.Cfg.JWTSecret, u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": view.PublicUser(u)})
}

func (h *H) Logout(c *gin.Context) {
	// Stateless JWT: the client simply discards the token.
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *H) Me(c *gin.Context) {
	u := middleware.CurrentUser(c)
	c.JSON(http.StatusOK, view.PublicUser(*u))
}

type updateMeReq struct {
	Nickname *string `json:"nickname"`
	Bio      *string `json:"bio"`
	Password *string `json:"password"`
}

func (h *H) UpdateMe(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req updateMeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	updates := map[string]any{}
	if req.Nickname != nil {
		n := strings.TrimSpace(*req.Nickname)
		if n == "" {
			fail(c, http.StatusBadRequest, "bad_nickname", "昵称不能为空")
			return
		}
		updates["nickname"] = n
	}
	if req.Bio != nil {
		updates["bio"] = strings.TrimSpace(*req.Bio)
	}
	if req.Password != nil {
		if len(*req.Password) < 6 {
			fail(c, http.StatusBadRequest, "weak_password", "密码至少 6 位")
			return
		}
		hash, _ := auth.HashPassword(*req.Password)
		updates["password_hash"] = hash
	}
	if len(updates) > 0 {
		h.DB.Model(u).Updates(updates)
	}
	var fresh models.User
	h.DB.First(&fresh, u.ID)
	c.JSON(http.StatusOK, view.PublicUser(fresh))
}
