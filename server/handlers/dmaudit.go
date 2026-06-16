package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"murmur/models"
	"murmur/view"

	"github.com/gin-gonic/gin"
)

func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// AdminDMConversations lists every direct-message pair for super-admin review.
// Bot conversations are excluded.
func (h *H) AdminDMConversations(c *gin.Context) {
	var botID uint
	var bot models.User
	if h.DB.Where("role = ?", models.RoleBot).First(&bot).Error == nil {
		botID = bot.ID
	}

	var dms []models.DirectMessage
	h.DB.Order("id DESC").Limit(20000).Find(&dms)

	type pair struct {
		a, b     uint
		lastAt   time.Time
		lastText string
		recalled bool
		count    int
	}
	order := []string{}
	byPair := map[string]*pair{}
	ids := map[uint]bool{}
	for _, dm := range dms {
		a, b := dm.SenderID, dm.ReceiverID
		if a == botID || b == botID {
			continue
		}
		if a > b {
			a, b = b, a
		}
		key := fmt.Sprintf("%d-%d", a, b)
		p, ok := byPair[key]
		if !ok {
			p = &pair{a: a, b: b, lastAt: dm.CreatedAt, lastText: dm.Content, recalled: dm.Recalled}
			byPair[key] = p
			order = append(order, key)
			ids[a] = true
			ids[b] = true
		}
		p.count++
	}

	idList := make([]uint, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	users := map[uint]models.User{}
	if len(idList) > 0 {
		var us []models.User
		h.DB.Where("id IN ?", idList).Find(&us)
		for _, u := range us {
			users[u.ID] = u
		}
	}

	out := make([]gin.H, 0, len(order))
	for _, key := range order {
		p := byPair[key]
		ua, oka := users[p.a]
		ub, okb := users[p.b]
		if !oka || !okb {
			continue
		}
		preview := truncRunes(p.lastText, 50)
		if p.recalled {
			preview = "[已撤回]"
		}
		out = append(out, gin.H{
			"user_a":  view.PublicUser(ua),
			"user_b":  view.PublicUser(ub),
			"last_at": p.lastAt,
			"preview": preview,
			"count":   p.count,
		})
	}
	c.JSON(http.StatusOK, out)
}

// AdminDMThread returns the full conversation between two users, including
// recalled content (super-admin review).
func (h *H) AdminDMThread(c *gin.Context) {
	a, _ := strconv.ParseUint(c.Query("a"), 10, 64)
	b, _ := strconv.ParseUint(c.Query("b"), 10, 64)
	if a == 0 || b == 0 {
		fail(c, http.StatusBadRequest, "bad_id", "无效的用户")
		return
	}
	var ua, ub models.User
	if h.DB.First(&ua, a).Error != nil || h.DB.First(&ub, b).Error != nil {
		fail(c, http.StatusNotFound, "not_found", "用户不存在")
		return
	}

	var rows []models.DirectMessage
	h.DB.Where(
		"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		a, b, b, a,
	).Order("id ASC").Limit(2000).Find(&rows)

	items := make([]gin.H, 0, len(rows))
	for _, dm := range rows {
		items = append(items, gin.H{
			"id":          dm.ID,
			"sender_id":   dm.SenderID,
			"receiver_id": dm.ReceiverID,
			"content":     dm.Content, // raw, including recalled (super-admin only)
			"recalled":    dm.Recalled,
			"recalled_by": dm.RecalledBy,
			"created_at":  dm.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"user_a": view.PublicUser(ua),
		"user_b": view.PublicUser(ub),
		"items":  items,
	})
}
