package controller

import (
	"net/http"
	"sync"
	"time"
	"x-ui/logger"
	"x-ui/web/job"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type LoginForm struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type IndexController struct {
	BaseController

	userService service.UserService
}

type loginFailure struct {
	Count     int
	LockedTil time.Time
	LastFail  time.Time
}

var loginFailures = struct {
	sync.Mutex
	items map[string]*loginFailure
}{items: map[string]*loginFailure{}}

const (
	maxLoginFailures = 5
	loginLockTime    = 15 * time.Minute
	loginFailureTTL  = 30 * time.Minute
)

func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.POST("/login", a.login)
	g.GET("/logout", a.logout)
}

func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "xui/")
		return
	}
	html(c, "login.html", "登录", nil)
}

func (a *IndexController) login(c *gin.Context) {
	var form LoginForm
	err := c.ShouldBind(&form)
	if err != nil {
		pureJsonMsg(c, false, "数据格式错误")
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, false, "请输入用户名")
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, false, "请输入密码")
		return
	}
	remoteIp := getRemoteIp(c)
	if isLoginLocked(remoteIp, form.Username) {
		logger.Infof("login temporarily locked for user %q from %s", form.Username, remoteIp)
		securityLog(c, "login_locked", false, " username=", form.Username)
		pureJsonMsg(c, false, "登录失败次数过多，请稍后再试")
		return
	}
	user := a.userService.CheckUser(form.Username, form.Password)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	if user == nil {
		recordLoginFailure(remoteIp, form.Username)
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, remoteIp, timeStr, 0)
		logger.Infof("wrong username or password for user %q from %s", form.Username, remoteIp)
		securityLog(c, "login_failed", false, " username=", form.Username)
		pureJsonMsg(c, false, "用户名或密码错误")
		return
	} else {
		clearLoginFailures(remoteIp, form.Username)
		logger.Infof("%s login success,Ip Address:%s\n", form.Username, remoteIp)
		job.NewStatsNotifyJob().UserLoginNotify(form.Username, remoteIp, timeStr, 1)
	}

	err = session.SetLoginUser(c, user)
	logger.Info("user", user.Id, "login success")
	securityLog(c, "login_success", err == nil, " username=", form.Username)
	jsonMsg(c, "登录", err)
}

func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Info("user", user.Id, "logout")
	}
	securityLog(c, "logout", true)
	session.ClearSession(c, c.GetString("base_path"))
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}

func loginFailureKey(ip string, username string) string {
	return ip + "|" + username
}

func isLoginLocked(ip string, username string) bool {
	loginFailures.Lock()
	defer loginFailures.Unlock()
	now := time.Now()
	cleanupLoginFailures(now)
	item := loginFailures.items[loginFailureKey(ip, username)]
	return item != nil && item.LockedTil.After(now)
}

func recordLoginFailure(ip string, username string) {
	loginFailures.Lock()
	defer loginFailures.Unlock()
	now := time.Now()
	cleanupLoginFailures(now)
	key := loginFailureKey(ip, username)
	item := loginFailures.items[key]
	if item == nil {
		item = &loginFailure{}
		loginFailures.items[key] = item
	}
	item.Count++
	item.LastFail = now
	if item.Count >= maxLoginFailures {
		item.LockedTil = now.Add(loginLockTime)
	}
}

func clearLoginFailures(ip string, username string) {
	loginFailures.Lock()
	defer loginFailures.Unlock()
	delete(loginFailures.items, loginFailureKey(ip, username))
}

func cleanupLoginFailures(now time.Time) {
	for key, item := range loginFailures.items {
		if item.LockedTil.Before(now) && now.Sub(item.LastFail) > loginFailureTTL {
			delete(loginFailures.items, key)
		}
	}
}
