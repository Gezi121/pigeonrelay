package repository

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"

	"nodeforge/internal/model"
)

const ewmaAlpha = 0.3

func ewmaUpdate(old, new float64) float64 {
	return ewmaAlpha*new + (1-ewmaAlpha)*old
}

// InsertOrUpdateIPStats atomically updates rolling EWMA metrics for an IP.
// Called after latency records are inserted.
func (r *Repo) InsertOrUpdateIPStats(ip string, latencyMs int, ok bool, tlsMs int, tlsOk bool) error {
	okF := 0.0
	if ok {
		okF = 1.0
	}
	lossF := 0.0
	if !ok {
		lossF = 1.0
	}
	tlsOkF := 0.0
	if tlsOk {
		tlsOkF = 1.0
	}

	var ewmaLat, ewmaTLS, ewmaRate, ewmaTLSRate, ewmaLoss, jitter, stddev, srateVar, p50, p95, tp float64
	var samples1h, samples24h int
	err := r.db.QueryRow("SELECT ewma_latency_ms, ewma_tls_ms, ewma_success_rate, ewma_tls_ok_rate, "+
		"ewma_loss_rate, jitter_ms, latency_stddev, success_rate_variance, "+
		"p50_latency_ms, p95_latency_ms, throughput_bytes_per_sec, samples_1h, samples_24h "+
		"FROM ip_stats WHERE ip_address=?", ip).
		Scan(&ewmaLat, &ewmaTLS, &ewmaRate, &ewmaTLSRate, &ewmaLoss, &jitter, &stddev, &srateVar, &p50, &p95, &tp, &samples1h, &samples24h)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newLat := float64(latencyMs)
	oldEWMALat := ewmaLat // capture BEFORE update for delta
	ewmaLat = ewmaUpdate(ewmaLat, newLat)
	ewmaRate = ewmaUpdate(ewmaRate, okF)
	ewmaLoss = ewmaUpdate(ewmaLoss, lossF)

	// Update TLS metrics when TLS was attempted (tlsMs>0 = success with timing,
	// tlsMs<0 = attempted but failed, tlsOk=true = success without timing)
	tlsAttempted := tlsMs > 0 || tlsOk || tlsMs < 0
	if tlsAttempted {
		if tlsMs > 0 {
			ewmaTLS = ewmaUpdate(ewmaTLS, float64(tlsMs))
		}
		ewmaTLSRate = ewmaUpdate(ewmaTLSRate, tlsOkF)
	}

	delta := math.Abs(newLat - oldEWMALat)
	jitter = ewmaUpdate(jitter, delta)
	stddev = ewmaUpdate(stddev, delta*delta)
	srateVar = ewmaUpdate(srateVar, math.Abs(okF-ewmaRate))

	if p50 == 0 {
		p50 = newLat
	} else if newLat > p50 {
		p50 = ewmaUpdate(p50, newLat)
	} else {
		p50 += 0.1 * (newLat - p50)
	}
	if p95 == 0 {
		p95 = newLat
	} else if newLat > p95 {
		p95 += 0.05 * (newLat - p95)
	}

	_, err = r.db.Exec(`INSERT INTO ip_stats
		(ip_address, ewma_latency_ms, ewma_tls_ms, ewma_success_rate, ewma_tls_ok_rate,
		 ewma_loss_rate, jitter_ms, latency_stddev, success_rate_variance,
		 p50_latency_ms, p95_latency_ms, throughput_bytes_per_sec,
		 samples_1h, samples_24h, last_tested_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(ip_address) DO UPDATE SET
		 ewma_latency_ms=excluded.ewma_latency_ms,
		 ewma_tls_ms=excluded.ewma_tls_ms,
		 ewma_success_rate=excluded.ewma_success_rate,
		 ewma_tls_ok_rate=excluded.ewma_tls_ok_rate,
		 ewma_loss_rate=excluded.ewma_loss_rate,
		 jitter_ms=excluded.jitter_ms,
		 latency_stddev=excluded.latency_stddev,
		 success_rate_variance=excluded.success_rate_variance,
		 p50_latency_ms=excluded.p50_latency_ms,
		 p95_latency_ms=excluded.p95_latency_ms,
		 throughput_bytes_per_sec=excluded.throughput_bytes_per_sec,
		 samples_1h=samples_1h+1,
		 samples_24h=samples_24h+1,
		 last_tested_at=excluded.last_tested_at,
		 updated_at=excluded.updated_at`,
		ip, ewmaLat, ewmaTLS, ewmaRate, ewmaTLSRate, ewmaLoss, jitter, stddev, srateVar, p50, p95, tp,
		1, 1, time.Now(), time.Now())
	return err
}

