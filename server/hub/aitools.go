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

// botTools returns the tool set offered to the model. The mute tool is only
// included where a single requester is known (direct messages).
func botTools(includeMute bool) []ai.ToolDef {
	tools := []ai.ToolDef{timeToolDef(), webSearchToolDef()}
	if includeMute {
		tools = append(tools, muteToolDef())
	}
	return tools
}

func timeToolDef() ai.ToolDef {
	return ai.ToolDef{
		Type: "function",
		Function: ai.ToolFunctionDef{
			Name:        "get_current_time",
			Description: "获取当前北京时间（Asia/Shanghai，UTC+8）。",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

func webSearchToolDef() ai.ToolDef {
	return ai.ToolDef{
		Type: "function",
		Function: ai.ToolFunctionDef{
			Name:        "web_search",
			Description: "使用 Bing 进行联网搜索，返回前几条结果的标题、摘要与链接。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词"},
				},
				"required": []string{"query"},
			},
		},
	}
}

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
	if requester != nil {
		switch requester.Role {
		case models.RoleSuperAdmin:
			role = "系统管理员"
		case models.RoleAdmin:
			role = "管理员"
		}
	}
	name := "未知"
	if requester != nil {
		name = requester.Username
	}
	return fmt.Sprintf(`你是群管理助手，可使用以下工具：get_current_time（查北京时间）、web_search（Bing 联网搜索）、mute_user（禁言成员）。
当前请求者是 @%s（身份：%s）。禁言权限规则（必须遵守）：
- 系统管理员：可禁言「管理员」和「普通成员」，不能禁言系统管理员或机器人。
- 管理员：只能禁言「普通成员」。
- 其他身份：无权禁言，请礼貌拒绝且不要调用 mute_user。
禁言单次最长 86400 分钟（60 天）。需要实时信息或时间时请调用相应工具，调用后用中文简要说明结果。`, name, role)
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
	default:
		return "未知工具：" + name
	}
}

func execTime() string {
	now := time.Now().In(beijingLoc)
	weekdays := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return fmt.Sprintf("当前北京时间：%s %s", now.Format("2006-01-02 15:04:05"), weekdays[int(now.Weekday())])
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
		return "搜索失败：" + err.Error()
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "搜索失败：" + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 3<<20))

	matches := bingItemRe.FindAllStringSubmatch(string(body), -1)
	var sb strings.Builder
	fmt.Fprintf(&sb, "「%s」的 Bing 搜索结果：\n", query)
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
		return "未找到结果（可能是网络受限或页面结构变化）。"
	}
	return sb.String()
}

// execMute enforces the role hierarchy server-side regardless of the model.
func (h *Hub) execMute(requester *models.User, argsJSON string) string {
	if requester == nil {
		return "无法识别请求者（请私信我执行禁言）。"
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
