package handlers

import (
	"net/http"

	"murmur/middleware"
	"murmur/models"
	"murmur/view"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *H) Mentions(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var mentions []models.Mention
	h.DB.Where("mentioned_user_id = ?", u.ID).Order("id DESC").Limit(100).Find(&mentions)

	var msgIDs []uint
	for _, m := range mentions {
		msgIDs = append(msgIDs, m.MessageID)
	}
	msgMap := map[uint]models.MessageDTO{}
	if len(msgIDs) > 0 {
		var msgs []models.Message
		h.DB.Where("id IN ?", msgIDs).Find(&msgs)
		for _, dto := range view.BuildMessageDTOs(h.DB, msgs, u.ID) {
			msgMap[dto.ID] = dto
		}
	}

	out := make([]models.MentionDTO, 0, len(mentions))
	for _, m := range mentions {
		dto := models.MentionDTO{
			ID:              m.ID,
			MessageID:       m.MessageID,
			ChannelID:       m.ChannelID,
			MentionedUserID: m.MentionedUserID,
			ReadAt:          m.ReadAt,
			CreatedAt:       m.CreatedAt,
		}
		if md, ok := msgMap[m.MessageID]; ok {
			mdCopy := md
			dto.Message = &mdCopy
		}
		out = append(out, dto)
	}
	c.JSON(http.StatusOK, out)
}

func (h *H) ReadMention(c *gin.Context) {
	u := middleware.CurrentUser(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的 ID")
		return
	}
	h.DB.Model(&models.Mention{}).
		Where("id = ? AND mentioned_user_id = ? AND read_at IS NULL", id, u.ID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
	c.Status(http.StatusNoContent)
}

func (h *H) ReadAllMentions(c *gin.Context) {
	u := middleware.CurrentUser(c)
	h.DB.Model(&models.Mention{}).
		Where("mentioned_user_id = ? AND read_at IS NULL", u.ID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
	c.Status(http.StatusNoContent)
}
