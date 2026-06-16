// Package settings manages the key/value site & AI configuration stored in the
// database, with an in-memory cache and transparent encryption of secret keys.
package settings

import (
	"strconv"
	"sync"

	"murmur/cryptox"
	"murmur/models"

	"gorm.io/gorm"
)

// Keys
const (
	SiteTitle        = "site_title"
	SiteDescription  = "site_description"
	RegistrationOpen = "registration_open"
	RegistrationRev  = "registration_review"
	RegistrationMail = "registration_email_verify"
	AllowDM          = "allow_dm"

	SMTPHost          = "smtp_host"
	SMTPPort          = "smtp_port"
	SMTPUsername      = "smtp_username"
	SMTPPassword      = "smtp_password"
	SMTPFrom          = "smtp_from"
	SMTPFromName      = "smtp_from_name"
	SMTPSSL           = "smtp_ssl"
	MailSubject       = "mail_subject"
	MailBody          = "mail_body"
	MaxMessageLength  = "max_message_length"
	Announcement      = "announcement"
	DefaultTheme      = "default_theme"
	RateLimitMessages = "rate_limit_messages"
	RateLimitWindow   = "rate_limit_window_seconds"
	RateLimitAdmin    = "rate_limit_admin_messages"

	AIEnabled      = "ai_enabled"
	AIBaseURL      = "ai_base_url"
	AIAPIKey       = "ai_api_key"
	AIModel        = "ai_model"
	AISystemPrompt = "ai_system_prompt"
	AITemperature  = "ai_temperature"
	AIMaxTokens    = "ai_max_tokens"
	AIContextLimit = "ai_context_char_limit"
	AICooldown     = "ai_cooldown_seconds"
	AIAllowDM      = "ai_allow_dm"
	AIToolsEnabled = "ai_tools_enabled"
	BotName        = "bot_name"
	BotAvatar      = "bot_avatar"
)

// Defaults applied on first boot.
var Defaults = map[string]string{
	SiteTitle:        "Murmur",
	SiteDescription:  "自托管实时聊天室",
	RegistrationOpen: "true",
	RegistrationRev:  "false",
	RegistrationMail: "false",
	AllowDM:          "true",

	SMTPHost:          "",
	SMTPPort:          "587",
	SMTPUsername:      "",
	SMTPPassword:      "",
	SMTPFrom:          "",
	SMTPFromName:      "Murmur",
	SMTPSSL:           "false",
	MailSubject:       "{site} 注册验证码",
	MailBody:          "【{site}】您的注册验证码是 {code}，10 分钟内有效。\n\n如非本人操作请忽略此邮件。",
	MaxMessageLength:  "2000",
	Announcement:      "",
	DefaultTheme:      "system",
	RateLimitMessages: "10",
	RateLimitWindow:   "30",
	RateLimitAdmin:    "60",

	AIEnabled:      "false",
	AIBaseURL:      "https://api.openai.com/v1",
	AIAPIKey:       "",
	AIModel:        "gpt-4o-mini",
	AISystemPrompt: "你是 Murmur 聊天室里的友好 AI 助手，请用简洁、口语化的中文回答大家。",
	AITemperature:  "0.7",
	AIMaxTokens:    "1024",
	AIContextLimit: "5000",
	AICooldown:     "10",
	AIAllowDM:      "false",
	AIToolsEnabled: "false",
	BotName:        "Murmur Bot",
	BotAvatar:      "",
}

// secretKeys are encrypted at rest and never returned in plaintext via the API.
var secretKeys = map[string]bool{AIAPIKey: true, SMTPPassword: true}

// publicKeys are exposed through the unauthenticated /api/settings endpoint.
var publicKeys = []string{
	SiteTitle, SiteDescription, RegistrationOpen, RegistrationRev, RegistrationMail,
	AllowDM, MaxMessageLength, Announcement, DefaultTheme, BotName, BotAvatar,
}

func IsSecret(key string) bool { return secretKeys[key] }

