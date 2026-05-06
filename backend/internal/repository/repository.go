package repository

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"nodeforge/internal/model"
)

type Repo struct{ db *sql.DB }

func New(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) CreateUser(username, passwordHash, role string, expireAt time.Time, quota int64) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`INSERT INTO users (username, password_hash, role, expire_at, monthly_quota_bytes)
		 VALUES (?,?,?,?,?) RETURNING id, username, password_hash, role, expire_at, monthly_quota_bytes, used_bytes, created_at, updated_at`,
		username, passwordHash, role, expireAt, quota,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ExpireAt, &u.MonthlyQuotaBytes, &u.UsedBytes, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *Repo) GetUserByUsername(username string) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, role, expire_at, monthly_quota_bytes, used_bytes, created_at, updated_at
		 FROM users WHERE username=?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ExpireAt, &u.MonthlyQuotaBytes, &u.UsedBytes, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repo) ListUsers() ([]model.User, error) {
	rows, err := r.db.Query(
		`SELECT id, username, password_hash, role, expire_at, monthly_quota_bytes, used_bytes, created_at, updated_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ExpireAt, &u.MonthlyQuotaBytes, &u.UsedBytes, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- subscription_sources ---

func (r *Repo) CreateSource(userID int64, sourceType, url, name, localNodes string) (*model.SubscriptionSource, error) {
	s := &model.SubscriptionSource{}
	err := r.db.QueryRow(
		`INSERT INTO subscription_sources (user_id, type, url, name, local_nodes) VALUES (?,?,?,?,?)
		 RETURNING id, user_id, type, url, name, last_fetch_status, last_fetch_at, raw_data, status, last_tested_at, raw_nodes_cache, local_nodes`,
		userID, sourceType, url, name, localNodes,
	).Scan(&s.ID, &s.UserID, &s.Type, &s.URL, &s.Name, &s.LastFetchStatus, &s.LastFetchAt, &s.RawData, &s.Status, &s.LastTestedAt, &s.RawNodesCache, &s.LocalNodes)
	return s, err
}

func (r *Repo) GetSourceByID(id, userID int64) (*model.SubscriptionSource, error) {
	s := &model.SubscriptionSource{}
	err := r.db.QueryRow(
		`SELECT id, user_id, type, url, name, last_fetch_status, last_fetch_at, raw_data, status, last_tested_at, raw_nodes_cache, local_nodes
		 FROM subscription_sources WHERE id=? AND user_id=?`, id, userID,
	).Scan(&s.ID, &s.UserID, &s.Type, &s.URL, &s.Name, &s.LastFetchStatus, &s.LastFetchAt, &s.RawData, &s.Status, &s.LastTestedAt, &s.RawNodesCache, &s.LocalNodes)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repo) ListSources(userID int64) ([]model.SubscriptionSource, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, type, url, name, last_fetch_status, last_fetch_at, raw_data, status, last_tested_at, raw_nodes_cache, local_nodes
		 FROM subscription_sources WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []model.SubscriptionSource
	for rows.Next() {
		var s model.SubscriptionSource
		if err := rows.Scan(&s.ID, &s.UserID, &s.Type, &s.URL, &s.Name, &s.LastFetchStatus, &s.LastFetchAt, &s.RawData, &s.Status, &s.LastTestedAt, &s.RawNodesCache, &s.LocalNodes); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func (r *Repo) UpdateSource(id, userID int64, sourceType, url, name, localNodes *string) (*model.SubscriptionSource, error) {
	s, err := r.GetSourceByID(id, userID)
	if err != nil {
		return nil, err
	}
	if sourceType != nil {
		s.Type = *sourceType
	}
	if url != nil {
		s.URL = *url
	}
	if name != nil {
		s.Name = *name
	}
	if localNodes != nil {
		s.LocalNodes = *localNodes
	}
	_, err = r.db.Exec(
		`UPDATE subscription_sources SET type=?, url=?, name=?, local_nodes=? WHERE id=? AND user_id=?`,
		s.Type, s.URL, s.Name, s.LocalNodes, id, userID,
	)
	return s, err
}

func (r *Repo) DeleteSource(id, userID int64) error {
	_, err := r.db.Exec(`DELETE FROM subscription_sources WHERE id=? AND user_id=?`, id, userID)
	return err
}

// --- node_groups ---

func (r *Repo) GetGroupByID(id, userID int64) (*model.NodeGroup, error) {
	g := &model.NodeGroup{}
	err := r.db.QueryRow(
		`SELECT id, user_id, name, name_prefix, group_type, source_type, source_id, unified_config, rule_config, advanced_prefer
		 FROM node_groups WHERE id=? AND user_id=?`, id, userID,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.NamePrefix, &g.GroupType, &g.SourceType, &g.SourceID, &g.UnifiedConfig, &g.RuleConfig, &g.AdvancedPrefer)
	if err != nil {
		return nil, err
	}

	// Fetch replacements
	rowsRepl, err := r.db.Query(`SELECT id, group_id, target_field, replace_value FROM group_replacements WHERE group_id=?`, id)
	if err == nil {
		defer rowsRepl.Close()
		for rowsRepl.Next() {
			var repl model.GroupReplacement
			if err := rowsRepl.Scan(&repl.ID, &repl.GroupID, &repl.TargetField, &repl.ReplaceValue); err == nil {
				g.Replacements = append(g.Replacements, repl)
			}
		}
	}

	// Fetch routings
	rowsRtg, err := r.db.Query(`SELECT id, group_id, routing_address FROM group_routings WHERE group_id=?`, id)
	if err == nil {
		defer rowsRtg.Close()
		for rowsRtg.Next() {
			var rtg model.GroupRouting
			if err := rowsRtg.Scan(&rtg.ID, &rtg.GroupID, &rtg.RoutingAddress); err == nil {
				g.Routings = append(g.Routings, rtg)
			}
		}
	}

	return g, nil
}

func (r *Repo) CreateGroup(userID int64, name, sourceType string, sourceID *int64) (*model.NodeGroup, error) {
	g := &model.NodeGroup{}
	err := r.db.QueryRow(
		`INSERT INTO node_groups (user_id, name, source_type, source_id) VALUES (?,?,?,?)
		 RETURNING id, user_id, name, name_prefix, group_type, source_type, source_id, unified_config, rule_config, advanced_prefer`,
		userID, name, sourceType, sourceID,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.NamePrefix, &g.GroupType, &g.SourceType, &g.SourceID, &g.UnifiedConfig, &g.RuleConfig, &g.AdvancedPrefer)
	return g, err
}

func (r *Repo) UpdateGroupConfig(id, userID int64, namePrefix, groupType string, replacements []model.GroupReplacement, routings []model.GroupRouting) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE node_groups SET name_prefix=?, group_type=? WHERE id=? AND user_id=?`,
		namePrefix, groupType, id, userID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM group_replacements WHERE group_id=?`, id)
	if err != nil {
		return err
	}
	for _, repl := range replacements {
		_, err = tx.Exec(
			`INSERT INTO group_replacements (group_id, target_field, replace_value) VALUES (?,?,?)`,
			id, repl.TargetField, repl.ReplaceValue,
		)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`DELETE FROM group_routings WHERE group_id=?`, id)
	if err != nil {
		return err
	}
	for _, rtg := range routings {
		_, err = tx.Exec(
			`INSERT INTO group_routings (group_id, routing_address) VALUES (?,?)`,
			id, rtg.RoutingAddress,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repo) ListGroups(userID int64) ([]model.NodeGroup, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, name, name_prefix, group_type, source_type, source_id, unified_config, rule_config, advanced_prefer
		 FROM node_groups WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []model.NodeGroup
	for rows.Next() {
		var g model.NodeGroup
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.NamePrefix, &g.GroupType, &g.SourceType, &g.SourceID, &g.UnifiedConfig, &g.RuleConfig, &g.AdvancedPrefer); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	// Note: We skip N+1 queries for list here to save performance,
	// typically the detail UI fetches GetGroupByID.
	return groups, rows.Err()
}

func (r *Repo) UpdateSourceFetchStatus(id int64, status, rawData string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE subscription_sources SET last_fetch_status=?, last_fetch_at=?, raw_data=? WHERE id=?`,
		status, now, rawData, id,
	)
	return err
}

