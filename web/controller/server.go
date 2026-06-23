package controller

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"x-ui/config"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/web/global"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

const (
	defaultLogTailLimit = 64 * 1024
	maxLogReadLimit     = 1024 * 1024
)

type ServerController struct {
	BaseController

	serverService service.ServerService

	lastStatus        *service.Status
	lastGetStatusTime time.Time

	lastVersions        []string
	lastGetVersionsTime time.Time
}

type logReadRequest struct {
	Limit int64 `form:"limit" json:"limit"`
}

type logReadResponse struct {
	Enabled   bool   `json:"enabled"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	Limit     int64  `json:"limit"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updatedAt"`
}

func NewServerController(g *gin.RouterGroup) *ServerController {
	a := &ServerController{
		lastGetStatusTime: time.Now(),
	}
	a.initRouter(g)
	a.startTask()
	return a
}

func (a *ServerController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/server")
	g.Use(a.checkLogin)
	g.POST("/status", a.status)
	g.POST("/getXrayVersion", a.getXrayVersion)
	g.POST("/installXray/:version", a.installXray)
	g.POST("/acme/apply", a.applyAcmeCert)
	g.POST("/genX25519", a.genX25519)
	g.POST("/logs/tail", a.logTail)
	g.POST("/logs/download", a.logDownload)
	g.GET("/logs/stream", a.logStream)
	g.GET("/backup/db", a.backupDB)
	g.POST("/traffic/history", a.trafficHistory)
}

func (a *ServerController) refreshStatus() {
	a.lastStatus = a.serverService.GetStatus(a.lastStatus)
}

func (a *ServerController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 2s", func() {
		now := time.Now()
		if now.Sub(a.lastGetStatusTime) > time.Minute*3 {
			return
		}
		a.refreshStatus()
	})
}

func (a *ServerController) status(c *gin.Context) {
	a.lastGetStatusTime = time.Now()

	jsonObj(c, a.lastStatus, nil)
}

func (a *ServerController) getXrayVersion(c *gin.Context) {
	now := time.Now()
	if now.Sub(a.lastGetVersionsTime) <= time.Minute {
		jsonObj(c, a.lastVersions, nil)
		return
	}

	versions, err := a.serverService.GetXrayVersions()
	if err != nil {
		jsonMsg(c, "获取版本", err)
		return
	}

	a.lastVersions = versions
	a.lastGetVersionsTime = time.Now()

	jsonObj(c, versions, nil)
}

func (a *ServerController) installXray(c *gin.Context) {
	version := strings.TrimSpace(c.Param("version"))
	if version == "" || strings.ContainsAny(version, "/\\") {
		err := errors.New("invalid xray version")
		securityLog(c, "xray_install", false, " version=", version)
		jsonMsg(c, "安装 xray", err)
		return
	}
	err := a.serverService.UpdateXray(version)
	securityLog(c, "xray_install", err == nil, " version=", version)
	jsonMsg(c, "安装 xray", err)
}

func (a *ServerController) logTail(c *gin.Context) {
	var request logReadRequest
	_ = c.ShouldBind(&request)

	response, err := readConfiguredLogTail(request.Limit)
	securityLog(c, "view_panel_log", err == nil, " limit=", request.Limit)
	jsonObj(c, response, err)
}

func (a *ServerController) logDownload(c *gin.Context) {
	var request logReadRequest
	_ = c.ShouldBind(&request)
	if request.Limit <= 0 {
		request.Limit = maxLogReadLimit
	}

	response, err := readConfiguredLogTail(request.Limit)
	securityLog(c, "download_panel_log", err == nil, " limit=", request.Limit)
	jsonObj(c, response, err)
}

func readConfiguredLogTail(limit int64) (*logReadResponse, error) {
	logPath := strings.TrimSpace(os.Getenv("XUI_LOG_FILE"))
	if logPath == "" {
		return &logReadResponse{Enabled: false}, nil
	}

	limit = clampLogReadLimit(limit)
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("log file is not a regular file")
	}

	size := info.Size()
	offset := size - limit
	if offset < 0 {
		offset = 0
	}
	readSize := size - offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, readSize)
	if _, err := io.ReadFull(file, data); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	return &logReadResponse{
		Enabled:   true,
		FileName:  filepath.Base(logPath),
		Size:      size,
		Offset:    offset,
		Limit:     limit,
		Truncated: offset > 0,
		Content:   string(data),
		UpdatedAt: info.ModTime().Unix(),
	}, nil
}

