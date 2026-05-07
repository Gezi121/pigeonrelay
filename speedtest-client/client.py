#!/usr/bin/env python3
"""PigeonRelay Speed Test Client v2
Three-phase: TCP sweep -> TLS verify -> precision re-test.
Raw samples reported per-IP, server computes EWMA distributions."""

import sys
sys.stdout.write(">>> PigeonRelay v2 starting...\n")
sys.stdout.flush()

import socket, ssl, time, json, requests, uuid, os, signal, logging, random, sqlite3
import ipaddress
from concurrent.futures import ThreadPoolExecutor, as_completed
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

# ---- Config from env ----
BASE_URL       = os.getenv("PIGEONRELAY_URL", "https://pigeonrelay.example.com")
LATENCY_TOKEN  = os.getenv("LATENCY_TOKEN", "default-latency-token")
CLIENT_ISP     = os.getenv("CLIENT_ISP", "unknown")       # china-telecom / china-unicom / china-mobile / overseas
CLIENT_REGION  = os.getenv("CLIENT_REGION", "unknown")

# Phase 1: TCP sweep
TCP_WORKERS    = int(os.getenv("TCP_WORKERS", "50"))
TCP_TIMEOUT    = int(os.getenv("TCP_TIMEOUT", "2"))
TCP_TRIES      = int(os.getenv("TCP_TRIES", "3"))            # measurements per IP per sweep
TCP_TOP_N      = int(os.getenv("TCP_TOP_N", "200"))
PHASE1_SHARDS  = int(os.getenv("PHASE1_SHARDS", "4"))
SHARD_INTERVAL = int(os.getenv("SHARD_INTERVAL", "45"))

# Phase 2: TLS verify
TLS_WORKERS    = int(os.getenv("TLS_WORKERS", "10"))        # very low concurrency
TLS_TIMEOUT    = int(os.getenv("TLS_TIMEOUT", "3"))
TLS_TOP_M      = int(os.getenv("TLS_TOP_M", "50"))          # pass to Phase 3

# Phase 3: precision re-test
FAST_INTERVAL  = int(os.getenv("FAST_INTERVAL", "120"))     # 2 min
FAST_TRIES     = int(os.getenv("FAST_TRIES", "1"))          # 1 per cycle (spread across time)
FAST_TIMEOUT   = float(os.getenv("FAST_TIMEOUT", "1.5"))
FAST_WORKERS   = int(os.getenv("FAST_WORKERS", "10"))

# General
MEASURE_SEC    = int(os.getenv("MEASURE_INTERVAL", "600"))
REPORT_SEC     = int(os.getenv("REPORT_INTERVAL", "1200"))
TLL_MIN_MS     = int(os.getenv("TLL_MIN_MS", "0"))          # 40 for telecom/unicom, 0 for mobile/overseas
MIN_VALID_MS   = int(os.getenv("MIN_VALID_MS", "15"))
TEST_PORT      = int(os.getenv("TEST_PORT", "443"))
TEST_SNI       = os.getenv("TEST_SNI", "")                  # SNI for TLS verification (use server health endpoint)
OOM_MAX_SEC    = int(os.getenv("OOM_MAX_SEC", "7200"))
MAX_IPS_PER_REPORT = int(os.getenv("MAX_IPS_PER_REPORT", "400"))

DATA_DIR = os.getenv("DATA_DIR", "data")
LOG_DIR  = os.path.join(DATA_DIR, "logs")
ID_FILE  = os.path.join(DATA_DIR, "client_id.txt")
DB_FILE  = os.path.join(DATA_DIR, "pending_reports.db")

os.makedirs(LOG_DIR, exist_ok=True)

