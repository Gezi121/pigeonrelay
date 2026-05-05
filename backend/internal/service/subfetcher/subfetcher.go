package subfetcher

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ProxyNode struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Server  string                 `json:"server"`
	Port    int                    `json:"port"`
	UUID    string                 `json:"uuid,omitempty"`
	Network string                 `json:"network,omitempty"`
	Original map[string]interface{} `json:"original,omitempty"`
	Raw     string                 `json:"raw"`
}

type FetchResult struct {
	Nodes      []ProxyNode `json:"nodes"`
	NodeCount  int         `json:"node_count"`
	Type       string      `json:"type"`
	Summary    string      `json:"summary"`
	RawContent string      `json:"raw_content"`
}

var client = &http.Client{Timeout: 30 * time.Second}

func Fetch(sourceURL string) (*FetchResult, error) {
	req, err := http.NewRequest("GET", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "PigeonRelay/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")

	var res *FetchResult
	var parseErr error

	if isClashYAML(content, contentType) {
		res, parseErr = parseClash(content)
	} else {
		res, parseErr = parseV2Ray(content)
	}

	if parseErr != nil {
		return nil, parseErr
	}
	if res != nil {
		res.RawContent = content
	}
	return res, nil
}

func isClashYAML(content, contentType string) bool {
	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "text/plain") {
		return strings.Contains(content, "proxies:") || strings.Contains(content, "Proxy:")
	}
	return strings.Contains(content, "proxies:")
}

func parseClash(content string) (*FetchResult, error) {
	var doc struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, fmt.Errorf("parse clash yaml: %w", err)
	}

	var nodes []ProxyNode
	for _, p := range doc.Proxies {
		n := ProxyNode{Raw: fmt.Sprintf("%v", p), Original: p}
		if name, ok := p["name"].(string); ok {
			n.Name = name
		}
		if t, ok := p["type"].(string); ok {
			n.Type = t
		}
		if s, ok := p["server"].(string); ok {
			n.Server = s
		}
		if port, ok := p["port"].(int); ok {
			n.Port = port
		}
		if u, ok := p["uuid"].(string); ok {
			n.UUID = u
		}
		if netw, ok := p["network"].(string); ok {
			n.Network = netw
		}
		nodes = append(nodes, n)
	}

	return &FetchResult{
		Nodes:     nodes,
		NodeCount: len(nodes),
		Type:      "clash",
		Summary:   fmt.Sprintf("Clash 订阅，共 %d 个节点", len(nodes)),
	}, nil
}

func parseV2Ray(content string) (*FetchResult, error) {
	decoded := tryBase64Decode(content)

	var nodes []ProxyNode
	lines := strings.Split(decoded, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		node := parseV2RayURI(line)
		if node != nil {
			nodes = append(nodes, *node)
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no valid v2ray nodes found")
	}

	return &FetchResult{
		Nodes:     nodes,
		NodeCount: len(nodes),
		Type:      "v2ray",
		Summary:   fmt.Sprintf("V2Ray 订阅，共 %d 个节点", len(nodes)),
	}, nil
}

func tryBase64Decode(s string) string {
	s = strings.TrimSpace(s)
	if !looksLikeBase64(s) {
		return s
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(s)
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(s)
			if err != nil {
				return s
			}
		}
	}
	return string(decoded)
}

func looksLikeBase64(s string) bool {
	return !strings.Contains(s, "://") && !strings.Contains(s, "proxies:")
}

func parseV2RayURI(uri string) *ProxyNode {
	u, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	n := &ProxyNode{Type: u.Scheme, Raw: uri}
	n.Original = make(map[string]interface{})
	switch u.Scheme {
	case "vmess":
		parseVmess(u, n)
	case "vless":
		parseVlessTrojan(u, n)
	case "trojan":
		parseVlessTrojan(u, n)
	case "ss":
		parseSS(u, n)
	case "hysteria2", "hy2":
		parseHysteria2(u, n)
	default:
		return nil
	}
	return n
}

func parseVmess(u *url.URL, n *ProxyNode) {
	n.Original["type"] = "vmess"
	data, err := base64.RawStdEncoding.DecodeString(u.Host)
	if err != nil {
		data, err = base64.StdEncoding.DecodeString(u.Host)
		if err != nil {
			n.Server = u.Host
			return
		}
	}
	var vm struct {
		Add  string `json:"add"`
		Port int    `json:"port"`
		ID   string `json:"id"`
		Net  string `json:"net"`
		PS   string `json:"ps"`
	}
	if err := yaml.Unmarshal(data, &vm); err == nil || true {
		n.Server = vm.Add
		n.Port = vm.Port
		n.UUID = vm.ID
		n.Network = vm.Net
		n.Name = vm.PS

		n.Original["server"] = vm.Add
		n.Original["port"] = vm.Port
		n.Original["uuid"] = vm.ID
		n.Original["network"] = vm.Net
		n.Original["name"] = vm.PS
	}
}

