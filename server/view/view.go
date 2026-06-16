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

	out := make([]models.MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dto := models.MessageDTO{
			ID:        m.ID,
			ChannelID: m.ChannelID,
			SenderID:  m.SenderID,
			Sender:    senders[m.SenderID],
			Content:   m.Content,
			Edited:    m.Edited,
			Deleted:   m.Deleted,
			IsBot:     m.IsBot,
			Mentions:  mentionMap[m.ID],
			Reactions: []models.ReactionDTO{},
			CreatedAt: m.CreatedAt,
		}
		if m.Deleted {
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
	return models.DirectMessageDTO{
		ID:         dm.ID,
		SenderID:   dm.SenderID,
		ReceiverID: dm.ReceiverID,
		Sender:     senders[dm.SenderID],
		Content:    dm.Content,
		ReadAt:     dm.ReadAt,
		CreatedAt:  dm.CreatedAt,
	}
}

// PublicUser strips sensitive fields from a user for API responses.
func PublicUser(u models.User) models.User {
	return cleanUser(u)
}
