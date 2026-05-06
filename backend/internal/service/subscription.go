package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type SubscriptionService struct {
	repo *repository.Repo
}

func NewSubscriptionService(repo *repository.Repo) *SubscriptionService {
	return &SubscriptionService{repo: repo}
}

func (s *SubscriptionService) Create(req *model.CreateSubLinkRequest) (*model.SubscriptionLink, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	hash := hex.EncodeToString(bytes)

	var expireAt *time.Time
	if req.ExpireAt != "" {
		t, err := time.Parse("2006-01-02", req.ExpireAt)
		if err == nil {
			expireAt = &t
		}
	}

	link := &model.SubscriptionLink{
		Hash:            hash,
		Name:            req.Name,
		MaxFetchPerDay:  100,
		Enabled:         true,
		TrafficLimitGB:  req.TrafficLimitGB,
		ExpireAt:        expireAt,
		Remarks:         req.Remarks,
		AllowedGroupIDs: req.AllowedGroupIDs,
	}

	if req.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		link.PasswordHash = string(h)
	}

	if err := s.repo.CreateSubLink(link); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *SubscriptionService) List() ([]model.SubscriptionLink, error) {
	return s.repo.ListSubLinks()
}

func (s *SubscriptionService) Update(hash string, name, remarks *string, enabled *bool, trafficLimitGB *int, expireAt *string) error {
	return s.repo.UpdateSubLink(hash, name, remarks, enabled, trafficLimitGB, expireAt)
}

func (s *SubscriptionService) Delete(hash string) error {
	return s.repo.DeleteSubLink(hash)
}

func (s *SubscriptionService) Get(hash string) (*model.SubscriptionLink, error) {
	link, err := s.repo.GetSubLink(hash)
	if err != nil {
		return nil, errors.New("subscription link not found")
	}
	if !link.Enabled {
		return nil, errors.New("subscription link disabled")
	}
	if link.ExpireAt != nil && !link.ExpireAt.IsZero() && time.Now().After(*link.ExpireAt) {
		return nil, errors.New("subscription link expired")
	}
	return link, nil
}

func (s *SubscriptionService) VerifyPassword(hash, password string) bool {
	link, err := s.repo.GetSubLink(hash)
	if err != nil {
		return false
	}
	if link.PasswordHash == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)) == nil
}