func parseVlessTrojan(u *url.URL, n *ProxyNode) {
	n.Original["type"] = u.Scheme
	n.Server = u.Hostname()
	portStr := u.Port()
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &n.Port)
	} else {
		n.Port = 443
	}

	n.UUID = u.User.Username()
	if n.UUID == "" {
		n.UUID = u.User.String()
	}
	n.Name = u.Fragment

	n.Original["server"] = n.Server
	n.Original["port"] = n.Port
	n.Original["name"] = n.Name
	if u.Scheme == "trojan" {
		n.Original["password"] = n.UUID
	} else {
		n.Original["uuid"] = n.UUID
	}

	q := u.Query()
	if q.Get("type") != "" {
		n.Network = q.Get("type")
		n.Original["network"] = n.Network
	}
	if q.Get("security") == "tls" || q.Get("security") == "reality" {
		n.Original["tls"] = true
	}
	if q.Get("sni") != "" {
		n.Original["sni"] = q.Get("sni")
	}
	if q.Get("pbk") != "" {
		opts := make(map[string]interface{})
		opts["public-key"] = q.Get("pbk")
		if q.Get("sid") != "" {
			opts["short-id"] = q.Get("sid")
		}
		n.Original["reality-opts"] = opts
	}

	if q.Get("alpn") != "" {
		alpnStr := q.Get("alpn")
		n.Original["alpn"] = strings.Split(alpnStr, ",")
	}

	if q.Get("path") != "" || q.Get("host") != "" {
		opts := make(map[string]interface{})
		if q.Get("path") != "" {
			opts["path"] = q.Get("path")
		}
		if q.Get("host") != "" {
			headers := make(map[string]string)
			headers["Host"] = q.Get("host")
			opts["headers"] = headers
		}
		if n.Network == "ws" {
			n.Original["ws-opts"] = opts
		} else if n.Network == "xhttp" {
			n.Original["xhttp-opts"] = opts
		}
	} else if n.Network == "grpc" {
		opts := make(map[string]interface{})
		if q.Get("serviceName") != "" {
			opts["grpc-service-name"] = q.Get("serviceName")
		}
		if q.Get("mode") == "multi" {
			opts["mode"] = "multi"
		}
		n.Original["grpc-opts"] = opts
	}
}

func parseSS(u *url.URL, n *ProxyNode) {
	n.Original["type"] = "ss"
	n.Server = u.Hostname()
	portStr := u.Port()
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &n.Port)
	} else {
		n.Port = 8388
	}
	n.Name = u.Fragment

	n.Original["server"] = n.Server
	n.Original["port"] = n.Port
	n.Original["name"] = n.Name

	userinfo := u.User.String()
	decoded, err := base64.RawURLEncoding.DecodeString(userinfo)
	if err == nil {
		userinfo = string(decoded)
	} else if decoded, err := base64.URLEncoding.DecodeString(userinfo); err == nil {
		userinfo = string(decoded)
	}
	parts := strings.SplitN(userinfo, ":", 2)
	if len(parts) == 2 {
		n.Original["cipher"] = parts[0]
		n.Original["password"] = parts[1]
	}

	q := u.Query()
	if plugin := q.Get("plugin"); plugin != "" {
		n.Original["plugin"] = strings.Split(plugin, ";")[0]
		pluginOpts := strings.TrimPrefix(plugin, n.Original["plugin"].(string)+";")
		if pluginOpts != "" {
			// Basic parsing for obfs plugin
			optsMap := make(map[string]interface{})
			opts := strings.Split(pluginOpts, ";")
			for _, opt := range opts {
				kv := strings.SplitN(opt, "=", 2)
				if len(kv) == 2 {
					if kv[0] == "obfs" {
						optsMap["mode"] = kv[1]
					} else if kv[0] == "obfs-host" {
						optsMap["host"] = kv[1]
					} else {
						optsMap[kv[0]] = kv[1]
					}
				}
			}
			n.Original["plugin-opts"] = optsMap
		}
	}
}

func parseHysteria2(u *url.URL, n *ProxyNode) {
	n.Original["type"] = "hysteria2"
	n.Type = "hysteria2"
	n.Server = u.Hostname()
	if portStr := u.Port(); portStr != "" {
		fmt.Sscanf(portStr, "%d", &n.Port)
	} else {
		n.Port = 443
	}
	n.Name = u.Fragment
	n.Original["server"] = n.Server
	n.Original["port"] = n.Port
	n.Original["name"] = n.Name
	n.Original["password"] = u.User.Username()
	n.Original["tls"] = true
	n.Original["udp"] = true
	n.Original["skip-cert-verify"] = false

	q := u.Query()
	if sni := q.Get("sni"); sni != "" {
		n.Original["sni"] = sni
	}
	if insecure := q.Get("insecure"); insecure == "1" {
		n.Original["skip-cert-verify"] = true
	}
}