# ---- Logging ----
def setup_logging():
    log = logging.getLogger("speedtest")
    log.setLevel(logging.DEBUG)
    fmt = logging.Formatter("%(asctime)s [%(levelname)s] %(message)s", datefmt="%Y-%m-%d %H:%M:%S")
    fh = logging.FileHandler(os.path.join(LOG_DIR, "speedtest.log"), encoding="utf-8")
    fh.setLevel(logging.DEBUG); fh.setFormatter(fmt); log.addHandler(fh)
    ch = logging.StreamHandler(sys.stdout); ch.setLevel(logging.INFO); ch.setFormatter(fmt); log.addHandler(ch)
    return log

log = setup_logging()

# ---- Local SQLite for pending reports ----
def init_db():
    db = sqlite3.connect(DB_FILE)
    db.execute("CREATE TABLE IF NOT EXISTS pending (id INTEGER PRIMARY KEY AUTOINCREMENT, payload TEXT NOT NULL, created_at TEXT DEFAULT (datetime('now')))")
    db.commit()
    return db

db = init_db()

def save_pending(payload_json: str):
    db.execute("INSERT INTO pending (payload) VALUES (?)", (payload_json,))
    db.commit()

def flush_pending():
    """Re-report all previously failed payloads. Returns True if all flushed."""
    rows = db.execute("SELECT id, payload FROM pending ORDER BY id").fetchall()
    all_ok = True
    for row_id, payload in rows:
        try:
            r = requests.post(f"{BASE_URL}/api/v1/latency/report",
                data=payload,
                headers={"Authorization": f"Bearer {LATENCY_TOKEN}", "Content-Type": "application/json"},
                timeout=(15, 90))
            if r.status_code == 200:
                db.execute("DELETE FROM pending WHERE id=?", (row_id,))
                db.commit()
                log.info("pending report %d flushed OK", row_id)
            else:
                log.warning("pending report %d retry FAIL HTTP %d", row_id, r.status_code)
                all_ok = False
                break
        except Exception as e:
            log.warning("pending report %d retry ERR: %s", row_id, e)
            all_ok = False
            break
    return all_ok

# ---- Client identity ----
def load_client_id():
    if os.path.isfile(ID_FILE):
        with open(ID_FILE) as f:
            cid = f.read().strip()
            if cid: return cid
    cid = str(uuid.uuid4())
    with open(ID_FILE, "w") as f: f.write(cid)
    log.info("new client_id=%s", cid)
    return cid

CLIENT_ID = load_client_id()

# ---- CF IP Pool ----
def fetch_cf_cidrs():
    try:
        r = requests.get("https://api.cloudflare.com/client/v4/ips", timeout=10)
        return r.json()["result"]["ipv4_cidrs"]
    except Exception as e:
        log.warning("fetch CF IPs failed: %s, using fallback", e)
        return [
            "173.245.48.0/20","103.21.244.0/22","103.22.200.0/22","103.31.4.0/22",
            "141.101.64.0/18","108.162.192.0/18","190.93.240.0/20","188.114.96.0/20",
            "197.234.240.0/22","198.41.128.0/17","162.158.0.0/15","104.16.0.0/13",
            "104.24.0.0/14","172.64.0.0/13","131.0.72.0/22",
        ]

def sample_ips(cidrs):
    ips = []
    for cidr in cidrs:
        try:
            net = ipaddress.ip_network(cidr, strict=False)
            for sn in net.subnets(new_prefix=24):
                hosts = list(sn.hosts())
                if hosts:
                    ips.append(str(random.choice(hosts)))
        except Exception:
            continue
    random.shuffle(ips)
    log.info("sampled %d IPs from %d CIDRs", len(ips), len(cidrs))
    return ips

# ---- Phase 1: TCP RTT ----
def iso_now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.") + f"{int(time.time()*1000)%1000:03d}Z"

