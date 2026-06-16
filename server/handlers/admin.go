package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"murmur/middleware"
	"murmur/models"
	"murmur/settings"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

func (h *H) Stats(c *gin.Context) {
	var users, pending, messages, aiCalls, channels int64
	h.DB.Model(&models.User{}).Where("role <> ?", models.RoleBot).Count(&users)
	h.DB.Model(&models.User{}).Where("status = ?", models.StatusPending).Count(&pending)
	h.DB.Model(&models.Message{}).Where("deleted = ?", false).Count(&messages)
	h.DB.Model(&models.Message{}).Where("is_bot = ?", true).Count(&aiCalls)
	h.DB.Model(&models.Channel{}).Count(&channels)

	now := time.Now()
	week := make([]int, 7)
	for i := 0; i < 7; i++ {
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var cnt int64
		h.DB.Model(&models.Message{}).Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).Count(&cnt)
		week[6-i] = int(cnt)
	}

	c.JSON(http.StatusOK, gin.H{
		"users":              users,
		"pending":            pending,
		"messages":           messages,
		"online":             h.Hub.OnlineCount(),
		"ai_calls":           aiCalls,
		"channels":           channels,
		"recent_messages_7d": week,
	})
}

func (h *H) AdminUsers(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := queryInt(c, "page_size", 20)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	db := h.DB.Model(&models.User{}).Where("role <> ?", models.RoleBot)
	if q != "" {
		like := "%" + q + "%"
		db = db.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}
	var total int64
	db.Count(&total)

	var users []models.User
	db.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users)
	items := make([]models.User, 0, len(users))
	for _, u := range users {
		items = append(items, view.PublicUser(u))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  int64(page*pageSize) < total,
	})
}

type adminUserReq struct {
	Status          *string `json:"status"`
	Role            *string `json:"role"`
	RateLimitPerMin *int    `json:"rate_limit_per_min"`
	Nickname        *string `json:"nickname"`
}

func (h *H) AdminUpdateUser(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var target models.User
	if err := h.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if target.Role == models.RoleBot {
		fail(c, http.StatusForbidden, "forbidden", "机器人账号请在 AI 设置中管理")
		return
	}
	if target.Role == models.RoleSuperAdmin {
		fail(c, http.StatusForbidden, "forbidden", "不能修改超级管理员")
		return
	}

	var req adminUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}

	// Plain admins may only touch normal users and may not change roles.
	if actor.Role == models.RoleAdmin {
		if target.Role != models.RoleUser {
			fail(c, http.StatusForbidden, "forbidden", "无权修改其他管理员")
			return
		}
		if req.Role != nil {
			fail(c, http.StatusForbidden, "forbidden", "无权修改角色")
			return
		}
	}

	updates := map[string]any{}
	if req.Role != nil {
		if actor.Role != models.RoleSuperAdmin {
			fail(c, http.StatusForbidden, "forbidden", "仅超级管理员可修改角色")
			return
		}
		if *req.Role != models.RoleUser && *req.Role != models.RoleAdmin {
			fail(c, http.StatusBadRequest, "bad_role", "无效的角色")
			return
		}
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		st := *req.Status
		if st != models.StatusActive && st != models.StatusBanned {
			fail(c, http.StatusBadRequest, "bad_status", "无效的状态")
			return
		}
		if st == models.StatusBanned && target.ID == actor.ID {
			fail(c, http.StatusForbidden, "forbidden", "不能封禁自己")
			return
		}
		updates["status"] = st
	}
	if req.RateLimitPerMin != nil {
		v := *req.RateLimitPerMin
		if v < -1 {
			v = -1
		}
		updates["rate_limit_per_min"] = v
	}
	if req.Nickname != nil {
		n := strings.TrimSpace(*req.Nickname)
		if n != "" {
			updates["nickname"] = n
		}
	}

	if len(updates) > 0 {
		h.DB.Model(&target).Updates(updates)
		h.audit(actor.ID, "user.update", fmt.Sprintf("user:%d", id), fmt.Sprintf("%+v", updates))
	}
	var fresh models.User
	h.DB.First(&fresh, id)
	c.JSON(http.StatusOK, view.PublicUser(fresh))
}

func (h *H) SuperDeleteUser(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var target models.User
	if err := h.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if target.Role == models.RoleSuperAdmin || target.Role == models.RoleBot {
		fail(c, http.StatusForbidden, "forbidden", "该账号不可删除")
		return
	}
	if target.ID == actor.ID {
		fail(c, http.StatusForbidden, "forbidden", "不能删除自己")
		return
	}
	h.DB.Where("user_id = ?", id).Delete(&models.Reaction{})
	h.DB.Where("mentioned_user_id = ?", id).Delete(&models.Mention{})
	h.DB.Delete(&target)
	h.audit(actor.ID, "user.delete", fmt.Sprintf("user:%d", id), target.Username)
	c.Status(http.StatusNoContent)
}

