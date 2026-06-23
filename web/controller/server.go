package controller

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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
