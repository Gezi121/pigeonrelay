package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_fk=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func runMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			expire_at DATETIME,
			monthly_quota_bytes INTEGER DEFAULT 0,
			used_bytes INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS subscription_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			type TEXT NOT NULL DEFAULT 'clash',
			url TEXT,
			name TEXT NOT NULL,
			last_fetch_status TEXT DEFAULT '',
			last_fetch_at DATETIME,
			raw_data TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS node_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			protocol_type TEXT NOT NULL,
			template_name TEXT NOT NULL,
			config_content TEXT DEFAULT '',
			content_type TEXT NOT NULL DEFAULT 'yaml',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS node_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'subscription',
			source_id INTEGER REFERENCES subscription_sources(id),
			unified_config TEXT DEFAULT '{}',
			rule_config TEXT DEFAULT '[]',
			advanced_prefer TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL REFERENCES node_groups(id),
			template_id INTEGER REFERENCES node_templates(id),
			original_node TEXT DEFAULT '',
			processed_node TEXT DEFAULT '',
			address_list TEXT DEFAULT '[]',
			enabled INTEGER DEFAULT 1,
			sort_order INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS config_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			backup_type TEXT NOT NULL,
			config_content TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS traffic_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			node_id INTEGER REFERENCES nodes(id),
			bytes_up INTEGER DEFAULT 0,
			bytes_down INTEGER DEFAULT 0,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// V2 migrations
		`CREATE TABLE IF NOT EXISTS group_replacements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
			target_field TEXT NOT NULL,
			replace_value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS group_routings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
			routing_address TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS latency_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL,
			latency_ms INTEGER NOT NULL,
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS domain_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			subdomain TEXT UNIQUE NOT NULL,
			full_domain TEXT NOT NULL,
			algorithm_type TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS proxy_nodes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL REFERENCES users(id),
				name TEXT NOT NULL,
				protocol TEXT NOT NULL DEFAULT 'vless',
				address TEXT NOT NULL DEFAULT '',
				port INTEGER DEFAULT 443,
				raw_url TEXT DEFAULT '',
				sni TEXT DEFAULT '',
				host TEXT DEFAULT '',
				path TEXT DEFAULT '',
				security TEXT DEFAULT '',
				alpn TEXT DEFAULT '',
				fingerprint TEXT DEFAULT '',
				transport TEXT DEFAULT 'tcp',
				encryption TEXT DEFAULT 'none',
				name_prefix TEXT DEFAULT '',
				local_opt_domain TEXT DEFAULT '',
				enabled INTEGER DEFAULT 1,
				status TEXT DEFAULT 'untested',
				last_tested_at DATETIME,
				sort_order INTEGER DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
		`CREATE TABLE IF NOT EXISTS node_replacements (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id INTEGER NOT NULL REFERENCES proxy_nodes(id) ON DELETE CASCADE,
				target_field TEXT NOT NULL,
				replace_value TEXT NOT NULL
			)`,
		`CREATE TABLE IF NOT EXISTS node_routings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id INTEGER NOT NULL REFERENCES proxy_nodes(id) ON DELETE CASCADE,
				routing_address TEXT NOT NULL
			)`,
		`CREATE TABLE IF NOT EXISTS node_cf_bindings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id INTEGER NOT NULL REFERENCES proxy_nodes(id) ON DELETE CASCADE,
				full_domain TEXT NOT NULL DEFAULT '',
				name_prefix TEXT NOT NULL DEFAULT '',
				sort_order INTEGER NOT NULL DEFAULT 0
			)`,
		`CREATE TABLE IF NOT EXISTS subscription_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hash TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			max_fetch_per_day INTEGER DEFAULT 100,
			enabled INTEGER DEFAULT 1,
			traffic_limit_gb INTEGER DEFAULT 0,
			expire_at DATETIME,
			remarks TEXT DEFAULT '',
			allowed_group_ids TEXT DEFAULT ''
		)`,
	}

	for _, s := range statements {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Simple alter tables for V2 (ignore errors if columns exist)
	db.Exec(`ALTER TABLE subscription_sources ADD COLUMN status TEXT DEFAULT 'untested'`)
	db.Exec(`ALTER TABLE subscription_sources ADD COLUMN last_tested_at DATETIME`)
	db.Exec(`ALTER TABLE subscription_sources ADD COLUMN raw_nodes_cache TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE subscription_sources ADD COLUMN local_nodes TEXT DEFAULT ''`)

	db.Exec(`ALTER TABLE node_groups ADD COLUMN name_prefix TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE node_groups ADD COLUMN group_type TEXT DEFAULT 'standard'`)

	db.Exec(`ALTER TABLE domain_mappings ADD COLUMN source TEXT DEFAULT 'local'`)
	db.Exec(`ALTER TABLE node_cf_bindings ADD COLUMN source TEXT DEFAULT 'local'`)
	db.Exec(`ALTER TABLE subscription_links ADD COLUMN name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE latency_records ADD COLUMN mac_address TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE latency_records ADD COLUMN client_id TEXT DEFAULT ''`)

	// speed test clients & custom algorithms
	db.Exec(`CREATE TABLE IF NOT EXISTS speed_test_clients (
		mac_address TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		last_seen DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS custom_algorithms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		algo_type TEXT NOT NULL DEFAULT 'fast',
		client_filter TEXT DEFAULT 'all',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	// settings table

	// ip_stats: rolling EWMA metrics for per-IP quality scoring
	db.Exec(`CREATE TABLE IF NOT EXISTS ip_stats (
	ip_address TEXT PRIMARY KEY,
	ewma_latency_ms REAL NOT NULL DEFAULT 0,
	ewma_tls_ms REAL NOT NULL DEFAULT 0,
	ewma_success_rate REAL NOT NULL DEFAULT 0,
	ewma_tls_ok_rate REAL NOT NULL DEFAULT 0,
	ewma_loss_rate REAL NOT NULL DEFAULT 0,
	jitter_ms REAL NOT NULL DEFAULT 0,
	latency_stddev REAL NOT NULL DEFAULT 0,
	success_rate_variance REAL NOT NULL DEFAULT 0,
	p50_latency_ms REAL NOT NULL DEFAULT 0,
	p95_latency_ms REAL NOT NULL DEFAULT 0,
	throughput_bytes_per_sec REAL NOT NULL DEFAULT 0,
	samples_1h INTEGER DEFAULT 0,
	samples_24h INTEGER DEFAULT 0,
	last_tested_at DATETIME,
	last_tls_ok_at DATETIME,
	test_priority REAL NOT NULL DEFAULT 0,
	cooldown_until DATETIME,
	per_client_json TEXT DEFAULT '{}',
	peak_hour_latency REAL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_ip_stats_priority ON ip_stats(test_priority DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_ip_stats_last_tested ON ip_stats(last_tested_at)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '')`)
	db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('clash_template',
		'port: 7890\nsocks-port: 7891\nallow-lan: true\nmode: rule\nlog-level: info\nexternal-controller: 127.0.0.1:9090\n\n# 节点配置\nproxies:\n${proxies}\n\n# 策略组配置\nproxy-groups:\n  - name: 🚀 节点选择\n    type: select\n    proxies:\n      - ♻️ 自动选择\n${proxy-names}\n    url: http://www.gstatic.com/generate_204\n    interval: 300\n\n  - name: ♻️ 自动选择\n    type: url-test\n    url: http://www.gstatic.com/generate_204\n    interval: 300\n    proxies:\n${proxy-names}\n\n# 路由规则\nrules:\n  - GEOIP,LAN,DIRECT\n  - GEOIP,CN,DIRECT\n  - MATCH,🚀 节点选择\n')`)

	return nil
}