def tcp_latency(ip, timeout=TCP_TIMEOUT):
    """Returns (latency_ms, iso_timestamp). latency=0 means failure."""
    sock = None
    t = iso_now()
    try:
        start = time.monotonic()
        sock = socket.create_connection((ip, TEST_PORT), timeout=timeout)
        sock.close()
        lat = int((time.monotonic() - start) * 1000)
        if lat < MIN_VALID_MS:
            return 0, t
        if TLL_MIN_MS > 0 and lat < TLL_MIN_MS:
            return 0, t
        return lat, t
    except Exception:
        if sock:
            try: sock.close()
            except Exception: pass
        return 0, t

def phase1_tcp_sweep(ip_list):
    """Sweep all /24-sampled IPs with TCP_TRIES per IP. Returns (top_ips, {ip: [(lat, tstamp)]})."""
    raw = defaultdict(list)
    ok, fail, start = 0, 0, time.monotonic()
    all_ips = ip_list * TCP_TRIES  # multiple passes for time-distributed samples

    with ThreadPoolExecutor(max_workers=TCP_WORKERS) as pool:
        futures = {pool.submit(tcp_latency, ip): ip for ip in all_ips}
        for future in as_completed(futures):
            ip = futures[future]
            lat, tstamp = future.result()
            if lat > 0:
                raw[ip].append((lat, tstamp)); ok += 1
            else:
                fail += 1

    # Sort candidates by avg latency
    scored = []
    for ip, pairs in raw.items():
        lats = [p[0] for p in pairs]
        avg = sum(lats) / len(lats)
        scored.append((avg, ip))

    scored.sort(key=lambda x: x[0])
    top_ips = [ip for _, ip in scored[:TCP_TOP_N]]

    elapsed = time.monotonic() - start
    log.info("phase1 done | ok=%d fail=%d top=%d elapsed=%.1fs", ok, fail, len(top_ips), elapsed)
    return top_ips, raw

# ---- Phase 2: TLS verification ----
def tls_latency(ip, sni=TEST_SNI, timeout=TLS_TIMEOUT):
    """Returns (tls_ms, ok, iso_timestamp)."""
    sock = None
    tls_sock = None
    t = iso_now()
    try:
        sock = socket.create_connection((ip, TEST_PORT), timeout=timeout)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        context.set_ciphers("ECDHE+AESGCM:ECDHE+CHACHA20:DHE+AESGCM")
        tls_sock = context.wrap_socket(sock, server_hostname=sni if sni else None)
        start_tls = time.monotonic()
        tls_sock.do_handshake()
        tls_time = int((time.monotonic() - start_tls) * 1000)
        tls_sock.close()
        return tls_time, True, t
    except Exception:
        if tls_sock:
            try: tls_sock.close()
            except Exception: pass
        elif sock:
            try: sock.close()
            except Exception: pass
        return 0, False, t