type Service struct {
	db     *gorm.DB
	encKey string
	mu     sync.RWMutex
	cache  map[string]string // plaintext effective values
}

func New(db *gorm.DB, encKey string) *Service {
	return &Service{db: db, encKey: encKey, cache: map[string]string{}}
}

// Bootstrap seeds any missing defaults and loads everything into cache.
func (s *Service) Bootstrap() error {
	var existing []models.Setting
	if err := s.db.Find(&existing).Error; err != nil {
		return err
	}
	have := map[string]bool{}
	for _, st := range existing {
		have[st.Key] = true
	}
	for k, v := range Defaults {
		if have[k] {
			continue
		}
		stored := v
		if secretKeys[k] && v != "" {
			enc, err := cryptox.Encrypt(s.encKey, v)
			if err != nil {
				return err
			}
			stored = enc
		}
		if err := s.db.Create(&models.Setting{Key: k, Value: stored}).Error; err != nil {
			return err
		}
	}
	return s.reload()
}

func (s *Service) reload() error {
	var rows []models.Setting
	if err := s.db.Find(&rows).Error; err != nil {
		return err
	}
	cache := map[string]string{}
	for _, r := range rows {
		val := r.Value
		if secretKeys[r.Key] {
			dec, err := cryptox.Decrypt(s.encKey, r.Value)
			if err == nil {
				val = dec
			}
		}
		cache[r.Key] = val
	}
	s.mu.Lock()
	s.cache = cache
	s.mu.Unlock()
	return nil
}

func (s *Service) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.cache[key]; ok {
		return v
	}
	return Defaults[key]
}

func (s *Service) GetBool(key string) bool {
	return s.Get(key) == "true" || s.Get(key) == "1"
}

func (s *Service) GetInt(key string) int {
	n, err := strconv.Atoi(s.Get(key))
	if err != nil {
		if d, ok := Defaults[key]; ok {
			n, _ = strconv.Atoi(d)
		}
	}
	return n
}

func (s *Service) GetFloat(key string) float64 {
	f, err := strconv.ParseFloat(s.Get(key), 64)
	if err != nil {
		if d, ok := Defaults[key]; ok {
			f, _ = strconv.ParseFloat(d, 64)
		}
	}
	return f
}

// Set writes a single key (encrypting secrets) and updates the cache.
func (s *Service) Set(key, value string) error {
	stored := value
	plaintext := value
	if secretKeys[key] {
		if value == "" {
			stored = ""
		} else {
			enc, err := cryptox.Encrypt(s.encKey, value)
			if err != nil {
				return err
			}
			stored = enc
		}
	}
	if err := s.db.Save(&models.Setting{Key: key, Value: stored}).Error; err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[key] = plaintext
	s.mu.Unlock()
	return nil
}

// SetMany updates multiple keys.
func (s *Service) SetMany(values map[string]string) error {
	for k, v := range values {
		if err := s.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// PublicMap returns the whitelist of public settings for the frontend.
func (s *Service) PublicMap() map[string]any {
	m := map[string]any{}
	for _, k := range publicKeys {
		switch k {
		case RegistrationOpen, RegistrationRev, RegistrationMail, AllowDM:
			m[k] = s.GetBool(k)
		case MaxMessageLength:
			m[k] = s.GetInt(k)
		default:
			m[k] = s.Get(k)
		}
	}
	return m
}

// AdminMap returns all settings for the admin UI with secrets masked.
func (s *Service) AdminMap() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := map[string]string{}
	for k := range Defaults {
		if secretKeys[k] {
			continue
		}
		if v, ok := s.cache[k]; ok {
			m[k] = v
		} else {
			m[k] = Defaults[k]
		}
	}
	// expose whether each secret is configured (without revealing it)
	for k := range secretKeys {
		if s.cache[k] != "" {
			m[k+"_set"] = "true"
		} else {
			m[k+"_set"] = "false"
		}
	}
	return m
}

func (s *Service) HasSecret(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[key] != ""
}
