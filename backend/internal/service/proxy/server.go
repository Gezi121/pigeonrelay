package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"nodeforge/internal/repository"
)

type trafficRecord struct {
	userID int64
	up     int64
	dn     int64
	sni    string
	client string
}

type Server struct {
	addr         string
	repo         *repository.Repo
	adminUserID  int64
	adminResolved bool
	totalBytesUp atomic.Int64
	totalBytesDn atomic.Int64
	trafficCh    chan trafficRecord
}

func NewServer(addr string, repo *repository.Repo) *Server {
	return &Server{
		addr:      addr,
		repo:      repo,
		trafficCh: make(chan trafficRecord, 256),
	}
}

func (s *Server) Stats() (up, dn int64) {
	return s.totalBytesUp.Load(), s.totalBytesDn.Load()
}

func (s *Server) resolveAdmin() {
	if s.adminResolved {
		return
	}
	if u, err := s.repo.GetAdminUser(); err == nil {
		s.adminUserID = u.ID
	}
	s.adminResolved = true
}

func (s *Server) Start() {
	// Background traffic writer: drains channel and batch-writes to DB
	go func() {
		batch := make([]trafficRecord, 0, 64)
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case r := <-s.trafficCh:
				batch = append(batch, r)
				if len(batch) >= 64 {
					s.flushTraffic(batch)
					batch = batch[:0]
				}
			case <-tick.C:
				if len(batch) > 0 {
					s.flushTraffic(batch)
					batch = batch[:0]
				}
			}
		}
	}()

	go func() {
		l, err := net.Listen("tcp", s.addr)
		if err != nil {
			log.Printf("[SNI Proxy] failed to listen on %s: %v", s.addr, err)
			return
		}
		log.Printf("[SNI Proxy] listening on %s", s.addr)
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("[SNI Proxy] accept error: %v", err)
				continue
			}
			go s.handleConnection(conn)
		}
	}()
}

func (s *Server) flushTraffic(batch []trafficRecord) {
	for _, r := range batch {
		if _, err := s.repo.CreateTrafficLog(r.userID, nil, r.up, r.dn); err != nil {
			log.Printf("[SNI Proxy] traffic log error: %v", err)
		}
		if err := s.repo.IncrementUserUsedBytes(r.userID, r.up+r.dn); err != nil {
			log.Printf("[SNI Proxy] increment used_bytes error: %v", err)
		}
		if r.up+r.dn > 1024 {
			log.Printf("[SNI Proxy] %s traffic %s: ↑%s ↓%s", r.client, r.sni, formatBytes(r.up), formatBytes(r.dn))
		}
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read initial payload (TLS ClientHello)
	buf := make([]byte, 4096)
	n, err := clientConn.Read(buf)
	if err != nil {
		return
	}
	clientConn.SetReadDeadline(time.Time{})

	// Parse PROXY Protocol header if present (nginx stream proxy_protocol on)
	payload := buf[:n]
	clientAddr, payload := parseProxyProtocol(payload, clientConn.RemoteAddr())
	sni, err := ExtractSNI(payload)
	if err != nil {
		log.Printf("[SNI Proxy] %s SNI extract failed: %v", clientAddr, err)
		return
	}
	log.Printf("[SNI Proxy] %s SNI=%s", clientAddr, sni)

	targetIP, err := s.resolveTargetIP(sni)
	if err != nil {
		log.Printf("[SNI Proxy] %s resolve failed for %s: %v", clientAddr, sni, err)
		return
	}
	log.Printf("[SNI Proxy] %s proxying %s -> %s:443", clientAddr, sni, targetIP)

	// Connect to target
	targetConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:443", targetIP), 5*time.Second)
	if err != nil {
		log.Printf("[SNI Proxy] %s dial %s failed: %v", clientAddr, targetIP, err)
		return
	}
	defer targetConn.Close()

	// Write the already read payload
	if _, err := targetConn.Write(payload); err != nil {
		return
	}

	// Proxy with byte counting. When target→client copy ends, unblock
	// the client→target goroutine by closing the target connection.
	var up, dn int64
	done := make(chan struct{})
	go func() {
		up, _ = io.Copy(targetConn, clientConn) // client → target
		close(done)
	}()
	dn, _ = io.Copy(clientConn, targetConn) // target → client
	targetConn.Close()                       // unblock the other goroutine
	<-done

	s.totalBytesUp.Add(up)
	s.totalBytesDn.Add(dn)

	if up+dn > 0 {
		s.resolveAdmin()
		if s.adminUserID > 0 {
			s.trafficCh <- trafficRecord{
				userID: s.adminUserID,
				up:     up,
				dn:     dn,
				sni:    sni,
				client: clientAddr,
			}
		}
	}
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
}

func (s *Server) resolveTargetIP(sni string) (string, error) {
	ip, _, err := s.repo.GetBestIPForDomain(sni)
	if err == nil && ip != "" {
		return ip, nil
	}
	ips, dnsErr := net.LookupHost(sni)
	if dnsErr == nil && len(ips) > 0 {
		log.Printf("[SNI Proxy] using DNS fallback for %s -> %s", sni, ips[0])
		return ips[0], nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve %s: domain_mappings: %v, DNS: %v", sni, err, dnsErr)
	}
	return "", fmt.Errorf("no IP found for %s", sni)
}
