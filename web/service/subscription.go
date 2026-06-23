package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
)

type SubscriptionService struct {
	proxyUserService ProxyUserService
	inboundService   InboundService
}

func (s *SubscriptionService) Render(token string, format string, host string, userAgent string, remoteIp string) (string, string, error) {
	proxyUser, err := s.proxyUserService.GetByToken(token)
	if err != nil {
		return "", "", err
	}
	if !proxyUser.Enable || (proxyUser.ExpiryTime > 0 && proxyUser.ExpiryTime < time.Now().UnixMilli()) {
		return "", "", fmt.Errorf("subscription is disabled or expired")
	}
	links, err := s.links(proxyUser, host)
	if err != nil {
		return "", "", err
	}
	_ = database.GetDB().Create(&model.SubscriptionAccess{
		ProxyUserId: proxyUser.Id,
		Format:      format,
		UserAgent:   userAgent,
		RemoteIp:    remoteIp,
		AccessedAt:  time.Now().Unix(),
	}).Error
	body := strings.Join(links, "\n")
	switch strings.ToLower(format) {
	case "", "base64", "b64":
		return base64.StdEncoding.EncodeToString([]byte(body)), "text/plain; charset=utf-8", nil
	case "plain", "raw":
		return body, "text/plain; charset=utf-8", nil
	default:
		return "", "", fmt.Errorf("unsupported subscription format: %s", format)
	}
}

func (s *SubscriptionService) links(proxyUser *model.ProxyUser, host string) ([]string, error) {
	ids, err := s.proxyUserService.GetInboundIds(proxyUser.Id)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(ids))
	for _, id := range ids {
		inbound, err := s.inboundService.GetInbound(id)
		if err != nil || inbound == nil || !inbound.Enable {
			continue
		}
		if link := buildSubscriptionLink(proxyUser, inbound, host); link != "" {
			links = append(links, link)
		}
	}
	return links, nil
}

func buildSubscriptionLink(proxyUser *model.ProxyUser, inbound *model.Inbound, host string) string {
	address := host
	if inbound.Listen != "" && inbound.Listen != "0.0.0.0" && inbound.Listen != "::" {
		address = inbound.Listen
	}
	remark := url.QueryEscape(proxyUser.Name + "-" + inbound.Remark)
	switch inbound.Protocol {
	case model.VLESS:
		return buildVLESSLink(proxyUser, inbound, address, remark)
	case model.VMess:
		return buildVMessLink(proxyUser, inbound, address)
	case model.Trojan:
		return buildTrojanLink(proxyUser, inbound, address, remark)
	case model.Shadowsocks:
		return buildSSLink(proxyUser, inbound, address, remark)
	default:
		return ""
	}
}

func buildVLESSLink(proxyUser *model.ProxyUser, inbound *model.Inbound, address string, remark string) string {
	query := url.Values{}
	query.Set("type", streamNetwork(inbound))
	query.Set("security", streamSecurity(inbound))
	if flow := firstClientString(inbound.Settings, "flow"); flow != "" {
		query.Set("flow", flow)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", proxyUser.UUID, address, inbound.Port, query.Encode(), remark)
}

func buildVMessLink(proxyUser *model.ProxyUser, inbound *model.Inbound, address string) string {
	payload := map[string]interface{}{
		"v":    "2",
		"ps":   proxyUser.Name + "-" + inbound.Remark,
		"add":  address,
		"port": fmt.Sprint(inbound.Port),
		"id":   proxyUser.UUID,
		"aid":  "0",
		"net":  streamNetwork(inbound),
		"type": "none",
		"host": "",
		"path": "",
		"tls":  streamSecurity(inbound),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data)
}

func buildTrojanLink(proxyUser *model.ProxyUser, inbound *model.Inbound, address string, remark string) string {
	password := proxyUser.Password
	if password == "" {
		password = firstClientString(inbound.Settings, "password")
	}
	query := url.Values{}
	if security := streamSecurity(inbound); security != "" {
		query.Set("security", security)
	}
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", url.QueryEscape(password), address, inbound.Port, query.Encode(), remark)
}

func buildSSLink(proxyUser *model.ProxyUser, inbound *model.Inbound, address string, remark string) string {
	var settings struct {
		Method   string `json:"method"`
		Password string `json:"password"`
	}
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	password := proxyUser.Password
	if password == "" {
		password = settings.Password
	}
	if settings.Method == "" || password == "" {
		return ""
	}
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(settings.Method + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, address, inbound.Port, remark)
}

func streamNetwork(inbound *model.Inbound) string {
	var stream map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		return "tcp"
	}
	if value, ok := stream["network"].(string); ok && value != "" {
		return value
	}
	return "tcp"
}

func streamSecurity(inbound *model.Inbound) string {
	var stream map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		return "none"
	}
	if value, ok := stream["security"].(string); ok && value != "" {
		return value
	}
	return "none"
}

func firstClientString(settingsText string, key string) string {
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(settingsText), &settings); err != nil {
		return ""
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok || len(clients) == 0 {
		return ""
	}
	client, ok := clients[0].(map[string]interface{})
	if !ok {
		return ""
	}
	value, _ := client[key].(string)
	return value
}
