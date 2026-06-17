package hub

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"murmur/ai"
	"murmur/models"
)

// aiMaxMuteMinutes caps AI-issued mutes at 60 days.
const aiMaxMuteMinutes = 60 * 24 * 60

var beijingLoc = time.FixedZone("CST", 8*3600)

// botTools returns the tool set offered to the model. Moderation tools are
// included where a requester is known (channel @mention or direct message).
func botTools(includeMod bool) []ai.ToolDef {
	tools := []ai.ToolDef{timeToolDef(), webSearchToolDef()}
	if includeMod {
		tools = append(tools,
			muteToolDef(), recallToolDef(), banToolDef(), setRoleToolDef(), setNickToolDef())
	}
	return tools
}

func objParams(props map[string]any, required ...string) map[string]any {
	p := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		p["required"] = required
	}
	return p
}

func timeToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "get_current_time",
		Description: "获取当前北京时间(Asia/Shanghai,UTC+8)。",
		Parameters:  objParams(map[string]any{}),
	}}
}

func webSearchToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "web_search",
		Description: "使用 Bing 进行联网搜索,返回前几条结果的标题、摘要与链接。",
		Parameters: objParams(map[string]any{
			"query": map[string]any{"type": "string", "description": "搜索关键词"},
		}, "query"),
	}}
}

func muteToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "mute_user",
		Description: "禁言指定成员一段时间(最长 86400 分钟 = 60 天)。",
		Parameters: objParams(map[string]any{
			"username": map[string]any{"type": "string", "description": "成员用户名(不带 @)"},
			"minutes":  map[string]any{"type": "integer", "description": "禁言分钟数,最长 86400"},
			"reason":   map[string]any{"type": "string", "description": "原因(可选)"},
		}, "username", "minutes"),
	}}
}

func recallToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "recall_message",
		Description: "撤回一条频道消息:可传 message_id 精确撤回,或传 username 撤回该成员最近一条消息。",
		Parameters: objParams(map[string]any{
			"message_id": map[string]any{"type": "integer", "description": "要撤回的消息 ID(优先)"},
			"username":   map[string]any{"type": "string", "description": "成员用户名(撤回其最近一条)"},
		}),
	}}
}

func banToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "ban_user",
		Description: "封禁某成员(封禁后无法登录)。",
		Parameters: objParams(map[string]any{
			"username": map[string]any{"type": "string", "description": "成员用户名(不带 @)"},
			"reason":   map[string]any{"type": "string", "description": "原因(可选)"},
		}, "username"),
	}}
}

func setRoleToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "set_role",
		Description: "设置成员身份(仅系统管理员可用):user=普通成员,admin=管理员。",
		Parameters: objParams(map[string]any{
			"username": map[string]any{"type": "string", "description": "成员用户名(不带 @)"},
			"role":     map[string]any{"type": "string", "enum": []string{"user", "admin"}, "description": "user 或 admin"},
		}, "username", "role"),
	}}
}

func setNickToolDef() ai.ToolDef {
	return ai.ToolDef{Type: "function", Function: ai.ToolFunctionDef{
		Name:        "set_nickname",
		Description: "修改成员的群昵称。",
		Parameters: objParams(map[string]any{
			"username": map[string]any{"type": "string", "description": "成员用户名(不带 @)"},
			"nickname": map[string]any{"type": "string", "description": "新的群昵称"},
		}, "username", "nickname"),
	}}
}

// botToolsPrompt tells the model who is asking and the moderation rules.
func botToolsPrompt(requester *models.User) string {
	role, name := "普通成员", "未知"
	if requester != nil {
		name = requester.Username
		switch requester.Role {
		case models.RoleSuperAdmin:
			role = "系统管理员"
		case models.RoleAdmin:
			role = "管理员"
		}
	}
	return fmt.Sprintf(`你是群管理助手。可用工具:get_current_time、web_search、mute_user、recall_message、ban_user、set_role、set_nickname。
当前请求者:@%s(身份:%s)。管理类操作权限(必须遵守):
- 系统管理员:可对「管理员」「普通成员」执行 禁言/撤回/封禁/改昵称;set_role(设身份)仅系统管理员可用;不可操作系统管理员或机器人。
- 管理员:只能对「普通成员」执行 禁言/撤回/封禁/改昵称;不能设身份。
- 其他身份:无权管理,请礼貌拒绝且不要调用管理类工具。
禁言最长 86400 分钟(60 天)。时间/搜索任何人可用。调用工具后用中文简要说明结果。`, name, role)
}