// ewmAlphaN returns the effective multiplier for n EWMA updates of equal value.
// ewma_new = (1-a)^n * old + (1-(1-a)^n) * value
func ewmaAlphaN(n int) float64 {
	return 1.0 - math.Pow(1.0-ewmaAlpha, float64(n))
}

// BatchUpdateIPStats processes multiple samples per IP in a single upsert,
// computing the net EWMA effect from aggregated statistics.
func (r *Repo) BatchUpdateIPStats(ip string, tcpSamples, tlsSamples []int, tlsOK, tlsFail int) error {
	if len(tcpSamples) == 0 && len(tlsSamples) == 0 && tlsFail == 0 {
		return nil
	}

	var ewmaLat, ewmaTLS, ewmaRate, ewmaTLSRate, ewmaLoss, jitter, stddev, srateVar, p50, p95, tp float64
	var samples1h, samples24h int
	err := r.db.QueryRow("SELECT ewma_latency_ms, ewma_tls_ms, ewma_success_rate, ewma_tls_ok_rate, "+
		"ewma_loss_rate, jitter_ms, latency_stddev, success_rate_variance, "+
		"p50_latency_ms, p95_latency_ms, throughput_bytes_per_sec, samples_1h, samples_24h "+
		"FROM ip_stats WHERE ip_address=?", ip).
		Scan(&ewmaLat, &ewmaTLS, &ewmaRate, &ewmaTLSRate, &ewmaLoss, &jitter, &stddev, &srateVar, &p50, &p95, &tp, &samples1h, &samples24h)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	nTCP := len(tcpSamples)
	nTLS := len(tlsSamples) + tlsOK + tlsFail

	if nTCP > 0 {
		var sumLat, sumOK, sumLoss float64
		for _, lat := range tcpSamples {
			if lat > 0 {
				sumLat += float64(lat)
				sumOK++
			} else {
				sumLoss++
			}
		}
		avgLat := sumLat / float64(nTCP)
		avgOK := sumOK / float64(nTCP)
		avgLoss := sumLoss / float64(nTCP)

		alphaN := ewmaAlphaN(nTCP)
		oldEWMALat := ewmaLat
		ewmaLat = (1-alphaN)*ewmaLat + alphaN*avgLat
		ewmaRate = (1-alphaN)*ewmaRate + alphaN*avgOK
		ewmaLoss = (1-alphaN)*ewmaLoss + alphaN*avgLoss
		// Jitter: use the delta between new batch mean and old EWMA
		delta := math.Abs(avgLat - oldEWMALat)
		jitter = ewmaUpdate(jitter, delta)

		// Approximate P50/P95 with batch mean
		if p50 == 0 {
			p50 = avgLat
		} else {
			p50 += 0.1 * (avgLat - p50)
		}
		if p95 == 0 {
			p95 = avgLat
		} else if avgLat > p95 {
			p95 += 0.05 * (avgLat - p95)
		}
	}

	if nTLS > 0 {
		alphaN := ewmaAlphaN(nTLS)
		tlsOKTotal := float64(len(tlsSamples) + tlsOK)
		tlsRate := tlsOKTotal / float64(nTLS)
		ewmaTLSRate = (1-alphaN)*ewmaTLSRate + alphaN*tlsRate

		if len(tlsSamples) > 0 {
			var sumTLS float64
			for _, ms := range tlsSamples {
				sumTLS += float64(ms)
			}
			avgTLS := sumTLS / float64(len(tlsSamples))
			ewmaTLS = (1-alphaN)*ewmaTLS + alphaN*avgTLS
		}
	}

	_, err = r.db.Exec(`INSERT INTO ip_stats
		(ip_address, ewma_latency_ms, ewma_tls_ms, ewma_success_rate, ewma_tls_ok_rate,
		 ewma_loss_rate, jitter_ms, latency_stddev, success_rate_variance,
		 p50_latency_ms, p95_latency_ms, throughput_bytes_per_sec,
		 samples_1h, samples_24h, last_tested_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(ip_address) DO UPDATE SET
		 ewma_latency_ms=excluded.ewma_latency_ms,
		 ewma_tls_ms=excluded.ewma_tls_ms,
		 ewma_success_rate=excluded.ewma_success_rate,
		 ewma_tls_ok_rate=excluded.ewma_tls_ok_rate,
		 ewma_loss_rate=excluded.ewma_loss_rate,
		 jitter_ms=excluded.jitter_ms,
		 latency_stddev=excluded.latency_stddev,
		 success_rate_variance=excluded.success_rate_variance,
		 p50_latency_ms=excluded.p50_latency_ms,
		 p95_latency_ms=excluded.p95_latency_ms,
		 throughput_bytes_per_sec=excluded.throughput_bytes_per_sec,
		 samples_1h=samples_1h+?,
		 samples_24h=samples_24h+?,
		 last_tested_at=excluded.last_tested_at,
		 updated_at=excluded.updated_at`,
		ip, ewmaLat, ewmaTLS, ewmaRate, ewmaTLSRate, ewmaLoss, jitter, stddev, srateVar, p50, p95, tp,
		nTCP+nTLS, nTCP+nTLS, time.Now(), time.Now())
	return err
}

