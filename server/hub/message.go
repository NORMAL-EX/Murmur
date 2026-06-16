package hub

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"murmur/ai"
	"murmur/models"
	"murmur/settings"
	"murmur/view"
)

func (h *Hub) reloadUser(id uint) *models.User {
	var u models.User
	if err := h.db.First(&u, id).Error; err != nil {
		return nil
	}
	return &u
}

func (h *Hub) effectiveLimit(u *models.User) (int, time.Duration) {
	switch {
	case u.RateLimitPerMin == models.RateUnlimited: // 0 => unlimited
		return 0, time.Minute
	case u.RateLimitPerMin > 0:
		return u.RateLimitPerMin, time.Minute
	default: // inherit global/role default
		win := time.Duration(h.st.GetInt(settings.RateLimitWindow)) * time.Second
		if win <= 0 {
			win = 30 * time.Second
		}
		if u.IsPrivileged() {
			return h.st.GetInt(settings.RateLimitAdmin), win
		}
		return h.st.GetInt(settings.RateLimitMessages), win
	}
}

func (h *Hub) checkRate(u *models.User) *Err {
	limit, win := h.effectiveLimit(u)
	ok, retry := h.rl.Allow(u.ID, limit, win)
	if !ok {
		return &Err{Status: http.StatusTooManyRequests, Code: "rate_limited",
			Message: "发送过于频繁,请稍候再试", RetryAfter: retry}
	}
	return nil
}

func (h *Hub) maxLen() int {
	n := h.st.GetInt(settings.MaxMessageLength)
	if n <= 0 {
		n = 2000
	}
	return n
}

// PostChannelMessage validates, persists, broadcasts and (if needed) triggers
// the AI bot. Shared by the WS read loop and any REST caller.
func (h *Hub) PostChannelMessage(sender *models.User, channelID uint, content, tempID string) (*models.MessageDTO, *Err) {
	u := h.reloadUser(sender.ID)
	if u == nil {
		return nil, newErr(http.StatusUnauthorized, "unauthorized", "用户不存在")
	}
	if u.Status == models.StatusBanned {
		return nil, newErr(http.StatusForbidden, "banned", "账号已被封禁")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, newErr(http.StatusBadRequest, "empty", "消息不能为空")
	}
	if utf8.RuneCountInString(content) > h.maxLen() {
		return nil, newErr(http.StatusBadRequest, "too_long", "消息超过长度限制")
	}

	var ch models.Channel
	if err := h.db.First(&ch, channelID).Error; err != nil {
		return nil, newErr(http.StatusNotFound, "no_channel", "频道不存在")
	}
	if ch.Readonly && !u.IsPrivileged() {
		return nil, newErr(http.StatusForbidden, "readonly", "该频道为只读")
	}
	if e := h.checkRate(u); e != nil {
		return nil, e
	}

	msg := models.Message{ChannelID: channelID, SenderID: u.ID, Content: content}
	if err := h.db.Create(&msg).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "db", "保存消息失败")
	}

	// Resolve @mentions and persist notifications.
	names := view.ExtractMentionUsernames(content)
	resolved := view.ResolveUsers(h.db, names)
	botMentioned := false
	for _, m := range resolved {
		if m.Role == models.RoleBot {
			botMentioned = true
			continue
		}
		if m.ID == u.ID || m.Status != models.StatusActive {
			continue
		}
		mention := models.Mention{MessageID: msg.ID, ChannelID: channelID, MentionedUserID: m.ID}
		h.db.Create(&mention)
	}
	if botName := strings.ToLower(strings.TrimSpace(h.st.Get(settings.BotName))); botName != "" {
		if strings.Contains(strings.ToLower(content), "@"+botName) {
			botMentioned = true
		}
	}

	dto := view.BuildMessageDTO(h.db, msg, u.ID)

	// Broadcast to everyone (temp_id lets the sender reconcile its optimistic copy).
	fields := map[string]any{"message": dto}
	if tempID != "" {
		fields["temp_id"] = tempID
	}
	h.broadcastAll(envelope("chat_message", fields))

	// Push mention notifications.
	for _, m := range resolved {
		if m.Role == models.RoleBot || m.ID == u.ID || m.Status != models.StatusActive {
			continue
		}
		h.notifyMention(m.ID, msg.ID, channelID, dto)
	}

	if botMentioned {
		go h.handleBotChannel(channelID)
	}

	return &dto, nil
}

