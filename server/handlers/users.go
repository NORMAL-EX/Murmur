package handlers

import (
	"net/http"
	"strings"

	"murmur/models"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

// ListUsers powers @mention autocomplete and member lists.
func (h *H) ListUsers(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	db := h.DB.Model(&models.User{}).
		Where("status = ? OR role = ?", models.StatusActive, models.RoleBot)
	if q != "" {
		like := "%" + q + "%"
		db = db.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}
	var users []models.User
	db.Order("role = 'bot' DESC, username ASC").Limit(50).Find(&users)
	out := make([]models.User, 0, len(users))
	for _, u := range users {
		out = append(out, view.PublicUser(u))
	}
	c.JSON(http.StatusOK, out)
}

func (h *H) GetUser(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户 ID")
		return
	}
	var u models.User
	if err := h.DB.First(&u, id).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	c.JSON(http.StatusOK, view.PublicUser(u))
}

func (h *H) PublicSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.St.PublicMap())
}
