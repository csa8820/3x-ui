package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
)

type DecoySiteController struct{ service service.DecoySiteService }

func NewDecoySiteController(g *gin.RouterGroup) {
	a := &DecoySiteController{}
	g.GET("/list", a.list)
	g.POST("/save", a.save)
	g.POST("/del/:id", a.remove)
	g.GET("/templates", a.templates)
	g.POST("/templates/install", a.install)
	g.POST("/templates/import", a.importZIP)
	g.GET("/templates/preview/:id/*path", a.preview)
}

func (a *DecoySiteController) list(c *gin.Context) {
	rows, err := a.service.List()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	inbounds, err := a.service.ListInboundReferences()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		status := website.GetManager().Status(fmt.Sprintf("site-%d", row.Id))
		refs := []int{}
		for _, ib := range inbounds {
			stream, _ := website.Parse(ib.StreamSettings)
			if stream.Website.SiteID == row.Id {
				refs = append(refs, ib.Id)
				if !status.Running {
					candidate := website.GetManager().Status(ib.Tag)
					if candidate.Running || candidate.Error != "" {
						status = candidate
					}
				}
			}
		}
		result = append(result, gin.H{"id": row.Id, "name": row.Name, "config": row.Config, "status": status, "inboundIds": refs})
	}
	jsonObj(c, result, nil)
}

func (a *DecoySiteController) save(c *gin.Context) {
	var row model.DecoySite
	if err := c.ShouldBind(&row); err != nil {
		jsonObj(c, nil, err)
		return
	}
	err := a.service.Save(&row)
	if err == nil {
		_ = (&service.XrayService{}).ReconcileWebsites()
	}
	jsonObj(c, row, err)
}
func (a *DecoySiteController) remove(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err == nil {
		err = a.service.Delete(id)
	}
	if err == nil {
		_ = (&service.XrayService{}).ReconcileWebsites()
	}
	jsonObj(c, nil, err)
}
func (a *DecoySiteController) templates(c *gin.Context) { jsonObj(c, website.Catalog(), nil) }
func (a *DecoySiteController) install(c *gin.Context) {
	var body struct {
		ID string `json:"id" form:"id"`
	}
	if err := c.ShouldBind(&body); err != nil {
		jsonObj(c, nil, err)
		return
	}
	err := website.InstallTemplate(c.Request.Context(), body.ID)
	jsonObj(c, website.Catalog(), err)
}
func (a *DecoySiteController) importZIP(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, website.MaxArchiveBytes+(1<<20))
	file, err := c.FormFile("file")
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	if file.Size > website.MaxArchiveBytes {
		jsonObj(c, nil, errors.New("template archive exceeds 25 MiB"))
		return
	}
	in, err := file.Open()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	defer in.Close()
	data, err := io.ReadAll(io.LimitReader(in, website.MaxArchiveBytes+1))
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	id, err := website.ImportTemplate(data)
	jsonObj(c, gin.H{"id": id}, err)
}
func (a *DecoySiteController) preview(c *gin.Context) {
	id := c.Param("id")
	if !website.ValidTemplate(id) {
		c.Status(404)
		return
	}
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Content-Security-Policy", "sandbox; default-src 'self' data: https:; style-src 'self' 'unsafe-inline' https:; script-src 'none'; frame-ancestors 'self'")
	c.Request.URL.Path = c.Param("path")
	website.TemplateHandler(website.Config{Template: id, Title: "Independent studio", Description: "A space for thoughtful projects and new ideas."}).ServeHTTP(c.Writer, c.Request)
}
