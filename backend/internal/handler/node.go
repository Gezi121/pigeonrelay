package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"nodeforge/internal/model"
)

func (h *Handler) ParseNodeLink(c *gin.Context) {
	var req model.ParseNodeLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	nodes, err := h.Node.ParseLinks(req.Links)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.ParseNodeLinkResponse{Nodes: nodes})
}

func (h *Handler) CreateNode(c *gin.Context) {
	var req model.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	n, err := h.Node.Create(userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, n)
}

func (h *Handler) ListNodes(c *gin.Context) {
	nodes, err := h.Node.List(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if nodes == nil {
		nodes = []model.ProxyNode{}
	}
	c.JSON(http.StatusOK, nodes)
}

func (h *Handler) GetNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	n, err := h.Node.GetByID(id, userID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) UpdateNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req model.UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	n, err := h.Node.Update(id, userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

func (h *Handler) DeleteNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Node.Delete(id, userID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) TestNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	ok, err := h.Node.Test(id, userID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": ok, "message": "tested"})
}

func (h *Handler) ReorderNodes(c *gin.Context) {
	var req struct {
		OrderedIDs []int64 `json:"ordered_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.OrderedIDs) == 0 {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "ordered_ids required"})
		return
	}
	if err := h.Node.Reorder(userID(c), req.OrderedIDs); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reordered"})
}

func (h *Handler) GetLatencyBest(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "domain query param required"})
		return
	}
	ip, latency, err := h.Node.GetLatencyBest(domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"domain": domain, "ip": ip, "latency_ms": latency})
}

func (h *Handler) GetLatencyBestDetail(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "domain required"})
		return
	}
	clientID := c.Query("client_id")
	detail, err := h.Node.GetLatencyBestDetailClient(domain, clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *Handler) GetLatencyBestIPTxt(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.String(http.StatusBadRequest, "domain required")
		return
	}
	ip, _, err := h.Node.GetLatencyBest(domain)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, ip)
}

// V1 version uses LATENCY_TOKEN auth (same as /api/v1/latency/report)
func (h *Handler) LatencyBestIPTxtV1(c *gin.Context) {
	expectedToken := os.Getenv("LATENCY_TOKEN")
	if expectedToken == "" {
		expectedToken, _ = h.Admin.GetSetting("latency_token")
	}
	if expectedToken == "" {
		expectedToken = "default-latency-token"
	}
	if c.GetHeader("Authorization") != "Bearer "+expectedToken {
		c.String(http.StatusUnauthorized, "invalid token")
		return
	}
	domain := c.Query("domain")
	if domain == "" {
		c.String(http.StatusBadRequest, "domain required")
		return
	}
	ip, _, err := h.Node.GetLatencyBest(domain)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, ip)
}

// --- CF bindings ---

func (h *Handler) ListCFBindings(c *gin.Context) {
	nodeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid node id"})
		return
	}
	list, err := h.Node.ListCFBindings(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.NodeCFBinding{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateCFBinding(c *gin.Context) {
	nodeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid node id"})
		return
	}
	var req model.CreateCFBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	b, err := h.Node.CreateCFBinding(nodeID, req.FullDomain, req.NamePrefix, req.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *Handler) DeleteCFBinding(c *gin.Context) {
	nodeID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	bindingID, err := strconv.ParseInt(c.Param("bid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid binding id"})
		return
	}
	if err := h.Node.DeleteCFBinding(bindingID, nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