func (h *Hub) notifyMention(userID, messageID, channelID uint, msg models.MessageDTO) {
	var mn models.Mention
	h.db.Where("message_id = ? AND mentioned_user_id = ?", messageID, userID).First(&mn)
	dto := models.MentionDTO{
		ID:              mn.ID,
		MessageID:       messageID,
		ChannelID:       channelID,
		MentionedUserID: userID,
		CreatedAt:       mn.CreatedAt,
		Message:         &msg,
	}
	h.sendToUser(userID, envelope("mention", map[string]any{"mention": dto}))
}

// PostDirectMessage validates, persists and delivers a 1:1 message.
func (h *Hub) PostDirectMessage(sender *models.User, toID uint, content, tempID string) (*models.DirectMessageDTO, *Err) {
	u := h.reloadUser(sender.ID)
	if u == nil {
		return nil, newErr(http.StatusUnauthorized, "unauthorized", "用户不存在")
	}
	if u.Status == models.StatusBanned {
		return nil, newErr(http.StatusForbidden, "banned", "账号已被封禁")
	}
	if !h.st.GetBool(settings.AllowDM) && !u.IsPrivileged() {
		return nil, newErr(http.StatusForbidden, "dm_disabled", "私信功能已关闭")
	}
	if toID == u.ID {
		return nil, newErr(http.StatusBadRequest, "self", "不能给自己发私信")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, newErr(http.StatusBadRequest, "empty", "消息不能为空")
	}
	if utf8.RuneCountInString(content) > h.maxLen() {
		return nil, newErr(http.StatusBadRequest, "too_long", "消息超过长度限制")
	}

	recipient := h.reloadUser(toID)
	if recipient == nil {
		return nil, newErr(http.StatusNotFound, "no_user", "对方不存在")
	}
	if e := h.checkRate(u); e != nil {
		return nil, e
	}

	dm := models.DirectMessage{SenderID: u.ID, ReceiverID: toID, Content: content}
	if err := h.db.Create(&dm).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "db", "保存私信失败")
	}
	dto := view.BuildDMDTO(h.db, dm)

	fields := map[string]any{"message": dto}
	if tempID != "" {
		fields["temp_id"] = tempID
	}
	h.sendToUser(u.ID, envelope("dm_message", fields))
	if toID != u.ID {
		h.sendToUser(toID, envelope("dm_message", map[string]any{"message": dto}))
	}

	// Bot direct messages.
	if recipient.Role == models.RoleBot {
		if !h.ai.Enabled() || !h.ai.HasKey() || !h.ai.AllowDM() {
			go h.postBotDM(u.ID, "🤖 机器人私信未开启。请管理员在后台「AI 设置」中开启 AI 及私信支持。")
		} else {
			go h.handleBotDM(u.ID)
		}
	}

	return &dto, nil
}

// ---- broadcast helpers used by REST handlers after DB mutations ----

func (h *Hub) BroadcastMessageUpdate(dto models.MessageDTO) {
	h.broadcastAll(envelope("message_update", map[string]any{"message": dto}))
}

func (h *Hub) BroadcastMessageDelete(messageID, channelID uint) {
	h.broadcastAll(envelope("message_delete", map[string]any{
		"message_id": messageID, "channel_id": channelID,
	}))
}

func (h *Hub) BroadcastReaction(messageID, channelID uint, reactions []models.ReactionDTO) {
	h.broadcastAll(envelope("reaction", map[string]any{
		"message_id": messageID, "channel_id": channelID, "reactions": reactions,
	}))
}

// ---- AI bot ----

func (h *Hub) botCooldownOK() bool {
	cd := h.st.GetInt(settings.AICooldown)
	if cd <= 0 {
		return true
	}
	ok, _ := h.rl.Allow(h.botID, 1, time.Duration(cd)*time.Second)
	return ok
}

func (h *Hub) handleBotChannel(channelID uint) {
	if !h.botCooldownOK() {
		return
	}
	if !h.ai.Enabled() || !h.ai.HasKey() {
		h.postBotMessage(channelID, "🤖 机器人未启用。请管理员在后台「AI 设置」中开启并填写 API Key。")
		return
	}
	limit := h.st.GetInt(settings.AIContextLimit)
	if limit <= 0 {
		limit = 5000
	}
	hist := h.buildChannelContext(channelID, limit)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := h.ai.Complete(ctx, hist)
	if err != nil {
		log.Printf("[ai] channel %d error: %v", channelID, err)
		reply = "🤖 抱歉,我暂时无法回复:" + err.Error()
	}
	h.postBotMessage(channelID, reply)
}

