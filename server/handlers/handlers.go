// Package handlers implements the REST + WebSocket HTTP layer.
package handlers

import (
	"strconv"

	"murmur/ai"
	"murmur/config"
	"murmur/hub"
	"murmur/middleware"
	"murmur/models"
	"murmur/settings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type H struct {
	DB  *gorm.DB
	Cfg *config.Config
	St  *settings.Service
	Hub *hub.Hub
	AI  *ai.Service
}

func New(db *gorm.DB, cfg *config.Config, st *settings.Service, h *hub.Hub, aiSvc *ai.Service) *H {
	return &H{DB: db, Cfg: cfg, St: st, Hub: h, AI: aiSvc}
}

// ---- response helpers ----

func fail(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{"code": code, "error": msg})
}

func failErr(c *gin.Context, e *hub.Err) {
	c.JSON(e.Status, gin.H{"code": e.Code, "error": e.Message, "retry_after": e.RetryAfter})
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(v), true
}

func queryInt(c *gin.Context, name string, def int) int {
	if v, err := strconv.Atoi(c.Query(name)); err == nil {
		return v
	}
	return def
}

func (h *H) audit(actorID uint, action, target, detail string) {
	h.DB.Create(&models.AuditLog{ActorID: actorID, Action: action, Target: target, Detail: detail})
}

// RegisterRoutes wires every endpoint onto the engine.
func (h *H) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// Public
	api.POST("/auth/register", h.Register)
	api.POST("/auth/login", h.Login)
	api.GET("/settings", h.PublicSettings)

	auth := middleware.Auth(h.Cfg, h.DB)

	// Authenticated
	authed := api.Group("")
	authed.Use(auth)
	{
		authed.POST("/auth/logout", h.Logout)
		authed.GET("/me", h.Me)
		authed.PATCH("/me", h.UpdateMe)
		authed.POST("/me/avatar", h.UploadAvatar)
		authed.POST("/uploads", h.UploadImage)

		authed.GET("/users", h.ListUsers)
		authed.GET("/users/:id", h.GetUser)

		authed.GET("/channels", h.ListChannels)
		authed.GET("/channels/:id/messages", h.ChannelMessages)
		authed.GET("/channels/:id/search", h.SearchMessages)

		authed.PATCH("/messages/:id", h.EditMessage)
		authed.DELETE("/messages/:id", h.DeleteMessage)
		authed.POST("/messages/:id/recall", h.RecallMessage)
		authed.POST("/messages/:id/reactions", h.ToggleReaction)

		authed.POST("/dm-messages/:id/recall", h.RecallDM)

		authed.GET("/dm/conversations", h.Conversations)
		authed.GET("/dm/:userId/messages", h.DMMessages)
		authed.POST("/dm/:userId", h.SendDM)
		authed.POST("/dm/:userId/read", h.MarkDMRead)

		authed.GET("/mentions", h.Mentions)
		authed.POST("/mentions/:id/read", h.ReadMention)
		authed.POST("/mentions/read-all", h.ReadAllMentions)
	}

	// Channel management (admin) under /api/channels per the API spec.
	chAdmin := api.Group("/channels")
	chAdmin.Use(auth, middleware.Admin())
	{
		chAdmin.POST("", h.CreateChannel)
		chAdmin.PATCH("/:id", h.UpdateChannel)
		chAdmin.DELETE("/:id", h.DeleteChannel)
	}

	// Admin
	admin := api.Group("/admin")
	admin.Use(auth, middleware.Admin())
	{
		admin.GET("/stats", h.Stats)
		admin.GET("/users", h.AdminUsers)
		admin.PATCH("/users/:id", h.AdminUpdateUser)
		admin.GET("/registrations", h.AdminRegistrations)
		admin.POST("/registrations/:id/approve", h.AdminApprove)
		admin.POST("/registrations/:id/reject", h.AdminReject)
		admin.GET("/settings", h.AdminGetSettings)
		admin.PUT("/settings", h.AdminPutSettings)
		admin.GET("/ai", h.AdminGetAI)
		admin.POST("/ai/test", h.AdminTestAI)
		admin.GET("/audit", h.AdminAudit)
	}

	// Super-admin-only.
	super := api.Group("/admin")
	super.Use(auth, middleware.SuperAdmin())
	{
		super.DELETE("/users/:id", h.SuperDeleteUser)
		super.GET("/messages/:id", h.SuperGetMessage)
		super.GET("/dm-messages/:id", h.SuperGetDM)
		super.GET("/dm/conversations", h.AdminDMConversations)
		super.GET("/dm/thread", h.AdminDMThread)
	}

	// WebSocket (JWT via ?token=).
	r.GET("/ws", auth, h.WS)
}
