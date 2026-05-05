package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"nodeforge/internal/model"
)

// V2ReportRequest is the v2 client report format with per-sample arrays + timestamps.
type V2ReportRequest struct {
	ClientID   string `json:"client_id"`
	ClientInfo struct {
		ISP     string `json:"isp"`
		Region  string `json:"region"`
		Version string `json:"version"`
	} `json:"client_info"`
	Data []struct {
		IP         string   `json:"ip"`
		TCPSamples []int    `json:"tcp_samples"`
		TCPTimes   []string `json:"tcp_times"`
		TLSSamples []int    `json:"tls_samples"`
		TLSTimes   []string `json:"tls_times"`
		TLSOK      int      `json:"tls_ok"`
		TLSFail    int      `json:"tls_fail"`
	} `json:"data"`
}

// tryV2Report attempts to parse and process a v2-format latency report.
func (h *Handler) tryV2Report(c *gin.Context) bool {
	var req V2ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Data) == 0 {
		return false
	}
	hasV2Field := false
	for _, d := range req.Data {
		if len(d.TCPSamples) > 0 {
			hasV2Field = true
			break
		}
	}
	if !hasV2Field {
		return false
	}

	clientID := req.ClientID
	entries := make([]struct {
		IP      string
		Records []model.LatencyRecordInput
	}, len(req.Data))
	for i, d := range req.Data {
		entries[i].IP = d.IP
		entries[i].Records = make([]model.LatencyRecordInput, 0, len(d.TCPSamples))
		for j, lat := range d.TCPSamples {
			t := ""
			if j < len(d.TCPTimes) {
				t = d.TCPTimes[j]
			}
			entries[i].Records = append(entries[i].Records,
				model.LatencyRecordInput{Time: t, Latency: lat})
		}
	}

	if err := h.Admin.GetRepo().InsertLatencyRecordsBatch(clientID, entries); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return true
	}

	for _, d := range req.Data {
		h.Admin.GetRepo().BatchUpdateIPStats(d.IP, d.TCPSamples, d.TLSSamples, d.TLSOK, d.TLSFail)
		if req.ClientInfo.ISP != "" && len(d.TCPSamples) > 0 {
			var sum int
			for _, lat := range d.TCPSamples {
				if lat > 0 {
					sum += lat
				}
			}
			if sum > 0 {
				h.Admin.GetRepo().UpdatePerClientStats(d.IP, req.ClientID,
					float64(sum)/float64(len(d.TCPSamples)))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "reported", "count": len(req.Data), "version": "v2"})
	return true
}

// LatencyTasksHandler returns server-directed tasks for the client.
func (h *Handler) LatencyTasksHandler(c *gin.Context) {
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

	clientID := c.Query("client_id")
	count, _ := strconv.Atoi(c.Query("count"))
	if count <= 0 {
		count = 20
	}

	tasks := h.Admin.GetRepo().GenerateTasks(clientID, count)

	tcpWorkers := "50"
	tlsWorkers := "10"
	if v, _ := h.Admin.GetSetting("speed_tcp_workers"); v != "" {
		tcpWorkers = v
	}
	if v, _ := h.Admin.GetSetting("speed_tls_workers"); v != "" {
		tlsWorkers = v
	}

	cfg := gin.H{
		"tcp_concurrency":      tcpWorkers,
		"tls_concurrency":      tlsWorkers,
		"measure_interval_sec": 600,
		"report_interval_sec":  1200,
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"config": cfg,
	})
}
