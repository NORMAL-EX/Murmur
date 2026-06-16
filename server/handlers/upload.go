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
const maxImageSize = 8 << 20  // 8 MiB

// saveUpload validates and stores an uploaded image, returning its public URL.
func (h *H) saveUpload(c *gin.Context, field, prefix string, maxSize int64) (string, bool) {
	file, err := c.FormFile(field)
	if err != nil {
		fail(c, http.StatusBadRequest, "no_file", "未找到上传文件")
		return "", false
	}
	if file.Size > maxSize {
		fail(c, http.StatusBadRequest, "too_large", fmt.Sprintf("图片不能超过 %dMB", maxSize>>20))
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		fail(c, http.StatusBadRequest, "bad_type", "仅支持 png/jpg/gif/webp 图片")
		return "", false
	}
	if err := os.MkdirAll(h.Cfg.UploadDir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "mkdir", "服务器存储错误")
		return "", false
	}
	u := middleware.CurrentUser(c)
	name := fmt.Sprintf("%s_%d_%d%s", prefix, u.ID, time.Now().UnixNano(), ext)
	dst := filepath.Join(h.Cfg.UploadDir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		fail(c, http.StatusInternalServerError, "save", "保存文件失败")
		return "", false
	}
	return "/uploads/" + name, true
}

func (h *H) UploadAvatar(c *gin.Context) {
	u := middleware.CurrentUser(c)
	url, ok := h.saveUpload(c, "avatar", "avatar", maxAvatarSize)
	if !ok {
		return
	}
	h.DB.Model(u).Update("avatar_url", url)
	c.JSON(http.StatusOK, gin.H{"avatar_url": url})
}

// UploadImage stores an image for use as a message attachment and returns its URL.
func (h *H) UploadImage(c *gin.Context) {
	url, ok := h.saveUpload(c, "file", "img", maxImageSize)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
