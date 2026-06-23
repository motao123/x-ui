package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/util/random"

	"github.com/google/uuid"
	"golang.org/x/crypto/curve25519"
	"gorm.io/gorm"
)

type QuickNodeService struct {
	inboundService   InboundService
	proxyUserService ProxyUserService
	xrayService      XrayService
}

type QuickRealityRequest struct {
	Remark          string `json:"remark" form:"remark"`
	Port            int    `json:"port" form:"port"`
	Listen          string `json:"listen" form:"listen"`
	Dest            string `json:"dest" form:"dest"`
	ServerName      string `json:"serverName" form:"serverName"`
	ProxyUserName   string `json:"proxyUserName" form:"proxyUserName"`
	CreateProxyUser bool   `json:"createProxyUser" form:"createProxyUser"`
}

type QuickRealityResult struct {
	InboundId       int    `json:"inboundId"`
	ProxyUserId     int    `json:"proxyUserId"`
	SubscriptionUrl string `json:"subscriptionUrl"`
	UUID            string `json:"uuid"`
	PrivateKey      string `json:"privateKey"`
	PublicKey       string `json:"publicKey"`
	ShortId         string `json:"shortId"`
}

func (s *QuickNodeService) CreateVLESSReality(req *QuickRealityRequest, ownerUserId int) (*QuickRealityResult, error) {
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Remark == "" {
		req.Remark = "VLESS Reality"
	}
	req.Dest = strings.TrimSpace(req.Dest)
	if req.Dest == "" {
		req.Dest = "www.microsoft.com:443"
	}
	req.ServerName = strings.TrimSpace(req.ServerName)
	if req.ServerName == "" {
		req.ServerName = strings.Split(req.Dest, ":")[0]
	}
	if req.Port == 0 {
		port, err := s.randomAvailablePort()
		if err != nil {
			return nil, err
		}
		req.Port = port
	}
	keyPair, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, err
	}
	shortId, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	clientUUID := uuid.NewString()
	settings := map[string]interface{}{
		"clients":    []map[string]interface{}{{"id": clientUUID, "flow": "xtls-rprx-vision", "email": req.ProxyUserName}},
		"decryption": "none",
	}
	stream := map[string]interface{}{
		"network":  "tcp",
		"security": "reality",
		"tcpSettings": map[string]interface{}{
			"acceptProxyProtocol": false,
			"header":              map[string]interface{}{"type": "none"},
		},
		"realitySettings": map[string]interface{}{
			"show":        false,
			"dest":        req.Dest,
			"xver":        0,
			"serverNames": []string{req.ServerName},
			"privateKey":  keyPair.PrivateKey,
			"publicKey":   keyPair.PublicKey,
			"shortIds":    []string{shortId},
			"fingerprint": "chrome",
			"maxTimeDiff": 0,
		},
	}
	settingsJSON, _ := json.Marshal(settings)
	streamJSON, _ := json.Marshal(stream)
	inbound := &model.Inbound{
		UserId:         ownerUserId,
		Up:             0,
		Down:           0,
		Total:          0,
		Remark:         req.Remark,
		Enable:         true,
		Listen:         req.Listen,
		Port:           req.Port,
		Protocol:       model.VLESS,
		Settings:       string(settingsJSON),
		StreamSettings: string(streamJSON),
		Sniffing:       `{"enabled":true,"destOverride":["http","tls","quic"]}`,
		Tag:            fmt.Sprintf("inbound-%d", req.Port),
	}
	result := &QuickRealityResult{UUID: clientUUID, PrivateKey: keyPair.PrivateKey, PublicKey: keyPair.PublicKey, ShortId: shortId}
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := s.inboundService.validateInbound(inbound); err != nil {
			return err
		}
		exist, err := s.inboundService.checkPortExist(inbound.Port, 0)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("端口已存在:", inbound.Port)
		}
		if err := tx.Create(inbound).Error; err != nil {
			return err
		}
		result.InboundId = inbound.Id
		if req.CreateProxyUser {
			name := strings.TrimSpace(req.ProxyUserName)
			if name == "" {
				name = req.Remark
			}
			proxyUser := &model.ProxyUser{
				Name:      name,
				Enable:    true,
				Token:     random.Seq(24),
				UUID:      clientUUID,
				Password:  random.Seq(16),
				CreatedAt: nowUnix(),
				UpdatedAt: nowUnix(),
			}
			if err := tx.Create(proxyUser).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.ProxyUserInbound{ProxyUserId: proxyUser.Id, InboundId: inbound.Id}).Error; err != nil {
				return err
			}
			result.ProxyUserId = proxyUser.Id
			result.SubscriptionUrl = "/sub/" + proxyUser.Token
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.xrayService.SetToNeedRestart()
	return result, nil
}

type X25519KeyPair struct {
	PrivateKey string
	PublicKey  string
}

func GenerateX25519KeyPair() (*X25519KeyPair, error) {
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return nil, err
	}
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64
	publicKey, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	return &X25519KeyPair{
		PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey[:]),
		PublicKey:  base64.RawURLEncoding.EncodeToString(publicKey),
	}, nil
}

func (s *QuickNodeService) randomAvailablePort() (int, error) {
	for i := 0; i < 64; i++ {
		portOffset, err := rand.Int(rand.Reader, big.NewInt(40000))
		if err != nil {
			return 0, err
		}
		port := 20000 + int(portOffset.Int64())
		exist, err := s.inboundService.checkPortExist(port, 0)
		if err != nil {
			return 0, err
		}
		if !exist {
			return port, nil
		}
	}
	return 0, common.NewError("无法找到可用端口")
}

func randomHex(bytesLen int) (string, error) {
	data := make([]byte, bytesLen)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

var nowUnix = func() int64 { return time.Now().Unix() }
