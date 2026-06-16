package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"murmur/middleware"
	"murmur/models"
	"murmur/settings"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

type editReq struct {
	Content string `json:"content"`
}

func (h *H) EditMessage(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的消息 ID")
		return
	}
	var msg models.Message
	if err := h.DB.First(&msg, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "消息不存在")
		return
	}
	if msg.Deleted {
		fail(c, http.StatusBadRequest, "deleted", "消息已删除")
		return
	}
	// Only the author may edit their own (non-bot) message.
	if msg.SenderID != u.ID || msg.IsBot {
		fail(c, http.StatusForbidden, "forbidden", "无权编辑该消息")
		return
	}
	var req editReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		fail(c, http.StatusBadRequest, "empty", "消息不能为空")
		return
	}
	maxLen := h.St.GetInt(settings.MaxMessageLength)
	if maxLen > 0 && utf8.RuneCountInString(content) > maxLen {
		fail(c, http.StatusBadRequest, "too_long", "消息超过长度限制")
		return
	}
	h.DB.Model(&msg).Updates(map[string]any{"content": content, "edited": true})
	h.DB.First(&msg, id)
	dto := view.BuildMessageDTO(h.DB, msg, u.ID)
	h.Hub.BroadcastMessageUpdate(dto)
	c.JSON(http.StatusOK, dto)
}

func (h *H) DeleteMessage(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的消息 ID")
		return
	}
	var msg models.Message
	if err := h.DB.First(&msg, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "消息不存在")
		return
	}
	// Author can delete own; admins can delete any.
	if msg.SenderID != u.ID && !u.IsPrivileged() {
		fail(c, http.StatusForbidden, "forbidden", "无权删除该消息")
		return
	}
	h.DB.Model(&msg).Update("deleted", true)
	if msg.SenderID != u.ID {
		h.audit(u.ID, "message.delete", fmt.Sprintf("message:%d", id), "")
	}
	h.Hub.BroadcastMessageDelete(msg.ID, msg.ChannelID)
	c.Status(http.StatusNoContent)
}

type reactionReq struct {
	Emoji string `json:"emoji"`
}

func (h *H) ToggleReaction(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的消息 ID")
		return
	}
	var msg models.Message
	if err := h.DB.First(&msg, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "消息不存在")
		return
	}
	if msg.Deleted {
		fail(c, http.StatusBadRequest, "deleted", "消息已删除")
		return
	}
	var req reactionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	emoji := strings.TrimSpace(req.Emoji)
	if emoji == "" || utf8.RuneCountInString(emoji) > 8 {
		fail(c, http.StatusBadRequest, "bad_emoji", "无效的表情")
		return
	}
	var existing models.Reaction
	err := h.DB.Where("message_id = ? AND user_id = ? AND emoji = ?", id, u.ID, emoji).First(&existing).Error
	if err == nil {
		h.DB.Delete(&existing)
	} else {
		h.DB.Create(&models.Reaction{MessageID: id, UserID: u.ID, Emoji: emoji})
	}
	dto := view.BuildMessageDTO(h.DB, msg, u.ID)
	h.Hub.BroadcastReaction(msg.ID, msg.ChannelID, dto.Reactions)
	c.JSON(http.StatusOK, gin.H{"message_id": msg.ID, "reactions": dto.Reactions})
}