func (s *SubscriptionService) GenerateConfig(hash string) (string, error) {
	nodes, err := s.repo.ListAllEnabledNodes()
	if err != nil {
		return "", err
	}

	var proxies []map[string]interface{}
	for _, n := range nodes {
		bindings, _ := s.repo.ListCFBindings(n.ID)
		if len(bindings) > 0 {
			for _, b := range bindings {
				p := nodeToProxy(n, b.FullDomain, b.NamePrefix)
				proxies = append(proxies, p)
			}
		} else if n.LocalOptDomain != "" {
			p := nodeToProxy(n, n.LocalOptDomain, n.NamePrefix)
			proxies = append(proxies, p)
		} else {
			p := nodeToProxy(n, "", n.NamePrefix)
			proxies = append(proxies, p)
		}
	}

	// Collect proxy names + server domains for DIRECT rules
	var proxyNames []string
	seenServers := make(map[string]bool)
	for _, p := range proxies {
		if n, ok := p["name"].(string); ok {
			proxyNames = append(proxyNames, fmt.Sprintf("      - \"%s\"", n))
		}
		if s, ok := p["server"].(string); ok && s != "" && !isIPAddress(s) {
			seenServers[s] = true
		}
	}
	var serverRules []string
	for s := range seenServers {
		serverRules = append(serverRules, fmt.Sprintf("  - DOMAIN-SUFFIX,%s,DIRECT", s))
	}
	sort.Strings(serverRules)
	serverDirect := strings.Join(serverRules, "\n")

	// Read template from settings
	tmpl, _ := s.repo.GetSetting("clash_template")
	if tmpl == "" {
		tmpl = "port: 7890\nsocks-port: 7891\nallow-lan: true\nmode: rule\nlog-level: info\nexternal-controller: 127.0.0.1:9090\n\ndns:\n  enable: false\n\n# 节点配置\nproxies:\n${proxies}\n\n# 策略组配置\nproxy-groups:\n  - name: 🚀 节点选择\n    type: select\n    proxies:\n      - ♻️ 自动选择\n${proxy-names}\n    url: http://www.gstatic.com/generate_204\n    interval: 300\n\n  - name: ♻️ 自动选择\n    type: url-test\n    url: http://www.gstatic.com/generate_204\n    interval: 300\n    proxies:\n${proxy-names}\n\n# 路由规则\nrules:\n${server-direct-rules}\n  - GEOIP,LAN,DIRECT\n  - GEOIP,CN,DIRECT\n  - MATCH,🚀 节点选择\n"
	}
	tmpl = strings.ReplaceAll(tmpl, "\\n", "\n")
	tmpl = strings.ReplaceAll(tmpl, "${server-direct-rules}", serverDirect)

	// Build proxy YAML (protocol-aware field names from mihomo wiki)
	var sb strings.Builder
	for i, p := range proxies {
		if i > 0 {
			sb.WriteString("\n")
		}
		proto, _ := p["type"].(string)
		sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", p["name"]))
		sb.WriteString(fmt.Sprintf("    type: %s\n", proto))
		sb.WriteString(fmt.Sprintf("    server: %s\n", p["server"]))
		sb.WriteString(fmt.Sprintf("    port: %v\n", p["port"]))

		// Auth
		if uuid, ok := p["uuid"]; ok && uuid.(string) != "" {
			sb.WriteString(fmt.Sprintf("    uuid: %s\n", uuid))
		}
		if password, ok := p["password"]; ok && password.(string) != "" {
			sb.WriteString(fmt.Sprintf("    password: \"%s\"\n", password))
		}
		if cipher, ok := p["cipher"]; ok && cipher.(string) != "" {
			sb.WriteString(fmt.Sprintf("    cipher: %s\n", cipher))
		}
		if proto == "vmess" {
			sb.WriteString("    alterId: 0\n")
			sb.WriteString("    cipher: auto\n")
		}

		sb.WriteString("    udp: true\n")
		if tls, ok := p["tls"]; ok && tls.(bool) {
			sb.WriteString("    tls: true\n")
			sb.WriteString("    skip-cert-verify: true\n")
		}

		// TLS SNI: always output both sni and servername
		// servername is required by mihomo/Clash Meta to validate certs
		// when server is an IP address (Go x509 IP SAN check)
		if sni, ok := p["sni"]; ok && sni.(string) != "" {
			sb.WriteString(fmt.Sprintf("    servername: %s\n", sni.(string)))
			sniKey := "sni"
			if proto == "vmess" {
				sniKey = "servername"
			}
			sb.WriteString(fmt.Sprintf("    %s: %s\n", sniKey, sni.(string)))
		}

		if netw, ok := p["network"]; ok && netw.(string) != "" {
			sb.WriteString(fmt.Sprintf("    network: %s\n", netw))
		}

		// Fingerprint: vmess→fingerprint, others→client-fingerprint
		fpKey := "client-fingerprint"
		if proto == "vmess" {
			fpKey = "fingerprint"
		}
		if fp, ok := p["fingerprint"]; ok && fp.(string) != "" {
			sb.WriteString(fmt.Sprintf("    %s: %s\n", fpKey, fp))
		}

		// ALPN as flow sequence
		if alpn, ok := p["alpn"]; ok {
			if alpnStr, ok2 := alpn.(string); ok2 && alpnStr != "" {
				parts := strings.Split(alpnStr, ",")
				var clean []string
				for _, a := range parts {
					a = strings.TrimSpace(a)
					if a != "" {
						clean = append(clean, a)
					}
				}
				if len(clean) > 0 {
					sb.WriteString(fmt.Sprintf("    alpn: [%s]\n", strings.Join(clean, ", ")))
				}
			}
		}

		// ws-opts / xhttp-opts
		for _, optsKey := range []string{"ws-opts", "xhttp-opts"} {
			if p[optsKey] == nil {
				continue
			}
			if optsMap, ok := p[optsKey].(map[string]interface{}); ok {
				sb.WriteString(fmt.Sprintf("    %s:\n", optsKey))
				if path, ok := optsMap["path"]; ok && path.(string) != "" {
					sb.WriteString(fmt.Sprintf("      path: \"%s\"\n", path))
				}
				if mode, ok := optsMap["mode"]; ok && mode.(string) != "" {
					sb.WriteString(fmt.Sprintf("      mode: %s\n", mode))
				}
				if headers, ok := optsMap["headers"].(map[string]string); ok {
					sb.WriteString("      headers:\n")
					for k, v := range headers {
						sb.WriteString(fmt.Sprintf("        %s: %s\n", k, v))
					}
				}
			}
		}

		// reality-opts
		if p["reality-opts"] != nil {
			if realm, ok := p["reality-opts"].(map[string]interface{}); ok {
				sb.WriteString("    reality-opts:\n")
				if pbk, ok := realm["public-key"]; ok && pbk.(string) != "" {
					sb.WriteString(fmt.Sprintf("      public-key: \"%s\"\n", pbk))
				}
				if sid, ok := realm["short-id"]; ok && sid.(string) != "" {
					sb.WriteString(fmt.Sprintf("      short-id: \"%s\"\n", sid))
				}
			}
		}

		// Hysteria2
		if up, ok := p["up"]; ok && up.(string) != "" {
			sb.WriteString(fmt.Sprintf("    up: \"%s\"\n", up))
		}
		if down, ok := p["down"]; ok && down.(string) != "" {
			sb.WriteString(fmt.Sprintf("    down: \"%s\"\n", down))
		}
		if obfs, ok := p["obfs"]; ok && obfs.(string) != "" {
			sb.WriteString(fmt.Sprintf("    obfs: %s\n", obfs))
		}
	}

	result := strings.Replace(tmpl, "${proxies}", strings.TrimRight(sb.String(), "\n"), 1)
	result = strings.ReplaceAll(result, "${proxy-names}", strings.Join(proxyNames, "\n"))
	// Inject server DIRECT rules before the first GEOIP rule
	if serverDirect != "" {
		result = strings.Replace(result, "  - GEOIP,LAN,DIRECT", serverDirect+"\n  - GEOIP,LAN,DIRECT", 1)
	}

	return result, nil
}

