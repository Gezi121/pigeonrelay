package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"nodeforge/internal/model"
	"nodeforge/internal/service"
	"nodeforge/internal/service/subfetcher"
)

type Handler struct {
	Auth    *service.AuthService
	Source  *service.SourceService
	Group   *service.GroupService
	Node    *service.NodeService
	Backup  *service.BackupService
	SubSvc  *service.SubscriptionService
	Admin   *service.AdminService
}

func New(auth *service.AuthService, source *service.SourceService, group *service.GroupService, nodeSvc *service.NodeService, backup *service.BackupService, subSvc *service.SubscriptionService, admin *service.AdminService) *Handler {
	return &Handler{Auth: auth, Source: source, Group: group, Node: nodeSvc, Backup: backup, SubSvc: subSvc, Admin: admin}
}

func userID(c *gin.Context) int64 {
	id, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	v, _ := id.(int64)
	return v
}

// --- auth ---

func (h *Handler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	resp, err := h.Auth.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// --- sources ---

func (h *Handler) CreateSource(c *gin.Context) {
	var req model.CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	s, err := h.Source.Create(userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *Handler) ListSources(c *gin.Context) {
	sources, err := h.Source.List(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if sources == nil {
		sources = []model.SubscriptionSource{}
	}
	c.JSON(http.StatusOK, sources)
}

func (h *Handler) GetSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	s, err := h.Source.GetByID(id, userID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) UpdateSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req model.UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	s, err := h.Source.Update(id, userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) DeleteSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Source.Delete(id, userID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) TestSource(c *gin.Context) {
	u := c.Query("url")
	if u == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "url query param required"})
		return
	}
	result, err := subfetcher.Fetch(u)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) TestAndSaveSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}

	s, err := h.Source.GetByID(id, userID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}

	result, err := subfetcher.Fetch(s.URL)
	if err != nil {
		h.Admin.GetRepo().UpdateSourceStatus(id, "error", "")
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Admin.GetRepo().UpdateSourceStatus(id, "tested", result.RawContent); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "failed to save cache"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// --- groups ---

func (h *Handler) CreateGroup(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		SourceType string `json:"source_type"`
		SourceID   *int64 `json:"source_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	g, err := h.Group.Create(userID(c), req.Name, req.SourceType, req.SourceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *Handler) ListGroups(c *gin.Context) {
	groups, err := h.Group.List(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if groups == nil {
		groups = []model.NodeGroup{}
	}
	c.JSON(http.StatusOK, groups)
}

func (h *Handler) GetGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	g, err := h.Group.GetByID(id, userID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) UpdateGroupConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req struct {
		NamePrefix   string                   `json:"name_prefix"`
		GroupType    string                   `json:"group_type"`
		Replacements []model.GroupReplacement `json:"replacements"`
		Routings     []model.GroupRouting     `json:"routings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.Group.UpdateConfig(id, userID(c), req.NamePrefix, req.GroupType, req.Replacements, req.Routings); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) GenerateConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	outputType := c.Query("type")
	if outputType == "" {
		outputType = "clash"
	}
	config, err := h.Group.GenerateConfig(id, userID(c), outputType)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.String(http.StatusOK, "%s", config)
}


// --- backups ---

func (h *Handler) ListBackups(c *gin.Context) {
	list, err := h.Backup.List(userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.ConfigBackup{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateBackup(c *gin.Context) {
	var req model.BackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	b, err := h.Backup.Save(userID(c), req.BackupType, req.ConfigContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (h *Handler) RestoreBackup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	content, err := h.Backup.Restore(id, userID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, model.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config_content": content})
}

// --- subscription links ---

func (h *Handler) CreateSubLink(c *gin.Context) {
	var req model.CreateSubLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	link, err := h.SubSvc.Create(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.CreateSubLinkResponse{Hash: link.Hash})
}

func (h *Handler) ServeSub(c *gin.Context) {
	hash := c.Param("hash")
	link, err := h.SubSvc.Get(hash)
	if err != nil {
		c.String(http.StatusNotFound, "subscription not found")
		return
	}
	if link.PasswordHash != "" {
		password := c.Query("token")
		if !h.SubSvc.VerifyPassword(hash, password) {
			c.String(http.StatusForbidden, "invalid token")
			return
		}
	}
	format := c.Query("format")
	var cfg string
	if format == "v2ray" || format == "base64" {
		cfg, err = h.SubSvc.GenerateV2Ray(hash)
	} else {
		cfg, err = h.SubSvc.GenerateConfig(hash)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "config generation failed")
		return
	}
	filename := link.Name
	if filename == "" {
		filename = link.Remarks
	}
	if filename == "" {
		filename = "PigeonRelay"
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(filename))
	c.Header("profile-title", filename)
	c.String(http.StatusOK, cfg)
}

func (h *Handler) ListSubLinks(c *gin.Context) {
	list, err := h.SubSvc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if list == nil {
		list = []model.SubscriptionLink{}
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) UpdateSubLink(c *gin.Context) {
	hash := c.Param("hash")
	var req struct {
		Name           *string `json:"name,omitempty"`
		Remarks        *string `json:"remarks,omitempty"`
		Enabled        *bool   `json:"enabled,omitempty"`
		TrafficLimitGB *int    `json:"traffic_limit_gb,omitempty"`
		ExpireAt       *string `json:"expire_at,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.SubSvc.Update(hash, req.Name, req.Remarks, req.Enabled, req.TrafficLimitGB, req.ExpireAt); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) DeleteSubLink(c *gin.Context) {
	hash := c.Param("hash")
	if err := h.SubSvc.Delete(hash); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- admin ---

func (h *Handler) AdminListUsers(c *gin.Context) {
	users, err := h.Admin.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	if users == nil {
		users = []model.User{}
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) AdminUpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	var req model.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if err := h.Admin.UpdateUser(id, req.MonthlyQuotaBytes, req.ExpireAt); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) AdminDeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Admin.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) AdminResetTraffic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Admin.ResetTraffic(id); err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "traffic reset"})
}

// --- settings ---

func (h *Handler) GetSettings(c *gin.Context) {
	keys := []string{"clash_template", "base_domain", "latency_token", "cf_zone_id", "cf_update_minutes"}
	out := gin.H{"version": "1.0.4"}
	for _, k := range keys {
		v, _ := h.Admin.GetSetting(k)
		out[k] = v
	}
	// Don't expose token value — just indicate whether it's set
	if tok, _ := h.Admin.GetSetting("cf_api_token"); tok != "" {
		out["cf_token_set"] = true
	}
	// Read optimize domains from domain_mappings, not settings
	out["cf_domains"] = h.cfOptimizeDomains()
	c.JSON(http.StatusOK, out)
}

func (h *Handler) cfOptimizeDomains() []gin.H {
	mappings, _ := h.Node.ListDomainMappings()
	var out []gin.H
	for _, dm := range mappings {
		if dm.Source == "external" || dm.FullDomain == "" {
			continue
		}
		root := rootDomain(dm.FullDomain)
		out = append(out, gin.H{
			"subdomain":   dm.Subdomain,
			"full_domain": dm.FullDomain,
			"root":        root,
		})
	}
	if out == nil {
		out = []gin.H{}
	}
	return out
}

func rootDomain(name string) string {
	parts := splitName(name)
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return name
}

func splitName(name string) []string {
	var parts []string
	start := 0
	for i, c := range name {
		if c == '.' {
			parts = append(parts, name[start:i])
			start = i + 1
		}
	}
	parts = append(parts, name[start:])
	return parts
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	for k, v := range req {
		if v == "" && (k == "cf_api_token" || k == "cf_zone_id") {
			continue
		}
		if err := h.Admin.SetSetting(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

func (h *Handler) VerifyCFToken(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "token required"})
		return
	}
	cfReq, _ := http.NewRequest("GET", "https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
	cfReq.Header.Set("Authorization", "Bearer "+req.Token)
	resp, err := http.DefaultClient.Do(cfReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: fmt.Sprintf("request failed: %v", err)})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Success  bool   `json:"success"`
		Messages []struct{ Message string } `json:"messages"`
		Result   struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	json.Unmarshal(body, &result)
	if resp.StatusCode == 200 && result.Success {
		c.JSON(http.StatusOK, gin.H{"valid": true, "status": result.Result.Status, "message": "令牌有效"})
	} else {
		msg := "验证失败"
		if len(result.Messages) > 0 {
			msg = result.Messages[0].Message
		}
		c.JSON(http.StatusOK, gin.H{"valid": false, "message": msg})
	}
}

func (h *Handler) UpdateAdminAccount(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid request"})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "username required"})
		return
	}
	if req.Password != "" {
		if err := h.Admin.UpdateAdminWithPassword(req.Username, req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
			return
		}
	} else {
		if err := h.Admin.UpdateAdminAccount(req.Username, ""); err != nil {
			c.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "account updated"})
}

func (h *Handler) TrafficOverview(c *gin.Context) {
	uid := userID(c)
	u, err := h.Admin.GetRepo().GetUserByID(uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"used_bytes": 0, "quota_bytes": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"used_bytes":  u.UsedBytes,
		"quota_bytes": u.MonthlyQuotaBytes,
	})
}
