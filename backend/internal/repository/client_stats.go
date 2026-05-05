package repository

import "log"

// UpdateAllClientBias recalculates bias for all active clients.
func (r *Repo) UpdateAllClientBias() error {
	rows, err := r.db.Query(`SELECT DISTINCT COALESCE(client_id, mac_address) FROM latency_records
		WHERE recorded_at >= datetime('now', '-24 hours') AND COALESCE(client_id, mac_address) != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var cid string
		if rows.Scan(&cid) == nil {
			if err := r.UpdateClientBias(cid); err == nil {
				count++
			}
		}
	}
	log.Printf("[bias] updated %d client biases", count)
	return rows.Err()
}

// UpdatePerClientStats records per-client stats for an IP in ip_stats.per_client_json.
func (r *Repo) UpdatePerClientStats(ip, clientID string, avgLatency float64) error {
	_, err := r.db.Exec(`UPDATE ip_stats SET
		per_client_json = json_set(COALESCE(per_client_json,'{}'),
			'$.'||?, json_object('avg_ms', ?, 'updated', datetime('now'))),
		updated_at = datetime('now')
		WHERE ip_address=?`,
		clientID, avgLatency, ip)
	return err
}