func nodeToProxy(n model.ProxyNode, overrideAddr, prefix string) map[string]interface{} {
	addr := n.Address
	if overrideAddr != "" {
		addr = overrideAddr
	}
	name := n.Name
	if prefix != "" {
		name = n.Name + "-" + prefix
	}
	p := map[string]interface{}{
		"name":   name,
		"type":   n.Protocol,
		"server": addr,
		"port":   n.Port,
	}

	// Parse raw_url for protocol-specific auth fields
	parseAuthFields(n.RawURL, n.Protocol, p)

	if n.Security == "tls" || n.Security == "reality" {
		p["tls"] = true
	}
	if n.SNI != "" {
		p["sni"] = n.SNI
	}
	// Split CDN: override SNI to match server domain so CF/stream proxy routes correctly
	if strings.Contains(addr, "cdnup.gugugezi.com") {
		p["sni"] = "cdnup.gugugezi.com"
	} else if strings.Contains(addr, "cdndown.gugugezi.com") {
		p["sni"] = "cdndown.gugugezi.com"
	}
	if n.Transport != "" && n.Transport != "tcp" {
		p["network"] = n.Transport
	}
	if n.Host != "" || n.Path != "" {
		path := "/"
		if n.Path != "" {
			path = n.Path
		}
		optsKey := "ws-opts"
		if n.Transport == "xhttp" {
			optsKey = "xhttp-opts"
		}
		opts := map[string]interface{}{"path": path}
		if n.Host != "" {
			opts["headers"] = map[string]string{"Host": n.Host}
		}
		p[optsKey] = opts
	}
	if n.Fingerprint != "" {
		p["fingerprint"] = n.Fingerprint
	}
	if n.ALPN != "" {
		p["alpn"] = n.ALPN
	}

	// Extract Reality public-key & short-id from raw_url
	if n.Security == "reality" {
		realityOpts := parseRealityFromURL(n.RawURL)
		if len(realityOpts) > 0 {
			p["reality-opts"] = realityOpts
		}
	}

	return p
}

