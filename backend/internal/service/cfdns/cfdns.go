package cfdns

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"nodeforge/internal/repository"
)

type Service struct {
	repo         *repository.Repo
	envToken     string
	envZoneID    string
	client       *http.Client
	cachedZoneID string
}

type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

func New(repo *repository.Repo, token, zoneID string) *Service {
	return &Service{
		repo:      repo,
		envToken:  token,
		envZoneID: zoneID,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) token() string {
	if v, _ := s.repo.GetSetting("cf_api_token"); v != "" {
		return v
	}
	return s.envToken
}

func (s *Service) zoneID() string {
	if v, _ := s.repo.GetSetting("cf_zone_id"); v != "" {
		return v
	}
	if s.envZoneID != "" {
		return s.envZoneID
	}
	return s.cachedZoneID
}

func (s *Service) resolveZoneIDFromToken(domain string) {
	if s.zoneID() != "" {
		return
	}
	root := rootDomain(domain)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", root)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.token())
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[cfdns] zone lookup failed: %v", err)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&result); err != nil {
		return
	}
	if len(result.Result) > 0 {
		s.cachedZoneID = result.Result[0].ID
		log.Printf("[cfdns] auto-discovered zone %s -> %s", root, s.cachedZoneID)
	}
}

func (s *Service) Enabled() bool {
	return s.token() != ""
}

// UpdateBestIPs reads local domain mappings from the DB, finds the best
// CF IP for each root domain, and updates the A record via CF API.
func (s *Service) UpdateBestIPs() {
	if !s.Enabled() {
		log.Printf("[cfdns] skipped: no token configured")
		return
	}
	mappings, err := s.repo.ListDomainMappings()
	if err != nil {
		log.Printf("[cfdns] list domains: %v", err)
		return
	}
	localCount := 0
	assigned := make(map[string]bool)
	for _, dm := range mappings {
		if dm.Source == "external" || dm.FullDomain == "" {
			continue
		}
		localCount++
		s.resolveZoneIDFromToken(dm.FullDomain)
		if s.zoneID() == "" {
			log.Printf("[cfdns] skip %s: no zone ID", dm.FullDomain)
			continue
		}
		root := rootDomain(dm.FullDomain)
		ips, err := s.repo.GetBestIPsForDomainV2(dm.FullDomain, 20)
		if err != nil || len(ips) == 0 {
			log.Printf("[cfdns] skip %s: no candidate IPs", dm.FullDomain)
			continue
		}
		var selected string
		for _, ip := range ips {
			if ip == "1.1.1.1" || assigned[ip] {
				continue
			}
			if tlsCheck(ip, root) {
				selected = ip
				break
			}
		}
		if selected == "" {
			for _, ip := range ips {
				if ip == "1.1.1.1" {
					continue
				}
				if tlsCheck(ip, root) {
					selected = ip
					break
				}
			}
		}
		if selected == "" {
			log.Printf("[cfdns] skip %s: all %d candidates failed TLS check", dm.FullDomain, len(ips))
			continue
		}
		assigned[selected] = true
		if selected == "" {
			log.Printf("[cfdns] skip %s: all %d candidates failed TLS check", dm.FullDomain, len(ips))
			continue
		}
		if err := s.upsertARecord(dm.FullDomain, selected); err != nil {
			log.Printf("[cfdns] update %s failed: %v", dm.FullDomain, err)
		} else {
			log.Printf("[cfdns] %s -> %s", dm.FullDomain, selected)
		}
	}
	log.Printf("[cfdns] done: %d local domains processed", localCount)
}

func (s *Service) upsertARecord(name, ip string) error {
	// Find and delete any existing record (A, CNAME, proxied, etc.)
	id, rtype, err := s.findAnyRecord(name)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}
	if id != "" {
		if rtype == "A" {
			// Update existing A record to grey cloud
			return s.updateRecord(id, name, ip)
		}
		// Delete non-A record (e.g. CNAME) and recreate as A
		if err := s.deleteRecord(id); err != nil {
			log.Printf("[cfdns] delete old %s record for %s: %v", rtype, name, err)
		}
	}
	return s.createRecord(name, ip)
}

func (s *Service) findAnyRecord(name string) (id, rtype string, err error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", s.zoneID(), name)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.token())
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var result struct {
		Result []dnsRecord `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&result); err != nil {
		return "", "", err
	}
	if len(result.Result) > 0 {
		return result.Result[0].ID, result.Result[0].Type, nil
	}
	return "", "", nil
}

func (s *Service) deleteRecord(id string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", s.zoneID(), id)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.token())
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *Service) createRecord(name, ip string) error {
	body, _ := json.Marshal(dnsRecord{
		Name:    name,
		Type:    "A",
		Content: ip,
		Proxied: false,
		TTL:     120,
	})
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", s.zoneID())
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.token())
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *Service) updateRecord(id, name, ip string) error {
	body, _ := json.Marshal(dnsRecord{
		Name:    name,
		Type:    "A",
		Content: ip,
		Proxied: false,
		TTL:     120,
	})
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", s.zoneID(), id)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.token())
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func tlsCheck(ip, sni string) bool {
	conn, err := net.DialTimeout("tcp", ip+":443", 3*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	tlsCfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true}
	tlsConn := tls.Client(conn, tlsCfg)
	err = tlsConn.Handshake()
	tlsConn.Close()
	return err == nil
}

func rootDomain(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return name
}
