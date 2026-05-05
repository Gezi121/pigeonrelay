package service

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type NodeService struct {
	repo *repository.Repo
}

func NewNodeService(repo *repository.Repo) *NodeService {
	return &NodeService{repo: repo}
}

func (s *NodeService) ParseLinks(raw string) ([]model.ParsedNode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty input")
	}

	lines := strings.Split(raw, "\n")
	var nodes []model.ParsedNode

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		n, err := parseSingleLink(line)
		if err != nil {
			// skip unparseable lines
			continue
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid links found")
	}
	return nodes, nil
}

func parseSingleLink(rawURL string) (model.ParsedNode, error) {
	n := model.ParsedNode{RawURL: rawURL}

	// Detect protocol scheme: vless:// vmess:// trojan:// ss:// hysteria2://
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return n, fmt.Errorf("invalid scheme")
	}
	scheme := rawURL[:schemeEnd]
	n.Protocol = scheme

	rest := rawURL[schemeEnd+3:]

	switch scheme {
	case "vless", "trojan":
		parseVlessTrojan(rest, &n)
	case "vmess":
		parseVmess(rest, &n)
	case "ss":
		parseShadowsocks(rest, &n)
	case "hysteria2", "hy2":
		n.Protocol = "hysteria2"
		parseHysteria2(rest, &n)
	default:
		return n, fmt.Errorf("unsupported protocol: %s", scheme)
	}

	if n.Name == "" {
		n.Name = fmt.Sprintf("%s-%s:%d", n.Protocol, n.Address, n.Port)
	}
	return n, nil
}

func parseVlessTrojan(rest string, n *model.ParsedNode) {
	// format: uuid@host:port?params#name
	// find # for name
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		n.Name, _ = url.QueryUnescape(rest[idx+1:])
		rest = rest[:idx]
	}

	// split at ?
	var queryStr string
	if idx := strings.Index(rest, "?"); idx >= 0 {
		queryStr = rest[idx+1:]
		rest = rest[:idx]
	}

	// rest: uuid@host:port
	atIdx := strings.LastIndex(rest, "@")
	if atIdx >= 0 {
		addrPort := rest[atIdx+1:]
		if colonIdx := strings.LastIndex(addrPort, ":"); colonIdx >= 0 {
			n.Address = addrPort[:colonIdx]
			n.Port, _ = strconv.Atoi(addrPort[colonIdx+1:])
		} else {
			n.Address = addrPort
		}
	}

	// parse query params
	if queryStr != "" {
		params, _ := url.ParseQuery(queryStr)
		n.SNI = params.Get("sni")
		n.Host = params.Get("host")
		n.Path = params.Get("path")
		n.Security = params.Get("security")
		n.ALPN = params.Get("alpn")
		n.Fingerprint = params.Get("fp")
		n.Encryption = params.Get("encryption")
		typ := params.Get("type")
		if typ == "" {
			typ = "tcp"
		}
		n.Transport = typ
	}
}

func parseVmess(rest string, n *model.ParsedNode) {
	// vmess://base64json
	// simplified: treat as opaque, extract name from #
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		n.Name, _ = url.QueryUnescape(rest[idx+1:])
	}
	n.Address = "?"
	n.Port = 0
}

func parseShadowsocks(rest string, n *model.ParsedNode) {
	// ss://base64@host:port or ss://base64#name
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		n.Name, _ = url.QueryUnescape(rest[idx+1:])
		rest = rest[:idx]
	}
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		addrPort := rest[atIdx+1:]
		if colonIdx := strings.LastIndex(addrPort, ":"); colonIdx >= 0 {
			n.Address = addrPort[:colonIdx]
			n.Port, _ = strconv.Atoi(addrPort[colonIdx+1:])
		}
	}
}

func parseHysteria2(rest string, n *model.ParsedNode) {
	// hysteria2://host:port?params#name
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		n.Name, _ = url.QueryUnescape(rest[idx+1:])
		rest = rest[:idx]
	}
	var queryStr string
	if idx := strings.Index(rest, "?"); idx >= 0 {
		queryStr = rest[idx+1:]
		rest = rest[:idx]
	}
	if colonIdx := strings.LastIndex(rest, ":"); colonIdx >= 0 {
		n.Address = rest[:colonIdx]
		n.Port, _ = strconv.Atoi(rest[colonIdx+1:])
	} else {
		n.Address = rest
	}
	if queryStr != "" {
		params, _ := url.ParseQuery(queryStr)
		n.SNI = params.Get("sni")
		n.Security = "tls"
	}
}

