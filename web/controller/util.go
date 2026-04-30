package controller

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"x-ui/config"
	"x-ui/logger"
	"x-ui/web/entity"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

func getUriId(c *gin.Context) int64 {
	s := struct {
		Id int64 `uri:"id"`
	}{}

	_ = c.BindUri(&s)
	return s.Id
}

func getRemoteIp(c *gin.Context) string {
	addr := c.Request.RemoteAddr
	ip, _, err := net.SplitHostPort(addr)
	if err == nil && ip != "" {
		return ip
	}
	return addr
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := entity.Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg + "成功"
		}
	} else {
		m.Success = false
		m.Msg = msg + "失败: " + safeErrorMessage(err)
		logger.Warning(msg+"失败: ", err)
	}
	c.JSON(http.StatusOK, m)
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var publicErr entity.PublicError
	if errors.As(err, &publicErr) {
		return publicErr.PublicMessage()
	}
	return "请求处理失败，请检查输入或查看服务端日志"
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, entity.Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, entity.Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func html(c *gin.Context, name string, title string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["title"] = title
	data["request_uri"] = c.Request.RequestURI
	data["base_path"] = c.GetString("base_path")
	csrfToken, err := session.GetCSRFToken(c)
	if err != nil {
		logger.Warning("failed to create csrf token: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	data["csrf_token"] = csrfToken
	c.HTML(http.StatusOK, name, getContext(data))
}

func getContext(h gin.H) gin.H {
	a := gin.H{
		"cur_ver": config.GetVersion(),
	}
	if h != nil {
		for key, value := range h {
			a[key] = value
		}
	}
	return a
}

func isAjax(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

func securityLog(c *gin.Context, event string, success bool, fields ...interface{}) {
	userID := "anonymous"
	if user := session.GetLoginUser(c); user != nil {
		userID = fmt.Sprint(user.Id)
	}
	args := []interface{}{"security_event=", event, " success=", success, " user_id=", userID, " ip=", getRemoteIp(c)}
	args = append(args, fields...)
	logger.Info(args...)
}
