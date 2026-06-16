package handlers

import (
	"crypto/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"murmur/auth"
	"murmur/mailer"
	"murmur/middleware"
	"murmur/models"
	"murmur/settings"
	"murmur/view"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// genCode returns a 6-digit numeric verification code.
func genCode() string {
	const digits = "0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = digits[int(b[i])%10]
	}
	return string(b)
}

func (h *H) smtpConfig() mailer.Config {
	return mailer.Config{
		Host:     h.St.Get(settings.SMTPHost),
		Port:     h.St.Get(settings.SMTPPort),
		Username: h.St.Get(settings.SMTPUsername),
		Password: h.St.Get(settings.SMTPPassword),
		From:     h.St.Get(settings.SMTPFrom),
		FromName: h.St.Get(settings.SMTPFromName),
		SSL:      h.St.GetBool(settings.SMTPSSL),
	}
}

func (h *H) sendVerifyEmail(to, code string) error {
	site := h.St.Get(settings.SiteTitle)
	subject := strings.TrimSpace(h.St.Get(settings.MailSubject))
	if subject == "" {
		subject = site + " 注册验证码"
	}
	body := strings.TrimSpace(h.St.Get(settings.MailBody))
	if body == "" {
		body = "【{site}】您的注册验证码是 {code}，10 分钟内有效。"
	}
	r := strings.NewReplacer("{site}", site, "{code}", code)
	return mailer.Send(h.smtpConfig(), to, r.Replace(subject), r.Replace(body))
}

// reservedUsernames cannot be registered by normal users.
var reservedUsernames = map[string]bool{"bot": true, "system": true}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}

func (h *H) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(req.Email)

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

	emailVerify := h.St.GetBool(settings.RegistrationMail)
	if emailVerify {
		if !emailRe.MatchString(req.Email) {
			fail(c, http.StatusBadRequest, "bad_email", "请输入有效的邮箱地址")
			return
		}
		if !h.smtpConfig().Configured() {
			fail(c, http.StatusServiceUnavailable, "mail_unconfigured", "邮件服务未配置，请联系管理员")
			return
		}
		var ec int64
		h.DB.Model(&models.User{}).Where("LOWER(email) = ?", strings.ToLower(req.Email)).Count(&ec)
		if ec > 0 {
			fail(c, http.StatusConflict, "email_exists", "该邮箱已被注册")
			return
		}
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
	switch {
	case emailVerify:
		status = models.StatusUnverified
	case h.St.GetBool(settings.RegistrationRev):
		status = models.StatusPending
	}
	u := models.User{
		Username:        req.Username,
		PasswordHash:    hash,
		Nickname:        nickname,
		Email:           req.Email,
		Role:            models.RoleUser,
		Status:          status,
		RateLimitPerMin: models.RateInherit,
	}
	if emailVerify {
		code := genCode()
		exp := time.Now().Add(10 * time.Minute)
		u.VerifyCode = code
		u.VerifyExpires = &exp
	}
	if err := h.DB.Create(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, "db", "注册失败")
		return
	}

	if emailVerify {
		if err := h.sendVerifyEmail(u.Email, u.VerifyCode); err != nil {
			c.JSON(http.StatusOK, gin.H{"email_verification": true, "email": u.Email, "mail_error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"email_verification": true, "email": u.Email})
		return
	}
	if status == models.StatusPending {
		c.JSON(http.StatusOK, gin.H{"pending": true})
		return
	}
	token, _ := auth.GenerateToken(h.Cfg.JWTSecret, u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": view.FullUser(u)})
}

type verifyEmailReq struct {
	Username string `json:"username"`
	Code     string `json:"code"`
}

// VerifyEmail confirms a registration code and activates (or queues for review)
// the account.
func (h *H) VerifyEmail(c *gin.Context) {
	var req verifyEmailReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	var u models.User
	if h.DB.Where("LOWER(username) = ?", strings.ToLower(strings.TrimSpace(req.Username))).First(&u).Error != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if u.Status != models.StatusUnverified {
		fail(c, http.StatusBadRequest, "not_unverified", "该账号无需验证")
		return
	}
	if u.VerifyCode == "" || u.VerifyExpires == nil || time.Now().After(*u.VerifyExpires) {
		fail(c, http.StatusBadRequest, "code_expired", "验证码已过期，请重新获取")
		return
	}
	if strings.TrimSpace(req.Code) != u.VerifyCode {
		fail(c, http.StatusBadRequest, "code_wrong", "验证码不正确")
		return
	}

	status := models.StatusActive
	if h.St.GetBool(settings.RegistrationRev) {
		status = models.StatusPending
	}
	h.DB.Model(&u).Updates(map[string]any{
		"email_verified": true,
		"status":         status,
		"verify_code":    "",
		"verify_expires": nil,
	})

	if status == models.StatusPending {
		c.JSON(http.StatusOK, gin.H{"pending": true})
		return
	}
	u.Status = status
	u.EmailVerified = true
	token, _ := auth.GenerateToken(h.Cfg.JWTSecret, u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": view.FullUser(u)})
}

type resendReq struct {
	Username string `json:"username"`
}

// ResendCode regenerates and re-sends the verification code.
func (h *H) ResendCode(c *gin.Context) {
	var req resendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	var u models.User
	if h.DB.Where("LOWER(username) = ?", strings.ToLower(strings.TrimSpace(req.Username))).First(&u).Error != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if u.Status != models.StatusUnverified {
		fail(c, http.StatusBadRequest, "not_unverified", "该账号无需验证")
		return
	}
	code := genCode()
	exp := time.Now().Add(10 * time.Minute)
	h.DB.Model(&u).Updates(map[string]any{"verify_code": code, "verify_expires": exp})
	if err := h.sendVerifyEmail(u.Email, code); err != nil {
		fail(c, http.StatusServiceUnavailable, "mail_failed", "验证码发送失败："+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
	if u.Status == models.StatusUnverified {
		fail(c, http.StatusForbidden, "unverified", "请先完成邮箱验证")
		return
	}
	if u.Status == models.StatusPending {
		fail(c, http.StatusForbidden, "pending", "账号待审核，请等待管理员通过")
		return
	}
	token, _ := auth.GenerateToken(h.Cfg.JWTSecret, u.ID, u.Role)
	c.JSON(http.StatusOK, gin.H{"token": token, "user": view.FullUser(u)})
}

func (h *H) Logout(c *gin.Context) {
	// Stateless JWT: the client simply discards the token.
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *H) Me(c *gin.Context) {
	u := middleware.CurrentUser(c)
	c.JSON(http.StatusOK, view.FullUser(*u))
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
	c.JSON(http.StatusOK, view.FullUser(fresh))
}