func parseAuthFields(rawURL, protocol string, p map[string]interface{}) {
	if rawURL == "" {
		return
	}
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return
	}
	rest := rawURL[schemeEnd+3:]

	// Remove fragment (#name)
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		rest = rest[:idx]
	}
	// Remove query (?params)
	base := rest
	if idx := strings.Index(rest, "?"); idx >= 0 {
		base = rest[:idx]
	}

	switch protocol {
	case "vless":
		// vless://uuid@host:port
		if atIdx := strings.LastIndex(base, "@"); atIdx >= 0 {
			p["uuid"] = base[:atIdx]
		}
	case "trojan":
		// trojan://password@host:port
		if atIdx := strings.LastIndex(base, "@"); atIdx >= 0 {
			p["password"] = base[:atIdx]
		}
	case "hysteria2", "hy2":
		// hysteria2://password@host:port or hysteria2://host:port?auth=password
		if atIdx := strings.LastIndex(base, "@"); atIdx >= 0 {
			p["password"] = base[:atIdx]
		}
	case "vmess":
		// vmess://base64 — skip, too complex for inline
		p["_note"] = "vmess: use raw url"
	case "ss":
		// ss://base64@host:port
		if atIdx := strings.LastIndex(base, "@"); atIdx >= 0 {
			// base64 encoded method:password
			decoded, err := base64.StdEncoding.DecodeString(base[:atIdx])
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 {
					p["cipher"] = parts[0]
					p["password"] = parts[1]
				}
			}
		}
	}
}

// parseRealityFromURL extracts reality-opts (public-key, short-id) from a raw URL's query params.
func parseRealityFromURL(rawURL string) map[string]interface{} {
	if rawURL == "" {
		return nil
	}
	// Extract query string: everything between ? and #
	qIdx := strings.Index(rawURL, "?")
	if qIdx < 0 {
		return nil
	}
	query := rawURL[qIdx+1:]
	if fIdx := strings.Index(query, "#"); fIdx >= 0 {
		query = query[:fIdx]
	}

	pbk := ""
	sid := ""
	for _, part := range strings.Split(query, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "pbk":
			pbk, _ = url.QueryUnescape(kv[1])
		case "sid":
			sid, _ = url.QueryUnescape(kv[1])
		}
	}
	if pbk == "" {
		return nil
	}
	opts := map[string]interface{}{"public-key": pbk}
	if sid != "" {
		opts["short-id"] = sid
	}
	return opts
}

func (s *SubscriptionService) GenerateV2Ray(hash string) (string, error) {
	nodes, err := s.repo.ListAllEnabledNodes()
	if err != nil {
		return "", err
	}

	var links []string
	for _, n := range nodes {
		bindings, _ := s.repo.ListCFBindings(n.ID)
		if len(bindings) > 0 {
			for _, b := range bindings {
				links = append(links, nodeToShareLink(n, b.FullDomain, b.NamePrefix))
			}
		} else if n.LocalOptDomain != "" {
			links = append(links, nodeToShareLink(n, n.LocalOptDomain, n.NamePrefix))
		} else {
			links = append(links, nodeToShareLink(n, "", n.NamePrefix))
		}
	}
	if len(links) == 0 {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n"))), nil
}

// nodeToShareLink generates a VLESS/VMess/Trojan/SS/Hy2 share link from proxy data.
func nodeToShareLink(n model.ProxyNode, overrideAddr, prefix string) string {
	addr := n.Address
	if overrideAddr != "" {
		addr = overrideAddr
	}
	name := n.Name
	if prefix != "" {
		name = n.Name + prefix
	}
	port := n.Port
	if port == 0 {
		port = 443
	}

	switch n.Protocol {
	case "vless":
		return buildVLESSLink(n, addr, port, name)
	case "vmess":
		return buildVMessLink(n, addr, port, name)
	case "trojan":
		return buildTrojanLink(n, addr, port, name)
	case "ss":
		return buildSSLink(n, addr, port, name)
	case "hysteria2", "hy2":
		return buildHy2Link(n, addr, port, name)
	default:
		return ""
	}
}

func buildVLESSLink(n model.ProxyNode, addr string, port int, name string) string {
	q := url.Values{}
	q.Set("encryption", "none")
	if n.Security == "tls" || n.Security == "reality" {
		q.Set("security", n.Security)
	}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if n.Fingerprint != "" {
		q.Set("fp", n.Fingerprint)
	}
	if n.Transport != "" && n.Transport != "tcp" {
		q.Set("type", n.Transport)
	}
	if n.Path != "" {
		q.Set("path", n.Path)
	}
	if n.Host != "" {
		q.Set("host", n.Host)
	}
	if n.Security == "reality" {
		if pbk, sid := parseRealityForLink(n.RawURL); pbk != "" {
			q.Set("pbk", pbk)
			if sid != "" {
				q.Set("sid", sid)
			}
		}
	}
	q.Set("mode", "auto")
	uuid := extractUUID(n.RawURL)
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, addr, port, q.Encode(), url.QueryEscape(name))
}

