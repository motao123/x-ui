package service

import (
	"encoding/json"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/util/json_util"
)

type EndpointService struct{}

func (s *EndpointService) List() ([]*model.Endpoint, error) {
	var endpoints []*model.Endpoint
	err := database.GetDB().Order("sort asc, id asc").Find(&endpoints).Error
	return endpoints, err
}

func (s *EndpointService) Save(endpoint *model.Endpoint) error {
	endpoint.Name = strings.TrimSpace(endpoint.Name)
	endpoint.Type = strings.TrimSpace(endpoint.Type)
	endpoint.Tag = strings.TrimSpace(endpoint.Tag)
	endpoint.Address = strings.TrimSpace(endpoint.Address)
	endpoint.Endpoint = strings.TrimSpace(endpoint.Endpoint)
	endpoint.SecretKey = strings.TrimSpace(endpoint.SecretKey)
	endpoint.PublicKey = strings.TrimSpace(endpoint.PublicKey)
	endpoint.Reserved = strings.TrimSpace(endpoint.Reserved)
	endpoint.Settings = strings.TrimSpace(endpoint.Settings)
	if endpoint.Name == "" {
		return common.NewError("端点名称不能为空")
	}
	if endpoint.Type == "" {
		endpoint.Type = "wireguard"
	}
	if endpoint.Type != "wireguard" && endpoint.Type != "custom" {
		return common.NewError("端点类型不支持")
	}
	if endpoint.Tag == "" {
		return common.NewError("出站 Tag 不能为空")
	}
	if strings.ContainsAny(endpoint.Tag, "\x00\n\r\t ") {
		return common.NewError("出站 Tag 不能包含空白或控制字符")
	}
	if endpoint.Type == "wireguard" {
		if endpoint.SecretKey == "" {
			return common.NewError("WireGuard SecretKey 不能为空")
		}
		if endpoint.Address == "" {
			return common.NewError("WireGuard Address 不能为空")
		}
		if endpoint.Mtu == 0 {
			endpoint.Mtu = 1420
		}
		if endpoint.Mtu < 576 || endpoint.Mtu > 9000 {
			return common.NewError("MTU 必须在 576 到 9000 之间")
		}
	}
	if endpoint.Settings != "" && !json.Valid([]byte(endpoint.Settings)) {
		return common.NewError("高级配置必须是合法 JSON")
	}
	now := time.Now().Unix()
	if endpoint.Id == 0 {
		endpoint.CreatedAt = now
	}
	endpoint.UpdatedAt = now
	return database.GetDB().Save(endpoint).Error
}

func (s *EndpointService) Delete(id int) error {
	return database.GetDB().Delete(&model.Endpoint{}, id).Error
}

func (s *EndpointService) CompileOutbounds(template json_util.RawMessage) (json_util.RawMessage, error) {
	endpoints, err := s.List()
	if err != nil {
		return template, err
	}
	return compileOutbounds(template, endpoints)
}

func compileOutbounds(template json_util.RawMessage, endpoints []*model.Endpoint) (json_util.RawMessage, error) {
	outbounds := make([]interface{}, 0)
	if len(template) > 0 {
		if err := json.Unmarshal(template, &outbounds); err != nil {
			return template, err
		}
	}
	for _, endpoint := range endpoints {
		if !endpoint.Enable {
			continue
		}
		outbound, err := buildEndpointOutbound(endpoint)
		if err != nil {
			return template, err
		}
		outbounds = append(outbounds, outbound)
	}
	data, err := json.Marshal(outbounds)
	if err != nil {
		return template, err
	}
	return json_util.RawMessage(data), nil
}

func buildEndpointOutbound(endpoint *model.Endpoint) (map[string]interface{}, error) {
	if endpoint.Type == "custom" {
		outbound := map[string]interface{}{}
		if endpoint.Settings == "" {
			return nil, common.NewError("自定义端点配置不能为空")
		}
		if err := json.Unmarshal([]byte(endpoint.Settings), &outbound); err != nil {
			return nil, err
		}
		outbound["tag"] = endpoint.Tag
		return outbound, nil
	}
	settings := map[string]interface{}{
		"secretKey": endpoint.SecretKey,
		"address":   csvValues(endpoint.Address),
		"mtu":       endpoint.Mtu,
	}
	if endpoint.PublicKey != "" || endpoint.Endpoint != "" || endpoint.Reserved != "" {
		peer := map[string]interface{}{}
		if endpoint.PublicKey != "" {
			peer["publicKey"] = endpoint.PublicKey
		}
		if endpoint.Endpoint != "" {
			peer["endpoint"] = endpoint.Endpoint
		}
		if reserved := intCSVValues(endpoint.Reserved); len(reserved) > 0 {
			peer["reserved"] = reserved
		}
		settings["peers"] = []interface{}{peer}
	}
	if endpoint.Settings != "" {
		extra := map[string]interface{}{}
		if err := json.Unmarshal([]byte(endpoint.Settings), &extra); err != nil {
			return nil, err
		}
		for key, value := range extra {
			settings[key] = value
		}
	}
	return map[string]interface{}{
		"tag":      endpoint.Tag,
		"protocol": "wireguard",
		"settings": settings,
	}, nil
}

func intCSVValues(raw string) []int {
	parts := strings.Split(raw, ",")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		var item int
		if err := json.Unmarshal([]byte(value), &item); err == nil {
			values = append(values, item)
		}
	}
	return values
}