func (h *H) AdminRegistrations(c *gin.Context) {
	var users []models.User
	h.DB.Where("status = ?", models.StatusPending).Order("id ASC").Find(&users)
	out := make([]models.User, 0, len(users))
	for _, u := range users {
		out = append(out, view.PublicUser(u))
	}
	c.JSON(http.StatusOK, out)
}

func (h *H) AdminApprove(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var target models.User
	if err := h.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if target.Status != models.StatusPending {
		fail(c, http.StatusBadRequest, "not_pending", "该用户无需审核")
		return
	}
	h.DB.Model(&target).Update("status", models.StatusActive)
	h.audit(actor.ID, "registration.approve", fmt.Sprintf("user:%d", id), target.Username)
	c.Status(http.StatusNoContent)
}

func (h *H) AdminReject(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var target models.User
	if err := h.DB.First(&target, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if target.Status != models.StatusPending {
		fail(c, http.StatusBadRequest, "not_pending", "该用户无需审核")
		return
	}
	h.DB.Delete(&target)
	h.audit(actor.ID, "registration.reject", fmt.Sprintf("user:%d", id), target.Username)
	c.Status(http.StatusNoContent)
}

func (h *H) AdminGetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.St.AdminMap())
}

func (h *H) AdminPutSettings(c *gin.Context) {
	actor := middleware.CurrentUser(c)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	allowed := map[string]bool{}
	for k := range settings.Defaults {
		allowed[k] = true
	}
	updates := map[string]string{}
	for k, v := range body {
		if !allowed[k] {
			continue
		}
		strVal := stringifySetting(v)
		// Never blank out the API key by accident; only update when non-empty.
		if k == settings.AIAPIKey && strings.TrimSpace(strVal) == "" {
			continue
		}
		updates[k] = strVal
	}
	if err := h.St.SetMany(updates); err != nil {
		fail(c, http.StatusInternalServerError, "save", "保存设置失败")
		return
	}

	// Keep the bot account in sync with its configurable name/avatar.
	if _, ok := updates[settings.BotName]; ok {
		h.DB.Model(&models.User{}).Where("role = ?", models.RoleBot).
			Update("nickname", h.St.Get(settings.BotName))
	}
	if _, ok := updates[settings.BotAvatar]; ok {
		h.DB.Model(&models.User{}).Where("role = ?", models.RoleBot).
			Update("avatar_url", h.St.Get(settings.BotAvatar))
	}

	h.audit(actor.ID, "settings.update", "settings", strings.Join(keys(updates), ","))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *H) AdminGetAI(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ai_enabled":            h.St.GetBool(settings.AIEnabled),
		"ai_base_url":           h.St.Get(settings.AIBaseURL),
		"ai_api_key_set":        h.St.HasSecret(settings.AIAPIKey),
		"ai_model":              h.St.Get(settings.AIModel),
		"ai_system_prompt":      h.St.Get(settings.AISystemPrompt),
		"ai_temperature":        h.St.GetFloat(settings.AITemperature),
		"ai_max_tokens":         h.St.GetInt(settings.AIMaxTokens),
		"ai_context_char_limit": h.St.GetInt(settings.AIContextLimit),
		"ai_cooldown_seconds":   h.St.GetInt(settings.AICooldown),
		"ai_allow_dm":           h.St.GetBool(settings.AIAllowDM),
		"bot_name":              h.St.Get(settings.BotName),
		"bot_avatar":            h.St.Get(settings.BotAvatar),
	})
}

func (h *H) AdminTestAI(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	reply, err := h.AI.Test(ctx)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接成功", "reply": reply})
}

func (h *H) AdminAudit(c *gin.Context) {
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := 30
	var total int64
	h.DB.Model(&models.AuditLog{}).Count(&total)
	var logs []models.AuditLog
	h.DB.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	var actorIDs []uint
	for _, l := range logs {
		actorIDs = append(actorIDs, l.ActorID)
	}
	actors := map[uint]models.User{}
	if len(actorIDs) > 0 {
		var us []models.User
		h.DB.Where("id IN ?", actorIDs).Find(&us)
		for _, u := range us {
			actors[u.ID] = u
		}
	}
	items := make([]models.AuditLogDTO, 0, len(logs))
	for _, l := range logs {
		dto := models.AuditLogDTO{
			ID: l.ID, ActorID: l.ActorID, Action: l.Action,
			Target: l.Target, Detail: l.Detail, CreatedAt: l.CreatedAt,
		}
		if a, ok := actors[l.ActorID]; ok {
			pu := view.PublicUser(a)
			dto.Actor = &pu
		}
		items = append(items, dto)
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  int64(page*pageSize) < total,
	})
}

func stringifySetting(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		// drop trailing .0 for integers
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
