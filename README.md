<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs" alt="Vue">
  <img src="https://img.shields.io/badge/Docker-blue?logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/github/actions/workflow/status/Gezi121/pigeonrelay/ci.yml?label=CI" alt="CI">
</p>

<h1 align="center">PigeonRelay</h1>
<p align="center"><strong>鸽阅</strong> · 自建代理订阅管理面板</p>
<p align="center">
  <em>聚合节点 &nbsp;·&nbsp; CF 优选 DNS 自动更新 &nbsp;·&nbsp; 智能延迟采集 &nbsp;·&nbsp; 多格式订阅生成</em>
</p>

---

## 这是什么？

PigeonRelay 是一个**代理订阅管理后台**。你通过 3x-ui 创建 VLESS/Reality/XHTTP/Hysteria2 节点后，把节点链接导入 PigeonRelay，它会：

1. 自动绑定 Cloudflare 优选域名，一条节点生成多条 CDN 加速代理
2. 生成 Clash / Mihomo / V2Ray 格式的订阅链接
3. 驱动测速客户端全网扫描 CF IP、收集延迟数据
4. 自动更新 CF DNS A 记录到最低延迟 IP
5. 提供流量统计、用户管理、备份恢复等商业化能力

**它不代理流量。** 流量始终走你的 Xray 服务器。它只管节点元数据和订阅配置。

---

## 快速开始

```bash
# VPS 上执行
curl -sSL https://raw.githubusercontent.com/Gezi121/pigeonrelay/main/deploy.sh | bash
```

访问 `http://<VPS_IP>:3214`，默认 `admin` / `admin123`。

> 部署后第一件事：Settings → 修改管理员密码 + 设置 Latency Token。

---

## 从零到壹：完整搭建流程

### 第一步：准备基础设施