// replyContext describes the message the requester is replying to, so the model
// can act on "this message" / "that person".
func (h *Hub) replyContext(msgID uint) string {
	var m models.Message
	if h.db.First(&m, msgID).Error != nil {
		return ""
	}
	sender := h.reloadUser(m.SenderID)
	uname := ""
	if sender != nil {
		uname = sender.Username
	}
	content := m.Content
	if m.Recalled || m.Deleted {
		content = "[已撤回/删除]"
	}
	content = strings.Join(strings.Fields(content), " ")
	if r := []rune(content); len(r) > 100 {
		content = string(r[:100]) + "…"
	}
	return fmt.Sprintf(`【回复上下文】请求者正在回复 @%s 的消息(message_id=%d):「%s」。若说"撤回这条"用 recall_message(message_id=%d);若说"那个人/他/她"指的就是 @%s。`,
		uname, msgID, content, msgID, uname)
}

// execBotTool dispatches a tool call requested by the model.
func (h *Hub) execBotTool(requester *models.User, name, argsJSON string) string {
	switch name {
	case "get_current_time":
		return execTime()
	case "web_search":
		var a struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		return execWebSearch(a.Query)
	case "mute_user":
		return h.execMute(requester, argsJSON)
	case "recall_message":
		return h.execRecall(requester, argsJSON)
	case "ban_user":
		return h.execBan(requester, argsJSON)
	case "set_role":
		return h.execSetRole(requester, argsJSON)
	case "set_nickname":
		return h.execSetNick(requester, argsJSON)
	default:
		return "未知工具:" + name
	}
}

func execTime() string {
	now := time.Now().In(beijingLoc)
	weekdays := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return fmt.Sprintf("当前北京时间:%s %s", now.Format("2006-01-02 15:04:05"), weekdays[int(now.Weekday())])
}

var (
	bingItemRe    = regexp.MustCompile(`(?s)<li class="b_algo".*?<h2>.*?<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?</h2>(.*?)</li>`)
	bingSnippetRe = regexp.MustCompile(`(?s)<p[^>]*>(.*?)</p>`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
)

func stripHTML(s string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTagRe.ReplaceAllString(s, "")))
}

// execWebSearch scrapes Bing's HTML results (no API key needed).
func execWebSearch(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "搜索词为空。"
	}
	endpoint := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&setmkt=zh-CN"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "搜索失败:" + err.Error()
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "搜索失败:" + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 3<<20))

	matches := bingItemRe.FindAllStringSubmatch(string(body), -1)
	var sb strings.Builder
	fmt.Fprintf(&sb, "「%s」的 Bing 搜索结果:\n", query)
	n := 0
	for _, m := range matches {
		if n >= 5 {
			break
		}
		title := stripHTML(m[2])
		if title == "" {
			continue
		}
		snippet := ""
		if sm := bingSnippetRe.FindStringSubmatch(m[3]); sm != nil {
			snippet = stripHTML(sm[1])
		}
		n++
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n", n, title, snippet, m[1])
	}
	if n == 0 {
		return "未找到结果(可能是网络受限或页面结构变化)。"
	}
	return sb.String()
}

// ---- moderation helpers ----

func (h *Hub) findUserByName(uname string) *models.User {
	uname = strings.TrimPrefix(strings.TrimSpace(uname), "@")
	if uname == "" {
		return nil
	}
	var u models.User
	if h.db.Where("LOWER(username) = ?", strings.ToLower(uname)).First(&u).Error != nil {
		return nil
	}
	return &u
}

// canModerate returns "" if requester may moderate target, else a refusal reason.
func canModerate(requester, target *models.User) string {
	if requester == nil || (requester.Role != models.RoleSuperAdmin && requester.Role != models.RoleAdmin) {
		return "拒绝:你无权执行该操作。"
	}
	if target.Role == models.RoleSuperAdmin || target.Role == models.RoleBot {
		return "拒绝:不能对该账号操作。"
	}
	if requester.Role == models.RoleAdmin && target.Role != models.RoleUser {
		return "拒绝:管理员只能管理普通成员。"
	}
	if target.ID == requester.ID {
		return "不能对自己操作。"
	}
	return ""
}

func (h *Hub) audit(action, target, detail string) {
	h.db.Create(&models.AuditLog{ActorID: h.botID, Action: action, Target: target, Detail: detail})
}

