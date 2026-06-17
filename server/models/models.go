package models

import "time"

// Roles
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleUser       = "user"
	RoleBot        = "bot"
)

// Statuses
const (
	StatusUnverified = "unverified" // awaiting email verification
	StatusPending    = "pending"    // awaiting admin approval
	StatusActive     = "active"
	StatusBanned     = "banned"
)

// RateLimit sentinels for User.RateLimitPerMin:
//
//	-1 => inherit the global/role default
//	 0 => unlimited
//	>0 => that many messages per minute
const (
	RateInherit   = -1
	RateUnlimited = 0
)

type User struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	Username        string     `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash    string     `gorm:"size:255" json:"-"`
	Nickname        string     `gorm:"size:64" json:"nickname"`
	AvatarURL       string     `gorm:"size:512" json:"avatar_url"`
	Bio             string     `gorm:"size:1024" json:"bio"`
	Email           string     `gorm:"size:255" json:"email,omitempty"`
	EmailVerified   bool       `gorm:"default:false" json:"email_verified"`
	Role            string     `gorm:"index;size:20;default:user" json:"role"`
	Status          string     `gorm:"index;size:20;default:active" json:"status"`
	RateLimitPerMin int        `gorm:"default:-1" json:"rate_limit_per_min"`
	MutedUntil      *time.Time `json:"muted_until,omitempty"`
	VerifyCode      string     `gorm:"size:16" json:"-"`
	VerifyExpires   *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"-"`
}

// Muted reports whether the user is currently muted.
func (u *User) Muted() bool {
	return u.MutedUntil != nil && time.Now().Before(*u.MutedUntil)
}

type Setting struct {
	Key       string    `gorm:"primarykey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Channel struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	Slug        string    `gorm:"uniqueIndex;size:64;not null" json:"slug"`
	Description string    `gorm:"size:512" json:"description"`
	Readonly    bool      `gorm:"default:false" json:"readonly"`
	Pinned      bool      `gorm:"default:false" json:"pinned"`
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type Message struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	ChannelID  uint       `gorm:"index" json:"channel_id"`
	SenderID   uint       `gorm:"index" json:"sender_id"`
	ReplyToID  *uint      `gorm:"index" json:"reply_to_id,omitempty"`
	Content    string     `gorm:"type:text" json:"content"`
	Edited     bool       `gorm:"default:false" json:"edited"`
	IsBot      bool       `gorm:"default:false" json:"is_bot"`
	Deleted    bool       `gorm:"index;default:false" json:"deleted"`
	Recalled   bool       `gorm:"default:false" json:"recalled"`
	RecalledBy uint       `json:"recalled_by"`
	RecalledAt *time.Time `json:"recalled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"-"`
}

type DirectMessage struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	SenderID   uint       `gorm:"index" json:"sender_id"`
	ReceiverID uint       `gorm:"index" json:"receiver_id"`
	ReplyToID  *uint      `gorm:"index" json:"reply_to_id,omitempty"`
	Content    string     `gorm:"type:text" json:"content"`
	ReadAt     *time.Time `json:"read_at"`
	Recalled   bool       `gorm:"default:false" json:"recalled"`
	RecalledBy uint       `json:"recalled_by"`
	RecalledAt *time.Time `json:"recalled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Mention struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	MessageID       uint       `gorm:"index" json:"message_id"`
	ChannelID       uint       `gorm:"index" json:"channel_id"`
	MentionedUserID uint       `gorm:"index" json:"mentioned_user_id"`
	ReadAt          *time.Time `json:"read_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Reaction struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	MessageID uint      `gorm:"index:idx_reaction,unique,priority:1" json:"message_id"`
	UserID    uint      `gorm:"index:idx_reaction,unique,priority:2" json:"user_id"`
	Emoji     string    `gorm:"index:idx_reaction,unique,priority:3;size:32" json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ActorID   uint      `gorm:"index" json:"actor_id"`
	Action    string    `gorm:"size:64" json:"action"`
	Target    string    `gorm:"size:128" json:"target"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// ---- DTOs returned over the wire ----

type ReactionDTO struct {
	Emoji   string `json:"emoji"`
	Count   int    `json:"count"`
	UserIDs []uint `json:"user_ids"`
	Reacted bool   `json:"reacted"`
}

// ReplyPreview is a compact view of the message being replied to.
type ReplyPreview struct {
	ID         uint   `json:"id"`
	SenderID   uint   `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
	Recalled   bool   `json:"recalled"`
}

type MessageDTO struct {
	ID         uint          `json:"id"`
	ChannelID  uint          `json:"channel_id"`
	SenderID   uint          `json:"sender_id"`
	Sender     *User         `json:"sender,omitempty"`
	ReplyTo    *ReplyPreview `json:"reply_to,omitempty"`
	Content    string        `json:"content"`
	Edited     bool          `json:"edited"`
	Deleted    bool          `json:"deleted"`
	Recalled   bool          `json:"recalled"`
	RecalledBy uint          `json:"recalled_by"`
	IsBot      bool          `json:"is_bot"`
	Mentions   []uint        `json:"mentions"`
	Reactions  []ReactionDTO `json:"reactions"`
	CreatedAt  time.Time     `json:"created_at"`
}

type DirectMessageDTO struct {
	ID         uint          `json:"id"`
	SenderID   uint          `json:"sender_id"`
	ReceiverID uint          `json:"receiver_id"`
	Sender     *User         `json:"sender,omitempty"`
	ReplyTo    *ReplyPreview `json:"reply_to,omitempty"`
	Content    string        `json:"content"`
	ReadAt     *time.Time    `json:"read_at"`
	Recalled   bool          `json:"recalled"`
	RecalledBy uint          `json:"recalled_by"`
	CreatedAt  time.Time     `json:"created_at"`
}

type ConversationDTO struct {
	User        User              `json:"user"`
	LastMessage *DirectMessageDTO `json:"last_message"`
	Unread      int               `json:"unread"`
}

type MentionDTO struct {
	ID              uint        `json:"id"`
	MessageID       uint        `json:"message_id"`
	ChannelID       uint        `json:"channel_id"`
	MentionedUserID uint        `json:"mentioned_user_id"`
	ReadAt          *time.Time  `json:"read_at"`
	CreatedAt       time.Time   `json:"created_at"`
	Message         *MessageDTO `json:"message,omitempty"`
}

type AuditLogDTO struct {
	ID        uint      `json:"id"`
	ActorID   uint      `json:"actor_id"`
	Actor     *User     `json:"actor,omitempty"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// IsPrivileged reports admin or super admin.
func (u *User) IsPrivileged() bool {
	return u.Role == RoleAdmin || u.Role == RoleSuperAdmin
}
