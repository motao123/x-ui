package configbuilder

import (
	"encoding/json"
	"fmt"
	"strings"
	"x-ui/database/model"
	"x-ui/util/json_util"
	"x-ui/xray"
)

func BuildInboundConfig(inbound *model.Inbound) *xray.InboundConfig {
	return BuildInboundConfigWithUsers(inbound, nil)
}

func BuildInboundConfigWithUsers(inbound *model.Inbound, users []*model.ProxyUser) *xray.InboundConfig {
	listen := inbound.Listen
	if listen != "" {
		listen = fmt.Sprintf("\"%v\"", listen)
	}
	streamSettings := normalizedStreamSettings(inbound)
	settings := NormalizedSettings(inbound.Protocol, inbound.Settings, streamSettings)
	settings = injectProxyUsers(inbound.Protocol, settings, users)
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           inbound.Port,
		Protocol:       string(inbound.Protocol),
		Settings:       json_util.RawMessage(settings),
		StreamSettings: json_util.RawMessage(streamSettings),
		Tag:            inbound.Tag,
		Sniffing:       json_util.RawMessage(inbound.Sniffing),
	}
}

func normalizedStreamSettings(inbound *model.Inbound) string {
	return inbound.StreamSettings
}

func injectProxyUsers(protocol model.Protocol, settingsText string, users []*model.ProxyUser) string {
	if len(users) == 0 || settingsText == "" {
		return settingsText
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(settingsText), &settings); err != nil {
		return settingsText
	}
	switch protocol {
	case model.VLESS:
		clients := make([]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{"id": user.UUID, "email": user.Name})
		}
		settings["clients"] = clients
		if _, ok := settings["decryption"]; !ok {
			settings["decryption"] = "none"
		}
	case model.VMess:
		clients := make([]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{"id": user.UUID, "email": user.Name, "alterId": 0})
		}
		settings["clients"] = clients
	case model.Trojan:
		clients := make([]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{"password": user.Password, "email": user.Name})
		}
		settings["clients"] = clients
	case model.Shadowsocks:
		if len(users) == 1 {
			settings["password"] = users[0].Password
		} else {
			clients := make([]interface{}, 0, len(users))
			for _, user := range users {
				clients = append(clients, map[string]interface{}{"password": user.Password, "email": user.Name})
			}
			settings["clients"] = clients
		}
	default:
		return settingsText
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return settingsText
	}
	return string(data)
}

func NormalizedSettings(protocol model.Protocol, settingsText string, streamSettings string) string {
	if settingsText == "" || (protocol != model.VLESS && protocol != model.Trojan) {
		return settingsText
	}
	security := ""
	if streamSettings != "" {
		var stream map[string]interface{}
		if err := json.Unmarshal([]byte(streamSettings), &stream); err == nil {
			if value, ok := stream["security"].(string); ok {
				security = value
			}
		}
	}
	if security == "xtls" || security == "reality" {
		return settingsText
	}

	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(settingsText), &settings); err != nil {
		return settingsText
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok {
		return settingsText
	}
	changed := false
	for _, item := range clients {
		client, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := client["flow"]; ok {
			delete(client, "flow")
			changed = true
		}
	}
	if !changed {
		return settingsText
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return settingsText
	}
	return strings.TrimSpace(string(data))
}
