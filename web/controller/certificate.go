package controller

import (
	"strconv"
	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type CertificateController struct {
	certificateService service.CertificateService
}

func NewCertificateController(g *gin.RouterGroup) *CertificateController {
	a := &CertificateController{}
	a.initRouter(g)
	return a
}

func (a *CertificateController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/certificate")
	g.POST("/list", a.list)
	g.POST("/upload", a.upload)
	g.POST("/apply", a.apply)
	g.POST("/content/:id", a.content)
	g.POST("/deployPanel/:id", a.deployPanel)
	g.POST("/del/:id", a.del)
	g.POST("/acme/list", a.listAcme)
	g.POST("/acme/save", a.saveAcme)
	g.POST("/acme/del/:id", a.delAcme)
	g.POST("/dns/list", a.listDNS)
	g.POST("/dns/save", a.saveDNS)
	g.POST("/dns/del/:id", a.delDNS)
}

func (a *CertificateController) list(c *gin.Context) {
	certificates, err := a.certificateService.List()
	jsonObj(c, certificates, err)
}

func (a *CertificateController) upload(c *gin.Context) {
	request := &service.CertificateUploadRequest{}
	if err := c.ShouldBind(request); err != nil {
		jsonMsg(c, "上传证书", err)
		return
	}
	certificate, err := a.certificateService.Upload(request)
	securityLog(c, "certificate_upload", err == nil, " name=", request.Name)
	jsonObj(c, certificate, err)
}

func (a *CertificateController) apply(c *gin.Context) {
	request := &service.CertificateApplyRequest{}
	if err := c.ShouldBind(request); err != nil {
		jsonMsg(c, "申请证书", err)
		return
	}
	task, err := a.certificateService.ApplyAsync(request)
	securityLog(c, "certificate_apply", err == nil, " domain=", request.Domain)
	jsonObj(c, task, err)
}

func (a *CertificateController) content(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "读取证书", err)
		return
	}
	content, err := a.certificateService.Content(id)
	securityLog(c, "certificate_read", err == nil, " certificate_id=", id)
	jsonObj(c, content, err)
}

func (a *CertificateController) deployPanel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "部署面板证书", err)
		return
	}
	err = a.certificateService.DeployToPanel(id)
	securityLog(c, "certificate_deploy_panel", err == nil, " certificate_id=", id)
	jsonMsg(c, "部署面板证书", err)
}

func (a *CertificateController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除证书", err)
		return
	}
	err = a.certificateService.Delete(id)
	securityLog(c, "certificate_delete", err == nil, " certificate_id=", id)
	jsonMsg(c, "删除证书", err)
}

func (a *CertificateController) listAcme(c *gin.Context) {
	accounts, err := a.certificateService.ListAcmeAccounts()
	jsonObj(c, accounts, err)
}

func (a *CertificateController) saveAcme(c *gin.Context) {
	account := &model.AcmeAccount{}
	if err := c.ShouldBind(account); err != nil {
		jsonMsg(c, "保存 ACME 账号", err)
		return
	}
	err := a.certificateService.SaveAcmeAccount(account)
	securityLog(c, "acme_save", err == nil, " acme_id=", account.Id)
	jsonMsg(c, "保存 ACME 账号", err)
}

func (a *CertificateController) delAcme(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除 ACME 账号", err)
		return
	}
	err = a.certificateService.DeleteAcmeAccount(id)
	securityLog(c, "acme_delete", err == nil, " acme_id=", id)
	jsonMsg(c, "删除 ACME 账号", err)
}

func (a *CertificateController) listDNS(c *gin.Context) {
	accounts, err := a.certificateService.ListDnsAccounts()
	jsonObj(c, accounts, err)
}

func (a *CertificateController) saveDNS(c *gin.Context) {
	account := &model.DnsAccount{}
	if err := c.ShouldBind(account); err != nil {
		jsonMsg(c, "保存 DNS 账号", err)
		return
	}
	err := a.certificateService.SaveDnsAccount(account)
	securityLog(c, "dns_save", err == nil, " dns_id=", account.Id)
	jsonMsg(c, "保存 DNS 账号", err)
}

func (a *CertificateController) delDNS(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除 DNS 账号", err)
		return
	}
	err = a.certificateService.DeleteDnsAccount(id)
	securityLog(c, "dns_delete", err == nil, " dns_id=", id)
	jsonMsg(c, "删除 DNS 账号", err)
}