// GetIPStats returns the rolling stats for a single IP.
func (r *Repo) GetIPStats(ip string) (*model.IPStats, error) {
	s := &model.IPStats{}
	err := r.db.QueryRow(`SELECT ip_address, ewma_latency_ms, ewma_tls_ms, ewma_success_rate,
		ewma_tls_ok_rate, ewma_loss_rate, jitter_ms, latency_stddev, success_rate_variance,
		p50_latency_ms, p95_latency_ms, throughput_bytes_per_sec,
		samples_1h, samples_24h, last_tested_at, last_tls_ok_at,
		test_priority, cooldown_until, per_client_json, peak_hour_latency
		FROM ip_stats WHERE ip_address=?`, ip).
		Scan(&s.IPAddress, &s.EWMALatencyMs, &s.EWMATLSMs, &s.EWMASuccessRate,
			&s.EWMATLSOkRate, &s.EWMALossRate, &s.JitterMs, &s.LatencyStddev,
			&s.SuccessRateVariance, &s.P50LatencyMs, &s.P95LatencyMs,
			&s.ThroughputBytesPerSec, &s.Samples1h, &s.Samples24h,
			&s.LastTestedAt, &s.LastTLSOKAt,
			&s.TestPriority, &s.CooldownUntil, &s.PerClientJSON, &s.PeakHourLatency)
	return s, err
}

// UpdateIPStatsTestPriority recalculates test_priority for the given IP.
func (r *Repo) UpdateIPStatsTestPriority(ip string) error {
	_, err := r.db.Exec(`UPDATE ip_stats SET test_priority =
		1.0/(1.0+ewma_latency_ms/100.0)
		* MAX(0.1, 1.0 - (strftime('%s','now') - strftime('%s', COALESCE(last_tested_at, datetime('now','-1 hour')))) / 3600.0)
		WHERE ip_address=?`, ip)
	return err
}

