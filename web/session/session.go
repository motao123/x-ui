package session

import (
	"crypto/subtle"
	"encoding/gob"
	"time"

	"x-ui/database/model"
	"x-ui/util/random"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUser     = "LOGIN_USER"
	csrfToken     = "CSRF_TOKEN"
	loginTime     = "LOGIN_TIME"
	lastActive    = "LAST_ACTIVE"
	sessionMaxAge = 24 * time.Hour
	idleTimeout   = 6 * time.Hour
)

func init() {
	gob.Register(model.User{})
}

func SetLoginUser(c *gin.Context, user *model.User) error {
	s := sessions.Default(c)
	s.Clear()
	safeUser := *user
	safeUser.Password = ""
	s.Set(loginUser, safeUser)
	now := time.Now().Unix()
	s.Set(loginTime, now)
	s.Set(lastActive, now)
	s.Set(csrfToken, random.Seq(32))
	return s.Save()
}

func GetLoginUser(c *gin.Context) *model.User {
	s := sessions.Default(c)
	obj := s.Get(loginUser)
	if obj == nil {
		return nil
	}
	user := obj.(model.User)
	return &user
}

func IsLogin(c *gin.Context) bool {
	if GetLoginUser(c) == nil {
		return false
	}
	s := sessions.Default(c)
	now := time.Now()
	loginAt, ok := sessionUnixTime(s.Get(loginTime))
	if !ok || now.Sub(loginAt) > sessionMaxAge {
		ClearSession(c, c.GetString("base_path"))
		return false
	}
	activeAt, ok := sessionUnixTime(s.Get(lastActive))
	if !ok || now.Sub(activeAt) > idleTimeout {
		ClearSession(c, c.GetString("base_path"))
		return false
	}
	s.Set(lastActive, now.Unix())
	_ = s.Save()
	return true
}

func sessionUnixTime(obj interface{}) (time.Time, bool) {
	switch v := obj.(type) {
	case int64:
		return time.Unix(v, 0), true
	case int:
		return time.Unix(int64(v), 0), true
	case int32:
		return time.Unix(int64(v), 0), true
	case float64:
		return time.Unix(int64(v), 0), true
	default:
		return time.Time{}, false
	}
}

func ClearSession(c *gin.Context, path string) {
	s := sessions.Default(c)
	s.Clear()
	s.Options(sessions.Options{
		Path:   path,
		MaxAge: -1,
	})
	s.Save()
}

func GetCSRFToken(c *gin.Context) (string, error) {
	s := sessions.Default(c)
	if obj := s.Get(csrfToken); obj != nil {
		if token, ok := obj.(string); ok && token != "" {
			return token, nil
		}
	}

	token := random.Seq(32)
	s.Set(csrfToken, token)
	return token, s.Save()
}

func ValidateCSRFToken(c *gin.Context, token string) bool {
	if token == "" {
		return false
	}
	s := sessions.Default(c)
	obj := s.Get(csrfToken)
	expected, ok := obj.(string)
	if !ok || expected == "" || len(expected) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}
