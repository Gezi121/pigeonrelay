package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"nodeforge/internal/service"
)

func TestLatencyReportAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{Admin: service.NewAdminService(nil)}
	r := gin.New()
	r.POST("/api/v1/latency/report", h.LatencyReportHandler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/latency/report", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %v", w.Code)
	}
}