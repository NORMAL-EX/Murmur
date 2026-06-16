package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"murmur/ai"
	"murmur/models"
	"murmur/settings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AuditLog{}, &models.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := settings.New(db, "test-enc-key")
	if err := st.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	return &Hub{db: db, st: st, ai: ai.New(st), botID: 999}
}

func seedUser(t *testing.T, h *Hub, username, role string) *models.User {
	t.Helper()
	u := &models.User{
		Username: username, Nickname: username, Role: role,
		Status: models.StatusActive, RateLimitPerMin: models.RateInherit,
	}
	if err := h.db.Create(u).Error; err != nil {
		t.Fatalf("seed %s: %v", username, err)
	}
	return u
}

func muteArgs(name string, mins int) string {
	b, _ := json.Marshal(map[string]any{"username": name, "minutes": mins})
	return string(b)
}

func TestExecTime(t *testing.T) {
	out := execTime()
	if !strings.Contains(out, "北京时间") || !strings.Contains(out, time.Now().In(beijingLoc).Format("2006")) {
		t.Fatalf("unexpected time output: %s", out)
	}
	t.Log(out)
}

func TestExecMutePermissions(t *testing.T) {
	h := newTestHub(t)
	super := seedUser(t, h, "root", models.RoleSuperAdmin)
	admin1 := seedUser(t, h, "adm1", models.RoleAdmin)
	seedUser(t, h, "adm2", models.RoleAdmin)
	seedUser(t, h, "alice", models.RoleUser)
	user := seedUser(t, h, "bob", models.RoleUser)

	if r := h.execMute(super, muteArgs("adm2", 30)); !strings.Contains(r, "已将") {
		t.Errorf("super->admin should succeed: %s", r)
	}
	if r := h.execMute(admin1, muteArgs("adm2", 30)); !strings.Contains(r, "拒绝") {
		t.Errorf("admin->admin should be denied: %s", r)
	}
	if r := h.execMute(admin1, muteArgs("alice", 30)); !strings.Contains(r, "已将") {
		t.Errorf("admin->user should succeed: %s", r)
	}
	if r := h.execMute(admin1, muteArgs("root", 30)); !strings.Contains(r, "拒绝") {
		t.Errorf("admin->super should be denied: %s", r)
	}
	if r := h.execMute(user, muteArgs("alice", 30)); !strings.Contains(r, "无权") {
		t.Errorf("normal user requester should be denied: %s", r)
	}

	h.execMute(super, muteArgs("alice", 999999))
	var alice models.User
	h.db.Where("username = ?", "alice").First(&alice)
	if alice.MutedUntil == nil {
		t.Fatal("alice should be muted")
	}
	if mins := time.Until(*alice.MutedUntil).Minutes(); mins > aiMaxMuteMinutes+5 {
		t.Errorf("mute should be capped at %d min, got ~%.0f", aiMaxMuteMinutes, mins)
	} else {
		t.Logf("alice muted ~%.0f min (cap %d)", mins, aiMaxMuteMinutes)
	}
}

// TestToolCallingLoop drives ai.CompleteWithTools against a mock OpenAI endpoint
// that first asks for a tool call, then returns a final answer.
func TestToolCallingLoop(t *testing.T) {
	h := newTestHub(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"get_current_time","arguments":"{}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"已为你查询时间。"}}]}`))
	}))
	defer srv.Close()

	_ = h.st.Set(settings.AIBaseURL, srv.URL)
	_ = h.st.Set(settings.AIAPIKey, "test")
	_ = h.st.Set(settings.AIModel, "mock")

	executed := ""
	exec := func(name, args string) string {
		executed = name
		return h.execBotTool(nil, name, args)
	}
	reply, err := h.ai.CompleteWithTools(context.Background(),
		[]ai.Message{{Role: "user", Content: "几点了"}}, botTools(false), exec)
	if err != nil {
		t.Fatalf("CompleteWithTools error: %v", err)
	}
	if executed != "get_current_time" {
		t.Errorf("expected get_current_time to run, got %q", executed)
	}
	if !strings.Contains(reply, "已为你查询时间") {
		t.Errorf("unexpected final reply: %s", reply)
	}
	if calls != 2 {
		t.Errorf("expected 2 round-trips, got %d", calls)
	}
	t.Logf("loop reply=%q calls=%d tool=%s", reply, calls, executed)
}

func TestExecWebSearch(t *testing.T) {
	out := execWebSearch("golang")
	t.Logf("web_search result:\n%s", out)
	if strings.HasPrefix(out, "搜索失败") {
		t.Skip("outbound network blocked here: " + out)
	}
}
