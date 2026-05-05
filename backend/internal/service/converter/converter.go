package converter

import (
	"encoding/json"
	"fmt"

	"nodeforge/internal/service/subfetcher"
)

type UnifiedConfig struct {
	ReplacePort int                    `json:"replace_port"`
	ExtraParams map[string]interface{} `json:"extra_params"`
}

type PreferConfig struct {
	Mode      string   `json:"mode"`
	Addresses []string `json:"addresses"`
}

func Process(nodes []subfetcher.ProxyNode, unified *UnifiedConfig, prefer *PreferConfig) []subfetcher.ProxyNode {
	if unified != nil {
		nodes = applyUnified(nodes, unified)
	}
	if prefer != nil {
		nodes = applyPrefer(nodes, prefer)
	}
	return nodes
}

func applyUnified(nodes []subfetcher.ProxyNode, c *UnifiedConfig) []subfetcher.ProxyNode {
	result := make([]subfetcher.ProxyNode, len(nodes))
	for i, n := range nodes {
		if c.ReplacePort > 0 {
			n.Port = c.ReplacePort
		}
		for k, v := range c.ExtraParams {
			n.Raw = mergeJSONParam(n.Raw, k, v)
		}
		result[i] = n
	}
	return result
}

func applyPrefer(nodes []subfetcher.ProxyNode, c *PreferConfig) []subfetcher.ProxyNode {
	if c.Mode != "custom" || len(c.Addresses) == 0 {
		return nodes
	}
	var result []subfetcher.ProxyNode
	for _, n := range nodes {
		for _, addr := range c.Addresses {
			dup := n
			dup.Server = addr
			dup.Name = fmt.Sprintf("%s-%s", n.Name, addr)
			result = append(result, dup)
		}
	}
	return result
}

func mergeJSONParam(raw, key string, val interface{}) string {
	m := map[string]interface{}{}
	if raw != "" {
		json.Unmarshal([]byte(raw), &m)
	}
	m[key] = val
	data, _ := json.Marshal(m)
	return string(data)
}