func (h *Hub) handleBotDM(userID uint) {
	if !h.botCooldownOK() {
		return
	}
	limit := h.st.GetInt(settings.AIContextLimit)
	if limit <= 0 {
		limit = 5000
	}
	hist := h.buildDMContext(userID, limit)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := h.ai.Complete(ctx, hist)
	if err != nil {
		log.Printf("[ai] dm %d error: %v", userID, err)
		reply = "🤖 抱歉,我暂时无法回复:" + err.Error()
	}
	h.postBotDM(userID, reply)
}

func (h *Hub) postBotMessage(channelID uint, content string) {
	msg := models.Message{ChannelID: channelID, SenderID: h.botID, Content: content, IsBot: true}
	if err := h.db.Create(&msg).Error; err != nil {
		log.Printf("[ai] save bot message failed: %v", err)
		return
	}
	dto := view.BuildMessageDTO(h.db, msg, 0)
	h.broadcastAll(envelope("chat_message", map[string]any{"message": dto}))
}

func (h *Hub) postBotDM(toUserID uint, content string) {
	dm := models.DirectMessage{SenderID: h.botID, ReceiverID: toUserID, Content: content}
	if err := h.db.Create(&dm).Error; err != nil {
		log.Printf("[ai] save bot dm failed: %v", err)
		return
	}
	dto := view.BuildDMDTO(h.db, dm)
	h.sendToUser(toUserID, envelope("dm_message", map[string]any{"message": dto}))
}

func (h *Hub) buildChannelContext(channelID uint, runeLimit int) []ai.Message {
	var msgs []models.Message
	h.db.Where("channel_id = ? AND deleted = ?", channelID, false).
		Order("id DESC").Limit(300).Find(&msgs)

	var picked []models.Message
	total := 0
	for _, m := range msgs { // newest first
		rc := utf8.RuneCountInString(m.Content)
		if len(picked) > 0 && total+rc > runeLimit {
			break
		}
		picked = append(picked, m)
		total += rc
		if total >= runeLimit {
			break
		}
	}

	// Resolve sender display names.
	idset := map[uint]bool{}
	for _, m := range picked {
		idset[m.SenderID] = true
	}
	var ids []uint
	for id := range idset {
		ids = append(ids, id)
	}
	var users []models.User
	if len(ids) > 0 {
		h.db.Where("id IN ?", ids).Find(&users)
	}
	names := map[uint]string{}
	for _, usr := range users {
		n := usr.Nickname
		if n == "" {
			n = usr.Username
		}
		names[usr.ID] = n
	}

	hist := make([]ai.Message, 0, len(picked))
	for i := len(picked) - 1; i >= 0; i-- { // chronological
		m := picked[i]
		if m.IsBot {
			hist = append(hist, ai.Message{Role: "assistant", Content: m.Content})
			continue
		}
		name := names[m.SenderID]
		if name == "" {
			name = "用户"
		}
		hist = append(hist, ai.Message{Role: "user", Content: name + ": " + m.Content})
	}
	return hist
}

func (h *Hub) buildDMContext(userID uint, runeLimit int) []ai.Message {
	var msgs []models.DirectMessage
	h.db.Where(
		"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
		userID, h.botID, h.botID, userID,
	).Order("id DESC").Limit(300).Find(&msgs)

	var picked []models.DirectMessage
	total := 0
	for _, m := range msgs {
		rc := utf8.RuneCountInString(m.Content)
		if len(picked) > 0 && total+rc > runeLimit {
			break
		}
		picked = append(picked, m)
		total += rc
		if total >= runeLimit {
			break
		}
	}

	hist := make([]ai.Message, 0, len(picked))
	for i := len(picked) - 1; i >= 0; i-- {
		m := picked[i]
		if m.SenderID == h.botID {
			hist = append(hist, ai.Message{Role: "assistant", Content: m.Content})
		} else {
			hist = append(hist, ai.Message{Role: "user", Content: m.Content})
		}
	}
	return hist
}
