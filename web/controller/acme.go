package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

const acmeBaseDir = "/etc/x-ui/acme"

var domainRegexp = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))+\.?$`)

const acmeApplyInterval = 30 * time.Minute

type acmeApplyState struct {
	Running bool
	Last    time.Time
}

var acmeApplyLimiter = struct {
	sync.Mutex
	items map[string]*acmeApplyState
}{items: map[string]*acmeApplyState{}}

type acmeApplyRequest struct {
	Domain string `form:"domain" json:"domain"`
	Email  string `form:"email" json:"email"`
}

type acmeApplyResponse struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type acmeUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	Key          crypto.PrivateKey      `json:"-"`
}

func (u *acmeUser) GetEmail() string {
	return u.Email
}

func (u acmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

func (a *ServerController) applyAcmeCert(c *gin.Context) {
	req := acmeApplyRequest{}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "申请证书", err)
		return
	}

	certFile, keyFile, err := requestAcmeCertificate(strings.TrimSpace(req.Domain), strings.TrimSpace(req.Email))
	securityLog(c, "acme_apply", err == nil, " domain=", strings.TrimSpace(req.Domain))
	jsonMsgObj(c, "申请证书", acmeApplyResponse{CertFile: certFile, KeyFile: keyFile}, err)
}

func requestAcmeCertificate(domain string, email string) (string, string, error) {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	if domain == "" {
		return "", "", errors.New("请输入域名")
	}
	if !domainRegexp.MatchString(domain) || strings.Contains(domain, "..") {
		return "", "", errors.New("域名格式不正确")
	}
	if email == "" {
		return "", "", errors.New("请输入邮箱地址，用于 ACME 证书注册")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", errors.New("邮箱格式不正确")
	}
	if err := validatePublicDomain(domain); err != nil {
		return "", "", err
	}
	releaseLimiter, err := acquireAcmeApply(domain)
	if err != nil {
		return "", "", err
	}
	defer releaseLimiter()

	domainDir := filepath.Join(acmeBaseDir, domain)
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		return "", "", err
	}

	user, err := loadOrCreateAcmeUser(domainDir, email)
	if err != nil {
		return "", "", err
	}

	config := lego.NewConfig(user)
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.RSA2048
	client, err := lego.NewClient(config)
	if err != nil {
		return "", "", err
	}
	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80")); err != nil {
		return "", "", err
	}

	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return "", "", err
		}
		user.Registration = reg
		if err := saveAcmeRegistration(domainDir, user); err != nil {
			return "", "", err
		}
	}

	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	})
	if err != nil {
		return "", "", fmt.Errorf("ACME 申请失败，请确认域名已解析到本机且 80 端口可访问: %w", err)
	}

	certFile := filepath.Join(domainDir, "fullchain.cer")
	keyFile := filepath.Join(domainDir, "private.key")
	if err := os.WriteFile(certFile, resource.Certificate, 0600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyFile, resource.PrivateKey, 0600); err != nil {
		return "", "", err
	}
	return certFile, keyFile, nil
}

func acquireAcmeApply(domain string) (func(), error) {
	acmeApplyLimiter.Lock()
	defer acmeApplyLimiter.Unlock()
	now := time.Now()
	state := acmeApplyLimiter.items[domain]
	if state == nil {
		state = &acmeApplyState{}
		acmeApplyLimiter.items[domain] = state
	}
	if state.Running {
		return nil, errors.New("该域名正在申请证书，请稍后再试")
	}
	if !state.Last.IsZero() && now.Sub(state.Last) < acmeApplyInterval {
		return nil, errors.New("该域名申请过于频繁，请稍后再试")
	}
	state.Running = true
	state.Last = now
	return func() {
		acmeApplyLimiter.Lock()
		defer acmeApplyLimiter.Unlock()
		state.Running = false
	}, nil
}

func validatePublicDomain(domain string) error {
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		return errors.New("域名未解析到有效 IP")
	}
	for _, ip := range ips {
		if isPublicIP(ip) {
			return nil
		}
	}
	return errors.New("域名未解析到公网 IP")
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, block := range privateBlocks {
		_, network, err := net.ParseCIDR(block)
		if err == nil && network.Contains(ip) {
			return false
		}
	}
	return true
}

func loadOrCreateAcmeUser(dir string, email string) (*acmeUser, error) {
	keyFile := filepath.Join(dir, "account.key")
	regFile := filepath.Join(dir, "account.json")
	user := &acmeUser{Email: email}

	if keyBytes, err := os.ReadFile(keyFile); err == nil {
		key, err := certcrypto.ParsePEMPrivateKey(keyBytes)
		if err != nil {
			return nil, err
		}
		user.Key = key
	} else {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		user.Key = key
		if err := os.WriteFile(keyFile, certcrypto.PEMEncode(key), 0600); err != nil {
			return nil, err
		}
	}

	if data, err := os.ReadFile(regFile); err == nil {
		_ = json.Unmarshal(data, user)
		if user.Email == "" {
			user.Email = email
		}
	}
	return user, nil
}

func saveAcmeRegistration(dir string, user *acmeUser) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "account.json"), data, 0600)
}

