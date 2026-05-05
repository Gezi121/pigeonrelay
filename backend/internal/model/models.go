package model

import "time"

type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	Role              string    `json:"role"`
	ExpireAt          time.Time `json:"expire_at"`
	MonthlyQuotaBytes int64     `json:"monthly_quota_bytes"`
	UsedBytes         int64     `json:"used_bytes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// --- V3: ProxyNode replaces SubscriptionSource + NodeGroup ---

type ProxyNode struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Name           string     `json:"name"`
	Protocol       string     `json:"protocol"` // vless/vmess/trojan/ss/hysteria2
	Address        string     `json:"address"`  // server IP or domain
	Port           int        `json:"port"`     // server port
	RawURL         string     `json:"raw_url"`  // original share link
	SNI            string     `json:"sni"`
	Host           string     `json:"host"`
	Path           string     `json:"path"`
	Security       string     `json:"security"`    // tls/reality/none
	ALPN           string     `json:"alpn"`        // h2,http/1.1
	Fingerprint    string     `json:"fingerprint"` // chrome/random/randomized
	Transport      string     `json:"transport"`   // ws/tcp/grpc
	Encryption     string     `json:"encryption"`  // none (for vmess)
	NamePrefix     string     `json:"name_prefix"`
	LocalOptDomain string     `json:"local_opt_domain"` // empty=off, or "dshax1".."dshax10"
	Enabled        bool       `json:"enabled"`
	Status         string     `json:"status"` // untested/ok/error
	LastTestedAt   *time.Time `json:"last_tested_at"`
	SortOrder      int        `json:"sort_order"`
	CreatedAt      time.Time  `json:"created_at"`
}

type NodeReplacement struct {
	ID           int64  `json:"id"`
	NodeID       int64  `json:"node_id"`
	TargetField  string `json:"target_field"` // port / sni / host / path
	ReplaceValue string `json:"replace_value"`
}

type NodeRouting struct {
	ID             int64  `json:"id"`
	NodeID         int64  `json:"node_id"`
	RoutingAddress string `json:"routing_address"`
}

type NodeCFBinding struct {
	ID         int64  `json:"id"`
	NodeID     int64  `json:"node_id"`
	FullDomain string `json:"full_domain"`
	NamePrefix string `json:"name_prefix"`
	Source     string `json:"source"` // "local" or "external"
	SortOrder  int    `json:"sort_order"`
}

// --- legacy models kept for DB compat, not used in new API ---
type SubscriptionSource struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	Type            string     `json:"type"`
	URL             string     `json:"url,omitempty"`
	Name            string     `json:"name"`
	LastFetchStatus string     `json:"last_fetch_status"`
	LastFetchAt     *time.Time `json:"last_fetch_at"`
	RawData         string     `json:"-"`
	Status          string     `json:"status" gorm:"default:'untested'"`
	LastTestedAt    *time.Time `json:"last_tested_at"`
	RawNodesCache   string     `json:"raw_nodes_cache"`
	LocalNodes      string     `json:"local_nodes"`
}

type NodeTemplate struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	ProtocolType  string `json:"protocol_type"`
	TemplateName  string `json:"template_name"`
	ConfigContent string `json:"config_content"`
	ContentType   string `json:"content_type"`
}

type NodeGroup struct {
	ID             int64              `json:"id"`
	UserID         int64              `json:"user_id"`
	Name           string             `json:"name"`
	NamePrefix     string             `json:"name_prefix"`
	GroupType      string             `json:"group_type" gorm:"default:'standard'"`
	SourceType     string             `json:"source_type"`
	SourceID       *int64             `json:"source_id"`
	UnifiedConfig  string             `json:"unified_config"`
	RuleConfig     string             `json:"rule_config"`
	AdvancedPrefer string             `json:"advanced_prefer"`
	Replacements   []GroupReplacement `json:"replacements" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	Routings       []GroupRouting     `json:"routings" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
}

type GroupReplacement struct {
	ID           int64  `json:"id" gorm:"primarykey"`
	GroupID      int64  `json:"group_id" gorm:"index"`
	TargetField  string `json:"target_field"`
	ReplaceValue string `json:"replace_value"`
}

type GroupRouting struct {
	ID             int64  `json:"id" gorm:"primarykey"`
	GroupID        int64  `json:"group_id" gorm:"index"`
	RoutingAddress string `json:"routing_address"`
}

type SpeedTestClient struct {
	ClientID   string     `json:"client_id"`
	Name       string     `json:"name"`
	Notes      string     `json:"notes"`
	LastSeen   *time.Time `json:"last_seen"`
	TodayCount int        `json:"today_count"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CustomAlgorithm struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	AlgoType     string `json:"algo_type"`     // fast/steady/tidal/composite
	ClientFilter string `json:"client_filter"` // "all" or comma-separated MACs
}