func (h *Hub) execMute(requester *models.User, argsJSON string) string {
	var a struct {
		Username string `json:"username"`
		Minutes  int    `json:"minutes"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	target := h.findUserByName(a.Username)
	if target == nil {
		return fmt.Sprintf("找不到成员 @%s。", strings.TrimPrefix(a.Username, "@"))
	}
	if reason := canModerate(requester, target); reason != "" {
		return reason
	}
	mins := a.Minutes
	if mins <= 0 {
		mins = 10
	}
	if mins > aiMaxMuteMinutes {
		mins = aiMaxMuteMinutes
	}
	until := time.Now().Add(time.Duration(mins) * time.Minute)
	h.db.Model(&models.User{}).Where("id = ?", target.ID).Update("muted_until", until)
	h.audit("ai.mute", fmt.Sprintf("user:%d", target.ID), fmt.Sprintf("by @%s, %d min: %s", requester.Username, mins, a.Reason))
	return fmt.Sprintf("已将 @%s 禁言 %d 分钟。", target.Username, mins)
}

func (h *Hub) execRecall(requester *models.User, argsJSON string) string {
	if requester == nil || !requester.IsPrivileged() {
		return "拒绝:你无权撤回他人消息。"
	}
	var a struct {
		MessageID uint   `json:"message_id"`
		Username  string `json:"username"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	var msg models.Message
	if a.MessageID > 0 {
		if h.db.Where("id = ? AND deleted = ? AND recalled = ?", a.MessageID, false, false).First(&msg).Error != nil {
			return "找不到该消息或它已被撤回。"
		}
	} else {
		target := h.findUserByName(a.Username)
		if target == nil {
			return "找不到要撤回消息的成员。"
		}
		if h.db.Where("sender_id = ? AND deleted = ? AND recalled = ?", target.ID, false, false).Order("id DESC").First(&msg).Error != nil {
			return fmt.Sprintf("@%s 没有可撤回的消息。", target.Username)
		}
	}
	sender := h.reloadUser(msg.SenderID)
	if sender == nil {
		return "消息发送者不存在。"
	}
	if reason := canModerate(requester, sender); reason != "" {
		return reason
	}
	now := time.Now()
	h.db.Model(&msg).Updates(map[string]any{"recalled": true, "recalled_by": h.botID, "recalled_at": now})
	h.BroadcastMessageRecall(msg.ID, msg.ChannelID, h.botID)
	h.audit("ai.recall", fmt.Sprintf("message:%d", msg.ID), fmt.Sprintf("by @%s", requester.Username))
	return fmt.Sprintf("已撤回 @%s 的一条消息。", sender.Username)
}

func (h *Hub) execBan(requester *models.User, argsJSON string) string {
	var a struct {
		Username string `json:"username"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	target := h.findUserByName(a.Username)
	if target == nil {
		return fmt.Sprintf("找不到成员 @%s。", strings.TrimPrefix(a.Username, "@"))
	}
	if reason := canModerate(requester, target); reason != "" {
		return reason
	}
	h.db.Model(&models.User{}).Where("id = ?", target.ID).Update("status", models.StatusBanned)
	h.audit("ai.ban", fmt.Sprintf("user:%d", target.ID), fmt.Sprintf("by @%s: %s", requester.Username, a.Reason))
	return fmt.Sprintf("已封禁 @%s。", target.Username)
}

func (h *Hub) execSetRole(requester *models.User, argsJSON string) string {
	if requester == nil || requester.Role != models.RoleSuperAdmin {
		return "拒绝:仅系统管理员可设置成员身份。"
	}
	var a struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	if a.Role != models.RoleUser && a.Role != models.RoleAdmin {
		return "无效身份(只能设为 user 或 admin)。"
	}
	target := h.findUserByName(a.Username)
	if target == nil {
		return fmt.Sprintf("找不到成员 @%s。", strings.TrimPrefix(a.Username, "@"))
	}
	if target.Role == models.RoleSuperAdmin || target.Role == models.RoleBot {
		return "拒绝:不能修改该账号。"
	}
	if target.ID == requester.ID {
		return "不能修改自己的身份。"
	}
	h.db.Model(&models.User{}).Where("id = ?", target.ID).Update("role", a.Role)
	h.audit("ai.set_role", fmt.Sprintf("user:%d", target.ID), fmt.Sprintf("by @%s -> %s", requester.Username, a.Role))
	label := "普通成员"
	if a.Role == models.RoleAdmin {
		label = "管理员"
	}
	return fmt.Sprintf("已将 @%s 设为%s。", target.Username, label)
}

func (h *Hub) execSetNick(requester *models.User, argsJSON string) string {
	var a struct {
		Username string `json:"username"`
		Nickname string `json:"nickname"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	nick := strings.TrimSpace(a.Nickname)
	if nick == "" {
		return "新的群昵称不能为空。"
	}
	target := h.findUserByName(a.Username)
	if target == nil {
		return fmt.Sprintf("找不到成员 @%s。", strings.TrimPrefix(a.Username, "@"))
	}
	if reason := canModerate(requester, target); reason != "" {
		return reason
	}
	h.db.Model(&models.User{}).Where("id = ?", target.ID).Update("nickname", nick)
	h.audit("ai.set_nickname", fmt.Sprintf("user:%d", target.ID), fmt.Sprintf("by @%s -> %s", requester.Username, nick))
	return fmt.Sprintf("已将 @%s 的群昵称改为「%s」。", target.Username, nick)
}
