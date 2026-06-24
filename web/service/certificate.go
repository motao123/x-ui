package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

const certificateBaseDir = "/etc/x-ui/certificates"

var certificateNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

type CertificateService struct {
	settingService SettingService
	taskService    TaskService
}

type CertificateUploadRequest struct {
	Name    string `json:"name" form:"name"`
	Domain  string `json:"domain" form:"domain"`
	CertPEM string `json:"certPem" form:"certPem"`
	KeyPEM  string `json:"keyPem" form:"keyPem"`
}

type CertificateApplyRequest struct {
	Name      string `json:"name" form:"name"`
	Domain    string `json:"domain" form:"domain"`
	Mode      string `json:"mode" form:"mode"`
	AcmeId    int    `json:"acmeId" form:"acmeId"`
	DnsId     int    `json:"dnsId" form:"dnsId"`
	AutoRenew bool   `json:"autoRenew" form:"autoRenew"`
}

type acmeRuntimeUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	Key          crypto.PrivateKey      `json:"-"`
}

func (u *acmeRuntimeUser) GetEmail() string                        { return u.Email }
func (u *acmeRuntimeUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *acmeRuntimeUser) GetPrivateKey() crypto.PrivateKey        { return u.Key }

func (s *CertificateService) List() ([]*model.Certificate, error) {
	var certificates []*model.Certificate
	err := database.GetDB().Order("id desc").Find(&certificates).Error
	return certificates, err
}

func (s *CertificateService) ListAcmeAccounts() ([]*model.AcmeAccount, error) {
	var accounts []*model.AcmeAccount
	err := database.GetDB().Order("id desc").Find(&accounts).Error
	return accounts, err
}

func (s *CertificateService) SaveAcmeAccount(account *model.AcmeAccount) error {
	account.Name = strings.TrimSpace(account.Name)
	account.Email = strings.TrimSpace(account.Email)
	account.Provider = strings.TrimSpace(account.Provider)
	if account.Name == "" || !certificateNameRegexp.MatchString(account.Name) {
		return common.NewError("ACME 名称只能包含字母、数字、点、下划线和中划线")
	}
	if _, err := mail.ParseAddress(account.Email); err != nil {
		return common.NewError("ACME 邮箱格式不正确")
	}
	if account.Provider == "" {
		account.Provider = "letsencrypt"
	}
	if account.Provider != "letsencrypt" && account.Provider != "zerossl" {
		return common.NewError("ACME 服务商不支持")
	}
	now := time.Now().Unix()
	if account.Id == 0 {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		account.PrivateKey = string(certcrypto.PEMEncode(key))
		account.CreatedAt = now
	} else {
		old := &model.AcmeAccount{}
		if err := database.GetDB().First(old, account.Id).Error; err != nil {
			return err
		}
		account.PrivateKey = old.PrivateKey
		account.CreatedAt = old.CreatedAt
	}
	account.UpdatedAt = now
	return database.GetDB().Save(account).Error
}

func (s *CertificateService) DeleteAcmeAccount(id int) error {
	return database.GetDB().Delete(&model.AcmeAccount{}, id).Error
}

func (s *CertificateService) ListDnsAccounts() ([]*model.DnsAccount, error) {
	var accounts []*model.DnsAccount
	err := database.GetDB().Order("id desc").Find(&accounts).Error
	return accounts, err
}

func (s *CertificateService) SaveDnsAccount(account *model.DnsAccount) error {
	account.Name = strings.TrimSpace(account.Name)
	account.Provider = strings.TrimSpace(account.Provider)
	account.Key = strings.TrimSpace(account.Key)
	account.Secret = strings.TrimSpace(account.Secret)
	if account.Name == "" || !certificateNameRegexp.MatchString(account.Name) {
		return common.NewError("DNS 名称只能包含字母、数字、点、下划线和中划线")
	}
	if account.Provider != "cloudflare" && account.Provider != "aliyun" {
		return common.NewError("DNS 服务商不支持")
	}
	if account.Secret == "" {
		return common.NewError("DNS Secret 不能为空")
	}
	now := time.Now().Unix()
	if account.Id == 0 {
		account.CreatedAt = now
	}
	account.UpdatedAt = now
	return database.GetDB().Save(account).Error
}

