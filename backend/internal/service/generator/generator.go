package generator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"nodeforge/internal/service/converter"
	"nodeforge/internal/service/subfetcher"
)

// deepCopy creates a deep copy of the map or slice to prevent modifications from leaking
func deepCopy(src interface{}) interface{} {
	switch v := src.(type) {
	case map[string]interface{}:
		dst := make(map[string]interface{})
		for k, val := range v {
			dst[k] = deepCopy(val)
		}
		return dst
	case map[interface{}]interface{}:
		dst := make(map[string]interface{})
		for k, val := range v {
			dst[fmt.Sprintf("%v", k)] = deepCopy(val)
		}
		return dst
	case []interface{}:
		dst := make([]interface{}, len(v))
		for i, val := range v {
			dst[i] = deepCopy(val)
		}
		return dst
	default:
		return src
	}
}

type GenerateRequest struct {
	Sources       []string               `json:"sources"`
	UnifiedConfig *converter.UnifiedConfig `json:"unified_config"`
	PreferConfig  *converter.PreferConfig  `json:"prefer_config"`
	OutputType    string                 `json:"output_type"`
}

func Generate(req *GenerateRequest) (string, error) {
	var allNodes []subfetcher.ProxyNode
	for _, src := range req.Sources {
		result, err := subfetcher.Fetch(src)
		if err != nil {
			return "", fmt.Errorf("fetch %s: %w", src, err)
		}
		allNodes = append(allNodes, result.Nodes...)
	}

	allNodes = converter.Process(allNodes, req.UnifiedConfig, req.PreferConfig)

	switch req.OutputType {
	case "v2ray", "json":
		return renderV2Ray(allNodes)
	default:
		return renderClash(allNodes)
	}
}

