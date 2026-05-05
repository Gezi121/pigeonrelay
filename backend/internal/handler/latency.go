package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"nodeforge/internal/model"
)

// BatchReportRequest: v1 batch format { client_id, data: [{ip, records[]}] }
type BatchReportRequest struct {
	ClientID string `json:"client_id"`
	Data     []struct {
		IP      string `json:"ip" binding:"required"`
		Records []struct {
			Time    string `json:"time" binding:"required"`
			Latency int    `json:"latency" binding:"required"`
			OK      int    `json:"ok,omitempty"`
			Total   int    `json:"total,omitempty"`
		} `json:"records" binding:"required"`
	} `json:"data"`
}

// SingleReportRequest: v0 single-IP format (backward compat)
type SingleReportRequest struct {
	IP         string `json:"ip" binding:"required"`
	MACAddress string `json:"mac_address"`
	ClientID   string `json:"client_id"`
	Records    []struct {
		Time    string `json:"time" binding:"required"`
		Latency int    `json:"latency" binding:"required"`
		OK      int    `json:"ok,omitempty"`
		Total   int    `json:"total,omitempty"`
	} `json:"records" binding:"required"`
}

func (h *Handler) LatencyReportHandler(c *gin.Context) {
	expectedToken, _ := h.Admin.GetSetting("latency_token")
	if expectedToken == "" {
		expectedToken = os.Getenv("LATENCY_TOKEN")
	}
	if expectedToken == "" {
		expectedToken = "default-latency-token"
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "Bearer "+expectedToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// Try v2 format first (per-sample arrays + TLS data, defined in latency_v2.go)
	if h.tryV2Report(c) {
		return
	}

	// Try v1 batch format
	var batchReq BatchReportRequest
	if err := c.ShouldBindJSON(&batchReq); err == nil && len(batchReq.Data) > 0 {
		entries := make([]struct {
			IP      string
			Records []model.LatencyRecordInput
		}, len(batchReq.Data))
		for i, d := range batchReq.Data {
			entries[i].IP = d.IP
			entries[i].Records = make([]model.LatencyRecordInput, 0)
			for _, r := range d.Records {
				entries[i].Records = append(entries[i].Records,
					model.LatencyRecordInput{Time: r.Time, Latency: r.Latency})
			}
		}
		if err := h.Admin.GetRepo().InsertLatencyRecordsBatch(batchReq.ClientID, entries); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, entry := range entries {
			lats := make([]int, 0, len(entry.Records))
			for _, rec := range entry.Records {
				lats = append(lats, rec.Latency)
			}
			h.Admin.GetRepo().BatchUpdateIPStats(entry.IP, lats, nil, 0, 0)
		}
		c.JSON(http.StatusOK, gin.H{"message": "reported", "count": len(batchReq.Data)})
		return
	}

	// Fallback: v0 single-IP format
	var singleReq SingleReportRequest
	if err := c.ShouldBindJSON(&singleReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	clientID := singleReq.ClientID
	if clientID == "" {
		clientID = singleReq.MACAddress
	}
	if err := h.Admin.GetRepo().InsertLatencyRecords(singleReq.IP, clientID, convertRecords(singleReq.Records)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	lats := make([]int, 0, len(singleReq.Records))
	for _, rec := range singleReq.Records {
		lats = append(lats, rec.Latency)
	}
	h.Admin.GetRepo().BatchUpdateIPStats(singleReq.IP, lats, nil, 0, 0)
	c.JSON(http.StatusOK, gin.H{"message": "reported"})
}

func convertRecords(records []struct {
	Time    string `json:"time" binding:"required"`
	Latency int    `json:"latency" binding:"required"`
	OK      int    `json:"ok,omitempty"`
	Total   int    `json:"total,omitempty"`
}) []model.LatencyRecordInput {
	out := make([]model.LatencyRecordInput, len(records))
	for i, r := range records {
		out[i] = model.LatencyRecordInput{Time: r.Time, Latency: r.Latency}
	}
	return out
}

func (h *Handler) LatencyByIP(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "ip required"})
		return
	}
	hours, _ := strconv.Atoi(c.Query("hours"))
	if hours <= 0 {
		hours = 24
	}
	bucketMin, _ := strconv.Atoi(c.Query("bucket"))
	records, err := h.Admin.GetRepo().GetLatencyByIPBucketed(ip, hours, bucketMin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, records)
}

func (h *Handler) LatencyTopIPs(c *gin.Context) {
	expectedToken, _ := h.Admin.GetSetting("latency_token")
	if expectedToken == "" {
		expectedToken = os.Getenv("LATENCY_TOKEN")
	}
	if expectedToken == "" {
		expectedToken = "default-latency-token"
	}
	if c.GetHeader("Authorization") != "Bearer "+expectedToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	picks, err := h.Admin.GetRepo().GetTopIPsV2()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	var ips []string
	for _, p := range picks {
		ips = append(ips, p.IP)
	}
	if ips == nil {
		ips = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"ips": ips})
}

func (h *Handler) LatencyDailyPicks(c *gin.Context) {
	picks, err := h.Admin.GetRepo().GetDailyPicks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, picks)
}

func (h *Handler) GetLatencyData(c *gin.Context) {
	hoursStr := c.Query("hours")
	hours, _ := strconv.Atoi(hoursStr)
	if hours <= 0 {
		hours = 24
	}

	records, err := h.Admin.GetRepo().GetLatencyRecords(hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if records == nil {
		records = []model.LatencyRecord{}
	}
	c.JSON(http.StatusOK, records)
}