func (s *CertificateService) DeleteDnsAccount(id int) error {
	return database.GetDB().Delete(&model.DnsAccount{}, id).Error
}

func (s *CertificateService) ApplyAsync(req *CertificateApplyRequest) (*Task, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Domain = strings.TrimSpace(req.Domain)
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Name == "" || !certificateNameRegexp.MatchString(req.Name) {
		return nil, common.NewError("证书名称只能包含字母、数字、点、下划线和中划线")
	}
	if req.Domain == "" {
		return nil, common.NewError("域名不能为空")
	}
	if req.Mode == "" {
		req.Mode = "http"
	}
	if req.Mode != "http" && req.Mode != "dns" {
		return nil, common.NewError("申请方式不支持")
	}
	if req.AcmeId == 0 {
		return nil, common.NewError("请选择 ACME 账号")
	}
	task := s.taskService.Start("申请证书 "+req.Domain, func(task *Task) {
		if err := s.applyCertificate(req, task); err != nil {
			task.Fail(err.Error())
			return
		}
		task.Done("证书申请完成")
	})
	return task, nil
}

func (s *CertificateService) Get(id int) (*model.Certificate, error) {
	certificate := &model.Certificate{}
	err := database.GetDB().First(certificate, id).Error
	return certificate, err
}

func (s *CertificateService) Upload(req *CertificateUploadRequest) (*model.Certificate, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || !certificateNameRegexp.MatchString(req.Name) {
		return nil, common.NewError("证书名称只能包含字母、数字、点、下划线和中划线")
	}
	req.Domain = strings.TrimSpace(req.Domain)
	req.CertPEM = strings.TrimSpace(req.CertPEM)
	req.KeyPEM = strings.TrimSpace(req.KeyPEM)
	if req.CertPEM == "" || req.KeyPEM == "" {
		return nil, common.NewError("证书和私钥不能为空")
	}
	if _, err := tls.X509KeyPair([]byte(req.CertPEM), []byte(req.KeyPEM)); err != nil {
		return nil, common.NewError("证书或私钥无效:", err)
	}
	notBefore, notAfter, err := parseCertificateTime([]byte(req.CertPEM))
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(certificateBaseDir, req.Name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	certFile := filepath.Join(dir, "fullchain.pem")
	keyFile := filepath.Join(dir, "private.key")
	if err := os.WriteFile(certFile, []byte(req.CertPEM+"\n"), 0640); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyFile, []byte(req.KeyPEM+"\n"), 0600); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	certificate := &model.Certificate{}
	db := database.GetDB()
	err = db.Where("name = ?", req.Name).First(certificate).Error
	if database.IsNotFound(err) {
		certificate = &model.Certificate{Name: req.Name, CreatedAt: now}
	} else if err != nil {
		return nil, err
	}
	certificate.Domain = req.Domain
	certificate.CertFile = certFile
	certificate.KeyFile = keyFile
	certificate.Source = "manual"
	certificate.Mode = "manual"
	certificate.NotBefore = notBefore.Unix()
	certificate.NotAfter = notAfter.Unix()
	certificate.UpdatedAt = now
	if err := db.Save(certificate).Error; err != nil {
		return nil, err
	}
	return certificate, nil
}

