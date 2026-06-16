package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"murmur/middleware"
	"murmur/models"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

func (h *H) ListChannels(c *gin.Context) {
	var channels []models.Channel
	h.DB.Order("pinned DESC, id ASC").Find(&channels)
	c.JSON(http.StatusOK, channels)
}

func (h *H) ChannelMessages(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的频道 ID")
		return
	}
	limit := queryInt(c, "limit", 30)
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	before := queryInt(c, "before", 0)

	q := h.DB.Where("channel_id = ?", id)
	if before > 0 {
		q = q.Where("id < ?", before)
	}
	var rows []models.Message
	q.Order("id DESC").Limit(limit + 1).Find(&rows)

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// reverse to chronological
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	items := view.BuildMessageDTOs(h.DB, rows, u.ID)
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"has_more":  hasMore,
		"page":      0,
		"page_size": limit,
		"total":     0,
	})
}

func (h *H) SearchMessages(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的频道 ID")
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusOK, []models.MessageDTO{})
		return
	}
	var rows []models.Message
	h.DB.Where("channel_id = ? AND deleted = ? AND content LIKE ?", id, false, "%"+query+"%").
		Order("id DESC").Limit(50).Find(&rows)
	c.JSON(http.StatusOK, view.BuildMessageDTOs(h.DB, rows, u.ID))
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func (h *H) makeSlug(name string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("ch-%d", time.Now().Unix())
	}
	base := s
	for i := 1; ; i++ {
		var count int64
		h.DB.Model(&models.Channel{}).Where("slug = ?", s).Count(&count)
		if count == 0 {
			return s
		}
		s = fmt.Sprintf("%s-%d", base, i)
	}
}

type channelReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Readonly    *bool  `json:"readonly"`
	Pinned      *bool  `json:"pinned"`
}

func (h *H) CreateChannel(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "bad_name", "频道名不能为空")
		return
	}
	ch := models.Channel{
		Name:        req.Name,
		Slug:        h.makeSlug(req.Name),
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   u.ID,
	}
	if req.Readonly != nil {
		ch.Readonly = *req.Readonly
	}
	if req.Pinned != nil {
		ch.Pinned = *req.Pinned
	}
	if err := h.DB.Create(&ch).Error; err != nil {
		fail(c, http.StatusInternalServerError, "db", "创建频道失败")
		return
	}
	h.audit(u.ID, "channel.create", fmt.Sprintf("channel:%d", ch.ID), ch.Name)
	c.JSON(http.StatusOK, ch)
}

func (h *H) UpdateChannel(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的频道 ID")
		return
	}
	var ch models.Channel
	if err := h.DB.First(&ch, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "频道不存在")
		return
	}
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	updates := map[string]any{}
	if strings.TrimSpace(req.Name) != "" {
		updates["name"] = strings.TrimSpace(req.Name)
	}
	updates["description"] = strings.TrimSpace(req.Description)
	if req.Readonly != nil {
		updates["readonly"] = *req.Readonly
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	h.DB.Model(&ch).Updates(updates)
	h.DB.First(&ch, id)
	h.audit(u.ID, "channel.update", fmt.Sprintf("channel:%d", id), ch.Name)
	c.JSON(http.StatusOK, ch)
}

func (h *H) DeleteChannel(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的频道 ID")
		return
	}
	var ch models.Channel
	if err := h.DB.First(&ch, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "频道不存在")
		return
	}
	var total int64
	h.DB.Model(&models.Channel{}).Count(&total)
	if total <= 1 {
		fail(c, http.StatusBadRequest, "last_channel", "至少保留一个频道")
		return
	}
	// Clean up dependent rows.
	var msgIDs []uint
	h.DB.Model(&models.Message{}).Where("channel_id = ?", id).Pluck("id", &msgIDs)
	if len(msgIDs) > 0 {
		h.DB.Where("message_id IN ?", msgIDs).Delete(&models.Reaction{})
	}
	h.DB.Where("channel_id = ?", id).Delete(&models.Mention{})
	h.DB.Where("channel_id = ?", id).Delete(&models.Message{})
	h.DB.Delete(&ch)
	h.audit(u.ID, "channel.delete", fmt.Sprintf("channel:%d", id), ch.Name)
	c.Status(http.StatusNoContent)
}