func buildTrojanLink(n model.ProxyNode, addr string, port int, name string) string {
	password := extractPassword(n.RawURL)
	q := url.Values{}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	q.Set("security", "tls")
	q.Set("type", "tcp")
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", url.QueryEscape(password), addr, port, q.Encode(), url.QueryEscape(name))
}

func buildSSLink(n model.ProxyNode, addr string, port int, name string) string {
	cipher, password := extractSSAuth(n.RawURL)
	userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, addr, port, url.QueryEscape(name))
}

func buildHy2Link(n model.ProxyNode, addr string, port int, name string) string {
	password := extractPassword(n.RawURL)
	q := url.Values{}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", url.QueryEscape(password), addr, port, q.Encode(), url.QueryEscape(name))
}

func buildVMessLink(n model.ProxyNode, addr string, port int, name string) string {
	uuid := extractUUID(n.RawURL)
	cfg := fmt.Sprintf(`{"v":"2","ps":"%s","add":"%s","port":%d,"id":"%s","aid":0,"scy":"auto","net":"tcp","type":"none","host":"","path":"","tls":""}`, name, addr, port, uuid)
	return "vmess://" + base64.StdEncoding.EncodeToString([]byte(cfg))
}

func extractUUID(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return ""
	}
	rest := rawURL[schemeEnd+3:]
	if idx := strings.Index(rest, "@"); idx >= 0 {
		return rest[:idx]
	}
	return ""
}

func extractPassword(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return ""
	}
	rest := rawURL[schemeEnd+3:]
	if idx := strings.Index(rest, "@"); idx >= 0 {
		return rest[:idx]
	}
	return ""
}

func extractSSAuth(rawURL string) (cipher, password string) {
	if rawURL == "" {
		return "", ""
	}
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return "", ""
	}
	rest := rawURL[schemeEnd+3:]
	if idx := strings.Index(rest, "@"); idx >= 0 {
		decoded, err := base64.StdEncoding.DecodeString(rest[:idx])
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}
	return "", ""
}

func parseRealityForLink(rawURL string) (pbk, sid string) {
	if rawURL == "" {
		return "", ""
	}
	qIdx := strings.Index(rawURL, "?")
	if qIdx < 0 {
		return "", ""
	}
	query := rawURL[qIdx+1:]
	if fIdx := strings.Index(query, "#"); fIdx >= 0 {
		query = query[:fIdx]
	}
	for _, part := range strings.Split(query, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "pbk":
			pbk, _ = url.QueryUnescape(kv[1])
		case "sid":
			sid, _ = url.QueryUnescape(kv[1])
		}
	}
	return
}

func (s *SubscriptionService) getConfigForHash(hash string) string {
	cfg, err := s.GenerateConfig(hash)
	if err != nil {
		return "# PigeonRelay\nproxies: []\n"
	}
	return cfg
}

func isIPAddress(s string) bool {
	return net.ParseIP(s) != nil
}