// GetBestIPForDomainV2 queries ip_stats for the best IP using the given algo type.
func (r *Repo) GetBestIPForDomainV2(fullDomain string) (string, int, error) {
	var algoName string
	err := r.db.QueryRow("SELECT algorithm_type FROM domain_mappings WHERE full_domain = ?", fullDomain).Scan(&algoName)
	if err != nil {
		return "", 0, fmt.Errorf("domain not mapped: %v", err)
	}

	algoType := r.resolveAlgoType(algoName)

	// Progressive gate: only require minimum data freshness, penalty is in score
	gate := "samples_1h >= 3 AND last_tested_at >= datetime('now','-2 hours')"
	countP := "CASE WHEN samples_1h >= 10 THEN 1.0 ELSE 1.0+(10-samples_1h)*0.08 END"
	rateP := rateGateSQL // progressive rate penalty (replaces hard 60% gate)
	stab := "CASE WHEN ewma_tls_ok_rate < 0.5 THEN 100.0 ELSE 1.0 + jitter_ms/200.0 + (p95_latency_ms-p50_latency_ms)/150.0 + (1.0-ewma_tls_ok_rate)*2.0 + ewma_loss_rate*3.0 END"

	var bestIP string
	var score int
	var query string

	switch algoType {
	case "steady":
		query = "SELECT ip_address, CAST((ewma_latency_ms/NULLIF(ewma_success_rate,0) + jitter_ms/200.0)*" + rateP + "*" + countP + "*" + stab + " AS INTEGER) FROM ip_stats WHERE " + gate + " ORDER BY 2 ASC LIMIT 1"
	case "tidal":
		query = "SELECT ip_address, CAST((COALESCE(peak_hour_latency,ewma_latency_ms)*0.6 + ewma_latency_ms*0.4)/NULLIF(ewma_success_rate,0)*" + rateP + "*" + countP + "*" + stab + " AS INTEGER) FROM ip_stats WHERE " + gate + " AND samples_24h >= 10 ORDER BY 2 ASC LIMIT 1"
	case "composite":
		query = "SELECT ip_address, CAST((ewma_latency_ms/(NULLIF(ewma_success_rate,0)*NULLIF(ewma_success_rate,0)) + jitter_ms/300.0)*" + rateP + "*" + countP + "*" + stab + " AS INTEGER) FROM ip_stats WHERE " + gate + " ORDER BY 2 ASC LIMIT 1"
	case "lowjitter":
		query = "SELECT ip_address, CAST((ewma_latency_ms/NULLIF(ewma_success_rate,0) + jitter_ms*1.5)*" + rateP + "*" + countP + "*" + stab + " AS INTEGER) FROM ip_stats WHERE " + gate + " ORDER BY 2 ASC LIMIT 1"
	default: // fast
		query = "SELECT ip_address, CAST(ewma_latency_ms/MIN(ewma_success_rate,0.95)*" + rateP + "*" + countP + "*" + stab + " AS INTEGER) FROM ip_stats WHERE " + gate + " ORDER BY 2 ASC LIMIT 1"
	}

	err = r.db.QueryRow(query).Scan(&bestIP, &score)
	if err != nil || bestIP == "" {
		log.Printf("[algo-v2] %s for %s: no IP in ip_stats, fallback to legacy", algoType, fullDomain)
		return r.GetBestIPForDomain(fullDomain)
	}
	return bestIP, score, nil
}

