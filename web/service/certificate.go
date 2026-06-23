package service

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
)

const certificateBaseDir = "/etc/x-ui/certificates"

var certificateNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

type CertificateService struct {
	settingService SettingService
}

type CertificateUploadRequest struct {
	Name    string `json:"name" form:"name"`
	Domain  string `json:"domain" form:"domain"`
	CertPEM string `json:"certPem" form:"certPem"`
	KeyPEM  string `json:"keyPem" form:"keyPem"`
}

func (s *CertificateService) List() ([]*model.Certificate, error) {
	var certificates []*model.Certificate
	err := database.GetDB().Order("id desc").Find(&certificates).Error
	return certificates, err
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
	certificate.NotBefore = notBefore.Unix()
	certificate.NotAfter = notAfter.Unix()
	certificate.UpdatedAt = now
	if err := db.Save(certificate).Error; err != nil {
		return nil, err
	}
	return certificate, nil
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
