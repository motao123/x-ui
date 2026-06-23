package model

import (
	"encoding/json"
	"fmt"
	"x-ui/util/json_util"
	"x-ui/xray"
)

type Protocol string

const (
	VMess       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Dokodemo    Protocol = "dokodemo-door"
	Http        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Socks       Protocol = "socks"
)

type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (User) TableName() string { return "users" }

type Inbound struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	UserId     int    `json:"-"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	Total      int64  `json:"total" form:"total"`
	Remark     string `json:"remark" form:"remark"`
	Enable     bool   `json:"enable" form:"enable"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`

	// config part
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port" gorm:"unique"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

func (Inbound) TableName() string { return "inbounds" }

func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen != "" {
		listen = fmt.Sprintf("\"%v\"", listen)
	}
	streamSettings := i.normalizedStreamSettings()
	settings := i.normalizedSettings(streamSettings)
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(settings),
		StreamSettings: json_util.RawMessage(streamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

func (i *Inbound) normalizedStreamSettings() string {
	if i.StreamSettings == "" {
		return i.StreamSettings
	}
	return i.StreamSettings
}

func (i *Inbound) normalizedSettings(streamSettings string) string {
	if i.Settings == "" || (i.Protocol != VLESS && i.Protocol != Trojan) {
		return i.Settings
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
		return i.Settings
	}

	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(i.Settings), &settings); err != nil {
		return i.Settings
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok {
		return i.Settings
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
		return i.Settings
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return i.Settings
	}
	return string(data)
}

type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

func (Setting) TableName() string { return "settings" }

// TrafficHistory 记录周期性的总流量快照，用于趋势图展示。
type TrafficHistory struct {
	Id       int   `json:"id" gorm:"primaryKey;autoIncrement"`
	Up       int64 `json:"up"`
	Down     int64 `json:"down"`
	RecordAt int64 `json:"recordAt" gorm:"index"`
}

func (TrafficHistory) TableName() string { return "traffic_histories" }
