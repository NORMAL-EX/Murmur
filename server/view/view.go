// Package view builds JSON DTOs from models and parses @mentions. It is shared
// by the REST handlers and the WebSocket hub.
package view

import (
	"regexp"
	"strings"

	"murmur/models"

	"gorm.io/gorm"
)

var mentionRe = regexp.MustCompile(`@([A-Za-z0-9_]{1,32})`)
var imgMdRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// previewText makes a short single-line preview, collapsing image markdown.
func previewText(s string, n int) string {
	s = strings.TrimSpace(imgMdRe.ReplaceAllString(s, "[图片]"))
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func displayName(u *models.User) string {
	if u == nil {
		return "用户"
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}

// ExtractMentionUsernames returns unique, lower-cased usernames referenced with
// @ in the content.
func ExtractMentionUsernames(content string) []string {
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// ResolveUsers returns the user records whose (lower-cased) username matches one
// of the given names.
func ResolveUsers(db *gorm.DB, names []string) []models.User {
	if len(names) == 0 {
		return nil
	}
	var users []models.User
	db.Where("LOWER(username) IN ?", names).Find(&users)
	return users
}

func cleanUser(u models.User) models.User {
	u.PasswordHash = ""
	u.Email = "" // email is private; only self & admins see it (via FullUser)
	return u
}

func loadUsers(db *gorm.DB, ids []uint) map[uint]*models.User {
	out := map[uint]*models.User{}
	if len(ids) == 0 {
		return out
	}
	var users []models.User
	db.Where("id IN ?", ids).Find(&users)
	for i := range users {
		u := cleanUser(users[i])
		out[u.ID] = &u
	}
	return out
}

func uniqueIDs(ids []uint) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// BuildMessageDTOs assembles full DTOs for a batch of messages, resolving
// senders, reactions and mentions with a handful of batched queries.
func BuildMessageDTOs(db *gorm.DB, msgs []models.Message, viewerID uint) []models.MessageDTO {
	if len(msgs) == 0 {
		return []models.MessageDTO{}
	}
	var msgIDs, senderIDs []uint
	for _, m := range msgs {
		msgIDs = append(msgIDs, m.ID)
		senderIDs = append(senderIDs, m.SenderID)
	}
	senders := loadUsers(db, uniqueIDs(senderIDs))

	// reactions
	var reactions []models.Reaction
	db.Where("message_id IN ?", msgIDs).Find(&reactions)
	type reactKey struct {
		mid   uint
		emoji string
	}
	reactMap := map[reactKey]*models.ReactionDTO{}
	order := map[uint][]string{}
	for _, r := range reactions {
		k := reactKey{r.MessageID, r.Emoji}
		dto, ok := reactMap[k]
		if !ok {
			dto = &models.ReactionDTO{Emoji: r.Emoji}
			reactMap[k] = dto
			order[r.MessageID] = append(order[r.MessageID], r.Emoji)
		}
		dto.Count++
		dto.UserIDs = append(dto.UserIDs, r.UserID)
		if r.UserID == viewerID {
			dto.Reacted = true
		}
	}

	// mentions
	var mentions []models.Mention
	db.Where("message_id IN ?", msgIDs).Find(&mentions)
	mentionMap := map[uint][]uint{}
	for _, mn := range mentions {
		mentionMap[mn.MessageID] = append(mentionMap[mn.MessageID], mn.MentionedUserID)
	}

	// reply previews
	var replyIDs []uint
	for _, m := range msgs {
		if m.ReplyToID != nil {
			replyIDs = append(replyIDs, *m.ReplyToID)
		}
	}
	replyMap := map[uint]*models.ReplyPreview{}
	if len(replyIDs) > 0 {
		var replied []models.Message
		db.Where("id IN ?", uniqueIDs(replyIDs)).Find(&replied)
		var rsIDs []uint
		for _, r := range replied {
			rsIDs = append(rsIDs, r.SenderID)
		}
		rs := loadUsers(db, uniqueIDs(rsIDs))
		for _, r := range replied {
			gone := r.Recalled || r.Deleted
			content := r.Content
			if gone {
				content = ""
			}
			replyMap[r.ID] = &models.ReplyPreview{
				ID: r.ID, SenderID: r.SenderID, SenderName: displayName(rs[r.SenderID]),
				Content: previewText(content, 80), Recalled: gone,
			}
		}
	}

	out := make([]models.MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dto := models.MessageDTO{
			ID:         m.ID,
			ChannelID:  m.ChannelID,
			SenderID:   m.SenderID,
			Sender:     senders[m.SenderID],
			Content:    m.Content,
			Edited:     m.Edited,
			Deleted:    m.Deleted,
			Recalled:   m.Recalled,
			RecalledBy: m.RecalledBy,
			IsBot:      m.IsBot,
			Mentions:   mentionMap[m.ID],
			Reactions:  []models.ReactionDTO{},
			CreatedAt:  m.CreatedAt,
		}
		if m.ReplyToID != nil {
			dto.ReplyTo = replyMap[*m.ReplyToID]
		}
		// Recalled/deleted content is never sent over the wire; super admins
		// fetch the original on demand via the admin reveal endpoint.
		if m.Deleted || m.Recalled {
			dto.Content = ""
		}
		for _, emoji := range order[m.ID] {
			dto.Reactions = append(dto.Reactions, *reactMap[reactKey{m.ID, emoji}])
		}
		if dto.Mentions == nil {
			dto.Mentions = []uint{}
		}
		out = append(out, dto)
	}
	return out
}

// BuildMessageDTO is the single-message convenience wrapper.
func BuildMessageDTO(db *gorm.DB, m models.Message, viewerID uint) models.MessageDTO {
	dtos := BuildMessageDTOs(db, []models.Message{m}, viewerID)
	return dtos[0]
}

func BuildDMDTO(db *gorm.DB, dm models.DirectMessage) models.DirectMessageDTO {
	senders := loadUsers(db, []uint{dm.SenderID})
	content := dm.Content
	if dm.Recalled {
		content = ""
	}
	var reply *models.ReplyPreview
	if dm.ReplyToID != nil {
		var r models.DirectMessage
		if db.First(&r, *dm.ReplyToID).Error == nil {
			ru := loadUsers(db, []uint{r.SenderID})[r.SenderID]
			c := r.Content
			if r.Recalled {
				c = ""
			}
			reply = &models.ReplyPreview{
				ID: r.ID, SenderID: r.SenderID, SenderName: displayName(ru),
				Content: previewText(c, 80), Recalled: r.Recalled,
			}
		}
	}
	return models.DirectMessageDTO{
		ID:         dm.ID,
		SenderID:   dm.SenderID,
		ReceiverID: dm.ReceiverID,
		Sender:     senders[dm.SenderID],
		ReplyTo:    reply,
		Content:    content,
		ReadAt:     dm.ReadAt,
		Recalled:   dm.Recalled,
		RecalledBy: dm.RecalledBy,
		CreatedAt:  dm.CreatedAt,
	}
}

// PublicUser strips sensitive fields (password, email) for API responses shown
// to other users.
func PublicUser(u models.User) models.User {
	return cleanUser(u)
}

// FullUser strips only the password hash, keeping the email. Use for the user
// themselves and for admin views.
func FullUser(u models.User) models.User {
	u.PasswordHash = ""
	return u
}
