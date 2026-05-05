# 本地优选完整教程

> 从零配置 Cloudflare 优选 DNS 自动更新，让订阅中 CDN 节点的域名始终解析到最低延迟的 CF Edge IP。

---

## 目录

1. [这是什么？](#这是什么)
2. [准备工作](#准备工作)
3. [获取 Cloudflare API Token](#第一步获取-cloudflare-api-token)
4. [PigeonRelay 中配置 CF Token](#第二步在-pigeonrelay-中配置-cf-token)
5. [部署测速客户端](#第三步部署测速客户端)
6. [添加优选域名映射](#第四步添加优选域名映射)
7. [Cloudflare DNS 中创建子域名](#第五步在-cloudflare-dns-中创建子域名)
8. [绑定优选域名到节点](#第六步绑定优选域名到节点)
9. [验证效果](#第七步验证效果)
10. [优选算法详解](#附录优选算法详解)
11. [常见问题](#常见问题)

---

## 这是什么？

本地优选是一套**自动化的 CF IP 优选系统**。它的工作流程是：

```
1. 测速客户端持续扫描全网 Cloudflare Edge IP 的 TCP/TLS 延迟
2. 数据上报到 PigeonRelay，服务端用 EWMA 算法平滑处理
3. 五种优选算法从海量 IP 中选出最优的一个
4. PigeonRelay 通过 CF API 自动更新子域名的 A 记录指向最优 IP
5. 你的订阅中节点域名解析到最优 IP → 代理延迟大幅降低
```

**为什么需要这个？** CF CDN 默认分配的 Edge IP 不一定是最快的。通过全网扫描 + 智能选择，可以把延迟从 200ms+ 降到 50ms 以下。

**需要的组件：**

| 组件 | 作用 | 部署位置 |
|------|------|---------|
| Cloudflare API Token | 允许 PigeonRelay 修改 DNS 记录 | CF Dashboard 申请 |
| 测速客户端 | 扫描全网 CF IP 延迟 | 任意一台能访问 CF 的机器 |
| 子域名 (DNS A 记录) | 承载最优 IP 的域名 | CF DNS 面板创建 |
| 域名映射 (PigeonRelay) | 关联子域名 → 优选算法 | PigeonRelay 本地优选页面 |
| CF 优选绑定 (PigeonRelay) | 节点 → 优选域名关联 | PigeonRelay 节点管理页面 |

---

## 准备工作

开始之前请确认：

- [ ] VPS 上 PigeonRelay 已部署并能正常访问
- [ ] 域名托管在 Cloudflare（DNS 管理在 CF）
- [ ] 3x-ui 或 Xray 已配置好 CDN 节点（XHTTP+TLS+CDN 或 WS+TLS+CDN）
- [ ] CDN 节点在 PigeonRelay 中已导入并可连通

---

## 第一步：获取 Cloudflare API Token

### 1.1 打开 API Token 页面

1. 浏览器打开 [dash.cloudflare.com](https://dash.cloudflare.com)
2. 登录后，点击右上角**头像** → **My Profile**
3. 左侧菜单选择 **API Tokens**
4. 点击蓝色 **Create Token** 按钮

### 1.2 选择权限模板

你会看到几个预设模板。选择 **Edit zone DNS**（使用此模板），或者点击 **Create Custom Token** 手动配置：

**自定义 Token 权限配置：**

| 设置项 | 值 |
|--------|-----|
| Token name | `PigeonRelay DNS Updater`（任意名称） |
| Permissions | `Zone` — `DNS` — `Edit` |
| Zone Resources | `Include` — `Specific zone` — `你的域名`（如 `gugugezi.com`） |
| Client IP Address Filtering | 不填（允许所有 IP） |
| TTL | 不填（永不过期）或按需设置 |

> 权限只需要 `Zone.DNS` Edit，不要给更多权限。最小权限原则避免 Token 泄露后影响其他 CF 功能。

### 1.3 复制 Token

点击 **Continue to summary** → **Create Token**。

> 创建成功后**立即复制 Token**！离开页面后 CF 不会再次显示完整 Token。
> Token 格式类似：`cfut_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

---

## 第二步：在 PigeonRelay 中配置 CF Token

### 2.1 打开 Settings 页面

PigeonRelay 左侧菜单 → **Settings**（设置）。

### 2.2 Cloudflare 配置区

页面中部找到 **Cloudflare** 区域，有三个字段：

| 字段 | 说明 | 示例 |
|------|------|------|
| **CF API Token** | 粘贴第一步复制的 Token | `cfut_xxxxxxxxxxxx...` |
| **CF Zone ID** | CF 中域名的 Zone ID | 可留空，系统自动发现 |
| **CF 更新间隔** | 多久更新一次 DNS（分钟） | 建议 30，最小 5 |

> **Zone ID 如何找？** CF Dashboard → 选择域名 → 右侧 Overview 页面 → API 区域 → Zone ID。如果留空，系统通过 Token 权限自动发现。

### 2.3 验证 Token

点击 **验证令牌** 按钮。如果显示 **"令牌有效"**（绿色），说明配置正确。

然后点击页面底部的 **保存设置**。

> 如果验证失败，检查：① Token 权限是否为 Zone.DNS Edit ② Zone Resources 是否选了正确的域名 ③ Token 是否完整复制无多余空格。

---

## 第三步：部署测速客户端

测速客户端（speedtest-client）负责持续扫描 Cloudflare IP 并上报延迟数据。**没有它，优选不会工作。**

### 3.1 两台机器部署（推荐）

建议在**不同运营商、不同地区**各部署一台测速客户端。例如：
- 一台在国内移动宽带下（如家庭 NAS/树莓派）
- 一台在 VPS 上（云主机）

多台客户端提供不同视角的延迟数据，优选算法能选出对各运营商都快的 IP。

### 3.2 Docker 部署

```bash
docker run -d \
  --name pigeonrelay-speedtest \
  --network host \
  --restart unless-stopped \
  -e PIGEONRELAY_URL=http://<VPS_IP>:3214 \
  -e LATENCY_TOKEN=<Settings页面中的Token> \
  -e CLIENT_ISP=移动 \
  -e CLIENT_REGION=华南 \
  ghcr.io/gezi121/pigeonrelay-speedtest:latest
```

### 3.3 环境变量详解

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `PIGEONRELAY_URL` | **是** | — | PigeonRelay 地址。VPS 上部署用 `http://127.0.0.1:3214`，其他机器用 `http://<VPS公网IP>:3214` |
| `LATENCY_TOKEN` | **是** | — | 与 Settings 页面的 **延迟上报 Token** 完全一致 |
| `CLIENT_ISP` | 否 | — | 运营商标签，用于按 ISP 筛选最优 IP。如 `移动` / `电信` / `联通` |
| `CLIENT_REGION` | 否 | — | 地区标签，如 `华南` / `华东` / `华北` / `西南` |
| `TCP_WORKERS` | 否 | 50 | TCP 并发连接数。宽带延迟高可降低到 30，VPS 可提高到 100 |
| `TLS_WORKERS` | 否 | 10 | TLS 并发连接数。过低 TLS 验证慢，过高可能触发 CF 限速 |
| `TCP_TIMEOUT` | 否 | 2 | TCP 连接超时秒数 |
| `MIN_VALID_MS` | 否 | 5 | 有效延迟下限。低于此值视为透明代理劫持，自动丢弃 |
| `PYTHONUNBUFFERED` | 否 | — | 设为 `1` 让 Docker 日志实时输出 |

### 3.4 验证客户端正常运行

```bash
# 查看实时日志
docker logs -f pigeonrelay-speedtest

# 正常输出示例：
# [Phase1] Loaded 5832 candidate IPs from 217 subnets
# [Phase1] Sweep complete: 200 IPs passed threshold
# [Phase2] TLS verify: 48/200 passed, 12 failed
# [Report] Sent 120 samples, server responded: OK
```

**在 PigeonRelay 中确认：** 左侧菜单 → **测速客户端**，应看到你的客户端已注册，显示名称、最后活跃时间和今日上报数。

### 3.5 关于三阶段测速原理

```
Phase 1 — TCP Sweep           Phase 2 — TLS Verify        Phase 3 — Precision
─────────────────────         ────────────────────        ──────────────────
从 CF 公布 IP 段采样            Phase 1 Top 200             Phase 2 Top 50
~6000 个 /24 子网各取 1 IP      低并发 TLS 握手              每 120s 精确复测
50 并发 × 3 次握手              淘汰 TLS 失败率 >50%          OOM guard: 最多 2000 IP
4 个分片轮流 (每 45s 一片)       选出 Top 50 进入 Phase 3     30min 冷却期防重复轰炸
```

服务端使用 **EWMA（指数加权移动平均）** 平滑延迟，α=0.3，半衰期约 23 分钟。避免单次网络抖动影响 IP 选择。

---

## 第四步：添加优选域名映射

测速客户端积累数据后（一般 10-30 分钟），开始创建域名到算法的映射。

### 4.1 理解域名映射

**域名映射** (domain_mapping) 定义了一个子域名应该用什么算法选 IP。

例如你创建了映射：
```
子域名: a.xls → 完整域名: a.xls.gugugezi.com → 算法: fast (极速)
```

系统会：
1. 用 `fast` 算法从 `ip_stats` 表中算出最优 CF IP
2. 通过 CF API 把 `a.xls.gugugezi.com` 的 A 记录更新为该 IP
3. 在本地优选页面展示当前 IP 和各项指标

### 4.2 规划子域名

建议按算法用途规划子域名：

| 子域名 | 完整域名 | 算法 | 用途 |
|--------|---------|------|------|
| `a.xls` | `a.xls.gugugezi.com` | 极速 (fast) | 游戏、低延迟 |
| `a1.xls` | `a1.xls.gugugezi.com` | 稳定 (steady) | 下载、视频 |
| `a2.xls` | `a2.xls.gugugezi.com` | 潮汐 (tidal) | 晚高峰主力 |
| `a3.xls` | `a3.xls.gugugezi.com` | 综合 (composite) | 通用 |
| `a4.xls` | `a4.xls.gugugezi.com` | 低抖动 (lowjitter) | VoIP/直播 |

### 4.3 单个添加域名映射

1. PigeonRelay → **本地优选** 页面 → 点击右上角 **添加域名映射**
2. 填写：

| 字段 | 填写 | 说明 |
|------|------|------|
| 子域名 | `a.xls` | 不含后缀的短名称 |
| 完整域名 | `a.xls.gugugezi.com` | 完整 FQDN |
| 算法类型 | `fast` / `steady` / `tidal` / `composite` / `lowjitter` | 五种算法选一 |
| 来源 | `local` | 自己管理的域名选 local |

3. 点击保存。保存后页面自动展示该域名当前对应的最优 IP。

### 4.4 批量添加

如果有连续编号的子域名（如 `a.xls` 到 `a4.xls`）：

1. 本地优选页面 → **批量创建**
2. 子域名列表：`a.xls,a1.xls,a2.xls,a3.xls,a4.xls`（逗号分隔）
3. 后缀：`.gugugezi.com`
4. 算法类型：选择默认算法
5. 来源：`local`
6. 创建

> 批量创建的所有域名使用相同的算法。如需不同算法，后续可逐个编辑。

---

## 第五步：在 Cloudflare DNS 中创建子域名

上一步在 PigeonRelay 中创建了映射，但 CF DNS 中还没有对应的记录。需要手动创建。

### 5.1 创建 A 记录

1. CF Dashboard → 你的域名 → **DNS** → **记录**
2. 点击 **添加记录**：

| 字段 | 填写 |
|------|------|
| 类型 | `A` |
| 名称 | `a.xls`（子域名部分，不含根域名） |
| IPv4 地址 | `1.1.1.1`（随意填，PigeonRelay 稍后自动覆盖） |
| 代理状态 | **仅 DNS (灰云)** — 关闭橙色云朵！ |
| TTL | `Auto` 或 `1 min` |

3. 重复创建所有需要的子域名（`a1.xls`、`a2.xls` 等）

> **必须灰云！** PigeonRelay 通过 API 修改 A 记录的 IP 地址。如果开启代理（橙色云朵），CF 会忽略 IP 修改，优选不生效。

### 5.2 为什么是灰云？

| 代理状态 | CF 行为 | 是否可用 |
|----------|---------|---------|
| 橙云 (Proxied) | CF 用自己的 Edge IP 替换你的 A 记录 | 不可用于优选 |
| 灰云 (DNS only) | 直接返回 A 记录的 IP，不做替换 | **可用于优选** |

橙云下你看到的永远是 CF 随机分配的 Edge IP，修改 A 记录毫无意义。灰云下 A 记录是什么就解析到什么。

---

## 第六步：绑定优选域名到节点

域名映射告诉系统"这个域名用哪个算法"，但还需要**绑定到具体节点**，订阅中才会生成使用该域名的代理条目。

### 6.1 操作步骤

1. PigeonRelay → **节点管理** → 找到你要绑定的 CDN 节点
2. Hover 节点卡片 → 点击 **设置**（齿轮图标）
3. 下拉到 **CF 优选绑定** 区域
4. 点击 **添加** 按钮

每行一个绑定，有三个字段：

| 字段 | 说明 | 示例 |
|------|------|------|
| 来源 | `本地` 或 `外部` | 你自己的域名选 `本地` |
| 域名选择 | 下拉选择已创建的域名映射 | `a.xls.gugugezi.com` |
| 名称前缀 | 订阅中代理节点的名称后缀 | `🏠本优移动-极速` |

### 6.2 名称前缀的作用

假设节点名是 `VLESS-XHTTP-美西CDN`，前缀是 `🏠本优移动-极速`，则订阅中生成的代理名为：

```
VLESS-XHTTP-美西CDN-🏠本优移动-极速
```

> 前缀中可以包含 emoji。编辑时直接在输入框里加符号即可，代码不做任何限制。

### 6.3 多选批量添加

如果需要为多个节点添加相同的优选域名：

1. 节点管理 → 勾选多个节点的 checkbox
2. 顶部出现 **批量操作栏** → 点击 **批量添加优选**
3. 选择来源（本地/外部）→ 填写名称前缀 → 粘贴域名列表（一行一个）
4. 确认 → 一次性为所有选中节点创建绑定

### 6.4 复制/粘贴绑定配置

某个节点的 CF 优选配置可以复制到其他节点：

1. 打开节点 A 的设置 → CF 优选绑定区 → 点击 **复制**
2. 打开节点 B 的设置 → CF 优选绑定区 → 点击 **粘贴**
3. 节点 B 立即获得与节点 A 完全相同的绑定配置

---

## 第七步：验证效果

### 7.1 确认 DNS 更新

等待一个 CF 更新周期（默认 30 分钟），然后检查：

```bash
# 方式一：dig 查询
dig +short a.xls.gugugezi.com

# 方式二：nslookup
nslookup a.xls.gugugezi.com 8.8.8.8

# 期望输出：一个 CF IP（如 104.26.x.x 或 172.67.x.x）
```

隔 10 分钟再查一次。如果 IP 变了，说明优选正在工作。

### 7.2 PigeonRelay 中查看

| 页面 | 查看内容 |
|------|---------|
| **本地优选** | 每个域名映射展示当前最优 IP、延迟 (ms)、成功率 (%)、抖动 (ms) |
| **延迟监控** | P50/P95/均值三条聚合线的时间序列图，24h 散点分布 |
| **测速客户端** | 今日上报总数、客户端最后活跃时间 |

### 7.3 客户端实测

1. mihomo 中拉取订阅（带你的节点名称前缀）
2. 选中一个 CDN 代理节点
3. 访问测速网站或用 curl 测试：

```bash
curl -x socks5h://127.0.0.1:7891 -s -o /dev/null -w "HTTP %{http_code} time=%{time_total}s" http://www.gstatic.com/generate_204
```

期望：HTTP 204，延迟比不使用优选时明显降低。

---

## 附录：优选算法详解

五种算法共享以下基础逻辑：
- **成功率门槛**：低于 60% 直接淘汰
- **样本数惩罚**：样本 < 10 个会被降权
- **稳定性因子**：综合抖动、P50-P95 差距、TLS 成功率、丢包率

### 极速 (fast)

```
score = avg_latency / min(success_rate, 0.95)
```

**规则：** 纯延迟优先，成功率只做上限惩罚（超过 95% 不再加分）。

**适用：** 游戏、即时通讯 — 要最低延迟，不在意偶尔丢包。

### 稳定 (steady)

```
score = avg_latency / success_rate + stddev / 200
```

**规则：** 延迟 + 方差平衡。方差项惩罚延迟波动大的 IP。

**适用：** 大文件下载、API 调用 — 需要持续稳定的吞吐。

### 潮汐 (tidal)

```
score = 0.6 × peak_hour_score + 0.4 × off_peak_score
```

**规则：** 历史晚高峰（18:00-23:00）数据权重 60%，平时数据 40%。

**适用：** 晚高峰主力 — 应对 ISP QoS 限速。

### 综合 (composite)

```
score = avg_latency / (success_rate²) + stddev / 300
```

**规则：** 成功率平方衰减 + 方差惩罚。各项均衡。

**适用：** 默认通用，不想纠结就选这个。

### 低抖动 (lowjitter)

```
score = avg_latency / success_rate + jitter × 1.5
```

**规则：** 大幅惩罚延迟抖动（相邻测量的 delta 绝对值的 EWMA）。

**适用：** VoIP、直播 — 延迟一致性比绝对值更重要。

---

## 常见问题

<details>
<summary><strong>Token 验证成功但 DNS 不更新？</strong></summary>

1. 确认 DNS 记录是**灰云**（DNS only），不是橙云
2. 检查 CF 更新间隔设置（Settings 页面），默认 30 分钟
3. 确认测速客户端有数据上报（测速客户端页面查看今日上报数）
4. 查看 PigeonRelay 容器日志：`docker logs pigeonrelay-pigeonrelay-1 | grep cfdns`
5. 检查 IP 数据是否满足算法门槛（成功率 ≥ 60%）
</details>

<details>
<summary><strong>本地优选页面显示"暂无数据"？</strong></summary>

- 测速客户端刚启动，需要 10-30 分钟积累数据
- 检查测速客户端日志：`docker logs pigeonrelay-speedtest`
- 检查 LATENCY_TOKEN 是否与 Settings 页面一致
- 检查 VPS 防火墙是否开放 3214 端口
</details>

<details>
<summary><strong>测速客户端日志显示 "HTTP 401 / invalid token"？</strong></summary>

LATENCY_TOKEN 与 Settings 页面的不一致。Settings 页面复制 Token → 更新测速客户端的 `-e LATENCY_TOKEN=xxx` → 重启容器。
</details>

<details>
<summary><strong>多个优选域名该绑定到一个节点还是不同节点？</strong></summary>

建议一个 CDN 节点绑定 1-2 个优选域名即可（如极速 + 稳定）。订阅中每条绑定生成一个独立代理，太多代理会让 mihomo 选择困难。

不同用途的优选域名可以绑定到不同节点（如游戏专用节点绑极速，下载节点绑稳定）。
</details>

<details>
<summary><strong>外部三方优选域名和自己的优选域名有什么区别？</strong></summary>

| | 本地优选 | 外部三方优选 |
|---|---|---|
| 域名 | 你自己管理的 | 第三方提供的 |
| DNS 控制 | 你可以改 CF DNS | 你无法控制 |
| 优选算法 | 你选择 | 第三方选择 |
| source 字段 | local | external |

推荐优先使用**本地优选**。外部优选域名作为备用，它们的质量取决于提供方的更新频率和算法。
</details>

<details>
<summary><strong>CF API 调用频率限制？</strong></summary>

Free plan 每月 1200 次 API 调用。每次 DNS 更新消耗 1-2 次调用（查询 + 更新）。如果 10 个子域名每 30 分钟更新一次，每天约 960 次，在限额内。建议更新间隔不小于 5 分钟。
</details>

<details>
<summary><strong>一台测速客户端够吗？</strong></summary>

一台可以工作，但多台更好。不同运营商和地区的延迟差异很大。例如移动宽带和电信宽带到同一个 CF IP 的延迟可能差 100ms+。建议至少一台在国内宽带下、一台在 VPS 上。
</details>
