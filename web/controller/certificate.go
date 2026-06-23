package controller

import (
	"strconv"
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
	g.POST("/content/:id", a.content)
	g.POST("/deployPanel/:id", a.deployPanel)
	g.POST("/del/:id", a.del)
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