func (r *Repo) UpdateSourceStatus(id int64, status, rawNodesCache string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE subscription_sources SET status=?, last_tested_at=?, raw_nodes_cache=? WHERE id=?`,
		status, now, rawNodesCache, id,
	)
	return err
}

func (r *Repo) CountUsers() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *Repo) GetUserByID(id int64) (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, role, expire_at, monthly_quota_bytes, used_bytes, created_at, updated_at
		 FROM users WHERE id=?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ExpireAt, &u.MonthlyQuotaBytes, &u.UsedBytes, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repo) CreateTrafficLog(userID int64, nodeID *int64, bytesUp, bytesDown int64) (*model.TrafficLog, error) {
	t := &model.TrafficLog{}
	err := r.db.QueryRow(
		`INSERT INTO traffic_logs (user_id, node_id, bytes_up, bytes_down) VALUES (?,?,?,?)
		 RETURNING id, user_id, node_id, bytes_up, bytes_down, timestamp`,
		userID, nodeID, bytesUp, bytesDown,
	).Scan(&t.ID, &t.UserID, &t.NodeID, &t.BytesUp, &t.BytesDown, &t.Timestamp)
	return t, err
}

func (r *Repo) IncrementUserUsedBytes(userID int64, delta int64) error {
	_, err := r.db.Exec(`UPDATE users SET used_bytes = used_bytes + ? WHERE id = ?`, delta, userID)
	return err
}

func (r *Repo) GetTrafficByUserDate(userID int64, date string) ([]model.TrafficLog, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, node_id, bytes_up, bytes_down, timestamp
		 FROM traffic_logs WHERE user_id=? AND date(timestamp)=? ORDER BY timestamp DESC LIMIT 100`,
		userID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.TrafficLog
	for rows.Next() {
		var t model.TrafficLog
		if err := rows.Scan(&t.ID, &t.UserID, &t.NodeID, &t.BytesUp, &t.BytesDown, &t.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, t)
	}
	return logs, rows.Err()
}

func (r *Repo) AggregateTraffic() error {
	_, err := r.db.Exec(
		`UPDATE users SET used_bytes = (
			SELECT COALESCE(SUM(bytes_up + bytes_down), 0)
			FROM traffic_logs
			WHERE traffic_logs.user_id = users.id
			AND strftime('%Y-%m', traffic_logs.timestamp) = strftime('%Y-%m', 'now')
		)`)
	return err
}

func (r *Repo) ResetMonthlyTraffic() error {
	_, err := r.db.Exec(
		`UPDATE users SET used_bytes = 0
		 WHERE strftime('%d', 'now') = '01'`)
	return err
}

// --- latency_records ---

func (r *Repo) InsertLatencyRecords(ip, clientID string, records []model.LatencyRecordInput) error {
	return r.InsertLatencyRecordsBatch(clientID, []struct {
		IP      string
		Records []model.LatencyRecordInput
	}{{IP: ip, Records: records}})
}

