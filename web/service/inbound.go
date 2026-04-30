package service

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/xray"

	"gorm.io/gorm"
)

type InboundService struct {
}

func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) checkPortExist(port int, ignoreId int) (bool, error) {
	db := database.GetDB()
	db = db.Model(model.Inbound{}).Where("port = ?", port)
	if ignoreId > 0 {
		db = db.Where("id != ?", ignoreId)
	}
	var count int64
	err := db.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *InboundService) AddInbound(inbound *model.Inbound) error {
	if err := s.validateInbound(inbound); err != nil {
		return err
	}
	exist, err := s.checkPortExist(inbound.Port, 0)
	if err != nil {
		return err
	}
	if exist {
		return common.NewError("端口已存在:", inbound.Port)
	}
	db := database.GetDB()
	return db.Save(inbound).Error
}

func (s *InboundService) AddInbounds(inbounds []*model.Inbound) error {
	for _, inbound := range inbounds {
		if err := s.validateInbound(inbound); err != nil {
			return err
		}
		exist, err := s.checkPortExist(inbound.Port, 0)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("端口已存在:", inbound.Port)
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, inbound := range inbounds {
		err = tx.Save(inbound).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InboundService) DelInbound(id int, userId int) error {
	db := database.GetDB()
	result := db.Where("id = ? and user_id = ?", id, userId).Delete(model.Inbound{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (s *InboundService) GetInboundForUser(id int, userId int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).Where("id = ? and user_id = ?", id, userId).First(inbound).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (s *InboundService) UpdateInbound(inbound *model.Inbound, userId int) error {
	if err := s.validateInbound(inbound); err != nil {
		return err
	}
	exist, err := s.checkPortExist(inbound.Port, inbound.Id)
	if err != nil {
		return err
	}
	if exist {
		return common.NewError("端口已存在:", inbound.Port)
	}

	oldInbound, err := s.GetInboundForUser(inbound.Id, userId)
	if err != nil {
		return err
	}
	oldInbound.Up = inbound.Up
	oldInbound.Down = inbound.Down
	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime
	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	oldInbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)

	db := database.GetDB()
	return db.Save(oldInbound).Error
}

func (s *InboundService) validateInbound(inbound *model.Inbound) error {
	if inbound.Port <= 0 || inbound.Port > 65535 {
		return common.NewError("端口不合法:", inbound.Port)
	}
	if inbound.Listen != "" && net.ParseIP(inbound.Listen) == nil {
		return common.NewError("监听地址不合法:", inbound.Listen)
	}
	protocols := map[model.Protocol]bool{
		model.VMess:       true,
		model.VLESS:       true,
		model.Dokodemo:    true,
		model.Http:        true,
		model.Trojan:      true,
		model.Shadowsocks: true,
		model.Socks:       true,

	}
	if !protocols[inbound.Protocol] {
		return common.NewError("协议不支持:", inbound.Protocol)
	}
	if err := validateJSONObject(inbound.Settings, "settings"); err != nil {
		return err
	}
	if !json.Valid([]byte(inbound.Settings)) {
		return common.NewError("settings 不是合法 JSON")
	}
	if inbound.StreamSettings != "" {
		if err := validateJSONKeys(inbound.StreamSettings, "streamSettings", allowedStreamSettingsKeys); err != nil {
			return err
		}
	}
	if inbound.Sniffing != "" {
		if err := validateJSONKeys(inbound.Sniffing, "sniffing", allowedSniffingKeys); err != nil {
			return err
		}
	}
	return nil
}

var allowedStreamSettingsKeys = map[string]bool{
	"network":             true,
	"security":            true,
	"tcpSettings":         true,
	"kcpSettings":         true,
	"wsSettings":          true,
	"httpSettings":        true,
	"dsSettings":          true,
	"quicSettings":        true,
	"sockopt":             true,
	"tlsSettings":         true,
	"realitySettings":     true,
	"grpcSettings":        true,
	"httpupgradeSettings": true,
	"splithttpSettings":   true,
}

var allowedSniffingKeys = map[string]bool{
	"enabled":      true,
	"destOverride": true,
	"metadataOnly": true,
	"routeOnly":    true,
}

var allowedStreamNetworks = map[string]bool{
	"tcp": true, "kcp": true, "ws": true, "http": true, "domainsocket": true,
	"quic": true, "grpc": true, "httpupgrade": true, "splithttp": true,
}

var allowedStreamSecurity = map[string]bool{
	"none": true, "tls": true, "reality": true, "": true,
}

func validateJSONObject(raw string, name string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return common.NewError(name + " 不是合法 JSON 对象")
	}
	return nil
}

func validateJSONKeys(raw string, name string, allowed map[string]bool) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return common.NewError(name + " 不是合法 JSON 对象")
	}
	for key := range obj {
		if !allowed[key] {
			return common.NewError(name+" 包含不支持的字段: ", key)
		}
	}
	if name == "streamSettings" {
		if err := validateStringEnum(obj, "network", allowedStreamNetworks, "streamSettings.network 不支持"); err != nil {
			return err
		}
		if err := validateStringEnum(obj, "security", allowedStreamSecurity, "streamSettings.security 不支持"); err != nil {
			return err
		}
	}
	return nil
}

func validateStringEnum(obj map[string]json.RawMessage, key string, allowed map[string]bool, msg string) error {
	raw, ok := obj[key]
	if !ok || string(raw) == "null" {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return common.NewError(key + " 必须是字符串")
	}
	if !allowed[value] {
		return common.NewError(msg+": ", value)
	}
	return nil
}

func (s *InboundService) AddTraffic(traffics []*xray.Traffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}
	db := database.GetDB()
	db = db.Model(model.Inbound{})
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Where("tag = ?", traffic.Tag).
				UpdateColumn("up", gorm.Expr("up + ?", traffic.Up)).
				UpdateColumn("down", gorm.Expr("down + ?", traffic.Down)).
				Error
			if err != nil {
				return
			}
		}
	}
	return
}

func (s *InboundService) DisableInvalidInbounds() (int64, error) {
	db := database.GetDB()
	now := time.Now().Unix() * 1000
	result := db.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return count, err
}
