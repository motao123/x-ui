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

type RouteRuleService struct{}

func (s *RouteRuleService) List() ([]*model.RouteRule, error) {
	var rules []*model.RouteRule
	err := database.GetDB().Order("sort asc, id asc").Find(&rules).Error
	return rules, err
}

func (s *RouteRuleService) Save(rule *model.RouteRule) error {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		return common.NewError("规则名称不能为空")
	}
	rule.OutboundTag = strings.TrimSpace(rule.OutboundTag)
	if rule.OutboundTag == "" {
		rule.OutboundTag = "blocked"
	}
	if rule.Domain == "" && rule.Ip == "" && rule.Protocol == "" && rule.InboundTag == "" {
		return common.NewError("至少填写一个匹配条件")
	}
	if err := validateCSVJSON(rule.Domain, "domain"); err != nil {
		return err
	}
	if err := validateCSVJSON(rule.Ip, "ip"); err != nil {
		return err
	}
	if err := validateCSVJSON(rule.Protocol, "protocol"); err != nil {
		return err
	}
	if err := validateCSVJSON(rule.InboundTag, "inboundTag"); err != nil {
		return err
	}
	now := time.Now().Unix()
	if rule.Id == 0 {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now
	return database.GetDB().Save(rule).Error
}

func (s *RouteRuleService) Delete(id int) error {
	return database.GetDB().Delete(&model.RouteRule{}, id).Error
}

func (s *RouteRuleService) CompileRouting(template json_util.RawMessage) (json_util.RawMessage, error) {
	rules, err := s.List()
	if err != nil {
		return template, err
	}
	return compileRouting(template, rules)
}

func compileRouting(template json_util.RawMessage, rules []*model.RouteRule) (json_util.RawMessage, error) {
	routing := map[string]interface{}{}
	if len(template) > 0 {
		if err := json.Unmarshal(template, &routing); err != nil {
			return template, err
		}
	}
	if routing == nil {
		routing = map[string]interface{}{}
	}
	existing, _ := routing["rules"].([]interface{})
	compiled := make([]interface{}, 0, len(existing)+len(rules))
	compiled = append(compiled, existing...)
	for _, rule := range rules {
		if !rule.Enable {
			continue
		}
		item := map[string]interface{}{"type": "field", "outboundTag": rule.OutboundTag}
		if values := csvValues(rule.Domain); len(values) > 0 {
			item["domain"] = values
		}
		if values := csvValues(rule.Ip); len(values) > 0 {
			item["ip"] = values
		}
		if values := csvValues(rule.Protocol); len(values) > 0 {
			item["protocol"] = values
		}
		if values := csvValues(rule.InboundTag); len(values) > 0 {
			item["inboundTag"] = values
		}
		compiled = append(compiled, item)
	}
	routing["rules"] = compiled
	data, err := json.Marshal(routing)
	if err != nil {
		return template, err
	}
	return json_util.RawMessage(data), nil
}

func csvValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func validateCSVJSON(raw string, field string) error {
	for _, value := range csvValues(raw) {
		if strings.ContainsAny(value, "\x00\n\r") {
			return common.NewError(field, "包含非法字符")
		}
	}
	return nil
}
