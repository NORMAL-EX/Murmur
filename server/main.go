package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"murmur/ai"
	"murmur/config"
	"murmur/db"
	"murmur/handlers"
	"murmur/hub"
	"murmur/ratelimit"
	"murmur/settings"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	gdb, err := db.Init(cfg)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	st := settings.New(gdb, cfg.EncryptionKey)
	if err := st.Bootstrap(); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}
	if err := db.Seed(gdb, cfg, st); err != nil {
		log.Fatalf("数据填充失败: %v", err)
	}

	bot, err := db.BotUser(gdb)
	if err != nil {
		log.Fatalf("机器人账号缺失: %v", err)
	}

	rl := ratelimit.New()
	aiSvc := ai.New(st)
	h := hub.New(gdb, st, rl, aiSvc, bot.ID, bot.Username)
	api := handlers.New(gdb, cfg, st, h, aiSvc)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	api.RegisterRoutes(r)

	// Uploaded files.
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Printf("无法创建上传目录: %v", err)
	}
	r.Static("/uploads", cfg.UploadDir)

	serveFrontend(r)

	addr := ":" + cfg.Port
	log.Printf("Murmur 服务已启动,监听 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// serveFrontend serves the built SPA (web/dist) if available, with history
// fallback to index.html for client-side routes.
func serveFrontend(r *gin.Engine) {
	candidates := []string{os.Getenv("STATIC_DIR"), "./web/dist", "../web/dist", "./dist", "./public"}
	var dir string
	for _, d := range candidates {
		if d == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(d, "index.html")); err == nil && !fi.IsDir() {
			dir = d
			break
		}
	}
	if dir == "" {
		log.Printf("未找到前端构建产物(web/dist),仅提供 API")
		return
	}
	log.Printf("提供前端静态资源: %s", dir)
	index := filepath.Join(dir, "index.html")

	if fi, err := os.Stat(filepath.Join(dir, "assets")); err == nil && fi.IsDir() {
		r.Static("/assets", filepath.Join(dir, "assets"))
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") || strings.HasPrefix(p, "/uploads") {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "error": "未找到"})
			return
		}
		// Serve a real static file if it exists (e.g. favicon), else the SPA shell.
		if p != "/" {
			candidate := filepath.Join(dir, filepath.Clean(p))
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				c.File(candidate)
				return
			}
		}
		c.File(index)
	})
}