func (s *CertificateService) applyCertificate(req *CertificateApplyRequest, task *Task) error {
	if req.Mode == "dns" {
		return common.NewError("DNS-01 账号管理已可用，证书申请当前请先使用 HTTP-01")
	}
	account := &model.AcmeAccount{}
	if err := database.GetDB().First(account, req.AcmeId).Error; err != nil {
		return err
	}
	task.Log("INFO", "读取 ACME 账号: "+account.Email)
	key, err := certcrypto.ParsePEMPrivateKey([]byte(account.PrivateKey))
	if err != nil {
		return err
	}
	user := &acmeRuntimeUser{Email: account.Email, Key: key}
	config := lego.NewConfig(user)
	if account.Provider == "zerossl" {
		config.CADirURL = "https://acme.zerossl.com/v2/DV90"
	} else {
		config.CADirURL = lego.LEDirectoryProduction
	}
	config.Certificate.KeyType = certcrypto.RSA2048
	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}
	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80")); err != nil {
		return err
	}
	task.Log("INFO", "注册或解析 ACME 账号")
	reg, err := client.Registration.ResolveAccountByKey()
	if err != nil {
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return err
		}
	}
	user.Registration = reg
	regData, _ := json.Marshal(user)
	task.Log("INFO", "开始 HTTP-01 申请: "+req.Domain)
	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{Domains: []string{req.Domain}, Bundle: true})
	if err != nil {
		return fmt.Errorf("ACME 申请失败，请确认域名解析到本机且 80 端口可访问: %w", err)
	}
	notBefore, notAfter, err := parseCertificateTime(resource.Certificate)
	if err != nil {
		return err
	}
	dir := filepath.Join(certificateBaseDir, req.Name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	certFile := filepath.Join(dir, "fullchain.pem")
	keyFile := filepath.Join(dir, "private.key")
	if err := os.WriteFile(certFile, resource.Certificate, 0640); err != nil {
		return err
	}
	if err := os.WriteFile(keyFile, resource.PrivateKey, 0600); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(dir, "account.json"), regData, 0600)
	now := time.Now().Unix()
	cert := &model.Certificate{}
	err = database.GetDB().Where("name = ?", req.Name).First(cert).Error
	if database.IsNotFound(err) {
		cert = &model.Certificate{Name: req.Name, CreatedAt: now}
	} else if err != nil {
		return err
	}
	cert.Domain = req.Domain
	cert.CertFile = certFile
	cert.KeyFile = keyFile
	cert.Source = "acme"
	cert.Mode = req.Mode
	cert.AcmeId = req.AcmeId
	cert.DnsId = req.DnsId
	cert.AutoRenew = req.AutoRenew
	cert.NotBefore = notBefore.Unix()
	cert.NotAfter = notAfter.Unix()
	cert.UpdatedAt = now
	task.Log("INFO", "保存证书记录")
	return database.GetDB().Save(cert).Error
}

func (s *CertificateService) Content(id int) (map[string]string, error) {
	certificate, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	cert, err := os.ReadFile(certificate.CertFile)
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(certificate.KeyFile)
	if err != nil {
		return nil, err
	}
	return map[string]string{"certPem": string(cert), "keyPem": string(key)}, nil
}

func (s *CertificateService) Delete(id int) error {
	certificate, err := s.Get(id)
	if err != nil {
		return err
	}
	if err := database.GetDB().Delete(&model.Certificate{}, id).Error; err != nil {
		return err
	}
	_ = os.Remove(certificate.CertFile)
	_ = os.Remove(certificate.KeyFile)
	_ = os.Remove(filepath.Dir(certificate.CertFile))
	return nil
}

func (s *CertificateService) DeployToPanel(id int) error {
	certificate, err := s.Get(id)
	if err != nil {
		return err
	}
	if _, err := tls.LoadX509KeyPair(certificate.CertFile, certificate.KeyFile); err != nil {
		return err
	}
	if err := s.settingService.SetCertFile(certificate.CertFile); err != nil {
		return err
	}
	return s.settingService.SetKeyFile(certificate.KeyFile)
}

func parseCertificateTime(certPEM []byte) (time.Time, time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, time.Time{}, errors.New("证书 PEM 格式无效")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return cert.NotBefore, cert.NotAfter, nil
}
