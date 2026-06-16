package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"murmur/auth"
	"murmur/mailer"
	"murmur/middleware"
	"murmur/models"
	"murmur/settings"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

type createUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// AdminCreateUser lets an admin create an active account directly (no review /
// email verification). Plain admins may only create normal users; super admins
// may also create admins.
func (h *H) AdminCreateUser(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(req.Email)

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
	role := models.RoleUser
	switch req.Role {
	case "", models.RoleUser:
		role = models.RoleUser
	case models.RoleAdmin:
		if actor.Role != models.RoleSuperAdmin {
			fail(c, http.StatusForbidden, "forbidden", "仅系统管理员可创建管理员")
			return
		}
		role = models.RoleAdmin
	default:
		fail(c, http.StatusBadRequest, "bad_role", "无效的角色")
		return
	}
	var cnt int64
	h.DB.Model(&models.User{}).Where("LOWER(username) = ?", strings.ToLower(req.Username)).Count(&cnt)
	if cnt > 0 {
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
	u := models.User{
		Username:        req.Username,
		PasswordHash:    hash,
		Nickname:        nickname,
		Email:           req.Email,
		EmailVerified:   true,
		Role:            role,
		Status:          models.StatusActive,
		RateLimitPerMin: models.RateInherit,
	}
	if err := h.DB.Create(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, "db", "创建用户失败")
		return
	}
	h.audit(actor.ID, "user.create", fmt.Sprintf("user:%d", u.ID), u.Username)
	c.JSON(http.StatusOK, view.FullUser(u))
}

type smtpTestReq struct {
	To string `json:"to"`
}

// AdminTestSMTP sends a test email using the current SMTP settings.
func (h *H) AdminTestSMTP(c *gin.Context) {
	var req smtpTestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	to := strings.TrimSpace(req.To)
	if !emailRe.MatchString(to) {
		fail(c, http.StatusBadRequest, "bad_email", "请输入有效的收件邮箱")
		return
	}
	cfg := h.smtpConfig()
	if !cfg.Configured() {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "SMTP 未配置完整(需主机/端口/发件邮箱)"})
		return
	}
	site := h.St.Get(settings.SiteTitle)
	body := fmt.Sprintf("这是一封来自 %s 的 SMTP 测试邮件。\n\n收到本邮件即表示邮件服务配置成功。", site)
	if err := mailer.Send(cfg, to, site+" · SMTP 测试", body); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已发送,请检查收件箱(含垃圾箱)"})
}