func renderClash(nodes []subfetcher.ProxyNode) (string, error) {
	proxies := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		p := make(map[string]interface{})
		if n.Original != nil {
			for k, v := range n.Original {
				p[k] = deepCopy(v)
			}
		}

		p["name"] = n.Name
		p["type"] = n.Type
		p["server"] = n.Server
		p["port"] = n.Port

		if n.UUID != "" {
			if n.Type == "trojan" {
				p["password"] = n.UUID
			} else {
				p["uuid"] = n.UUID
			}
		}
		if n.Network != "" {
			p["network"] = n.Network

			if n.Network == "xhttp" {
				if wsOpts, ok := p["ws-opts"]; ok {
					p["xhttp-opts"] = wsOpts
					delete(p, "ws-opts")
				}
			}
		}
		proxies = append(proxies, p)
	}

	doc := map[string]interface{}{
		"proxies": proxies,
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderV2Ray(nodes []subfetcher.ProxyNode) (string, error) {
	var lines []string
	for _, n := range nodes {
		line := n.Raw
		if line == "" || strings.Contains(line, "map[") || strings.HasPrefix(line, "map[") || strings.Contains(line, "proxies:") {
			line = buildV2RayURI(n)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func buildV2RayURI(n subfetcher.ProxyNode) string {
	switch n.Type {
	case "vmess":
		content := map[string]interface{}{
			"v": "2",
			"add": n.Server,
			"port": n.Port,
			"ps":  n.Name,
		}
		if n.UUID != "" {
			content["id"] = n.UUID
		}
		if n.Network != "" {
			content["net"] = n.Network
		}
		if n.Original != nil {
			if tls, ok := n.Original["tls"].(bool); ok && tls {
				content["tls"] = "tls"
			}
			if sni, ok := n.Original["sni"].(string); ok {
				content["sni"] = sni
			}
			if wsOpts, ok := n.Original["ws-opts"].(map[string]interface{}); ok {
				if path, ok := wsOpts["path"].(string); ok {
					content["path"] = path
				}
				if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
					if host, ok := headers["Host"].(string); ok {
						content["host"] = host
					}
				} else if headers, ok := wsOpts["headers"].(map[interface{}]interface{}); ok {
					if host, ok := headers["Host"].(string); ok {
						content["host"] = host
					}
				}
			} else if wsOpts, ok := n.Original["ws-opts"].(map[interface{}]interface{}); ok {
				if path, ok := wsOpts["path"].(string); ok {
					content["path"] = path
				}
			}
		}
		data, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf("vmess://%s", encoded)

	case "vless", "trojan":
		password := n.UUID
		if n.Type == "trojan" && n.Original != nil {
			if pwd, ok := n.Original["password"].(string); ok {
				password = pwd
			}
		}
		u := fmt.Sprintf("%s://%s@%s:%d?type=%s", n.Type, password, n.Server, n.Port, n.Network)
		if n.Original != nil {
			if tls, ok := n.Original["tls"].(bool); ok && tls {
				u += "&security=tls"
			}
			if reality, ok := n.Original["reality-opts"].(map[interface{}]interface{}); ok {
				u += "&security=reality"
				if pbk := fmt.Sprintf("%v", reality["public-key"]); pbk != "<nil>" && pbk != "" {
					u += fmt.Sprintf("&pbk=%s", url.QueryEscape(pbk))
				}
				if sid := fmt.Sprintf("%v", reality["short-id"]); sid != "<nil>" && sid != "" {
					u += fmt.Sprintf("&sid=%s", url.QueryEscape(sid))
				}
			} else if reality, ok := n.Original["reality-opts"].(map[string]interface{}); ok {
				u += "&security=reality"
				if pbk := fmt.Sprintf("%v", reality["public-key"]); pbk != "<nil>" && pbk != "" {
					u += fmt.Sprintf("&pbk=%s", url.QueryEscape(pbk))
				}
				if sid := fmt.Sprintf("%v", reality["short-id"]); sid != "<nil>" && sid != "" {
					u += fmt.Sprintf("&sid=%s", url.QueryEscape(sid))
				}
			}
			if sni, ok := n.Original["sni"].(string); ok {
				u += fmt.Sprintf("&sni=%s", sni)
			}
			if alpn, ok := n.Original["alpn"].([]interface{}); ok && len(alpn) > 0 {
				var alpnStrs []string
				for _, a := range alpn {
					alpnStrs = append(alpnStrs, fmt.Sprintf("%v", a))
				}
				u += fmt.Sprintf("&alpn=%s", url.QueryEscape(strings.Join(alpnStrs, ",")))
			}

			optsKey := "ws-opts"
			if n.Network == "xhttp" {
				optsKey = "xhttp-opts"
			}
			if opts, ok := n.Original[optsKey].(map[string]interface{}); ok {
				if path, ok := opts["path"].(string); ok && path != "" {
					u += fmt.Sprintf("&path=%s", url.QueryEscape(path))
				}
				if headers, ok := opts["headers"].(map[string]interface{}); ok {
					if host, ok := headers["Host"].(string); ok && host != "" {
						u += fmt.Sprintf("&host=%s", url.QueryEscape(host))
					}
				} else if headers, ok := opts["headers"].(map[interface{}]interface{}); ok {
					if host, ok := headers["Host"].(string); ok && host != "" {
						u += fmt.Sprintf("&host=%s", url.QueryEscape(host))
					}
				}
			} else if opts, ok := n.Original[optsKey].(map[interface{}]interface{}); ok {
				if path, ok := opts["path"].(string); ok && path != "" {
					u += fmt.Sprintf("&path=%s", url.QueryEscape(path))
				}
			}

			if n.Network == "grpc" {
				if grpcOpts, ok := n.Original["grpc-opts"].(map[string]interface{}); ok {
					if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
						u += fmt.Sprintf("&serviceName=%s", url.QueryEscape(sn))
					}
					if mode, ok := grpcOpts["mode"].(string); ok && mode != "" {
						u += fmt.Sprintf("&mode=%s", url.QueryEscape(mode))
					}
				} else if grpcOpts, ok := n.Original["grpc-opts"].(map[interface{}]interface{}); ok {
					if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
						u += fmt.Sprintf("&serviceName=%s", url.QueryEscape(sn))
					}
				}
			}
		}
		u += fmt.Sprintf("#%s", url.QueryEscape(n.Name))
		return u

	case "ss":
		method := "aes-256-gcm"
		password := "password"
		if n.Original != nil {
			if m, ok := n.Original["cipher"].(string); ok && m != "" {
				method = m
			}
			if p, ok := n.Original["password"].(string); ok && p != "" {
				password = p
			}
		}
		userinfo := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", method, password)))
		u := fmt.Sprintf("ss://%s@%s:%d", userinfo, n.Server, n.Port)

		var pluginOpts []string
		if n.Original != nil {
			if plugin, ok := n.Original["plugin"].(string); ok && plugin != "" {
				u += fmt.Sprintf("/?plugin=%s", url.QueryEscape(plugin))
				if pOpts, ok := n.Original["plugin-opts"].(map[string]interface{}); ok {
					for k, v := range pOpts {
						if k == "mode" {
							pluginOpts = append(pluginOpts, fmt.Sprintf("obfs=%v", v))
						} else if k == "host" {
							pluginOpts = append(pluginOpts, fmt.Sprintf("obfs-host=%v", v))
						} else {
							pluginOpts = append(pluginOpts, fmt.Sprintf("%s=%v", k, v))
						}
					}
				} else if pOpts, ok := n.Original["plugin-opts"].(map[interface{}]interface{}); ok {
					if mode, ok := pOpts["mode"]; ok {
						pluginOpts = append(pluginOpts, fmt.Sprintf("obfs=%v", mode))
					}
				}
				if len(pluginOpts) > 0 {
					u += url.QueryEscape(";" + strings.Join(pluginOpts, ";"))
				}
			}
		}
		if strings.Contains(u, "?") {
			u += fmt.Sprintf("#%s", url.QueryEscape(n.Name))
		} else {
			u += fmt.Sprintf("#%s", url.QueryEscape(n.Name))
		}
		return u

	case "hysteria2", "hy2":
		password := ""
		if n.Original != nil {
			if pwd, ok := n.Original["password"].(string); ok {
				password = pwd
			}
		}
		u := fmt.Sprintf("hysteria2://%s@%s:%d/?", password, n.Server, n.Port)
		if n.Original != nil {
			if sni, ok := n.Original["sni"].(string); ok {
				u += fmt.Sprintf("sni=%s&", sni)
			}
			if insecure, ok := n.Original["skip-cert-verify"].(bool); ok && insecure {
				u += "insecure=1&"
			}
		}
		u += fmt.Sprintf("#%s", url.QueryEscape(n.Name))
		return strings.ReplaceAll(u, "/?#", "#")

	default:
		return ""
	}
}
