# PigeonRelay 测速客户端

Cloudflare 全网 IP 延迟扫描客户端。三阶段测速（TCP → TLS → 精确复测），持续上报延迟数据给 PigeonRelay，驱动 CF DNS 优选。

Docker 镜像由 GitHub Actions 自动构建（amd64 / arm64 / armv7），推送到 `ghcr.io/gezi121/pigeonrelay-speedtest:latest`。

---

## 前置条件

- [ ] PigeonRelay 服务端已部署并能访问
- [ ] Settings 页面已设置 **Latency Token**（`LATENCY_TOKEN`）
- [ ] 部署机器能访问 `api.cloudflare.com`（获取 CF IP 段）
- [ ] 部署机器能访问 PigeonRelay 的端口（默认 3214）

---

## 一键部署

```bash
docker rm -f pigeonrelay-speedtest 2>/dev/null
docker run -d \
  --name pigeonrelay-speedtest \
  --restart unless-stopped \
  -e PIGEONRELAY_URL="http://<VPS_IP>:3214" \
  -e LATENCY_TOKEN="<Settings页面中的Token>" \
  -e CLIENT_ISP="移动" \
  -e CLIENT_REGION="华南" \
  -e TCP_WORKERS="50" \
  -e PYTHONUNBUFFERED=1 \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest
```

> 把 `<VPS_IP>` 换成你的 PigeonRelay 地址，`<Settings页面中的Token>` 换成 Settings 页的 Latency Token。

---

## Docker Compose 部署

```yaml
# docker-compose.yml
services:
  speedtest:
    image: ghcr.io/gezi121/pigeonrelay-speedtest:latest
    container_name: pigeonrelay-speedtest
    restart: unless-stopped
    environment:
      - PIGEONRELAY_URL=http://<VPS_IP>:3214
      - LATENCY_TOKEN=<your-token>
      - CLIENT_ISP=移动
      - CLIENT_REGION=华南
      - TCP_WORKERS=50
      - TLS_WORKERS=10
      - PYTHONUNBUFFERED=1
    volumes:
      - ./data:/app/data
```

```bash
docker compose up -d
```

---

## 测速原理

客户端采用三阶段设计，兼顾覆盖面与精确度：

```
Phase 1 — TCP Sweep           Phase 2 — TLS Verify        Phase 3 — Precision
─────────────────────         ────────────────────        ──────────────────
从 CF API 拉取 IPv4 CIDR         Phase 1 Top 200            Phase 2 Top 50
每 /24 子网随机取 1 IP           低并发 TLS 握手             每 120s 精确复测
~6000 IP, 50 并发 × 3 次握手    淘汰 TLS 失败率 >50%         OOM guard 上限 2000 IP
4 分片轮转 (每 45s 一片)          选出 Top 50 进入 Phase 3     30min 冷却期
```

### Phase 1 — TCP Sweep

从 `api.cloudflare.com/client/v4/ips` 获取 CF 公布的 IPv4 CIDR 段，每 /24 子网随机抽样一个 IP。约 6000 候选 IP，50 并发 TCP :443 握手，每个 IP 测 3 次取中位数。

4 个分片轮流（每 45s 一片），避免瞬间高并发触发运营商 QoS。取延迟最低的 Top 200 进入 Phase 2。

### Phase 2 — TLS Verify

低并发（默认 10 worker）对 Phase 1 的 Top 200 做完整 TLS 握手。记录 TLS 成功/失败。TLS 失败率超过 50% 的 IP 直接淘汰。Top 50 进入 Phase 3。

> TLS 验证很重要：有些 IP TCP 延迟极低但 TLS 握手卡住（CF 边缘拒绝非 HTTP 的 TLS），不验证会导致 mihomo timeout。

### Phase 3 — Precision Re-test

每 120 秒对 Phase 2 Top 50 做 TCP + TLS 精确复测。OOM guard 限制最多追踪 2000 个 IP。30 分钟冷却期防止重复轰炸已失败的 IP。

### 数据上报

每 20 分钟（可配置）合并上报一次延迟数据到 PigeonRelay。单次最多 400 条采样（防止请求过大）。失败自动缓存重试。

服务端收到后用 **EWMA (α=0.3, 半衰期 ~23min)** 平滑延迟，避免瞬时抖动干扰。

---

## 完整环境变量

### 必填

| 变量 | 说明 | 示例 |
|------|------|------|
| `PIGEONRELAY_URL` | PigeonRelay 地址 | `http://1.2.3.4:3214` |
| `LATENCY_TOKEN` | Settings 页面的 Latency Token | 与 Settings 完全一致 |

### 运营商标识

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLIENT_ISP` | `unknown` | 运营商标签。如 `移动` / `电信` / `联通` / `overseas` |
| `CLIENT_REGION` | `unknown` | 地区标签。如 `华南` / `华东` / `华北` / `西南` |

> ISP 和 Region 标签用于 PigeonRelay 的 **自定义算法** 功能。你可以在 Algorithms 页面创建按运营商筛选 IP 的算法。

### 并发控制

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TCP_WORKERS` | `50` | Phase 1 TCP 并发线程数。家庭宽带建议 50，VPS 可到 150 |
| `TLS_WORKERS` | `10` | Phase 2 TLS 并发数。过高可能触发 CF 限速 |
| `TCP_TIMEOUT` | `2` | TCP 拨号超时（秒）。网络差改 3 |
| `TCP_TRIES` | `3` | 每 IP 测量次数。3 次取中位数 |
| `TCP_TOP_N` | `200` | Phase 1 进入 Phase 2 的 IP 数量 |
| `TLS_TIMEOUT` | `3` | TLS 握手超时（秒） |
| `TLS_TOP_M` | `50` | Phase 2 进入 Phase 3 的 IP 数量 |
| `FAST_INTERVAL` | `120` | Phase 3 精确复测间隔（秒） |
| `FAST_TRIES` | `1` | Phase 3 每次测量次数 |
| `FAST_TIMEOUT` | `1.5` | Phase 3 超时（秒） |
| `FAST_WORKERS` | `10` | Phase 3 并发数 |