func clampLogReadLimit(limit int64) int64 {
	if limit <= 0 {
		return defaultLogTailLimit
	}
	if limit > maxLogReadLimit {
		return maxLogReadLimit
	}
	return limit
}

func (a *ServerController) logStream(c *gin.Context) {
	logPath := strings.TrimSpace(os.Getenv("XUI_LOG_FILE"))
	if logPath == "" {
		c.Header("Content-Type", "text/event-stream")
		c.String(200, "event:disabled\ndata:{}\n\n")
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		securityLog(c, "stream_panel_log", false)
		c.String(500, "open log failed")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		securityLog(c, "stream_panel_log", false)
		c.String(500, "stat log failed")
		return
	}
	if !info.Mode().IsRegular() {
		securityLog(c, "stream_panel_log", false)
		c.String(500, "log is not regular file")
		return
	}

	securityLog(c, "stream_panel_log", true)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(500, "streaming not supported")
		return
	}

	// 发送初始元数据
	meta := fmt.Sprintf("data:{\"fileName\":%q,\"size\":%d}\n\n", filepath.Base(logPath), info.Size())
	c.Writer.WriteString(meta)
	flusher.Flush()

	// 发送最近 16KB 历史
	tailLimit := int64(16 * 1024)
	size := info.Size()
	offset := size - tailLimit
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			c.Writer.WriteString("data:" + strings.TrimRight(string(line), "\n") + "\n\n")
		}
		if err != nil {
			break
		}
	}
	flusher.Flush()

	// follow 模式：持续读新内容
	ctx := c.Request.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastSize := size
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := file.Stat()
			if err != nil {
				continue
			}
			curSize := info.Size()
			// 文件被截断或轮转，重置
			if curSize < lastSize {
				file.Seek(0, io.SeekStart)
			} else if curSize == lastSize {
				continue
			}
			// 读取新增内容
			buf := make([]byte, 0, 4096)
			tmp := make([]byte, 4096)
			for {
				n, err := file.Read(tmp)
				if n > 0 {
					buf = append(buf, tmp[:n]...)
				}
				if err != nil {
					break
				}
				if n == 0 {
					break
				}
			}
			if len(buf) > 0 {
				lastSize = curSize
				for _, line := range strings.Split(string(buf), "\n") {
					if line == "" {
						continue
					}
					c.Writer.WriteString("data:" + line + "\n\n")
				}
				flusher.Flush()
			}
		}
	}
}

func (a *ServerController) backupDB(c *gin.Context) {
	dbPath := config.GetDBPath()
	info, err := os.Stat(dbPath)
	if err != nil {
		securityLog(c, "backup_db", false)
		jsonMsg(c, "备份数据库", err)
		return
	}
	if !info.Mode().IsRegular() {
		securityLog(c, "backup_db", false)
		jsonMsg(c, "备份数据库", errors.New("database is not a regular file"))
		return
	}
	securityLog(c, "backup_db", true)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="x-ui-backup-%s.db"`, time.Now().Format("20060102-150405")))
	c.File(dbPath)
}

func (a *ServerController) trafficHistory(c *gin.Context) {
	var records []model.TrafficHistory
	since := time.Now().AddDate(0, 0, -7).Unix()
	err := database.GetDB().
		Where("record_at >= ?", since).
		Order("record_at asc").
		Find(&records).Error
	securityLog(c, "traffic_history", err == nil)
	jsonObj(c, records, err)
}

func (a *ServerController) genX25519(c *gin.Context) {
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		jsonMsg(c, "生成密钥", err)
		return
	}
	// RFC 7748: clamp private key
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		jsonMsg(c, "生成密钥", err)
		return
	}

	result := map[string]string{
		"Private key": base64.RawURLEncoding.EncodeToString(privateKey[:]),
		"Public key":  base64.RawURLEncoding.EncodeToString(publicKey),
	}
	jsonObj(c, result, nil)
}
