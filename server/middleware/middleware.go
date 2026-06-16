// Package middleware provides JWT authentication and role guards for Gin.
package middleware

import (
	"net/http"
	"strings"

	"murmur/auth"
	"murmur/config"
	"murmur/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const userKey = "currentUser"

func extractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	// WebSocket clients cannot set headers, so allow a query param fallback.
	return c.Query("token")
}

// Auth verifies the JWT, loads the user and rejects banned/pending accounts.
func Auth(cfg *config.Config, gdb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "error": "未登录"})
			return
		}
		claims, err := auth.ParseToken(cfg.JWTSecret, tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "error": "登录已失效"})
			return
		}
		var u models.User
		if err := gdb.First(&u, claims.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "error": "用户不存在"})
			return
		}
		if u.Status == models.StatusBanned {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "banned", "error": "账号已被封禁"})
			return
		}
		if u.Status == models.StatusPending {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "pending", "error": "账号待审核"})
			return
		}
		if u.Role == models.RoleBot {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "bot", "error": "机器人账号不可登录"})
			return
		}
		c.Set(userKey, &u)
		c.Next()
	}
}

// Admin requires admin or super_admin.
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || !u.IsPrivileged() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

// SuperAdmin requires super_admin.
func SuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || u.Role != models.RoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "error": "需要超级管理员权限"})
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user set by Auth.
func CurrentUser(c *gin.Context) *models.User {
	v, ok := c.Get(userKey)
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}
