package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"murmur/middleware"

	"github.com/gin-gonic/gin"
)

var allowedImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

const maxAvatarSize = 5 << 20 // 5 MiB

func (h *H) UploadAvatar(c *gin.Context) {
	u := middleware.CurrentUser(c)
	file, err := c.FormFile("avatar")
	if err != nil {
		fail(c, http.StatusBadRequest, "no_file", "未找到上传文件")
		return
	}
	if file.Size > maxAvatarSize {
		fail(c, http.StatusBadRequest, "too_large", "图片不能超过 5MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		fail(c, http.StatusBadRequest, "bad_type", "仅支持 png/jpg/gif/webp 图片")
		return
	}
	if err := os.MkdirAll(h.Cfg.UploadDir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "mkdir", "服务器存储错误")
		return
	}
	name := fmt.Sprintf("avatar_%d_%d%s", u.ID, time.Now().UnixNano(), ext)
	dst := filepath.Join(h.Cfg.UploadDir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		fail(c, http.StatusInternalServerError, "save", "保存文件失败")
		return
	}
	url := "/uploads/" + name
	h.DB.Model(u).Update("avatar_url", url)
	c.JSON(http.StatusOK, gin.H{"avatar_url": url})
}
