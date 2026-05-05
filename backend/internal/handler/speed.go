package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nodeforge/internal/model"
)

// --- speed test clients ---

func (h *Handler) ListSpeedClients(c *gin.Context) {
	list, err := h.Admin.ListSpeedClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.SpeedTestClient{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) UpsertSpeedClient(c *gin.Context) {
	var req struct {
		ClientID string `json:"client_id"`
		Name     string `json:"name"`
		Notes    string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ClientID == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "client_id required"})
		return
	}
	if err := h.Admin.UpsertSpeedClient(req.ClientID, req.Name, req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) DeleteClientRecords(c *gin.Context) {
	clientID := c.Param("clientId")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "clientId required"})
		return
	}
	n, err := h.Admin.DeleteClientRecords(clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": n})
}

// --- custom algorithms ---

func (h *Handler) CreateAlgorithm(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		AlgoType     string `json:"algo_type"`
		ClientFilter string `json:"client_filter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "name required"})
		return
	}
	if req.AlgoType == "" {
		req.AlgoType = "fast"
	}
	if req.ClientFilter == "" {
		req.ClientFilter = "all"
	}
	a, err := h.Admin.CreateAlgorithm(req.Name, req.AlgoType, req.ClientFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, a)
}

func (h *Handler) ListAlgorithms(c *gin.Context) {
	list, err := h.Admin.ListAlgorithms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.CustomAlgorithm{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) UpdateAlgorithm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req struct {
		Name         *string `json:"name,omitempty"`
		AlgoType     *string `json:"algo_type,omitempty"`
		ClientFilter *string `json:"client_filter,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.Admin.UpdateAlgorithm(id, req.Name, req.AlgoType, req.ClientFilter); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) DeleteAlgorithm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Admin.DeleteAlgorithm(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
