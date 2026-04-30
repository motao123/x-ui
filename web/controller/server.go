package controller

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"x-ui/web/global"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

type ServerController struct {
	BaseController

	serverService service.ServerService

	lastStatus        *service.Status
	lastGetStatusTime time.Time

	lastVersions        []string
	lastGetVersionsTime time.Time
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
		"Private key": base64.RawStdEncoding.EncodeToString(privateKey[:]),
		"Public key":  base64.RawStdEncoding.EncodeToString(publicKey),
	}
	jsonObj(c, result, nil)
}
