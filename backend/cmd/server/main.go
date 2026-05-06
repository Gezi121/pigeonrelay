package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"nodeforge/internal/config"
	"nodeforge/internal/database"
	"nodeforge/internal/handler"
	"nodeforge/internal/middleware"
	"nodeforge/internal/repository"
	"nodeforge/internal/service"
	"nodeforge/internal/service/cfdns"
	"nodeforge/internal/service/notifier"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = randomHex(32)
		log.Printf("warn: JWT_SECRET not set, generated random secret (hidden)")
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	repo := repository.New(db)
	authSvc := service.NewAuthService(repo, cfg.JWTSecret)
	sourceSvc := service.NewSourceService(repo)
	nodeSvc := service.NewNodeService(repo)
	groupSvc := service.NewGroupService(repo)
	notifierSvc := notifier.New(cfg.NotificationWebhook)
	backupSvc := service.NewBackupService(repo, cfg.BackupRetentionCount)
	subSvc := service.NewSubscriptionService(repo)
	adminSvc := service.NewAdminService(repo)

	cfdnsSvc := cfdns.New(repo, cfg.CFAPIToken, cfg.CFZoneID)
	scheduler := service.NewScheduler(repo, notifierSvc, cfdnsSvc)
	scheduler.Start()

	if err := authSvc.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Printf("warn: ensure admin: %v", err)
	}

	h := handler.New(authSvc, sourceSvc, groupSvc, nodeSvc, backupSvc, subSvc, adminSvc)

	r := gin.Default()
	r.Use(corsMiddleware())
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/api/v1/latency/report", h.LatencyReportHandler)
	r.GET("/api/v1/latency/top-ips", h.LatencyTopIPs)
	r.GET("/api/v1/latency/tasks", h.LatencyTasksHandler)
	r.GET("/api/v1/latency/best-ip", h.LatencyBestIPTxtV1)

	r.POST("/api/auth/login", h.Login)

	r.GET("/sub/:hash", h.ServeSub)

	api := r.Group("/api")
	api.Use(middleware.AuthRequired(cfg.JWTSecret))
	{
		api.GET("/sources", h.ListSources)
		api.POST("/nodes/parse", h.ParseNodeLink)
		api.GET("/nodes", h.ListNodes)
		api.POST("/nodes", h.CreateNode)
		api.GET("/nodes/:id", h.GetNode)
		api.PUT("/nodes/:id", h.UpdateNode)
		api.DELETE("/nodes/:id", h.DeleteNode)
		api.POST("/nodes/:id/test", h.TestNode)
		api.POST("/nodes/reorder", h.ReorderNodes)
		api.GET("/nodes/:id/cf-bindings", h.ListCFBindings)
		api.POST("/nodes/:id/cf-bindings", h.CreateCFBinding)
		api.DELETE("/nodes/:id/cf-bindings/:bid", h.DeleteCFBinding)
		api.POST("/sources", h.CreateSource)
		api.GET("/sources/test", h.TestSource)
		api.POST("/sources/:id/test", h.TestAndSaveSource)
		api.GET("/sources/:id", h.GetSource)
		api.PUT("/sources/:id", h.UpdateSource)
		api.DELETE("/sources/:id", h.DeleteSource)
		api.GET("/groups", h.ListGroups)
		api.POST("/groups", h.CreateGroup)
		api.GET("/groups/:id", h.GetGroup)
		api.PUT("/groups/:id", h.UpdateGroupConfig)
		api.GET("/generate/:id", h.GenerateConfig)
		api.GET("/latency", h.GetLatencyData)
		api.GET("/latency/daily-picks", h.LatencyDailyPicks)
		api.GET("/latency/ip", h.LatencyByIP)
		api.GET("/latency/best", h.GetLatencyBest)
		api.GET("/latency/best-detail", h.GetLatencyBestDetail)
		api.GET("/latency/best-ip", h.GetLatencyBestIPTxt)
		api.GET("/domain-mappings", h.ListDomainMappings)
		api.POST("/domain-mappings", h.CreateDomainMapping)
		api.POST("/domain-mappings/batch", h.BatchCreateDomainMappings)
		api.PUT("/domain-mappings/:id", h.UpdateDomainMapping)
		api.DELETE("/domain-mappings/:id", h.DeleteDomainMapping)
		api.GET("/domains/ping", h.PingDomain)
		api.GET("/speed-clients", h.ListSpeedClients)
		api.PUT("/speed-clients", h.UpsertSpeedClient)
		api.DELETE("/speed-clients/:clientId/records", h.DeleteClientRecords)
		api.GET("/algorithms", h.ListAlgorithms)
		api.POST("/algorithms", h.CreateAlgorithm)
		api.PUT("/algorithms/:id", h.UpdateAlgorithm)
		api.DELETE("/algorithms/:id", h.DeleteAlgorithm)
		api.GET("/backups", h.ListBackups)
		api.POST("/backups", h.CreateBackup)
		api.POST("/backups/:id/restore", h.RestoreBackup)
		api.POST("/sub-links", h.CreateSubLink)
		api.GET("/sub-links", h.ListSubLinks)
		api.PUT("/sub-links/:hash", h.UpdateSubLink)
		api.DELETE("/sub-links/:hash", h.DeleteSubLink)
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", middleware.AdminRequired(), h.UpdateSettings)
api.GET("/traffic/overview", h.TrafficOverview)
		api.POST("/cf/verify-token", h.VerifyCFToken)
		api.PUT("/admin/account", middleware.AdminRequired(), h.UpdateAdminAccount)
		api.GET("/admin/users", middleware.AdminRequired(), h.AdminListUsers)
		api.PUT("/admin/users/:id", middleware.AdminRequired(), h.AdminUpdateUser)
		api.DELETE("/admin/users/:id", middleware.AdminRequired(), h.AdminDeleteUser)
		api.POST("/admin/users/:id/reset-traffic", middleware.AdminRequired(), h.AdminResetTraffic)
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}
	serveStatic(r, staticDir)

	log.Printf("PigeonRelay listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func serveStatic(r *gin.Engine, dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("[static] dir %s not found, skipping static serve", dir)
		return
	}
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		full := filepath.Join(dir, path)
		if _, err := os.Stat(full); err == nil {
			c.File(full)
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(filepath.Join(dir, "index.html"))
	})
	log.Printf("[static] serving %s", dir)
}

func randomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
