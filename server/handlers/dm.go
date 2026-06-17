package handlers

import (
	"net/http"

	"murmur/middleware"
	"murmur/models"
	"murmur/view"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *H) Conversations(c *gin.Context) {
	u := middleware.CurrentUser(c)

	var dms []models.DirectMessage
	h.DB.Where("sender_id = ? OR receiver_id = ?", u.ID, u.ID).
		Order("id DESC").Limit(5000).Find(&dms)

	type agg struct {
		last   models.DirectMessage
		unread int
	}
	order := []uint{}
	byPartner := map[uint]*agg{}
	for _, dm := range dms {
		partner := dm.SenderID
		if dm.SenderID == u.ID {
			partner = dm.ReceiverID
		}
		a, ok := byPartner[partner]
		if !ok {
			a = &agg{last: dm}
			byPartner[partner] = a
			order = append(order, partner)
		}
		if dm.ReceiverID == u.ID && dm.ReadAt == nil {
			a.unread++
		}
	}

	// load partner users
	var ids []uint
	for id := range byPartner {
		ids = append(ids, id)
	}
	users := map[uint]models.User{}
	if len(ids) > 0 {
		var us []models.User
		h.DB.Where("id IN ?", ids).Find(&us)
		for _, usr := range us {
			users[usr.ID] = usr
		}
	}

	out := make([]models.ConversationDTO, 0, len(order))
	for _, pid := range order {
		a := byPartner[pid]
		usr, ok := users[pid]
		if !ok {
			continue
		}
		last := view.BuildDMDTO(h.DB, a.last)
		out = append(out, models.ConversationDTO{
			User:        view.PublicUser(usr),
			LastMessage: &last,
			Unread:      a.unread,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *H) DMMessages(c *gin.Context) {
	u := middleware.CurrentUser(c)
	other, ok := parseUintParam(c, "userId")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	limit := queryInt(c, "limit", 30)
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	before := queryInt(c, "before", 0)

	q := h.DB.Where(
		"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		u.ID, other, other, u.ID,
	)
	if before > 0 {
		q = q.Where("id < ?", before)
	}
	var rows []models.DirectMessage
	q.Order("id DESC").Limit(limit + 1).Find(&rows)

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

	// Mark partner's messages as read now that they're being viewed.
	h.DB.Model(&models.DirectMessage{}).
		Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL", other, u.ID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))

	items := make([]models.DirectMessageDTO, 0, len(rows))
	for _, dm := range rows {
		items = append(items, view.BuildDMDTO(h.DB, dm))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"has_more":  hasMore,
		"page":      0,
		"page_size": limit,
		"total":     0,
	})
}

type sendDMReq struct {
	Content string `json:"content"`
	ReplyTo uint   `json:"reply_to"`
}

func (h *H) SendDM(c *gin.Context) {
	u := middleware.CurrentUser(c)
	other, ok := parseUintParam(c, "userId")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var req sendDMReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "参数错误")
		return
	}
	dto, e := h.Hub.PostDirectMessage(u, other, req.Content, "", req.ReplyTo)
	if e != nil {
		failErr(c, e)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *H) MarkDMRead(c *gin.Context) {
	u := middleware.CurrentUser(c)
	other, ok := parseUintParam(c, "userId")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	h.DB.Model(&models.DirectMessage{}).
		Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL", other, u.ID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
	c.Status(http.StatusNoContent)
}