// GetTopIPsV2 returns top 20 IPs from ip_stats.
func (r *Repo) GetTopIPsV2() ([]model.DailyPick, error) {
	rows, err := r.db.Query(`
		SELECT ip_address,
		       CAST(ewma_latency_ms AS INTEGER),
		       CAST(p50_latency_ms AS INTEGER),
		       CAST(p95_latency_ms AS INTEGER),
		       samples_1h,
		       CAST(ewma_success_rate*100 AS INTEGER),
		       CAST(ewma_success_rate*100 AS INTEGER)
		FROM ip_stats
		WHERE ewma_success_rate >= 0.6 AND samples_1h >= 3
		ORDER BY last_tested_at DESC, ewma_success_rate DESC
		LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var picks []model.DailyPick
	for rows.Next() {
		var p model.DailyPick
		if err := rows.Scan(&p.IP, &p.AvgMs, &p.MinMs, &p.MaxMs, &p.Count, &p.Rate, &p.Success); err != nil {
			return nil, err
		}
		picks = append(picks, p)
	}
	if len(picks) == 0 {
		return r.GetTopIPs()
	}
	return picks, rows.Err()
}

// GetBestIPsForDomainV2 returns top-N IPs from ip_stats.
func (r *Repo) GetBestIPsForDomainV2(fullDomain string, limit int) ([]string, error) {
	var algoName string
	err := r.db.QueryRow("SELECT algorithm_type FROM domain_mappings WHERE full_domain = ?", fullDomain).Scan(&algoName)
	if err != nil {
		return nil, err
	}
	algoType := r.resolveAlgoType(algoName)
	if algoType == "tidal" || algoType == "lowjitter" {
		algoType = "fast"
	}

	query := "SELECT ip_address FROM ip_stats WHERE ewma_success_rate >= 0.6 ORDER BY ewma_latency_ms ASC LIMIT " + fmt.Sprintf("%d", limit)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ips []string
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return r.GetBestIPsForDomain(fullDomain, limit)
	}
	return ips, nil
}

// UpdatePeakHourLatencyBatch recomputes peak_hour_latency for all recently active IPs.
func (r *Repo) UpdatePeakHourLatencyBatch() error {
	_, err := r.db.Exec(`
		UPDATE ip_stats SET peak_hour_latency = (
			SELECT AVG(latency_ms) FROM latency_records
			WHERE ip_address = ip_stats.ip_address
			  AND recorded_at >= datetime('now', '-7 days')
			  AND CAST(strftime('%H', recorded_at) AS INTEGER) BETWEEN 18 AND 23
			  AND latency_ms > 0
		)
		WHERE last_tested_at >= datetime('now', '-24 hours')
	`)
	return err
}

// RefreshSamplesCounts resets sample counters based on actual records.
func (r *Repo) RefreshSamplesCounts() error {
	_, err := r.db.Exec(`
		UPDATE ip_stats SET
			samples_1h = (SELECT COUNT(*) FROM latency_records
				WHERE ip_address=ip_stats.ip_address AND recorded_at >= datetime('now','-1 hour')),
			samples_24h = (SELECT COUNT(*) FROM latency_records
				WHERE ip_address=ip_stats.ip_address AND recorded_at >= datetime('now','-24 hours'))
	`)
	return err
}

// BackfillIPStatsFromRecords populates ip_stats from existing latency_records.
func (r *Repo) BackfillIPStatsFromRecords() (int64, error) {
	result, err := r.db.Exec(`
		INSERT OR IGNORE INTO ip_stats (ip_address, ewma_latency_ms, ewma_success_rate,
			samples_1h, samples_24h, last_tested_at, updated_at)
		SELECT ip_address,
			AVG(CASE WHEN latency_ms>0 THEN latency_ms END),
			1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*),
			COUNT(CASE WHEN recorded_at >= datetime('now','-1 hour') THEN 1 END),
			COUNT(CASE WHEN recorded_at >= datetime('now','-24 hours') THEN 1 END),
			MAX(recorded_at),
			datetime('now')
		FROM latency_records
		WHERE recorded_at >= datetime('now', '-7 days')
		GROUP BY ip_address
		HAVING COUNT(*) >= 3
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ---- Task Generation ----

// Task represents a server-directed speed-test instruction for a client.
type Task struct {
	IP       string `json:"ip"`
	Action   string `json:"action"`
	Priority int    `json:"priority"`
}

// GenerateTasks produces a list of tasks for a given client.
func (r *Repo) GenerateTasks(clientID string, limit int) []Task {
	var tasks []Task

	// 1. Stale recheck: IPs with good history but not tested recently
	staleIPs, _ := r.GetStaleHighPriorityIPs(limit / 4)
	for _, ip := range staleIPs {
		tasks = append(tasks, Task{IP: ip, Action: "test_ip", Priority: 9})
	}

	// 2. TLS supplement: IPs that are algorithm candidates but lack TLS data
	tlsGapIPs, _ := r.GetCandidateIPsWithoutTLS(limit / 4)
	for _, ip := range tlsGapIPs {
		tasks = append(tasks, Task{IP: ip, Action: "test_ip", Priority: 7})
	}

	// 3. Anomaly recheck: IPs with sudden success_rate drops
	anomalyIPs, _ := r.GetAnomalyIPs(limit / 4)
	for _, ip := range anomalyIPs {
		tasks = append(tasks, Task{IP: ip, Action: "test_ip", Priority: 5})
	}

	// 4. Stale bulk recheck
	tasks = append(tasks, Task{Action: "recheck_stale", Priority: 3})

	// Update priorities for all assigned IPs
	for _, t := range tasks {
		if t.IP != "" {
			r.UpdateIPStatsTestPriority(t.IP)
		}
	}

	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks
}

// GetStaleHighPriorityIPs returns IPs with good EWMA latency that haven't been tested recently.
func (r *Repo) GetStaleHighPriorityIPs(limit int) ([]string, error) {
	rows, err := r.db.Query(`SELECT ip_address FROM ip_stats
		WHERE ewma_success_rate >= 0.6 AND ewma_latency_ms > 0
		  AND (last_tested_at IS NULL OR last_tested_at < datetime('now','-30 minutes'))
		ORDER BY ewma_latency_ms ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ips []string
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

// GetCandidateIPsWithoutTLS returns algorithm candidate IPs that lack TLS verification.
func (r *Repo) GetCandidateIPsWithoutTLS(limit int) ([]string, error) {
	rows, err := r.db.Query(`SELECT ip_address FROM ip_stats
		WHERE ewma_success_rate >= 0.6 AND ewma_latency_ms < 300
		  AND ewma_tls_ok_rate = 0 AND samples_1h >= 3
		ORDER BY ewma_latency_ms ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ips []string
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

// ---- Client Quality Weighting ----

// UpdateClientBias calculates and stores per-client latency bias vs global average.
// Called periodically (e.g., daily) to adjust for client network quality differences.
func (r *Repo) UpdateClientBias(clientID string) error {
	var clientAvg, globalAvg float64
	r.db.QueryRow(`SELECT AVG(1.0*latency_ms) FROM latency_records
		WHERE client_id=? AND latency_ms>0 AND recorded_at>=datetime('now','-24 hours')`,
		clientID).Scan(&clientAvg)
	r.db.QueryRow(`SELECT AVG(1.0*latency_ms) FROM latency_records
		WHERE latency_ms>0 AND recorded_at>=datetime('now','-24 hours')`).Scan(&globalAvg)
	if globalAvg <= 0 || clientAvg <= 0 {
		return nil
	}
	bias := clientAvg / globalAvg
	_, err := r.db.Exec(`INSERT INTO speed_test_clients (mac_address, name, notes, last_seen) VALUES (?, '', ?, datetime('now'))
		ON CONFLICT(mac_address) DO UPDATE SET notes=excluded.notes, last_seen=datetime('now')`,
		clientID, fmt.Sprintf("bias=%.3f", bias))
	return err
}

func (r *Repo) GetClientWeight(clientID string) float64 {
	var notes string
	err := r.db.QueryRow(`SELECT COALESCE(notes,'') FROM speed_test_clients WHERE mac_address=?`, clientID).Scan(&notes)
	if err != nil || notes == "" {
		return 1.0
	}
	var bias float64
	if n, _ := fmt.Sscanf(notes, "bias=%f", &bias); n == 1 && bias > 0 {
		if bias <= 1.2 {
			return 1.0
		}
		if bias >= 2.0 {
			return 0.3
		}
		return 1.0 - (bias-1.2)*0.875
	}
	return 1.0
}

// ---- Progressive Rate Penalty ----

// ratePenalty returns a smooth penalty multiplier based on EWMA success rate.
// Replaces the hard 60% gate: 0.95+ = 1.0x, 0.6 ≈ 2.5x, 0.4 ≈ 100x (eliminated).
func ratePenalty(rate float64) float64 {
	if rate >= 0.95 {
		return 1.0
	}
	if rate <= 0.4 {
		return 100.0
	}
	return 1.0 + math.Pow((0.95-rate)*10, 2)
}

// rateGate returns the SQL expression for progressive rate penalty.
const rateGateSQL = `CASE WHEN ewma_success_rate >= 0.95 THEN 1.0
	WHEN ewma_success_rate <= 0.4 THEN 100.0
	ELSE 1.0 + ((0.95-ewma_success_rate)*10)*((0.95-ewma_success_rate)*10) END`

// ---- Client-Adjusted Latency ----

// ClientAdjustedLatency returns EWMA latency adjusted for a specific client's bias.
// For a client with high bias (consistently measures higher latency than global average),
// the adjusted value compensates to estimate what this client would actually experience.
func (r *Repo) ClientAdjustedLatency(ip, clientID string) float64 {
	stats, err := r.GetIPStats(ip)
	if err != nil || stats == nil {
		return 0
	}
	weight := r.GetClientWeight(clientID)
	// Lower weight = poor-quality client. Use reciprocal to inflate the EWMA
	// so the algorithm doesn't over-select this IP for poor clients.
	if weight < 1.0 {
		return stats.EWMALatencyMs / weight
	}
	return stats.EWMALatencyMs
}

// GetAnomalyIPs returns IPs whose success_rate dropped recently.
func (r *Repo) GetAnomalyIPs(limit int) ([]string, error) {
	rows, err := r.db.Query(`SELECT ip_address FROM ip_stats
		WHERE ewma_success_rate > 0.3
		  AND success_rate_variance > 0.2
		  AND samples_1h >= 5
		ORDER BY success_rate_variance DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ips []string
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}


// GetBestIPDetailClient returns client-adjusted best IP metrics.
func (r *Repo) GetBestIPDetailClient(fullDomain, clientID string) (*model.BestIPDetail, error) {
	ip, score, err := r.GetBestIPForDomainV2(fullDomain)
	if err != nil {
		return nil, err
	}

	// Apply client-specific latency adjustment if available
	adjustedLat := r.ClientAdjustedLatency(ip, clientID)
	if adjustedLat > 0 {
		score = int(adjustedLat)
	}

	d := &model.BestIPDetail{IP: ip, Score: score}
	var algoName string
	r.db.QueryRow("SELECT algorithm_type FROM domain_mappings WHERE full_domain=?", fullDomain).Scan(&algoName)
	d.AlgoType = r.resolveAlgoType(algoName)

	// Query ip_stats for metrics
	row := r.db.QueryRow(`SELECT CAST(ewma_latency_ms AS INTEGER),
		CAST(ewma_success_rate*100 AS INTEGER),
		CAST(ewma_tls_ok_rate*100 AS INTEGER),
		CAST(latency_stddev/100 AS INTEGER),
		CAST(jitter_ms AS INTEGER),
		samples_1h FROM ip_stats WHERE ip_address=?`, ip)
	row.Scan(&d.AvgMs, &d.SuccessRt, &d.StableRt, &d.Stddev, &d.Unstable, &d.Total)
	return d, nil
}