func (s *NodeService) Create(userID int64, req *model.CreateNodeRequest) (*model.ProxyNode, error) {
	return s.repo.CreateNode(userID, req)
}

func (s *NodeService) GetByID(id, userID int64) (*model.ProxyNode, error) {
	return s.repo.GetNodeByID(id, userID)
}

func (s *NodeService) List(userID int64) ([]model.ProxyNode, error) {
	return s.repo.ListNodes(userID)
}

func (s *NodeService) Update(id, userID int64, req *model.UpdateNodeRequest) (*model.ProxyNode, error) {
	return s.repo.UpdateNode(id, userID, req)
}

func (s *NodeService) Delete(id, userID int64) error {
	return s.repo.DeleteNode(id, userID)
}

func (s *NodeService) Reorder(userID int64, orderedIDs []int64) error {
	return s.repo.ReorderNodes(userID, orderedIDs)
}

func (s *NodeService) Test(id, userID int64) (bool, error) {
	n, err := s.repo.GetNodeByID(id, userID)
	if err != nil {
		return false, err
	}
	ok := testNode(n)
	status := "ok"
	if !ok {
		status = "error"
	}
	s.repo.UpdateNodeStatus(id, status)
	return ok, nil
}

func testNode(n *model.ProxyNode) bool {
	switch n.Protocol {
	case "hysteria2", "hy2":
		// Hysteria2 uses QUIC/UDP; TCP dial would falsely fail.
		// Resolve host as minimal liveness check.
		_, err := net.LookupHost(n.Address)
		return err == nil
	default:
		return tcpping(fmt.Sprintf("%s:%d", n.Address, n.Port))
	}
}

func (s *NodeService) GetLatencyBest(fullDomain string) (string, int, error) {
	return s.repo.GetBestIPForDomainV2(fullDomain)
}

func (s *NodeService) GetLatencyBestDetail(fullDomain string) (*model.BestIPDetail, error) {
	return s.repo.GetBestIPDetail(fullDomain)
}

func (s *NodeService) GetLatencyBestDetailClient(fullDomain, clientID string) (*model.BestIPDetail, error) {
	if clientID == "" {
		return s.repo.GetBestIPDetail(fullDomain)
	}
	return s.repo.GetBestIPDetailClient(fullDomain, clientID)
}

func (s *NodeService) ListDomainMappings() ([]model.DomainMapping, error) {
	return s.repo.ListDomainMappings()
}

func (s *NodeService) UpdateDomainMappingFull(id int64, subdomain, fullDomain, algorithmType string) error {
	return s.repo.UpdateDomainMappingFull(id, subdomain, fullDomain, algorithmType)
}

func (s *NodeService) UpdateDomainMapping(id int64, algorithmType string) error {
	return s.repo.UpdateDomainMappingFull(id, "", "", algorithmType)
}

func (s *NodeService) CreateDomainMapping(subdomain, fullDomain, algorithmType, source string) (*model.DomainMapping, error) {
	return s.repo.CreateDomainMapping(subdomain, fullDomain, algorithmType, source)
}

func (s *NodeService) BatchCreateDomainMappings(source string, subdomains []string, suffix string, domains []string) ([]model.DomainMapping, error) {
	return s.repo.BatchCreateDomainMappings(source, subdomains, suffix, domains)
}

func (s *NodeService) DeleteDomainMapping(id int64) error {
	return s.repo.DeleteDomainMapping(id)
}

func (s *NodeService) CreateCFBinding(nodeID int64, fullDomain, namePrefix, source string) (*model.NodeCFBinding, error) {
	return s.repo.CreateCFBinding(nodeID, fullDomain, namePrefix, source)
}

func (s *NodeService) ListCFBindings(nodeID int64) ([]model.NodeCFBinding, error) {
	return s.repo.ListCFBindings(nodeID)
}

func (s *NodeService) DeleteCFBinding(id, nodeID int64) error {
	return s.repo.DeleteCFBinding(id, nodeID)
}

func (s *NodeService) PingDomain(domain string) (bool, string, int, error) {
	ips, err := net.LookupHost(domain)
	if err != nil || len(ips) == 0 {
		return false, "", 0, nil
	}
	ip := ips[0]
	start := time.Now()
	conn, err := net.DialTimeout("tcp", ip+":443", 3*time.Second)
	if err != nil {
		return false, ip, 0, nil
	}
	conn.Close()
	return true, ip, int(time.Since(start).Milliseconds()), nil
}
