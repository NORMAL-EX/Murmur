package hub

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"murmur/ai"
	"murmur/models"
)

// aiMaxMuteMinutes caps AI-issued mutes at 60 days.
const aiMaxMuteMinutes = 60 * 24 * 60

func muteToolDef() ai.ToolDef {
	return ai.ToolDef{
		Type: "function",
		Function: ai.ToolFunctionDef{
			Name:        "mute_user",
			Description: "禁言指定成员一段时间（最长 86400 分钟 = 60 天）。仅当请求者有相应权限时才会生效。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"username": map[string]any{"type": "string", "description": "要禁言的成员用户名（不带 @）"},
					"minutes":  map[string]any{"type": "integer", "description": "禁言时长（分钟），最长 86400"},
					"reason":   map[string]any{"type": "string", "description": "禁言原因（可选）"},
				},
				"required": []string{"username", "minutes"},
			},
		},
	}
}

// botToolsPrompt tells the model who is asking and what they may do.
func botToolsPrompt(requester *models.User) string {
	role := "普通成员"
	switch requester.Role {
	case models.RoleSuperAdmin:
		role = "系统管理员"
	case models.RoleAdmin:
		role = "管理员"
	}
	return fmt.Sprintf(`你是群管理助手，可调用 mute_user 工具禁言成员。当前请求者是 @%s（身份：%s）。
权限规则（必须遵守）：
- 系统管理员：可禁言「管理员」和「普通成员」，不能禁言系统管理员或机器人。
- 管理员：只能禁言「普通成员」。
- 其他身份：无权禁言，请礼貌拒绝且不要调用工具。
单次最长 86400 分钟（60 天）。仅当请求者明确要求禁言某人时才调用工具，调用后用中文简要说明结果。`, requester.Username, role)
}

// execBotTool runs a tool call requested by the model on behalf of requester,
// enforcing the role hierarchy server-side regardless of what the model asks.
func (h *Hub) execBotTool(requester *models.User, name, argsJSON string) string {
	if name != "mute_user" {
		return "未知工具：" + name
	}
	if requester == nil {
		return "无法识别请求者。"
	}
	var a struct {
		Username string `json:"username"`
		Minutes  int    `json:"minutes"`
		Reason   string `json:"reason"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)

	if requester.Role != models.RoleSuperAdmin && requester.Role != models.RoleAdmin {
		return "拒绝：你无权指挥我禁言他人。"
	}
	uname := strings.TrimPrefix(strings.TrimSpace(a.Username), "@")
	if uname == "" {
		return "缺少要禁言的成员用户名。"
	}
	var target models.User
	if h.db.Where("LOWER(username) = ?", strings.ToLower(uname)).First(&target).Error != nil {
		return fmt.Sprintf("找不到成员 @%s。", uname)
	}
	if target.Role == models.RoleSuperAdmin || target.Role == models.RoleBot {
		return "拒绝：不能禁言该账号。"
	}
	if requester.Role == models.RoleAdmin && target.Role != models.RoleUser {
		return "拒绝：管理员只能禁言普通成员。"
	}
	if target.ID == requester.ID {
		return "不能禁言自己。"
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
	h.db.Create(&models.AuditLog{
		ActorID: h.botID,
		Action:  "ai.mute",
		Target:  fmt.Sprintf("user:%d", target.ID),
		Detail:  fmt.Sprintf("requested by @%s, %d min, reason: %s", requester.Username, mins, a.Reason),
	})
	return fmt.Sprintf("已将 @%s 禁言 %d 分钟。", target.Username, mins)
}