type BestIPDetail struct {
	IP        string `json:"ip"`
	Score     int    `json:"score"`
	AvgMs     int    `json:"avg_ms"`
	SuccessRt int    `json:"success_rt"`
	StableRt  int    `json:"stable_rt"`
	Stddev    int    `json:"stddev"`
	Unstable  int    `json:"unstable"`
	Total     int    `json:"total"`
	AlgoType  string `json:"algo_type"`
}

type DailyPick struct {
	IP       string `json:"ip"`
	AvgMs    int    `json:"avg_ms"`
	MinMs    int    `json:"min_ms"`
	MaxMs    int    `json:"max_ms"`
	Count    int    `json:"count"`
	Success  int    `json:"success"`
	Rate     int    `json:"rate"`     // success rate % (TCP connected)
	Unstable int    `json:"unstable"` // count >200ms
	StableRt int    `json:"stable_rt"` // % of successful that are <=200ms
}

type LatencyRecord struct {
	ID         int64     `json:"id" gorm:"primarykey"`
	IPAddress  string    `json:"ip_address" gorm:"index"`
	ClientID   string    `json:"client_id"` // reporting client UUID
	LatencyMs  int       `json:"latency_ms"`
	RecordedAt time.Time `json:"recorded_at" gorm:"index"`
}

type DomainMapping struct {
	ID            int64  `json:"id" gorm:"primarykey"`
	Subdomain     string `json:"subdomain" gorm:"uniqueIndex"`
	FullDomain    string `json:"full_domain"`
	AlgorithmType string `json:"algorithm_type"`
	Source        string `json:"source"` // "local" or "external"
}

type SubscriptionLink struct {
	ID              int64      `json:"id" gorm:"primarykey"`
	Hash            string     `json:"hash" gorm:"uniqueIndex"`
	Name            string     `json:"name"`
	PasswordHash    string     `json:"-"`
	MaxFetchPerDay  int        `json:"max_fetch_per_day"`
	Enabled         bool       `json:"enabled"`
	TrafficLimitGB  int        `json:"traffic_limit_gb"`
	ExpireAt        *time.Time `json:"expire_at"`
	Remarks         string     `json:"remarks"`
	AllowedGroupIDs string     `json:"allowed_group_ids"`
}

type Node struct {
	ID            int64  `json:"id"`
	GroupID       int64  `json:"group_id"`
	TemplateID    *int64 `json:"template_id"`
	OriginalNode  string `json:"original_node"`
	ProcessedNode string `json:"processed_node"`
	AddressList   string `json:"address_list"`
	Enabled       bool   `json:"enabled"`
	SortOrder     int    `json:"sort_order"`
}

type ConfigBackup struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	BackupType    string    `json:"backup_type"`
	ConfigContent string    `json:"config_content"`
	CreatedAt     time.Time `json:"created_at"`
}

type TrafficLog struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	NodeID    *int64    `json:"node_id"`
	BytesUp   int64     `json:"bytes_up"`
	BytesDown int64     `json:"bytes_down"`
	Timestamp time.Time `json:"timestamp"`
}

type IPStats struct {
	IPAddress              string     `json:"ip_address"`
	EWMALatencyMs          float64    `json:"ewma_latency_ms"`
	EWMATLSMs              float64    `json:"ewma_tls_ms"`
	EWMASuccessRate        float64    `json:"ewma_success_rate"`
	EWMATLSOkRate          float64    `json:"ewma_tls_ok_rate"`
	EWMALossRate           float64    `json:"ewma_loss_rate"`
	JitterMs               float64    `json:"jitter_ms"`
	LatencyStddev          float64    `json:"latency_stddev"`
	SuccessRateVariance    float64    `json:"success_rate_variance"`
	P50LatencyMs           float64    `json:"p50_latency_ms"`
	P95LatencyMs           float64    `json:"p95_latency_ms"`
	ThroughputBytesPerSec  float64    `json:"throughput_bytes_per_sec"`
	Samples1h              int        `json:"samples_1h"`
	Samples24h             int        `json:"samples_24h"`
	LastTestedAt           *time.Time `json:"last_tested_at"`
	LastTLSOKAt            *time.Time `json:"last_tls_ok_at"`
	TestPriority           float64    `json:"test_priority"`
	CooldownUntil          *time.Time `json:"cooldown_until"`
	PerClientJSON          string     `json:"per_client_json"`
	PeakHourLatency        *float64   `json:"peak_hour_latency"`
}
