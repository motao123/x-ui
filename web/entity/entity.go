package entity

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"time"
	"x-ui/util/common"
	"x-ui/xray"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

type PublicError interface {
	error
	PublicMessage() string
}

type Pager struct {
	Current  int         `json:"current"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	OrderBy  string      `json:"order_by"`
	Desc     bool        `json:"desc"`
	Key      string      `json:"key"`
	List     interface{} `json:"list"`
}

type AllSetting struct {
	WebListen          string `json:"webListen" form:"webListen"`
	WebPort            int    `json:"webPort" form:"webPort"`
	WebCertFile        string `json:"webCertFile" form:"webCertFile"`
	WebKeyFile         string `json:"webKeyFile" form:"webKeyFile"`
	WebBasePath        string `json:"webBasePath" form:"webBasePath"`
	WebTrustedProxies  string `json:"webTrustedProxies" form:"webTrustedProxies"`
	SubscriptionHost   string `json:"subscriptionHost" form:"subscriptionHost"`
	TgBotEnable        bool   `json:"tgBotEnable" form:"tgBotEnable"`
	TgBotToken         string `json:"tgBotToken" form:"tgBotToken"`
	TgBotChatId        int    `json:"tgBotChatId" form:"tgBotChatId"`
	TgRunTime          string `json:"tgRunTime" form:"tgRunTime"`
	XrayTemplateConfig string `json:"xrayTemplateConfig" form:"xrayTemplateConfig"`

	TimeLocation string `json:"timeLocation" form:"timeLocation"`
}

func (s *AllSetting) CheckValid() error {
	if s.WebListen != "" {
		ip := net.ParseIP(s.WebListen)
		if ip == nil {
			return common.NewError("web listen is not valid ip:", s.WebListen)
		}
	}

	if s.WebPort <= 0 || s.WebPort > 65535 {
		return common.NewError("web port is not a valid port:", s.WebPort)
	}

	if (s.WebCertFile == "") != (s.WebKeyFile == "") {
		return common.NewError("cert file and key file must be configured together")
	}

	if s.WebCertFile != "" {
		if !isSafeAbsPath(s.WebCertFile) || !isSafeAbsPath(s.WebKeyFile) {
			return common.NewError("cert file or key file path invalid")
		}
		_, err := tls.LoadX509KeyPair(s.WebCertFile, s.WebKeyFile)
		if err != nil {
			return common.NewError("cert file or key file invalid")
		}
	}

	if !strings.HasPrefix(s.WebBasePath, "/") {
		s.WebBasePath = "/" + s.WebBasePath
	}
	if !strings.HasSuffix(s.WebBasePath, "/") {
		s.WebBasePath += "/"
	}
	if strings.ContainsAny(s.WebBasePath, "?#") || strings.Contains(s.WebBasePath, "//") || strings.Contains(s.WebBasePath, "..") {
		return common.NewError("web base path is invalid:", s.WebBasePath)
	}

	if err := validateTrustedProxies(s.WebTrustedProxies); err != nil {
		return err
	}

	if err := validateSubscriptionHost(s.SubscriptionHost); err != nil {
		return err
	}

	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(s.XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}

	_, err = time.LoadLocation(s.TimeLocation)
	if err != nil {
		return common.NewError("time location not exist:", s.TimeLocation)
	}

	return nil
}

func isSafeAbsPath(path string) bool {
	if path == "" || strings.ContainsRune(path, '\x00') || !filepath.IsAbs(path) {
		return false
	}
	cleaned := filepath.Clean(path)
	return cleaned == path && !strings.Contains(path, "..")
}

func validateSubscriptionHost(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "/?#@\x00") {
		return common.NewError("subscription public host is invalid:", raw)
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	if host == "" {
		return common.NewError("subscription public host is invalid:", raw)
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return common.NewError("subscription public host is invalid:", raw)
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return common.NewError("subscription public host is invalid:", raw)
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return common.NewError("subscription public host is invalid:", raw)
		}
	}
	return nil
}

// validateTrustedProxies 校验 webTrustedProxies 字段：空值合法（不信任任何代理），
// 非空时必须是逗号分隔的合法 IP 或 CIDR 列表。
func validateTrustedProxies(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, _, err := net.ParseCIDR(item); err != nil {
				return common.NewError("web trusted proxies contains invalid CIDR:", item)
			}
			continue
		}
		if net.ParseIP(item) == nil {
			return common.NewError("web trusted proxies contains invalid IP:", item)
		}
	}
	return nil
}

// ParseTrustedProxies 将逗号分隔的可信代理配置解析为 CIDR 列表。
// 用于每次请求时回溯 X-Forwarded-For。空配置返回 nil。
func ParseTrustedProxies(raw string) []*net.IPNet {
	var nets []*net.IPNet
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			if _, ipnet, err := net.ParseCIDR(item); err == nil {
				nets = append(nets, ipnet)
			}
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			nets = append(nets, singleIPNet(ip))
		}
	}
	return nets
}

func singleIPNet(ip net.IP) *net.IPNet {
	var bits int
	if ip.To4() != nil {
		bits = 32
	} else {
		bits = 128
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
}

// ResolveClientIP 根据受信代理网段从 X-Forwarded-For 链中回溯出真实客户端 IP。
// remoteAddr 为 TCP 直连对端地址。如果直连对端不在受信代理集合内，直接返回其对端 IP，
// 忽略任何 XFF 头，避免伪造。
func ResolveClientIP(remoteAddr string, trustedProxies []*net.IPNet, xff string) string {
	peer := hostFromAddr(remoteAddr)
	if len(trustedProxies) == 0 {
		return peer
	}
	if peer == "" || !ipInAny(peer, trustedProxies) {
		// 直连对端不是受信代理，XFF 不可信
		return peer
	}
	// XFF 形如 "client, proxy1, proxy2"；从右向左跳过受信代理，第一个非可信即真实客户端
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if candidate == "" {
			continue
		}
		if ipInAny(candidate, trustedProxies) {
			continue
		}
		return candidate
	}
	return peer
}

func ipInAny(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func hostFromAddr(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	return host
}