func (r *Repo) InsertLatencyRecordsBatch(clientID string, entries []struct {
	IP      string
	Records []model.LatencyRecordInput
}) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO latency_records (ip_address, client_id, latency_ms, recorded_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, entry := range entries {
		for _, rec := range entry.Records {
			t, err := time.Parse(time.RFC3339Nano, rec.Time)
			if err != nil {
				t = now
			}
			if _, err := stmt.Exec(entry.IP, clientID, rec.Latency, t); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *Repo) GetLatencyRecords(hours int) ([]model.LatencyRecord, error) {
	rows, err := r.db.Query(
		`SELECT id, ip_address, latency_ms, recorded_at
		 FROM latency_records
		 WHERE recorded_at >= datetime('now', ?)
		 ORDER BY recorded_at ASC`, fmt.Sprintf("-%d hours", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.LatencyRecord
	for rows.Next() {
		var rec model.LatencyRecord
		if err := rows.Scan(&rec.ID, &rec.IPAddress, &rec.LatencyMs, &rec.RecordedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *Repo) GetDailyPicks() ([]model.DailyPick, error) {
	rows, err := r.db.Query(`
		SELECT ip_address,
		       CAST(AVG(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER),
		       CAST(MIN(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER),
		       CAST(MAX(latency_ms) AS INTEGER),
		       COUNT(*),
		       COUNT(CASE WHEN latency_ms>0 THEN 1 END),
		       CAST(COUNT(CASE WHEN latency_ms>0 THEN 1 END)*100/COUNT(*) AS INTEGER),
		       COUNT(CASE WHEN latency_ms>500 THEN 1 END),
		       CAST(COUNT(CASE WHEN latency_ms>0 AND latency_ms<=500 THEN 1 END)*100/
		            NULLIF(COUNT(CASE WHEN latency_ms>0 THEN 1 END),0) AS INTEGER)
		FROM latency_records
		WHERE recorded_at >= datetime('now', '-24 hours')
		GROUP BY ip_address
		HAVING COUNT(CASE WHEN latency_ms>500 THEN 1 END)*100/COUNT(*) < 50
		   AND COUNT(*) >= 6
		   AND MAX(recorded_at) >= datetime('now', '-4 hours')
		ORDER BY AVG(CASE WHEN latency_ms>0 THEN latency_ms END) ASC
		LIMIT 60
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []model.DailyPick
	for rows.Next() {
		var p model.DailyPick
		if err := rows.Scan(&p.IP, &p.AvgMs, &p.MinMs, &p.MaxMs, &p.Count, &p.Success, &p.Rate, &p.Unstable, &p.StableRt); err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	if len(all) == 0 {
		return []model.DailyPick{}, nil
	}

	// Find max sample count for confidence weighting
	maxCount := 0
	for _, p := range all {
		if p.Count > maxCount {
			maxCount = p.Count
		}
	}

	// Adjust each IP's score: penalize low sample counts
	// adjusted = avg * (1 + 0.4 * (1 - count/maxCount))
	// An IP with 10/50 samples gets 40% penalty; 50/50 gets 0%
	type scored struct {
		p     model.DailyPick
		score float64
	}
	var ranked []scored
	for _, p := range all {
		ratio := float64(p.Count) / float64(maxCount) // 1.0 = fully sampled
		penalty := 1.0 + 0.4*(1.0-ratio)
		ranked = append(ranked, scored{p: p, score: float64(p.AvgMs) * penalty})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score < ranked[j].score })

	var picks []model.DailyPick
	for i := 0; i < len(ranked) && i < 20; i++ {
		picks = append(picks, ranked[i].p)
	}
	return picks, rows.Err()
}

func (r *Repo) GetBestIPDetail(fullDomain string) (*model.BestIPDetail, error) {
	ip, score, err := r.GetBestIPForDomain(fullDomain)
	if err != nil {
		return nil, err
	}
	d := &model.BestIPDetail{IP: ip, Score: score}
	// Get the algo type for this domain
	var algoName string
	r.db.QueryRow(`SELECT algorithm_type FROM domain_mappings WHERE full_domain=?`, fullDomain).Scan(&algoName)
	d.AlgoType = r.resolveAlgoType(algoName)

	// Query metrics for this IP
	row := r.db.QueryRow(`
		SELECT CAST(AVG(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER),
		       CAST(COUNT(CASE WHEN latency_ms>0 THEN 1 END)*100/NULLIF(COUNT(*),0) AS INTEGER),
		       CAST(COUNT(CASE WHEN latency_ms>0 AND latency_ms<=500 THEN 1 END)*100/NULLIF(COUNT(CASE WHEN latency_ms>0 THEN 1 END),0) AS INTEGER),
		       CAST(MAX(0, AVG(CASE WHEN latency_ms>0 THEN latency_ms*latency_ms END)-AVG(CASE WHEN latency_ms>0 THEN latency_ms END)*AVG(CASE WHEN latency_ms>0 THEN latency_ms END))/100 AS INTEGER),
		       COUNT(CASE WHEN latency_ms>500 THEN 1 END),
		       COUNT(*)
		FROM latency_records
		WHERE ip_address=? AND recorded_at >= datetime('now','-3 hours')
	`, ip)
	if err := row.Scan(&d.AvgMs, &d.SuccessRt, &d.StableRt, &d.Stddev, &d.Unstable, &d.Total); err != nil {
		log.Printf("[best-detail] scan %s: %v", ip, err)
	}
	return d, nil
}

func (r *Repo) GetTopIPs() ([]model.DailyPick, error) {
	rows, err := r.db.Query(`
		SELECT ip_address,
		       CAST(AVG(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER),
		       CAST(MIN(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER),
		       CAST(MAX(latency_ms) AS INTEGER),
		       COUNT(*),
		       COUNT(CASE WHEN latency_ms>0 THEN 1 END),
		       CAST(COUNT(CASE WHEN latency_ms>0 THEN 1 END)*100/COUNT(*) AS INTEGER)
		FROM latency_records
		WHERE recorded_at >= datetime('now', '-3 hours')
		GROUP BY ip_address
		HAVING 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) >= 0.6
		   AND COUNT(*) >= 5
		ORDER BY MAX(recorded_at) DESC, 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) DESC
		LIMIT 20
		`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var picks []model.DailyPick
	for rows.Next() {
		var p model.DailyPick
		if err := rows.Scan(&p.IP, &p.AvgMs, &p.MinMs, &p.MaxMs, &p.Count, &p.Success, &p.Rate); err != nil {
			return nil, err
		}
		picks = append(picks, p)
	}
	if picks == nil {
		picks = []model.DailyPick{}
	}
	return picks, rows.Err()
}

func (r *Repo) CleanupLatencyRecords(days int) (int64, error) {
	res, err := r.db.Exec(
		`DELETE FROM latency_records WHERE recorded_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *Repo) GetLatencyByIP(ip string, hours int) ([]model.LatencyRecord, error) {
	return r.GetLatencyByIPBucketed(ip, hours, 0)
}

// GetLatencyByIPBucketed returns per-IP latency, optionally aggregated into time buckets.
// bucketMinutes=0 returns raw records; >0 returns one avg per bucket.
func (r *Repo) GetLatencyByIPBucketed(ip string, hours, bucketMinutes int) ([]model.LatencyRecord, error) {
	var rows *sql.Rows
	var err error

	if bucketMinutes > 0 {
		rows, err = r.db.Query(
			`SELECT 0, ip_address, CAST(AVG(CASE WHEN latency_ms>0 THEN latency_ms END) AS INTEGER), MAX(recorded_at)
			 FROM latency_records
			 WHERE ip_address=? AND recorded_at >= datetime('now', ?)
			 GROUP BY strftime('%%Y-%%m-%%dT%%H:', recorded_at) || printf('%02d', (CAST(strftime('%%M', recorded_at) AS INTEGER)/?)*?)
			 ORDER BY recorded_at ASC`,
			ip, fmt.Sprintf("-%d hours", hours), bucketMinutes, bucketMinutes)
	} else {
		rows, err = r.db.Query(
			`SELECT id, ip_address, latency_ms, recorded_at
			 FROM latency_records
			 WHERE ip_address=? AND recorded_at >= datetime('now', ?)
			 ORDER BY recorded_at ASC`, ip, fmt.Sprintf("-%d hours", hours))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []model.LatencyRecord
	for rows.Next() {
		var rec model.LatencyRecord
		if err := rows.Scan(&rec.ID, &rec.IPAddress, &rec.LatencyMs, &rec.RecordedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if records == nil {
		records = []model.LatencyRecord{}
	}
	return records, rows.Err()
}

// --- config_backups ---

func (r *Repo) CreateBackup(userID int64, backupType, content string) (*model.ConfigBackup, error) {
	b := &model.ConfigBackup{}
	err := r.db.QueryRow(
		`INSERT INTO config_backups (user_id, backup_type, config_content) VALUES (?,?,?)
		 RETURNING id, user_id, backup_type, config_content, created_at`,
		userID, backupType, content,
	).Scan(&b.ID, &b.UserID, &b.BackupType, &b.ConfigContent, &b.CreatedAt)
	return b, err
}

func (r *Repo) GetBackupContent(id, userID int64) (string, error) {
	var content string
	err := r.db.QueryRow(
		`SELECT config_content FROM config_backups WHERE id=? AND user_id=?`, id, userID,
	).Scan(&content)
	return content, err
}

func (r *Repo) ListBackups(userID int64, limit int) ([]model.ConfigBackup, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, backup_type, config_content, created_at
		 FROM config_backups WHERE user_id=? ORDER BY id DESC LIMIT ?`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []model.ConfigBackup
	for rows.Next() {
		var b model.ConfigBackup
		if err := rows.Scan(&b.ID, &b.UserID, &b.BackupType, &b.ConfigContent, &b.CreatedAt); err != nil {
			return nil, err
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

func (r *Repo) DeleteOldBackups(userID int64, keep int) error {
	_, err := r.db.Exec(
		`DELETE FROM config_backups WHERE user_id=? AND id NOT IN (
			SELECT id FROM config_backups WHERE user_id=? ORDER BY id DESC LIMIT ?
		)`, userID, userID, keep,
	)
	return err
}

// --- subscription links ---

func (r *Repo) CreateSubLink(link *model.SubscriptionLink) error {
	_, err := r.db.Exec(
		`INSERT INTO subscription_links (hash, name, password_hash, max_fetch_per_day, enabled, traffic_limit_gb, expire_at, remarks, allowed_group_ids)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		link.Hash, link.Name, link.PasswordHash, link.MaxFetchPerDay, link.Enabled, link.TrafficLimitGB, link.ExpireAt, link.Remarks, link.AllowedGroupIDs,
	)
	return err
}

func (r *Repo) GetSubLink(hash string) (*model.SubscriptionLink, error) {
	l := &model.SubscriptionLink{}
	err := r.db.QueryRow(
		`SELECT id, hash, COALESCE(name,''), password_hash, max_fetch_per_day, enabled, traffic_limit_gb, expire_at, remarks, allowed_group_ids
		 FROM subscription_links WHERE hash=?`, hash,
	).Scan(&l.ID, &l.Hash, &l.Name, &l.PasswordHash, &l.MaxFetchPerDay, &l.Enabled, &l.TrafficLimitGB, &l.ExpireAt, &l.Remarks, &l.AllowedGroupIDs)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *Repo) ListSubLinks() ([]model.SubscriptionLink, error) {
	rows, err := r.db.Query(
		`SELECT id, hash, COALESCE(name,''), password_hash, max_fetch_per_day, enabled, traffic_limit_gb, expire_at, remarks, allowed_group_ids
		 FROM subscription_links ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.SubscriptionLink
	for rows.Next() {
		var l model.SubscriptionLink
		if err := rows.Scan(&l.ID, &l.Hash, &l.Name, &l.PasswordHash, &l.MaxFetchPerDay, &l.Enabled, &l.TrafficLimitGB, &l.ExpireAt, &l.Remarks, &l.AllowedGroupIDs); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// --- settings ---

func (r *Repo) GetSetting(key string) (string, error) {
	var v string
	err := r.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	return v, err
}

func (r *Repo) SetSetting(key, value string) error {
	_, err := r.db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?,?)`, key, value)
	return err
}

// --- speed_test_clients ---

func (r *Repo) ListSpeedClients() ([]model.SpeedTestClient, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(lr.client_id,lr.mac_address,''), COALESCE(sc.name,''), COALESCE(sc.notes,''), sc.last_seen, sc.created_at,
			(SELECT COUNT(*) FROM latency_records WHERE COALESCE(client_id,mac_address)=COALESCE(lr.client_id,lr.mac_address) AND date(recorded_at)=date('now'))
		FROM (SELECT DISTINCT COALESCE(client_id,mac_address) as client_id, mac_address FROM latency_records WHERE date(recorded_at)=date('now') AND COALESCE(client_id,mac_address)!='') lr
		LEFT JOIN speed_test_clients sc ON sc.mac_address = COALESCE(lr.client_id,lr.mac_address)
		ORDER BY lr.client_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.SpeedTestClient
	for rows.Next() {
		var c model.SpeedTestClient
		var lastSeen sql.NullTime
		var createdAt sql.NullTime
		if err := rows.Scan(&c.ClientID, &c.Name, &c.Notes, &lastSeen, &createdAt, &c.TodayCount); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			c.LastSeen = &lastSeen.Time
		}
		if createdAt.Valid {
			c.CreatedAt = createdAt.Time
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *Repo) DeleteClientRecords(clientID string) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM latency_records WHERE COALESCE(client_id,mac_address)=?`, clientID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *Repo) UpsertSpeedClient(clientID, name, notes string) error {
	_, err := r.db.Exec(
		`INSERT INTO speed_test_clients (mac_address, name, notes) VALUES (?,?,?)
		 ON CONFLICT(mac_address) DO UPDATE SET name=excluded.name, notes=excluded.notes`,
		clientID, name, notes)
	return err
}

// --- custom_algorithms ---

func (r *Repo) CreateAlgorithm(name, algoType, clientFilter string) (*model.CustomAlgorithm, error) {
	a := &model.CustomAlgorithm{}
	err := r.db.QueryRow(
		`INSERT INTO custom_algorithms (name, algo_type, client_filter) VALUES (?,?,?)
		 RETURNING id, name, algo_type, client_filter`,
		name, algoType, clientFilter,
	).Scan(&a.ID, &a.Name, &a.AlgoType, &a.ClientFilter)
	return a, err
}

func (r *Repo) ListAlgorithms() ([]model.CustomAlgorithm, error) {
	rows, err := r.db.Query(
		`SELECT id, name, algo_type, client_filter FROM custom_algorithms ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.CustomAlgorithm
	for rows.Next() {
		var a model.CustomAlgorithm
		if err := rows.Scan(&a.ID, &a.Name, &a.AlgoType, &a.ClientFilter); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *Repo) UpdateAlgorithm(id int64, name, algoType, clientFilter *string) error {
	if name != nil {
		r.db.Exec(`UPDATE custom_algorithms SET name=? WHERE id=?`, *name, id)
	}
	if algoType != nil {
		r.db.Exec(`UPDATE custom_algorithms SET algo_type=? WHERE id=?`, *algoType, id)
	}
	if clientFilter != nil {
		r.db.Exec(`UPDATE custom_algorithms SET client_filter=? WHERE id=?`, *clientFilter, id)
	}
	return nil
}

func (r *Repo) DeleteAlgorithm(id int64) error {
	_, err := r.db.Exec(`DELETE FROM custom_algorithms WHERE id=?`, id)
	return err
}

func (r *Repo) GetAlgorithmClientFilter(algoName string) string {
	// Built-in types use DB settings not custom_algorithms table
	switch algoName {
	case "fast", "steady", "tidal", "composite", "lowjitter":
		return ""
	}
	var filter string
	r.db.QueryRow(`SELECT client_filter FROM custom_algorithms WHERE name=?`, algoName).Scan(&filter)
	return filter
}

func (r *Repo) GetAdminUser() (*model.User, error) {
	u := &model.User{}
	err := r.db.QueryRow(
		`SELECT id, username, password_hash, role, expire_at, monthly_quota_bytes, used_bytes, created_at, updated_at
		 FROM users WHERE role='admin' LIMIT 1`,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.ExpireAt, &u.MonthlyQuotaBytes, &u.UsedBytes, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repo) UpdateAdminAccount(username, passwordHash string) error {
	if passwordHash != "" {
		_, err := r.db.Exec(`UPDATE users SET username=?, password_hash=? WHERE role='admin'`, username, passwordHash)
		return err
	}
	_, err := r.db.Exec(`UPDATE users SET username=? WHERE role='admin'`, username)
	return err
}

// --- sub links ---

func (r *Repo) UpdateSubLink(hash string, name, remarks *string, enabled *bool, trafficLimitGB *int, expireAt *string) error {
	if name != nil {
		r.db.Exec(`UPDATE subscription_links SET name=? WHERE hash=?`, *name, hash)
	}
	if remarks != nil {
		r.db.Exec(`UPDATE subscription_links SET remarks=? WHERE hash=?`, *remarks, hash)
	}
	if enabled != nil {
		r.db.Exec(`UPDATE subscription_links SET enabled=? WHERE hash=?`, *enabled, hash)
	}
	if trafficLimitGB != nil {
		r.db.Exec(`UPDATE subscription_links SET traffic_limit_gb=? WHERE hash=?`, *trafficLimitGB, hash)
	}
	if expireAt != nil {
		r.db.Exec(`UPDATE subscription_links SET expire_at=? WHERE hash=?`, *expireAt, hash)
	}
	return nil
}

func (r *Repo) ListAllEnabledNodes() ([]model.ProxyNode, error) {
	rows, err := r.db.Query(
		`SELECT id,user_id,name,protocol,address,port,raw_url,sni,host,path,security,alpn,fingerprint,transport,encryption,name_prefix,local_opt_domain,enabled,status,last_tested_at,sort_order,created_at
		 FROM proxy_nodes WHERE enabled=1 ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []model.ProxyNode
	for rows.Next() {
		var n model.ProxyNode
		if err := rows.Scan(&n.ID, &n.UserID, &n.Name, &n.Protocol, &n.Address, &n.Port, &n.RawURL, &n.SNI, &n.Host, &n.Path, &n.Security, &n.ALPN, &n.Fingerprint, &n.Transport, &n.Encryption, &n.NamePrefix, &n.LocalOptDomain, &n.Enabled, &n.Status, &n.LastTestedAt, &n.SortOrder, &n.CreatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *Repo) DeleteSubLink(hash string) error {
	_, err := r.db.Exec(`DELETE FROM subscription_links WHERE hash=?`, hash)
	return err
}

// --- admin ---

func (r *Repo) UpdateUserQuota(id int64, quota int64) error {
	_, err := r.db.Exec(`UPDATE users SET monthly_quota_bytes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, quota, id)
	return err
}

func (r *Repo) UpdateUserExpire(id int64, expireAt time.Time) error {
	_, err := r.db.Exec(`UPDATE users SET expire_at=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, expireAt, id)
	return err
}

func (r *Repo) DeleteUser(id int64) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

// --- V3 proxy_nodes ---

func (r *Repo) CreateNode(userID int64, req *model.CreateNodeRequest) (*model.ProxyNode, error) {
	n := &model.ProxyNode{}
	err := r.db.QueryRow(
		`INSERT INTO proxy_nodes (user_id,name,protocol,address,port,raw_url,sni,host,path,security,alpn,fingerprint,transport,encryption,name_prefix,local_opt_domain)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 RETURNING id,user_id,name,protocol,address,port,raw_url,sni,host,path,security,alpn,fingerprint,transport,encryption,name_prefix,local_opt_domain,enabled,status,sort_order,created_at`,
		userID, req.Name, req.Protocol, req.Address, req.Port, req.RawURL, req.SNI, req.Host, req.Path, req.Security, req.ALPN, req.Fingerprint, req.Transport, req.Encryption, req.NamePrefix, req.LocalOptDomain,
	).Scan(&n.ID, &n.UserID, &n.Name, &n.Protocol, &n.Address, &n.Port, &n.RawURL, &n.SNI, &n.Host, &n.Path, &n.Security, &n.ALPN, &n.Fingerprint, &n.Transport, &n.Encryption, &n.NamePrefix, &n.LocalOptDomain, &n.Enabled, &n.Status, &n.SortOrder, &n.CreatedAt)
	return n, err
}

func (r *Repo) GetNodeByID(id, userID int64) (*model.ProxyNode, error) {
	n := &model.ProxyNode{}
	err := r.db.QueryRow(
		`SELECT id,user_id,name,protocol,address,port,raw_url,sni,host,path,security,alpn,fingerprint,transport,encryption,name_prefix,local_opt_domain,enabled,status,last_tested_at,sort_order,created_at
		 FROM proxy_nodes WHERE id=? AND user_id=?`, id, userID,
	).Scan(&n.ID, &n.UserID, &n.Name, &n.Protocol, &n.Address, &n.Port, &n.RawURL, &n.SNI, &n.Host, &n.Path, &n.Security, &n.ALPN, &n.Fingerprint, &n.Transport, &n.Encryption, &n.NamePrefix, &n.LocalOptDomain, &n.Enabled, &n.Status, &n.LastTestedAt, &n.SortOrder, &n.CreatedAt)
	return n, err
}

func (r *Repo) ListNodes(userID int64) ([]model.ProxyNode, error) {
	rows, err := r.db.Query(
		`SELECT id,user_id,name,protocol,address,port,raw_url,sni,host,path,security,alpn,fingerprint,transport,encryption,name_prefix,local_opt_domain,enabled,status,last_tested_at,sort_order,created_at
		 FROM proxy_nodes WHERE user_id=? ORDER BY sort_order, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []model.ProxyNode
	for rows.Next() {
		var n model.ProxyNode
		if err := rows.Scan(&n.ID, &n.UserID, &n.Name, &n.Protocol, &n.Address, &n.Port, &n.RawURL, &n.SNI, &n.Host, &n.Path, &n.Security, &n.ALPN, &n.Fingerprint, &n.Transport, &n.Encryption, &n.NamePrefix, &n.LocalOptDomain, &n.Enabled, &n.Status, &n.LastTestedAt, &n.SortOrder, &n.CreatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (r *Repo) UpdateNode(id, userID int64, req *model.UpdateNodeRequest) (*model.ProxyNode, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	exec := func(q string, args ...interface{}) {
		if err == nil {
			_, err = tx.Exec(q, args...)
		}
	}

	if req.Name != nil {
		exec(`UPDATE proxy_nodes SET name=? WHERE id=? AND user_id=?`, *req.Name, id, userID)
	}
	if req.Address != nil {
		exec(`UPDATE proxy_nodes SET address=? WHERE id=? AND user_id=?`, *req.Address, id, userID)
	}
	if req.Port != nil {
		exec(`UPDATE proxy_nodes SET port=? WHERE id=? AND user_id=?`, *req.Port, id, userID)
	}
	if req.SNI != nil {
		exec(`UPDATE proxy_nodes SET sni=? WHERE id=? AND user_id=?`, *req.SNI, id, userID)
	}
	if req.Host != nil {
		exec(`UPDATE proxy_nodes SET host=? WHERE id=? AND user_id=?`, *req.Host, id, userID)
	}
	if req.Path != nil {
		exec(`UPDATE proxy_nodes SET path=? WHERE id=? AND user_id=?`, *req.Path, id, userID)
	}
	if req.Security != nil {
		exec(`UPDATE proxy_nodes SET security=? WHERE id=? AND user_id=?`, *req.Security, id, userID)
	}
	if req.ALPN != nil {
		exec(`UPDATE proxy_nodes SET alpn=? WHERE id=? AND user_id=?`, *req.ALPN, id, userID)
	}
	if req.Fingerprint != nil {
		exec(`UPDATE proxy_nodes SET fingerprint=? WHERE id=? AND user_id=?`, *req.Fingerprint, id, userID)
	}
	if req.Transport != nil {
		exec(`UPDATE proxy_nodes SET transport=? WHERE id=? AND user_id=?`, *req.Transport, id, userID)
	}
	if req.Encryption != nil {
		exec(`UPDATE proxy_nodes SET encryption=? WHERE id=? AND user_id=?`, *req.Encryption, id, userID)
	}
	if req.NamePrefix != nil {
		exec(`UPDATE proxy_nodes SET name_prefix=? WHERE id=? AND user_id=?`, *req.NamePrefix, id, userID)
	}
	if req.LocalOptDomain != nil {
		exec(`UPDATE proxy_nodes SET local_opt_domain=? WHERE id=? AND user_id=?`, *req.LocalOptDomain, id, userID)
	}
	if req.Enabled != nil {
		exec(`UPDATE proxy_nodes SET enabled=? WHERE id=? AND user_id=?`, *req.Enabled, id, userID)
	}

	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetNodeByID(id, userID)
}

func (r *Repo) DeleteNode(id, userID int64) error {
	_, err := r.db.Exec(`DELETE FROM proxy_nodes WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (r *Repo) ReorderNodes(userID int64, orderedIDs []int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range orderedIDs {
		if _, err := tx.Exec(`UPDATE proxy_nodes SET sort_order=? WHERE id=? AND user_id=?`, i*10, id, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repo) UpdateNodeStatus(id int64, status string) error {
	now := time.Now()
	_, err := r.db.Exec(`UPDATE proxy_nodes SET status=?, last_tested_at=? WHERE id=?`, status, now, id)
	return err
}

func (r *Repo) ResetUserTraffic(id int64) error {
	_, err := r.db.Exec(`UPDATE users SET used_bytes=0 WHERE id=?`, id)
	return err
}

// --- node_cf_bindings ---

func (r *Repo) CreateCFBinding(nodeID int64, fullDomain, namePrefix, source string) (*model.NodeCFBinding, error) {
	if source == "" {
		source = "local"
	}
	b := &model.NodeCFBinding{}
	err := r.db.QueryRow(
		`INSERT INTO node_cf_bindings (node_id, full_domain, name_prefix, source) VALUES (?,?,?,?)
		 RETURNING id, node_id, full_domain, name_prefix, COALESCE(source,'local'), sort_order`,
		nodeID, fullDomain, namePrefix, source,
	).Scan(&b.ID, &b.NodeID, &b.FullDomain, &b.NamePrefix, &b.Source, &b.SortOrder)
	return b, err
}

func (r *Repo) ListCFBindings(nodeID int64) ([]model.NodeCFBinding, error) {
	rows, err := r.db.Query(
		`SELECT id, node_id, full_domain, name_prefix, COALESCE(source,'local'), sort_order
		 FROM node_cf_bindings WHERE node_id=? ORDER BY sort_order, id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.NodeCFBinding
	for rows.Next() {
		var b model.NodeCFBinding
		if err := rows.Scan(&b.ID, &b.NodeID, &b.FullDomain, &b.NamePrefix, &b.Source, &b.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *Repo) DeleteCFBinding(id, nodeID int64) error {
	_, err := r.db.Exec(`DELETE FROM node_cf_bindings WHERE id=? AND node_id=?`, id, nodeID)
	return err
}

// --- domain_mappings CRUD ---

func (r *Repo) CreateDomainMapping(subdomain, fullDomain, algorithmType, source string) (*model.DomainMapping, error) {
	if source == "" {
		source = "local"
	}
	m := &model.DomainMapping{}
	err := r.db.QueryRow(
		`INSERT INTO domain_mappings (subdomain, full_domain, algorithm_type, source) VALUES (?,?,?,?)
		 RETURNING id, subdomain, full_domain, algorithm_type, COALESCE(source,'local')`,
		subdomain, fullDomain, algorithmType, source,
	).Scan(&m.ID, &m.Subdomain, &m.FullDomain, &m.AlgorithmType, &m.Source)
	return m, err
}

func (r *Repo) BatchCreateDomainMappings(source string, subdomains []string, suffix string, domains []string) ([]model.DomainMapping, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var result []model.DomainMapping
	if source == "external" {
		for _, d := range domains {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			m := &model.DomainMapping{}
			err := tx.QueryRow(
				`INSERT INTO domain_mappings (subdomain, full_domain, algorithm_type, source) VALUES (?,?,?,?)
				 RETURNING id, subdomain, full_domain, algorithm_type, COALESCE(source,'local')`,
				d, d, "fast", "external",
			).Scan(&m.ID, &m.Subdomain, &m.FullDomain, &m.AlgorithmType, &m.Source)
			if err != nil {
				continue
			}
			result = append(result, *m)
		}
	} else {
		if source == "" {
			source = "local"
		}
		for _, sub := range subdomains {
			sub = strings.TrimSpace(sub)
			if sub == "" {
				continue
			}
			full := sub + suffix
			m := &model.DomainMapping{}
			err := tx.QueryRow(
				`INSERT INTO domain_mappings (subdomain, full_domain, algorithm_type, source) VALUES (?,?,?,?)
				 RETURNING id, subdomain, full_domain, algorithm_type, COALESCE(source,'local')`,
				sub, full, "fast", source,
			).Scan(&m.ID, &m.Subdomain, &m.FullDomain, &m.AlgorithmType, &m.Source)
			if err != nil {
				continue
			}
			result = append(result, *m)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repo) DeleteDomainMapping(id int64) error {
	_, err := r.db.Exec(`DELETE FROM domain_mappings WHERE id=?`, id)
	return err
}

// --- proxy / sni ---

func (r *Repo) ListDomainMappings() ([]model.DomainMapping, error) {
	rows, err := r.db.Query(`SELECT id, subdomain, full_domain, algorithm_type, COALESCE(source,'local') FROM domain_mappings ORDER BY source, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.DomainMapping
	for rows.Next() {
		var m model.DomainMapping
		if err := rows.Scan(&m.ID, &m.Subdomain, &m.FullDomain, &m.AlgorithmType, &m.Source); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repo) GetDomainMapping(id int64) (*model.DomainMapping, error) {
	m := &model.DomainMapping{}
	err := r.db.QueryRow(
		`SELECT id, subdomain, full_domain, algorithm_type, COALESCE(source,'local') FROM domain_mappings WHERE id=?`, id,
	).Scan(&m.ID, &m.Subdomain, &m.FullDomain, &m.AlgorithmType, &m.Source)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *Repo) UpdateDomainMappingFull(id int64, subdomain, fullDomain, algorithmType string) error {
	_, err := r.db.Exec(`UPDATE domain_mappings SET subdomain=?, full_domain=?, algorithm_type=? WHERE id=?`,
		subdomain, fullDomain, algorithmType, id)
	return err
}

func (r *Repo) EnsureDomainMappings(baseDomain string) error {
	for i := 1; i <= 10; i++ {
		sub := fmt.Sprintf("dshax%d", i)
		full := fmt.Sprintf("%s.%s", sub, baseDomain)
		_, err := r.db.Exec(
			`INSERT OR IGNORE INTO domain_mappings (subdomain, full_domain, algorithm_type, source) VALUES (?,?,?,?)`,
			sub, full, "fast", "local",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) resolveAlgoType(algoName string) string {
	switch algoName {
	case "fast", "steady", "tidal", "composite", "lowjitter":
		return algoName
	}
	var t string
	r.db.QueryRow(`SELECT algo_type FROM custom_algorithms WHERE name=?`, algoName).Scan(&t)
	if t != "" {
		return t
	}
	return "fast"
}

func (r *Repo) GetBestIPForDomain(fullDomain string) (string, int, error) {
	var algoName string
	err := r.db.QueryRow(`SELECT algorithm_type FROM domain_mappings WHERE full_domain = ?`, fullDomain).Scan(&algoName)
	if err != nil {
		return "", 0, fmt.Errorf("domain not mapped: %v", err)
	}

	algoType := r.resolveAlgoType(algoName)
	clientFilter := r.GetAlgorithmClientFilter(algoName)
	macFilter := ""
	var macArgs []interface{}
	if clientFilter != "" && clientFilter != "all" {
		macs := strings.Split(clientFilter, ",")
		var placeholders []string
		for _, m := range macs {
			m = strings.TrimSpace(m)
			if m != "" {
				placeholders = append(placeholders, "?")
				macArgs = append(macArgs, m)
			}
		}
		if len(placeholders) > 0 {
			macFilter = " AND COALESCE(client_id,mac_address) IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	baseWhere := `FROM latency_records WHERE recorded_at >= datetime('now', '-3 hours')` + macFilter

	var bestIP string
	var score int
	var query string

	// Shared SQL fragments with correct precedence
	ok := `COUNT(CASE WHEN latency_ms>0 THEN 1 END)`
	total := `COUNT(*)`
	avg := `AVG(CASE WHEN latency_ms>0 THEN latency_ms END)`
	invRate := `1.0*` + total + `/NULLIF(` + ok + `,0)`   // 1/success_rate = COUNT_ALL/COUNT_OK
	gate := `HAVING 1.0*` + ok + `/` + total + ` >= 0.6` +
		` AND ` + total + ` >= 10` +
		` AND MAX(recorded_at) >= datetime("now","-2 hours")`
	countPenalty := `CASE WHEN ` + total + ` >= 50 THEN 1.0 ELSE 1.0+(50-` + total + `)*0.015 END`
	// Variance (no SQRT — SQLite math functions not compiled in container)
	variance := `MAX(0, AVG(CASE WHEN latency_ms>0 THEN latency_ms*latency_ms END)-` + avg + `*` + avg + `)`

	switch algoType {
	case "steady":
		// AVG/rate + variance/200 ≈ AVG/rate + σ²/200  (σ=10→penalty=0.5, σ=50→penalty=12.5)
		query = `SELECT ip_address, CAST((` + avg + `*` + invRate + ` + 1.0*` + variance + `/200)*` + countPenalty + ` AS INTEGER) ` + baseWhere + ` GROUP BY ip_address ` + gate + ` ORDER BY 2 ASC LIMIT 1`
	case "tidal":
		return r.bestIPTidal(fullDomain, baseWhere, macArgs)
	case "composite":
		query = `SELECT ip_address, CAST((` + avg + `*` + invRate + `*` + invRate + ` + 1.0*` + variance + `/300)*` + countPenalty + ` AS INTEGER) ` + baseWhere + ` GROUP BY ip_address ` + gate + ` ORDER BY 2 ASC LIMIT 1`
	case "lowjitter":
		return r.bestIPLowJitter(fullDomain, baseWhere, macArgs)
	default:
		query = `SELECT ip_address, CAST(` + avg + `/MIN(1.0*` + ok + `/` + total + `,0.95)*` + countPenalty + ` AS INTEGER) ` + baseWhere + ` GROUP BY ip_address ` + gate + ` ORDER BY 2 ASC LIMIT 1`
	}

	if len(macArgs) > 0 {
		err = r.db.QueryRow(query, macArgs...).Scan(&bestIP, &score)
	} else {
		err = r.db.QueryRow(query).Scan(&bestIP, &score)
	}

	if err != nil || bestIP == "" {
		log.Printf("[algo] %s for %s: no IP found (err=%v, query=%s)", algoType, fullDomain, err, query)
		return "1.1.1.1", 0, nil
	}
	return bestIP, score, nil
}

// GetBestIPsForDomain returns top-N IPs for a domain (for TLS round-robin selection).
func (r *Repo) GetBestIPsForDomain(fullDomain string, limit int) ([]string, error) {
	// Reuse the same algo resolution + client filter logic from GetBestIPForDomain
	var algoName string
	err := r.db.QueryRow(`SELECT algorithm_type FROM domain_mappings WHERE full_domain = ?`, fullDomain).Scan(&algoName)
	if err != nil {
		return nil, err
	}
	algoType := r.resolveAlgoType(algoName)
	if algoType == "tidal" || algoType == "lowjitter" {
		algoType = "fast" // fallback to fast for top-N query
	}

	clientFilter := r.GetAlgorithmClientFilter(algoName)
	macFilter := ""
	var macArgs []interface{}
	if clientFilter != "" && clientFilter != "all" {
		macs := strings.Split(clientFilter, ",")
		var placeholders []string
		for _, m := range macs {
			m = strings.TrimSpace(m)
			if m != "" {
				placeholders = append(placeholders, "?")
				macArgs = append(macArgs, m)
			}
		}
		if len(placeholders) > 0 {
			macFilter = " AND COALESCE(client_id,mac_address) IN (" + strings.Join(placeholders, ",") + ")"
		}
	}

	avg := `AVG(CASE WHEN latency_ms>0 THEN latency_ms END)`
	gate := `HAVING 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) >= 0.6`
	query := `SELECT ip_address FROM latency_records WHERE recorded_at >= datetime('now','-3 hours')` + macFilter + ` GROUP BY ip_address ` + gate + ` ORDER BY ` + avg + ` ASC LIMIT ` + fmt.Sprintf("%d", limit)

	var ips []string
	var rows *sql.Rows
	if len(macArgs) > 0 {
		rows, err = r.db.Query(query, macArgs...)
	} else {
		rows, err = r.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			ips = append(ips, ip)
		}
	}
	return ips, nil
}

// bestIPTidal: queries peak (18-24) and off-peak latency separately from 24h history,
// weights peak 60% / off-peak 40%, then penalizes by overall success rate.
func (r *Repo) bestIPTidal(fullDomain, baseWhere string, args []interface{}) (string, int, error) {
	where := `WHERE recorded_at >= datetime('now','-24 hours')` + strings.Replace(baseWhere, `FROM latency_records WHERE`, `AND `, 1)
	candidateQuery := `SELECT ip_address FROM latency_records ` + where + ` GROUP BY ip_address HAVING 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) >= 0.6 ORDER BY AVG(CASE WHEN latency_ms>0 THEN latency_ms END) ASC LIMIT 20`
	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = r.db.Query(candidateQuery, args...)
	} else {
		rows, err = r.db.Query(candidateQuery)
	}
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	type score struct {
		ip    string
		score int
	}
	var best score
	best.score = 999999
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		// Peak (18-23h) avg/success
		var peakAvg, peakRate float64
		peakQ := `SELECT COALESCE(AVG(CASE WHEN latency_ms>0 THEN latency_ms END),0), COALESCE(1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/NULLIF(COUNT(*),0),0) FROM latency_records ` + where + ` AND ip_address=? AND CAST(strftime('%H',recorded_at) AS INTEGER) BETWEEN 18 AND 23`
		if len(args) > 0 {
			qa := append([]interface{}{ip}, args...)
			r.db.QueryRow(peakQ, qa...).Scan(&peakAvg, &peakRate)
		} else {
			r.db.QueryRow(peakQ, ip).Scan(&peakAvg, &peakRate)
		}
		// Off-peak avg/success
		var offAvg, offRate float64
		offQ := `SELECT COALESCE(AVG(CASE WHEN latency_ms>0 THEN latency_ms END),0), COALESCE(1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/NULLIF(COUNT(*),0),0) FROM latency_records ` + where + ` AND ip_address=? AND CAST(strftime('%H',recorded_at) AS INTEGER) NOT BETWEEN 18 AND 23`
		if len(args) > 0 {
			qa := append([]interface{}{ip}, args...)
			r.db.QueryRow(offQ, qa...).Scan(&offAvg, &offRate)
		} else {
			r.db.QueryRow(offQ, ip).Scan(&offAvg, &offRate)
		}
		peakScore := 999999.9
		offScore := 999999.9
		if peakRate > 0 {
			peakScore = peakAvg / peakRate
		}
		if offRate > 0 {
			offScore = offAvg / offRate
		}
		s := int(peakScore*0.6 + offScore*0.4)
		if s < best.score {
			best.ip = ip
			best.score = s
		}
	}
	if best.ip == "" {
		return "1.1.1.1", 0, nil
	}
	return best.ip, best.score, nil
}

// bestIPLowJitter: computes jitter (avg |lat_i - lat_{i-1}|) in Go and penalizes heavily.
func (r *Repo) bestIPLowJitter(fullDomain, baseWhere string, args []interface{}) (string, int, error) {
	where := `WHERE recorded_at >= datetime('now','-3 hours')` + strings.Replace(baseWhere, `FROM latency_records WHERE`, `AND `, 1)
	candidateQuery := `SELECT ip_address, AVG(CASE WHEN latency_ms>0 THEN latency_ms END), 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) FROM latency_records ` + where + ` GROUP BY ip_address HAVING 1.0*COUNT(CASE WHEN latency_ms>0 THEN 1 END)/COUNT(*) >= 0.6 ORDER BY AVG(CASE WHEN latency_ms>0 THEN latency_ms END) ASC LIMIT 20`
	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = r.db.Query(candidateQuery, args...)
	} else {
		rows, err = r.db.Query(candidateQuery)
	}
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	type scored struct {
		ip    string
		score int
	}
	var best scored
	best.score = 999999
	var processed int
	for rows.Next() {
		if processed >= 20 {
			break
		}
		processed++
		var ip string
		var avg, rate float64
		if err := rows.Scan(&ip, &avg, &rate); err != nil {
			continue
		}
		// Fetch ordered latencies for jitter (last 100 records only)
		recQ := `SELECT CASE WHEN latency_ms>0 THEN latency_ms ELSE NULL END FROM latency_records ` + where + ` AND ip_address=? ORDER BY recorded_at ASC LIMIT 100`
		var recs *sql.Rows
		if len(args) > 0 {
			qa := append([]interface{}{ip}, args...)
			recs, err = r.db.Query(recQ, qa...)
		} else {
			recs, err = r.db.Query(recQ, ip)
		}
		if err != nil {
			continue
		}
		var prev *float64
		var jitterSum, n float64
		for recs.Next() {
			var v float64
			if recs.Scan(&v) != nil || v <= 0 {
				continue
			}
			lat := &v
			if prev != nil {
				diff := *lat - *prev
				if diff < 0 {
					diff = -diff
				}
				jitterSum += diff
				n++
			}
			prev = lat
		}
		recs.Close()
		jitter := 0.0
		if n > 0 {
			jitter = jitterSum / n
		}
		s := int(avg/rate + jitter*1.5)
		if s < best.score {
			best.ip = ip
			best.score = s
		}
	}
	if best.ip == "" {
		return "1.1.1.1", 0, nil
	}
	return best.ip, best.score, nil
}