你需要：
- 一台境外 VPS（推荐 Ubuntu 22.04+）
- 一个托管在 Cloudflare 的域名
- [3x-ui 面板](https://github.com/mhsanaei/3x-ui) 已安装

```bash
# 如果还没装 3x-ui
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

### 第二步：在 3x-ui 创建代理节点

**直连节点 (Reality VLESS)：**
```
协议: VLESS    端口: 443      传输: tcp
安全: reality  目标: www.apple.com:443
flow: xtls-rprx-vision
→ 导出 share link，记下 publicKey 和 shortId
```

**CDN 节点 (XHTTP+TLS)：**
```
协议: VLESS    端口: 8006     传输: xhttp
安全: tls      证书: 域名证书
路径: /xhttp-cdn
→ 导出 share link
```

> CDN 节点需要域名在 Cloudflare 开启橙色云朵代理，SSL/TLS 设为 **完全(严格)**。

**Hysteria2 节点 (可选)：**
```
协议: hysteria2    端口: 3011
→ 导出 share link
```

### 第三步：部署 PigeonRelay

```bash
curl -sSL https://raw.githubusercontent.com/Gezi121/pigeonrelay/main/deploy.sh | bash
```

或手动：

```yaml
# docker-compose.yml
services:
  pigeonrelay:
    image: ghcr.io/gezi121/pigeonrelay:latest
    network_mode: host
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - PORT=3214
      - JWT_SECRET=your-64-char-random-string
      - ADMIN_USER=admin
      - ADMIN_PASS=your-secure-password
      - LATENCY_TOKEN=your-latency-token
    restart: always
```

```bash
docker compose up -d
```

### 第四步：导入节点

1. 在 3x-ui 每个节点上点 **分享** → 复制 VLESS 链接
2. PigeonRelay → **节点管理** → **批量导入** → 粘贴链接 → 解析 → 导入
3. 导入后可修改名称、地址、SNI、传输方式等字段

### 第五步：创建订阅

1. **Sub Links** → 新建 → 填写名称
2. 可选：设置密码、流量上限 (GB)、到期日期
3. 复制链接：`http://<VPS_IP>:3214/sub/<hash>`
4. 粘贴到 Mihomo / Clash Verge / Shadowrocket

### 第六步：(可选) 部署测速客户端

```bash
docker run -d --name speedtest --network host --restart unless-stopped \
  -e PIGEONRELAY_URL=http://<VPS_IP>:3214 \
  -e LATENCY_TOKEN=your-latency-token \
  -e CLIENT_ISP=移动 \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest
```

客户端开始扫描全网 Cloudflare IP（约 6000 个），每 5 分钟一轮，上报延迟数据。数据积累后开启 CF DNS 自动优选。

---

## Cloudflare 优选配置

> 完整从零教程见 [docs/local-opt-guide.md](docs/local-opt-guide.md)

### CF API Token

1. [Cloudflare Dashboard](https://dash.cloudflare.com) → My Profile → API Tokens
2. Create Token → 用 `Edit zone DNS` 模板
3. Zone Resources: `Include` → 你的域名
4. 创建后复制 Token → PigeonRelay Settings 页面填入

Token 需要 `Zone.DNS` Edit 权限。Zone ID 可留空，系统自动从 Token 权限发现。

### CF 优选绑定

一个节点可以绑定**多个** CF 优选域名，每条绑定在订阅中生成独立的代理配置：

1. 节点管理 → hover 节点 → **设置** → CF 优选绑定
2. 选择来源 (本地/外部) → 选择域名 → 填写名称前缀
3. 可用 **批量添加** 一次性粘贴多个域名
4. 可用 **复制/粘贴** 在节点间迁移绑定配置

**名称前缀示例：**
```
前缀 = "三方优选1"
→ 订阅中节点名 = "VLESS-美西CDN-三方优选1"
```

### 本地优选 (DNS 自动更新)

配置 CF Token 后，系统每 N 分钟自动将子域名 A 记录指向最低延迟 CF IP：

1. **本地优选** 页面 → 添加域名映射
2. 子域名 (如 `a.xls`) + 后缀 (如 `.gugugezi.com`) → 得到 `a.xls.gugugezi.com`
3. 选择优选算法
4. 系统自动展示当前最优 IP，并自动更新 CF DNS

### 五种优选算法

| 算法 | 原理 | 推荐场景 |
|------|------|---------|
| **极速** | 纯延迟最低 | 游戏、即时通讯 |
| **稳定** | 低方差 + 延迟平衡 | 大文件下载 |
| **潮汐** | 历史高峰数据权重 60% | 晚高峰主力 |
| **综合** | 多因子加权 | 默认通用 |
| **低抖动** | 注重延迟一致性 | VoIP、直播 |

全部算法硬门槛：成功率 ≥ 60%。详见 [docs/algorithms.md](docs/algorithms.md)。

---

## Clash 模板定制

Settings 页面可编辑 Clash 订阅模板，使用三个变量：

| 变量 | 替换为 |
|------|--------|
| `${proxies}` | 所有代理节点 YAML |
| `${proxy-names}` | 节点名列表（带缩进，放在 proxy-groups 的 proxies 下） |
| `${server-direct-rules}` | 自动生成的 `DOMAIN-SUFFIX,server,DIRECT` 规则 |

修改后保存即生效，无需重启。

---

## 订阅格式

Sub Links 页面每个链接提供两个导出按钮：

| 按钮 | 格式 | 适用客户端 |
|------|------|-----------|
| **Clash** | YAML (Clash Meta 格式) | Mihomo / Clash Verge / Clash Meta |
| **V2Ray** | Base64 (VLESS/VMess 分享链接) | v2rayN / Shadowrocket / V2Box |

Clash 链接：`http://host:3214/sub/:hash`  
V2Ray 链接：`http://host:3214/sub/:hash?format=v2ray`  
带密码保护：`http://host:3214/sub/:hash?token=<password>`

禁用某节点后（节点管理 → 点击 👁/⊘），该节点不再出现在订阅中。

---

## 测速客户端

### 原理

三阶段设计，兼顾覆盖面和精确度：

```
Phase 1 ─ TCP Sweep        Phase 2 ─ TLS Verify       Phase 3 ─ Precision
─────────────────────      ────────────────────       ──────────────────
全网 ~6000 CF IP             Top 200 IP                 Top 50 IP
50 并发 TCP 握手            10 并发 TLS 握手           每 120s 精确复测
每 IP 3 次，4 分片轮转      淘汰 TLS 失败 >50%         30min 冷却期
```

服务端用 EWMA (α=0.3, 半衰期 ~23min) 平滑延迟，避免瞬时抖动干扰。

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `PIGEONRELAY_URL` | 是 | PigeonRelay 地址，如 `http://1.2.3.4:3214` |
| `LATENCY_TOKEN` | 是 | 与 Settings 页面中的 Token 一致 |
| `CLIENT_ISP` | 否 | 运营商标签：移动/电信/联通 |
| `CLIENT_REGION` | 否 | 地区标签：华南/华东/华北 |
| `TCP_WORKERS` | 否 | TCP 并发数 (默认 50) |
| `TLS_WORKERS` | 否 | TLS 并发数 (默认 10) |

---

## 节点管理详解

### 排序

节点在订阅中的先后顺序由 **排序** 决定。越靠上的节点在 Mihomo 的 `proxy-groups` 中排在越前面，url-test 策略组也会优先选择排名靠前的节点。

**操作：** 节点管理 → hover 节点卡片 → 点击 **↑** 或 **↓** 按钮。

每点击一次，当前节点与上/下一个节点交换位置。排序实时保存，订阅即时生效。

默认排序规则：直连 → XHTTP CDN → WS CDN → UpCDN → DownCDN。手动调整后以手动顺序为准。

### 启用/禁用

禁用的节点**不会出现在订阅中**，但保留所有配置（CF 绑定、名称、参数等）。

**操作：** 节点管理 → hover 节点卡片 → 点击 **👁/⊘** 按钮。

| 图标 | 状态 | 订阅中 |
|------|------|--------|
| 👁 | 已启用 | **出现** |
| ⊘ (红色) | 已禁用 | **不出现** |

> 用途：临时下线某个节点（如端口被封、证书过期），修好后一键恢复，不用重新配置。

### CF 优选绑定

每个节点可以绑定多个优选域名。订阅生成时，一条节点 + N 个绑定 = N 条代理配置。

**操作：** 节点管理 → hover → **设置** → CF 优选绑定区：

- **单个添加**：选来源 → 选域名 → 填前缀 → 保存
- **批量添加**：点击"批量添加" → 粘贴域名（一行一个）→ 确认
- **复制/粘贴**：点击"复制" → 打开另一个节点的设置 → 点"粘贴"
- **多选批量**：勾选多个节点 → 顶部"批量添加优选" → 一键添加到所有选中节点

### 名称与符号

节点名称和前缀中的 **emoji 完全由你控制**。代码不做任何硬编码：

- 节点名称：直接在节点管理页面的名称字段编辑（如 `⚡VLESS-Reality-美西直连`）
- 前缀符号：在 CF 优选绑定的 `name_prefix` 字段编辑（如 `🏠本优移动-极速`）

---

## 订阅管理

### 多用户与配额

PigeonRelay 支持多用户。管理员可以在 **Admin** 页面管理：

| 功能 | 说明 |
|------|------|
| 创建用户 | 在 Admin 页面分配用户名/密码 |
| 流量配额 | 编辑 `monthly_quota_bytes`（字节），超限后订阅返回空 |
| 到期控制 | 编辑 `expire_at`，到期后订阅返回过期中止 |
| 流量清零 | 每月 1 日自动清零，或手动 Reset Traffic |

### 备份与恢复

**创建备份：** Backups 页面 → 创建备份 → 输入名称 → 保存。

备份包含所有节点、绑定、域名映射、设置。数据库文件本身也在 volume 中持久化。

**恢复：** Backups 页面 → 点击备份条目旁的"恢复"按钮。

---

## 通知

设置 `NOTIFICATION_WEBHOOK` 环境变量（或 config.yaml），系统自动发送：

- 用户账号即将到期（≤7 天）
- 流量使用超过 80% 阈值
- 订阅源抓取失败（Legacy 功能）

支持钉钉、飞书、企业微信等兼容 Webhook 的平台。消息格式：`{"text": "告警内容"}`。

---

## API 速查

### 公开（无认证）
| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/auth/login` | 登录获取 JWT |
| `POST` | `/api/v1/latency/report` | 测速上报 (需 LATENCY_TOKEN) |
| `GET` | `/api/v1/latency/tasks` | 获取测速任务 (需 LATENCY_TOKEN) |
| `GET` | `/sub/:hash` | 订阅输出 |

### 需 JWT Bearer
| 方法 | 路径 | 说明 |
|------|------|------|
| `GET/POST` | `/api/nodes` | 节点列表/创建 |
| `PUT/DELETE` | `/api/nodes/:id` | 节点编辑/删除 |
| `POST` | `/api/nodes/parse` | 批量解析链接 |
| `POST` | `/api/nodes/reorder` | 节点排序 |
| `POST` | `/api/nodes/:id/test` | 连通性测试 |
| `GET/POST/DELETE` | `/api/nodes/:id/cf-bindings` | CF 优选绑定 |
| `GET/POST/PUT/DELETE` | `/api/domain-mappings` | 本地优选域名 |
| `POST` | `/api/domain-mappings/batch` | 批量创建域名映射 |
| `GET/POST/PUT/DELETE` | `/api/sub-links` | 订阅短链管理 |
| `GET/PUT` | `/api/settings` | 系统设置 |
| `GET` | `/api/traffic/overview` | 当月流量概览 |
| `GET` | `/api/latency?hours=24` | 延迟时序数据 |
| `GET` | `/api/latency/daily-picks` | 每日精选 IP |
| `GET` | `/api/latency/best-detail?domain=X` | 域名最优 IP 详情 |
| `GET/PUT/DELETE` | `/api/speed-clients` | 测速客户端管理 |
| `GET/POST/PUT/DELETE` | `/api/algorithms` | 自定义算法 |
| `GET/POST` | `/api/backups` | 备份列表/创建 |
| `POST` | `/api/backups/:id/restore` | 恢复备份 |

### 管理员
| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/admin/users` | 用户列表 |
| `PUT` | `/api/admin/users/:id` | 编辑用户 |
| `DELETE` | `/api/admin/users/:id` | 删除用户 |
| `POST` | `/api/admin/users/:id/reset-traffic` | 流量清零 |

---

## 项目结构

```
pigeonrelay/
├── backend/
│   ├── cmd/server/main.go           # 入口
│   └── internal/
│       ├── handler/                  # HTTP 处理器 (auth/node/domain/latency/admin)
│       ├── repository/               # SQL 查询 + EWMA 算法
│       ├── service/
│       │   ├── cfdns/                # Cloudflare DNS API
│       │   ├── subscription.go       # 订阅生成 (Clash/V2Ray)
│       │   └── scheduler.go          # 定时任务
│       └── database/                 # SQLite WAL + 表迁移
├── frontend/
│   └── src/
│       ├── views/                    # 13 个页面 (Nodes/Settings/Admin/...)
│       ├── components/ui/            # Vue 组件库
│       └── api.js                    # API 层
├── speedtest-client/                 # Python 测速客户端
├── docs/                             # 设计文档
├── deploy.sh                         # 一键部署脚本
├── Dockerfile                        # 多阶段构建
└── .github/workflows/ci.yml          # CI/CD
```

---

## FAQ

<details>
<summary><strong>直连节点全部 timeout？</strong></summary>
国内网络对 VLESS+Reality 存在 DPI 干扰，直连 VPS IP 的 Reality 握手会被阻断。改用 CDN 节点 (XHTTP+TLS+Cloudflare) 即可。
</details>

<details>
<summary><strong>订阅拉到某个节点但 mihomo 里用不了？</strong></summary>
检查该节点是否被禁用（节点管理页 👁/⊘ 图标）。禁用后不输出到订阅。也可能是节点的 UUID 与 Xray 入站不一致。
</details>

<details>
<summary><strong>CF 优选 DNS 不更新？</strong></summary>
① 确认 Token 有 Zone.DNS Edit 权限 ② 确认 zone_id 正确或已自动发现 ③ 测速客户端正常上报数据（Speed Clients 页面检查今日上报数）④ DNS 记录须为灰云 (DNS only)。
</details>

<details>
<summary><strong>测速客户端不上报？</strong></summary>
① LATENCY_TOKEN 与 Settings 页面一致 ② VPS 防火墙允许 3214 端口 ③ `docker logs` 查看客户端日志。
</details>

<details>
<summary><strong>忘记管理员密码？</strong></summary>

```bash
docker exec -it pigeonrelay-pigeonrelay-1 sh -c \
  "sqlite3 /app/data/pigeonrelay.db \"DELETE FROM users WHERE username='admin';\""
docker restart pigeonrelay-pigeonrelay-1
# 重新用 admin / admin123 登录，系统自动重建账户
```
</details>

<details>
<summary><strong>如何迁移到新 VPS？</strong></summary>
复制 `~/pigeonrelay/data/pigeonrelay.db` 到新 VPS 相同路径，执行 deploy.sh 即可。所有数据（节点、设置、延迟数据）随数据库文件迁移。
</details>

<details>
<summary><strong>如何选择优选算法？</strong></summary>
游戏/通话选 **极速**，下载选 **稳定**，晚高峰主力选 **潮汐**，不确定选 **综合**，VoIP/直播选 **低抖动**。
</details>

<details>
<summary><strong>订阅代理太多想精简？</strong></summary>
① 禁用不需要的节点 ② CF 优选绑定中删除多余的域名 ③ 两种方式减少后订阅中不再生成对应代理。
</details>

---

## 更多文档

- [docs/local-opt-guide.md](docs/local-opt-guide.md) — 本地优选从零配置完整教程
- [docs/algorithms.md](docs/algorithms.md) — IP 优选算法原理与公式
- [docs/speedtest-refactor-design.md](docs/speedtest-refactor-design.md) — 测速系统重构设计

---

## 友链

- [LINUX DO](https://linux.do/) — 深度技术交流社区

---

## License

允许自由使用、修改和升级本项目。如需公开发布修改后的版本，须在显著位置注明原始项目地址 [github.com/Gezi121/pigeonrelay](https://github.com/Gezi121/pigeonrelay)。

本软件仅供个人非商业用途。未经授权不得用于任何商业行为（包括但不限于销售代理服务、提供付费订阅、集成到商业化产品中）。

&copy; 2025 Gezi. All rights reserved.