### 分片与间隔

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PHASE1_SHARDS` | `4` | Phase 1 分片数。分片越多，每片 IP 越少，对网络冲击越小 |
| `SHARD_INTERVAL` | `45` | 分片间隔（秒）。间隔越久，测速压力越低，但一轮更慢 |
| `MEASURE_INTERVAL` | `600` | Phase 1 全量扫描间隔（秒）。600 = 10 分钟 |
| `REPORT_INTERVAL` | `1200` | 数据上报间隔（秒）。1200 = 20 分钟 |

### 过滤与保护

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `TLL_MIN_MS` | `0` | 低延迟过滤门槛（ms）。国内移动设 0，电信/联通建议设 40（过滤透明代理劫持） |
| `MIN_VALID_MS` | `15` | 有效延迟下限（ms）。低于此值的测量结果丢弃 |
| `TEST_PORT` | `443` | 测试端口。一般不改 |
| `TEST_SNI` | 空 | TLS 测试的 SNI。空则用 IP |
| `OOM_MAX_SEC` | `7200` | 内存中 IP 数据最大保留时间（秒） |
| `MAX_IPS_PER_REPORT` | `400` | 单次上报最大 IP 数，防请求过大 |

### 基础

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PYTHONUNBUFFERED` | — | 设为 `1` 让 Docker 日志实时输出 |
| `DATA_DIR` | `data` | 数据持久化目录 |

---

## 多运营商部署

不同运营商到同一 CF IP 的延迟差异很大。建议各运营商至少部署一台：

```bash
# 电信 (设 TLL_MIN_MS=40 过滤透明代理)
docker rm -f pr-speedtest-telecom 2>/dev/null
docker run -d --name pr-speedtest-telecom --restart unless-stopped \
  -e PIGEONRELAY_URL="http://<VPS_IP>:3214" \
  -e LATENCY_TOKEN="xxx" \
  -e CLIENT_ISP="电信" \
  -e CLIENT_REGION="华东" \
  -e TLL_MIN_MS="40" \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest

# 移动 (TLL_MIN_MS=0, 无透明代理)
docker rm -f pr-speedtest-mobile 2>/dev/null
docker run -d --name pr-speedtest-mobile --restart unless-stopped \
  -e PIGEONRELAY_URL="http://<VPS_IP>:3214" \
  -e LATENCY_TOKEN="xxx" \
  -e CLIENT_ISP="移动" \
  -e CLIENT_REGION="华南" \
  -e TLL_MIN_MS="0" \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest
```

部署后在 PigeonRelay → **Algorithms** 创建按 `client_id` 筛选的算法，不同运营商使用不同的优选策略。

---

## 查看日志

```bash
# Docker 日志（实时）
docker logs -f pigeonrelay-speedtest

# 正常输出示例：
# [Phase1] Loaded 5832 candidate IPs from 217 /24 subnets
# [Phase1] Shard 1/4: 1458 IPs, 3 tries/IP
# [Phase1] Shard 1 complete: 187 passed threshold
# [Phase2] TLS verify on 200 IPs...
# [Phase2] 173/200 passed TLS verification
# [Phase3] Precision retest on 50 IPs
# [Report] Buffered 235 IPs, sending report...
# [Report] Server responded: OK (235 samples accepted)
```

## 常见问题

<details>
<summary><strong>客户端启动了但测速客户端页面不显示？</strong></summary>

客户端首次上报数据后才会注册。等待一个上报周期（默认 20 分钟），或检查日志确认上报返回了 HTTP 200。
</details>

<details>
<summary><strong>日志显示 "HTTP 401 / invalid token"？</strong></summary>

`LATENCY_TOKEN` 与 PigeonRelay Settings 页面的 Token 不一致。复制 Settings 中的 Token，更新 `-e LATENCY_TOKEN=xxx`，重启容器。
</details>

<details>
<summary><strong>Phase 1 扫描了但数据很少？</strong></summary>

`MIN_VALID_MS=15` 过滤掉了延迟极低的虚假数据。如果你的网络确实延迟很低，降低这个值到 5。

`TLL_MIN_MS` 也会过滤。国内移动宽带一般设 0，电信/联通设 40（过滤透明代理劫持）。
</details>

<details>
<summary><strong>Phase 2 TLS 失败率很高？</strong></summary>

正常。很多 CF IP 不支持 TLS 握手（边缘节点只做 HTTP 转发）。TLS 失败的 IP 会被自动淘汰。50%+ 失败率是预期行为。
</details>

<details>
<summary><strong>能不能在路由器/NAS 上跑？</strong></summary>

能。Docker 兼容即可。将 `TCP_WORKERS` 降到 30，`PHASE1_SHARDS` 调大（如 6-8 片），减少对家用网络的冲击。

树莓派 ARM 架构直接使用同一个镜像（`ghcr.io/gezi121/pigeonrelay-speedtest:latest`），CI 会自动构建 arm64/armv7 版本。
</details>

<details>
<summary><strong>一台客户端够用吗？</strong></summary>

一台可工作，但多运营商多地区部署效果更好。不同运营商的优选结果可能差异很大。建议至少在国内主要宽带下各部署一台。
</details>

---

## 开发者部署（不使用 Docker）

```bash
pip install requests
export PIGEONRELAY_URL=http://1.2.3.4:3214
export LATENCY_TOKEN=your-token
python3 client.py
```
