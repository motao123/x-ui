package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"

	"gorm.io/gorm"
)

const warpAPIBase = "https://api.cloudflareclient.com/v0a4005"
const warpUserAgent = "okhttp/3.12.1"

type WarpService struct {
	endpointService EndpointService
}

type WarpData struct {
	Address       string `json:"address"`
	Reserved      string `json:"reserved"`
	PublicKey     string `json:"publicKey"`
	PrivateKey    string `json:"privateKey"`
	PeerEndpoint  string `json:"peerEndpoint"`
	PeerPublicKey string `json:"peerPublicKey"`
	DeviceName    string `json:"deviceName"`
	DeviceModel   string `json:"deviceModel"`
	DeviceEnabled bool   `json:"deviceEnabled"`
	AccountType   string `json:"accountType"`
	Role          string `json:"role"`
	WarpPlusData  int64  `json:"warpPlusData"`
	Quota         int64  `json:"quota"`
	Usage         int64  `json:"usage"`
}

type warpRegisterResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	Account struct {
		License     string `json:"license"`
		AccountType string `json:"account_type"`
		Role        string `json:"role"`
		PremiumData int64  `json:"premium_data"`
		Quota       int64  `json:"quota"`
		Usage       int64  `json:"usage"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				Host string `json:"host"`
			} `json:"endpoint"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

func (s *WarpService) Get() (*model.WarpAccount, error) {
	account := &model.WarpAccount{}
	err := database.GetDB().First(account).Error
	if err == nil {
		return account, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (s *WarpService) Delete() error {
	return database.GetDB().Where("1 = 1").Delete(&model.WarpAccount{}).Error
}

func (s *WarpService) Register() (*WarpData, error) {
	keyPair, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	resp, err := warpDoRequest(http.MethodPost, warpAPIBase+"/reg", "", map[string]interface{}{
		"key":   keyPair.PublicKey,
		"tos":   time.Now().UTC().Format(time.RFC3339),
		"type":  "PC",
		"name":  hostname,
		"model": "x-ui",
	})
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	account := &model.WarpAccount{AccessToken: resp.Token, DeviceId: resp.ID, LicenseKey: resp.Account.License, PublicKey: keyPair.PublicKey, PrivateKey: keyPair.PrivateKey, CreatedAt: now, UpdatedAt: now}
	db := database.GetDB()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.WarpAccount{}).Error; err != nil {
			return err
		}
		return tx.Create(account).Error
	}); err != nil {
		return nil, err
	}
	return buildWarpData(keyPair.PrivateKey, keyPair.PublicKey, resp)
}

func (s *WarpService) Refresh() (*WarpData, error) {
	account, err := s.Get()
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, common.NewError("没有可用的 WARP 账号")
	}
	resp, err := warpDoRequest(http.MethodGet, fmt.Sprintf("%s/reg/%s", warpAPIBase, account.DeviceId), account.AccessToken, nil)
	if err != nil {
		return nil, err
	}
	if resp.Account.License != "" && resp.Account.License != account.LicenseKey {
		account.LicenseKey = resp.Account.License
		account.UpdatedAt = time.Now().Unix()
		_ = database.GetDB().Save(account).Error
	}
	return buildWarpData(account.PrivateKey, account.PublicKey, resp)
}

func (s *WarpService) SetLicense(license string) (*WarpData, error) {
	license = strings.TrimSpace(license)
	if license == "" {
		return nil, common.NewError("License 不能为空")
	}
	account, err := s.Get()
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, common.NewError("没有可用的 WARP 账号")
	}
	resp, err := warpDoRequest(http.MethodPut, fmt.Sprintf("%s/reg/%s/account", warpAPIBase, account.DeviceId), account.AccessToken, map[string]string{"license": license})
	if err != nil {
		return nil, err
	}
	account.LicenseKey = resp.Account.License
	account.UpdatedAt = time.Now().Unix()
	if err := database.GetDB().Save(account).Error; err != nil {
		return nil, err
	}
	return buildWarpData(account.PrivateKey, account.PublicKey, resp)
}

func (s *WarpService) SetAutoUpdate(days int) (*model.WarpAccount, error) {
	if days < 0 || days > 365 {
		return nil, common.NewError("自动更新周期必须在 0 到 365 天之间")
	}
	account, err := s.Get()
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, common.NewError("没有可用的 WARP 账号")
	}
	account.AutoUpdate = days
	account.UpdatedAt = time.Now().Unix()
	return account, database.GetDB().Save(account).Error
}

func (s *WarpService) CreateEndpoint(tag string) (*model.Endpoint, *WarpData, error) {
	data, err := s.Refresh()
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(tag) == "" {
		tag = "warp"
	}
	endpoint := &model.Endpoint{Name: "Cloudflare WARP", Enable: true, Type: "wireguard", Tag: strings.TrimSpace(tag), Address: data.Address, Endpoint: data.PeerEndpoint, SecretKey: data.PrivateKey, PublicKey: data.PeerPublicKey, Reserved: data.Reserved, Mtu: 1420, Sort: 100}
	var existing model.Endpoint
	err = database.GetDB().Where("tag = ?", endpoint.Tag).First(&existing).Error
	if err == nil {
		endpoint.Id = existing.Id
		endpoint.CreatedAt = existing.CreatedAt
	} else if err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}
	if err := s.endpointService.Save(endpoint); err != nil {
		return nil, nil, err
	}
	return endpoint, data, nil
}

func warpDoRequest(method, url, token string, body interface{}) (*warpRegisterResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", warpUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Cloudflare API 返回 %d: %s", resp.StatusCode, string(data))
	}
	result := &warpRegisterResponse{}
	return result, json.Unmarshal(data, result)
}

func buildWarpData(privateKey string, publicKey string, resp *warpRegisterResponse) (*WarpData, error) {
	if len(resp.Config.Peers) == 0 {
		return nil, common.NewError("Cloudflare 响应缺少 peer 信息")
	}
	address := []string{}
	if resp.Config.Interface.Addresses.V4 != "" {
		address = append(address, resp.Config.Interface.Addresses.V4+"/32")
	}
	if resp.Config.Interface.Addresses.V6 != "" {
		address = append(address, resp.Config.Interface.Addresses.V6+"/128")
	}
	addressData, err := json.Marshal(address)
	if err != nil {
		return nil, err
	}
	reserved, err := parseWarpReserved(resp.Config.ClientID)
	if err != nil {
		return nil, err
	}
	peer := resp.Config.Peers[0]
	return &WarpData{Address: string(addressData), Reserved: reserved, PrivateKey: privateKey, PublicKey: publicKey, PeerEndpoint: peer.Endpoint.Host, PeerPublicKey: peer.PublicKey, DeviceName: resp.Name, DeviceModel: resp.Model, DeviceEnabled: resp.Enabled, AccountType: resp.Account.AccountType, Role: resp.Account.Role, WarpPlusData: resp.Account.PremiumData, Quota: resp.Account.Quota, Usage: resp.Account.Usage}, nil
}

func parseWarpReserved(clientID string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(clientID)
	if err != nil {
		return "", err
	}
	if len(raw) < 3 {
		return "", common.NewError("client_id 长度无效")
	}
	data, err := json.Marshal([]int{int(raw[0]), int(raw[1]), int(raw[2])})
	return string(data), err
}