def phase2_tls_verify(ip_list):
    """TLS handshake test on Phase 1 top N IPs. Low concurrency."""
    raw = defaultdict(lambda: {"tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0})
    ok, fail = 0, 0

    with ThreadPoolExecutor(max_workers=TLS_WORKERS) as pool:
        futures = {}
        for ip in ip_list:
            time.sleep(random.uniform(0.05, 0.2))
            futures[pool.submit(tls_latency, ip)] = ip

        for future in as_completed(futures):
            ip = futures[future]
            tls_time, tls_ok, tstamp = future.result()
            if tls_ok:
                raw[ip]["tls_samples"].append(tls_time)
                raw[ip]["tls_times"].append(tstamp)
                raw[ip]["tls_ok"] += 1
                ok += 1
            else:
                raw[ip]["tls_fail"] += 1
                fail += 1

    valid = {}
    for ip, data in raw.items():
        total = data["tls_ok"] + data["tls_fail"]
        if total > 0 and data["tls_fail"] / total <= 0.5:
            valid[ip] = data

    scored = []
    for ip, data in valid.items():
        if data["tls_samples"]:
            scored.append((sum(data["tls_samples"]) / len(data["tls_samples"]), ip))
    scored.sort(key=lambda x: x[0])
    top_m = [ip for _, ip in scored[:TLS_TOP_M]]

    log.info("phase2 done | ok=%d fail=%d valid=%d top=%d", ok, fail, len(valid), len(top_m))
    return top_m, valid

# ---- Phase 3: Precision re-test ----
def phase3_fast_test(top_ips):
    """TCP+TLS dual test on top M IPs, raw samples with per-measurement timestamps."""
    results = defaultdict(lambda: {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0})

    all_ips = top_ips * FAST_TRIES
    with ThreadPoolExecutor(max_workers=FAST_WORKERS) as pool:
        futures = {pool.submit(tcp_latency, ip, FAST_TIMEOUT): ip for ip in all_ips}
        for future in as_completed(futures):
            ip = futures[future]
            lat, tstamp = future.result()
            results[ip]["tcp_samples"].append(lat)
            results[ip]["tcp_times"].append(tstamp)

    tls_candidates = [ip for ip in top_ips if results[ip]["tcp_samples"] and
                      sum(1 for x in results[ip]["tcp_samples"] if x > 0) >= FAST_TRIES // 2]
    if tls_candidates:
        with ThreadPoolExecutor(max_workers=min(TLS_WORKERS, len(tls_candidates))) as pool:
            futures = {pool.submit(tls_latency, ip): ip for ip in tls_candidates}
            for future in as_completed(futures):
                ip = futures[future]
                tls_time, tls_ok, tstamp = future.result()
                results[ip]["tls_samples"].append(tls_time)
                results[ip]["tls_times"].append(tstamp)
                if tls_ok:
                    results[ip]["tls_ok"] += 1
                else:
                    results[ip]["tls_fail"] += 1

    return results

# ---- Reporting ----
def build_report_payloads(buffer):
    """Build chunked JSON payloads from buffer, respecting MAX_IPS_PER_REPORT."""
    ip_list = list(buffer.items())
    payloads = []
    for chunk_start in range(0, len(ip_list), MAX_IPS_PER_REPORT):
        chunk = ip_list[chunk_start:chunk_start + MAX_IPS_PER_REPORT]
        data = []
        for ip, samples in chunk:
            data.append({
                "ip": ip,
                "tcp_samples": samples.get("tcp_samples", []),
                "tcp_times": samples.get("tcp_times", []),
                "tls_samples": samples.get("tls_samples", []),
                "tls_times": samples.get("tls_times", []),
                "tls_ok": samples.get("tls_ok", 0),
                "tls_fail": samples.get("tls_fail", 0),
            })
        payloads.append(json.dumps({
            "client_id": CLIENT_ID,
            "client_info": {"isp": CLIENT_ISP, "region": CLIENT_REGION, "version": "2.0.0"},
            "data": data,
        }))
    return payloads

def report_buffer(buffer):
    """Send all buffer data to server in chunks. Returns True if all succeeded."""
    payloads = build_report_payloads(buffer)
    all_ok = True
    for i, payload_json in enumerate(payloads):
        ok = report_to_server(payload_json)
        if not ok:
            all_ok = False
            log.warning("report chunk %d/%d FAIL", i+1, len(payloads))
        else:
            log.info("report chunk %d/%d OK | %d bytes", i+1, len(payloads), len(payload_json))
    return all_ok

def report_to_server(payload_json):
    """Send payload to server. On failure, persist to local SQLite for retry."""
    try:
        r = requests.post(f"{BASE_URL}/api/v1/latency/report",
            data=payload_json,
            headers={"Authorization": f"Bearer {LATENCY_TOKEN}", "Content-Type": "application/json"},
            timeout=(15, 90))
        if r.status_code == 200:
            return True
        log.warning("report FAIL HTTP %d: %s", r.status_code, r.text[:200])
    except Exception as e:
        log.error("report ERR: %s", e)

    # Persist for retry
    try:
        save_pending(payload_json)
        log.info("report saved to pending queue")
    except Exception as e2:
        log.error("failed to save pending: %s", e2)
    return False

# ---- Backoff / cooldown tracking ----
cooldown = {}  # ip -> cooldown_until_timestamp

def is_cooldown(ip):
    return ip in cooldown and time.time() < cooldown[ip]

def set_cooldown(ip, minutes=30):
    cooldown[ip] = time.time() + minutes * 60

# ---- OOM guard ----
def oom_guard(buffer, lock):
    with lock:
        now = time.time()
        stale_ips = []
        for ip in list(buffer.keys()):
            # Remove entries older than OOM_MAX_SEC
            stale = True
            for key in ("tcp_samples", "tcp_times", "tls_samples", "tls_times"):
                if key in buffer[ip] and buffer[ip][key]:
                    stale = False
                    break
            if stale:
                stale_ips.append(ip)
            # Cap list sizes
            for key in ("tcp_samples", "tcp_times", "tls_samples", "tls_times"):
                if key in buffer[ip] and len(buffer[ip][key]) > 200:
                    buffer[ip][key] = buffer[ip][key][-100:]
        for ip in stale_ips:
            del buffer[ip]
        total_ips = len(buffer)
        if total_ips > 2000:
            log.warning("OOM: buffer too large (%d IPs), discarding half", total_ips)
            keys = sorted(buffer.keys(), key=lambda k: len(buffer[k].get("tcp_samples", [])) + len(buffer[k].get("tls_samples", [])))
            for k in keys[:len(keys)//2]:
                del buffer[k]

# fetch_top_ips (legacy, used as fallback)
def fetch_top_ips():
    try:
        r = requests.get(f"{BASE_URL}/api/v1/latency/top-ips",
            headers={"Authorization": f"Bearer {LATENCY_TOKEN}"}, timeout=10)
        if r.status_code == 200:
            ips = r.json().get("ips", [])
            log.info("fetched %d top IPs from server", len(ips))
            return ips
        log.warning("fetch top-ips FAIL %d", r.status_code)
    except Exception as e:
        log.warning("fetch top-ips ERR: %s", e)
    return []

# fetch_tasks: Pull server-directed tasks
def fetch_tasks():
    try:
        r = requests.get(f"{BASE_URL}/api/v1/latency/tasks",
            headers={"Authorization": f"Bearer {LATENCY_TOKEN}"},
            params={"client_id": CLIENT_ID, "count": 10}, timeout=10)
        if r.status_code == 200:
            body = r.json()
            tasks = body.get("tasks", [])
            cfg = body.get("config", {})
            return tasks, cfg
        log.warning("fetch tasks FAIL %d", r.status_code)
    except Exception as e:
        log.warning("fetch tasks ERR: %s", e)
    return [], {}

# execute_server_tasks: Run server-directed precision tests
def execute_server_tasks(tasks, buffer, buf_lock):
    tested_ips = set()
    for task in tasks:
        ip = task.get("ip", "")
        action = task.get("action", "")
        priority = task.get("priority", 0)

        if action == "test_ip" and ip and ip not in tested_ips and not is_cooldown(ip):
            lat, tstamp = tcp_latency(ip, TCP_TIMEOUT)
            tls_ms, tls_ok = 0, False
            if TEST_SNI:
                tls_ms, tls_ok, _ = tls_latency(ip)
            with buf_lock:
                if ip not in buffer:
                    buffer[ip] = {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0}
                buffer[ip]["tcp_samples"].append(lat)
                buffer[ip]["tcp_times"].append(tstamp)
                if tls_ok:
                    buffer[ip]["tls_samples"].append(tls_ms)
                    buffer[ip]["tls_times"].append(_)
                    buffer[ip]["tls_ok"] += 1
                else:
                    buffer[ip]["tls_fail"] += 1
            tested_ips.add(ip)
            log.info("server-task test_ip: %s tcp=%d tls_ok=%r priority=%d", ip, lat, tls_ok, priority)

        elif action == "test_cidr" and ip:
            # Fetch CIDR subsample and test
            try:
                net = ipaddress.ip_network(ip, strict=False)
                sn_list = list(net.subnets(new_prefix=24))
                if sn_list:
                    sn = random.choice(sn_list)
                    hosts = list(sn.hosts())
                    if hosts:
                        test_ip = str(random.choice(hosts))
                        lat, tstamp = tcp_latency(test_ip, TCP_TIMEOUT)
                        tls_ms, tls_ok = 0, False
                        if TEST_SNI:
                            tls_ms, tls_ok, _ = tls_latency(test_ip)
                        with buf_lock:
                            if test_ip not in buffer:
                                buffer[test_ip] = {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0}
                            buffer[test_ip]["tcp_samples"].append(lat)
                            buffer[test_ip]["tcp_times"].append(tstamp)
                            if tls_ok:
                                buffer[test_ip]["tls_samples"].append(tls_ms)
                                buffer[test_ip]["tls_ok"] += 1
                        tested_ips.add(test_ip)
                        log.info("server-task test_cidr: %s/%s tcp=%d", ip, test_ip, lat)
            except Exception as e:
                log.debug("test_cidr parse error: %s", e)

        elif action == "pause":
            duration = task.get("duration", 300)
            log.info("server-task pause: %ds", duration)
            time.sleep(min(duration, 60))  # max 60s per cycle

    return tested_ips

# ---- Main Loop ----
def main():
    log.info("start v2 | client_id=%s server=%s isp=%s region=%s workers_tcp=%d workers_tls=%d tll=%d",
             CLIENT_ID, BASE_URL, CLIENT_ISP, CLIENT_REGION, TCP_WORKERS, TLS_WORKERS, TLL_MIN_MS)

    # Flush any pending reports from previous run
    flush_pending()

    # Fetch CF IP pool
    ips = sample_ips(fetch_cf_cidrs())
    buffer = defaultdict(dict)
    last_report = time.time()
    last_phase1 = 0
    phase1_top_n = []
    phase2_top_m = []
    running = True

    import threading
    buf_lock = threading.Lock()

    def shutdown(sig, frame):
        nonlocal running; running = False
        log.info("shutdown signal, draining...")
        report_buffer(buffer)

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    while running:
        cycle_start = time.time()

        try:
            # ---- Report (check BEFORE Phase 1 to avoid 600s delay) ----
            if time.time() - last_report >= REPORT_SEC:
                with buf_lock:
                    if len(buffer) > 0:
                        if report_buffer(buffer):
                            buffer.clear()
                            last_report = time.time()
                flush_pending()

            # ---- Phase 1: Full sweep (every MEASURE_SEC, sharded) ----
            if time.time() - last_phase1 >= MEASURE_SEC:
                shard_size = max(1, len(ips) // PHASE1_SHARDS)
                phase1_all_raw = defaultdict(list)
                for shard_idx in range(PHASE1_SHARDS):
                    if not running: break
                    shard_start = shard_idx * shard_size
                    shard_ips = ips[shard_start:shard_start + shard_size]
                    log.info("phase1 shard %d/%d | %d IPs", shard_idx + 1, PHASE1_SHARDS, len(shard_ips))

                    top_n, raw = phase1_tcp_sweep(shard_ips)
                    for ip, pairs in raw.items():
                        for lat, _ in pairs:
                            phase1_all_raw[ip].append(lat)

                    # Merge into buffer
                    with buf_lock:
                        for ip, pairs in raw.items():
                            if "tcp_samples" not in buffer[ip]:
                                buffer[ip] = {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0}
                            for lat, tstamp in pairs:
                                buffer[ip]["tcp_samples"].append(lat)
                                buffer[ip]["tcp_times"].append(tstamp)

                    if shard_idx < PHASE1_SHARDS - 1:
                        log.info("phase1 shard %d/%d done, waiting %ds...", shard_idx + 1, PHASE1_SHARDS, SHARD_INTERVAL)
                        for _ in range(SHARD_INTERVAL):
                            if not running: break
                            time.sleep(1)

                # Aggregate shard results: best IPs across all shards
                scored = []
                for ip, lats in phase1_all_raw.items():
                    if lats:
                        scored.append((sum(lats) / len(lats), ip))
                scored.sort(key=lambda x: x[0])
                phase1_top_n = [ip for _, ip in scored[:TCP_TOP_N]]
                last_phase1 = time.time()

                # ---- Phase 2: TLS verify Phase 1 top N ----
                if phase1_top_n and TEST_SNI:
                    phase2_top_m, tls_data = phase2_tls_verify(phase1_top_n)
                    with buf_lock:
                        for ip, data in tls_data.items():
                            if "tls_samples" not in buffer[ip]:
                                buffer[ip] = {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0}
                            buffer[ip]["tls_samples"].extend(data.get("tls_samples", []))
                            buffer[ip]["tls_times"].extend(data.get("tls_times", []))
                            buffer[ip]["tls_ok"] = buffer[ip].get("tls_ok", 0) + data.get("tls_ok", 0)
                            buffer[ip]["tls_fail"] = buffer[ip].get("tls_fail", 0) + data.get("tls_fail", 0)
                else:
                    phase2_top_m = phase1_top_n[:TLS_TOP_M]

            # ---- Server-directed tasks (every cycle) ----
            server_tasks, server_cfg = fetch_tasks()
            if server_tasks:
                execute_server_tasks(server_tasks, buffer, buf_lock)

            # ---- Phase 3: Fast re-test of top M (every FAST_INTERVAL) ----
            if phase2_top_m:
                candidates = [ip for ip in phase2_top_m if not is_cooldown(ip)]
                if candidates:
                    phase3_data = phase3_fast_test(candidates)
                    with buf_lock:
                        for ip, data in phase3_data.items():
                            if "tcp_samples" not in buffer[ip]:
                                buffer[ip] = {"tcp_samples": [], "tcp_times": [], "tls_samples": [], "tls_times": [], "tls_ok": 0, "tls_fail": 0}
                            buffer[ip]["tcp_samples"].extend(data.get("tcp_samples", []))
                            buffer[ip]["tcp_times"].extend(data.get("tcp_times", []))
                            buffer[ip]["tls_samples"].extend(data.get("tls_samples", []))
                            buffer[ip]["tls_times"].extend(data.get("tls_times", []))
                            buffer[ip]["tls_times"].extend(data.get("tls_times", []))
                            buffer[ip]["tls_ok"] = buffer[ip].get("tls_ok", 0) + data.get("tls_ok", 0)
                            buffer[ip]["tls_fail"] = buffer[ip].get("tls_fail", 0) + data.get("tls_fail", 0)

                    # Mark consecutive failures for cooldown
                    for ip, data in phase3_data.items():
                        total_tries = len(data.get("tcp_samples", []))
                        successes = sum(1 for x in data.get("tcp_samples", []) if x > 0)
                        if total_tries > 0 and successes == 0:
                            set_cooldown(ip, 30)
                            log.info("cooldown %s: all %d probes failed", ip, total_tries)

            # ---- OOM guard ----
            oom_guard(buffer, buf_lock)

        except Exception as e:
            log.error("main loop error: %s", e, exc_info=True)

        # Sleep until next cycle, respecting Phase 3 interval
        elapsed = time.time() - cycle_start
        sleep_sec = max(1, min(FAST_INTERVAL - int(elapsed), MEASURE_SEC - int(elapsed)))
        for _ in range(sleep_sec):
            if not running:
                break
            time.sleep(1)

    log.info("shutdown complete")

if __name__ == "__main__":
    main()
