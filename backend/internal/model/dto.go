package model

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// --- legacy source DTOs (still used by source service) ---

type CreateSourceRequest struct {
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	Name       string `json:"name"`
	LocalNodes string `json:"local_nodes,omitempty"`
}

type UpdateSourceRequest struct {
	Type       *string `json:"type,omitempty"`
	URL        *string `json:"url,omitempty"`
	Name       *string `json:"name,omitempty"`
	LocalNodes *string `json:"local_nodes,omitempty"`
}

// --- V3 Node DTOs ---

type ParseNodeLinkRequest struct {
	Links string `json:"links"` // one or more share links, newline-separated
}

type ParseNodeLinkResponse struct {
	Nodes []ParsedNode `json:"nodes"`
}

type ParsedNode struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	SNI         string `json:"sni"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	Security    string `json:"security"`
	ALPN        string `json:"alpn"`
	Fingerprint string `json:"fingerprint"`
	Transport   string `json:"transport"`
	Encryption  string `json:"encryption"`
	RawURL      string `json:"raw_url"`
}

type CreateNodeRequest struct {
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	RawURL         string `json:"raw_url"`
	SNI            string `json:"sni"`
	Host           string `json:"host"`
	Path           string `json:"path"`
	Security       string `json:"security"`
	ALPN           string `json:"alpn"`
	Fingerprint    string `json:"fingerprint"`
	Transport      string `json:"transport"`
	Encryption     string `json:"encryption"`
	NamePrefix     string `json:"name_prefix"`
	LocalOptDomain string `json:"local_opt_domain"`
}

type UpdateNodeRequest struct {
	Name           *string `json:"name,omitempty"`
	Address        *string `json:"address,omitempty"`
	Port           *int    `json:"port,omitempty"`
	SNI            *string `json:"sni,omitempty"`
	Host           *string `json:"host,omitempty"`
	Path           *string `json:"path,omitempty"`
	Security       *string `json:"security,omitempty"`
	ALPN           *string `json:"alpn,omitempty"`
	Fingerprint    *string `json:"fingerprint,omitempty"`
	Transport      *string `json:"transport,omitempty"`
	Encryption     *string `json:"encryption,omitempty"`
	NamePrefix     *string `json:"name_prefix,omitempty"`
	LocalOptDomain *string `json:"local_opt_domain,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

type BackupRequest struct {
	BackupType    string `json:"backup_type"`
	ConfigContent string `json:"config_content"`
}

type CreateSubLinkRequest struct {
	Name            string `json:"name"`
	Password        string `json:"password,omitempty"`
	TrafficLimitGB  int    `json:"traffic_limit_gb"`
	ExpireAt        string `json:"expire_at"`
	Remarks         string `json:"remarks"`
	AllowedGroupIDs string `json:"allowed_group_ids"`
}

type CreateSubLinkResponse struct {
	Hash string `json:"hash"`
}

type AdminUpdateUserRequest struct {
	MonthlyQuotaBytes *int64  `json:"monthly_quota_bytes,omitempty"`
	ExpireAt          *string `json:"expire_at,omitempty"`
}

type BatchDomainMappingRequest struct {
	Source     string   `json:"source"`     // "local" or "external"
	Subdomains []string `json:"subdomains"` // for local
	Suffix     string   `json:"suffix"`     // for local, e.g. ".your-domain.com"
	Domains    []string `json:"domains"`    // for external
}

type LatencyRecordInput struct {
	Time    string `json:"time" binding:"required"`
	Latency int    `json:"latency" binding:"required"`
	OK      int    `json:"ok,omitempty"`    // success count if aggregated
	Total   int    `json:"total,omitempty"` // total tries if aggregated
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// --- CF binding DTO ---

type CreateCFBindingRequest struct {
	FullDomain string `json:"full_domain"`
	NamePrefix string `json:"name_prefix"`
	Source     string `json:"source"` // "local" or "external"
}

// --- Domain mapping DTO ---

type CreateDomainMappingRequest struct {
	Subdomain     string `json:"subdomain"`
	FullDomain    string `json:"full_domain"`
	AlgorithmType string `json:"algorithm_type"`
	Source        string `json:"source"` // "local" or "external"
}
