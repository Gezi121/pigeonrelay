package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nodeforge/internal/model"
)

func (h *Handler) ListDomainMappings(c *gin.Context) {
	list, err := h.Node.ListDomainMappings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.DomainMapping{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) UpdateDomainMapping(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req struct {
		Subdomain     string `json:"subdomain"`
		FullDomain    string `json:"full_domain"`
		AlgorithmType string `json:"algorithm_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.Node.UpdateDomainMappingFull(id, req.Subdomain, req.FullDomain, req.AlgorithmType); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) CreateDomainMapping(c *gin.Context) {
	var req model.CreateDomainMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	m, err := h.Node.CreateDomainMapping(req.Subdomain, req.FullDomain, req.AlgorithmType, req.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) BatchCreateDomainMappings(c *gin.Context) {
	var req model.BatchDomainMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	list, err := h.Node.BatchCreateDomainMappings(req.Source, req.Subdomains, req.Suffix, req.Domains)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, list)
}

func (h *Handler) PingDomain(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "domain query param required"})
		return
	}
	reachable, ip, latency, err := h.Node.PingDomain(domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"domain": domain, "reachable": reachable, "ip": ip, "latency_ms": latency})
}

func (h *Handler) DeleteDomainMapping(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Node.DeleteDomainMapping(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
