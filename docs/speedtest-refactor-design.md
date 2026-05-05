# 测速系统重构设计

## 当前问题清单

### 客户端 (speedtest-client/client.py)

| # | 问题 | 影响 |
|---|------|------|
| 1 | /24 子网随机采样无智能筛选，大量无效 IP 浪费带宽和连接数 | ~16000 段 × 每个 IP = 大量无用连接 |
| 2 | 仅做 TCP 三次握手，不测 TLS 握手 → 延迟数据不代表真实代理延迟 | TCP 通 ≠ TLS 通，cfdns 里 tlsCheck 失败率高 |
| 3 | 每 IP 测 5 次取平均上报，服务端丢失原始数据分布 | 无法计算真正的 jitter/stddev |
| 4 | 上报失败后直接丢弃 chunk 无恢复 | 数据丢失 |
| 5 | 全量扫描 + 高频 fast track 无节流 → 有被封 IP 风险 | 同一 IP 短时间内大量连接 |
| 6 | 无客户端身份/ISP 标记，服务端无法区分数据质量 | 劣质客户端数据污染全局 IP 评分 |
| 7 | sub-15ms 硬过滤可能误杀真正的低延迟 IP（如香港节点） | 移动用户直连香港 CF 可达 10-20ms |
| 8 | 5 次 TCP 握手**全部并发执行**且仅持续 ~2s，间歇性丢包完全不可见 | Clash 实际使用中频繁丢包的 IP，测速报告 100% 成功率 |

### 服务端接收 (handler/latency.go + repository)

| # | 问题 | 影响 |
|---|------|------|
| 9 | 所有客户端数据等权合并，无客户端质量/ISP 区分 | 一个劣质网络客户端拉低全局 IP 评分 |
| 10 | 3h 窗口内新旧数据权重相同，无时间衰减 | 2.5 小时前的数据和 1 分钟前的数据等权 |
| 11 | 批量 INSERT 无去重合并，同 IP 大量冗余记录 | 存储膨胀，查询变慢 |
| 12 | 无预计算滚动指标表 → 每次算法查询扫全表 | 随着数据量增长查询越来越慢 |
| 13 | 客户端上报即接受，无服务端校验/过滤 | 异常数据直接入库 |

### 权重算法 (repository.go GetBestIPForDomain)

| # | 问题 | 影响 |
|---|------|------|
| 14 | tidal/lowjitter 的 N+1 查询：20 候选 × 2 次查询 = 40+ SQL | 单次优选耗时 200-500ms |
| 15 | countPenalty 只对 fast/steady/composite 的 SQL 内生效 | tidal/lowjitter 无样本数惩罚 |
| 16 | jitter 从已聚合的 5 分钟窗口平均计算，非原始散点 | jitter 值被严重低估 |
| 17 | success_rate < 60% 硬门槛过于粗糙，无渐进惩罚 | 59% 成功率 = 直接淘汰，与 0% 同等待遇 |
| 18 | 无客户端维度——一个 IP 在不同客户端上表现差异巨大但算法无视 | 算法选出的是"均值"最优，不是"你的网络"最优 |

### 架构层面

| # | 问题 | 影响 |
|---|------|------|
| 19 | 服务端无法向客户端下发指令，完全被动接收 | 无法指定客户端测特定 IP/段 |
| 20 | 客户端和服务端之间无心跳/状态同步 | 无法判断客户端是否在线 |
| 21 | tcpping.go 基本闲置 | 冗余代码 |
| 22 | 无 IP 级别的持久化评分快照 | 每次查询重新计算，浪费 CPU |

---

## 生产环境已验证的问题

旧算法在实际使用中暴露出两个核心矛盾：

### 问题A：TCP 测速很好 → Clash 实际 timeout

```
现象：算法选出的 IP TCP 延迟 85ms / 成功率 98%，看着完美
      但 Clash 客户端配置后频繁 timeout，实际不可用
```

**根因链**：

| 环节 | 发生了什么 | 对应问题编号 |
|------|-----------|-------------|
| 客户端只测 TCP 三次握手 | TLS ClientHello 根本没发。CF 边缘节点对纯 TCP SYN 一定响应，但 TLS 握手可能被中间设备 RST 或路由到不可用的节点 | #2 |
| 假蔷 IP 未过滤 | 电信/联通出口的 transparent proxy 对 TCP SYN 返回 SYN-ACK（伪装成 CF IP 响应），延迟仅 30ms。但后续 TLS 握手直接失败——因为压根没到真正的 CF | #7 |
| 上报的是 5 次均值 | 如果 5 次中 2 次 TLS 级别失败、3 次 TCP 级别成功，上报 avg=85ms/ok=3——服务端完全不知道 TLS 层面有问题 | #3 |
| cfdns 的 tlsCheck 只在 DNS 更新时做 | 算法查询 (`GetBestIPForDomain`) **不做 TLS 验证**，只在 cfdns 更新 DNS 记录时调用 `tlsCheck`。算法层选出的 IP 根本没经过 TLS 验证 | 架构缺陷 |
| 服务端无 TLS 指标 | `ip_stats` 不存在。算法公式里 `success_rate` 只反映 TCP 成功率，不是 TLS 成功率。一个 TCP 100% 但 TLS 0% 的 IP 可以拿到完美评分 | #9, #13 |

**为什么第三方优选 IP 不 timeout**：XIU2/cfnb 等工具用下载测速（HTTP GET）做终排——能完成 HTTP 下载说明 TCP + TLS + HTTP 全链路通。它们的排序依据天然包含了 TLS 可达性。

### 问题B：我们选的 IP 延迟比第三方优选差

```
现象：同样是 CF IP，第三方优选（如 bestcf.top）返回的 IP 实测 50ms
      我们算法选的 IP 实测 120ms
```

**根因链**：

| 环节 | 发生了什么 | 对应问题编号 |
|------|-----------|-------------|
| 纯 TCP 延迟 ≠ 代理真实延迟 | TCP RTT 只测到 CF 边缘的 SYN-ACK 时间。TLS 握手要额外 1-2 个 RTT，不同 IP 的 TLS 完成时间差异可达 50-200ms | #2 |
| 无时间衰减，旧数据拖累 | 3h 窗口等权：2.5h 前 50ms 的数据和 1min 前 200ms 的数据权重相同。一个 3h 前表现好但现在已拥塞的 IP 可以靠历史数据维持高分 | #10 |
| 劣质客户端污染 | 一个部署在差网络（比如移动 4G 热点）的客户端，所有 IP 延迟都 >300ms。它的上报会平均拉高所有 IP 的全局评分，好客户端测到的低延迟被稀释 | #9 |
| 无下载速度维度 | 两个 IP 都是 TCP 80ms，一个能跑 50Mbps，一个只能跑 2Mbps。延迟相同但实际体验天差地别——算法完全无法区分 | 缺失能力 |
| 算法选出的是"全局均值最优" | 不同客户端到同一 IP 的延迟差异巨大。全局平均（对所有客户端）延迟最低的 IP，对**你的**客户端不一定是最优的 | #18 |
| 第三方工具频繁重扫 | XIU2 每次运行都是全量重测，数据绝对新鲜。我们依赖客户端周期性上报，一个 IP 可能几小时没更新 | #10 |

### 新设计如何针对性解决

| 问题 | 解决措施 | 设计章节 |
|------|---------|---------|
| TCP 通 ≠ 代理通 | 客户端 Phase 2 TLS 握手验证 + 上报 `tls_samples` + 服务端 `ewma_tls_ok_rate` 参与评分 | 1.1, 2.1, 3.1 |
| 假蔷 IP | 分运营商过滤：电信/联通 `tll_min=40ms` + TLS 失败判定 | 1.3 |
| 算法层无 TLS 验证 | 所有算法从 `ip_stats.ewma_tls_ok_rate` 读取 TLS 成功率，低于阈值的 IP 自动降权 | 3.1 |
| 时间衰减 | EWMA (α=0.3) 替代等权窗口，1h 前数据权重降至 1/6 | 2.2 |
| 客户端污染 | 客户端偏差系数 (`clientWeight`) 自动降权劣质客户端数据 | 2.4 |
| 无下载速度维度 | Phase 3 带宽测速 + `throughput_score` 参与最终评分（参考 CF 官方阈值表） | 3.1 Layer 3 |
| 非客户端感知 | `clientAdjustedLatency` 根据特定客户端的历史偏差调整预估 | 3.6 |
| 数据不新鲜 | `test_priority` 调度客户端优先复测高价值但久未更新的 IP | 4.2, 4.3 |

### 问题C：IP 实际频繁丢包但测速客户端从未报告丢包

```
现象：Clash 日志显示某 IP 经常 timeout/connection reset，延迟波动剧烈
      但在测速客户端的历史报告里，这个 IP 的成功率 100%、延迟稳定 80ms
```

**根因分析——测的是什么 vs 实际用的是什么**：

```
Clash 实际代理连接的生命周期:
  Client → TCP SYN → TLS ClientHello → TLS ServerHello → 密钥交换
  → HTTP/2 SETTINGS → 持续数据传输 (几分钟到几小时)
  ↑                                                    ↑
  我们测的只有这一段 (~2ms)                              丢包发生在这里

我们的测速客户端所做的一切:
  socket.create_connection((ip, 443)) → 收到 SYN-ACK → sock.close()
  总时长: ~80ms (一个 RTT)
  实际 TCP 连接持续时间: <1ms (连接后立即关闭)
```

**丢包不可见的三层原因**：

| 层 | 原因 | 详细机制 |
|----|------|---------|
| **时间维度** | 5 次 TCP 握手全部在 ~2s 内并发完成 | 间歇性丢包（每 30s 丢一波）在 2s 窗口内恰好没发生。就像你在 2 秒内看了 5 次红绿灯全是绿的，就报告这条路 100% 畅通——但这条路每隔 30 秒就会堵一次 |
| **连接深度** | SYN-ACK 只需 1 个 RTT | TCP SYN 只要 1 个包出去、1 个包回来（共 2 个包）。而真实代理连接需要数十个 RTT 的持续交互。经验规律：**丢包率随连接持续时间的平方根增长**。1ms 的连接 0% 丢包 ≈ 10min 连接可能有 5% 丢包 |
| **行为维度** | 纯 SYN 包，无载荷 | CF 边缘对不同类型流量的处理优先级不同。SYN 包被 QoS 标记为最高优先级（快速响应），而持续数据流可能走不同的队列 |

**为什么 XIU2/cfnb 的用户很少报告这个问题**：
- XIU2 的下载测速：下载 300MB 文件持续 10 秒。这个 10 秒的 TCP 流如果丢包，TCP 重传会被 curl 的 `speed_download` 捕获——速度直接掉下来
- cfnb 的 1MB 下载：虽然只有 ~1 秒，但足够覆盖一个完整的 TCP 慢启动 + 几个 RTT，丢包会直接反映在 `bandwidth < 阈值` 被过滤掉
- **有下载测速 = 有持续的 TCP 流 = 丢包会暴露**

**这不是个别问题，是普遍的系统性盲区**：

| 丢失类型 | 5次并发SYN能否检测 | 实际发生概率 | Clash用户体感 |
|---------|-------------------|-------------|-------------|
| 持续丢包 (always 10%) | **可能**（5次中约0.5次丢） | 低 | 所有连接都受影响 |
| 间歇丢包 (30s burst) | **几乎不可能**（窗口2s） | **高** | 每隔一会就卡一下 |
| 拥塞丢包 (高峰期) | **不可能**（测速在闲时） | **高** | 晚高峰各种 timeout |
| 队列丢包 (bufferbloat) | **完全不可能**（无排队） | 中 | 延迟不稳定，偶尔超时 |
| 路由抖动 (BGP change) | **不可能**（单次采样） | 低 | 突然断连几秒 |

### 新设计如何解决

| 措施 | 解决什么 | 实现位置 |
|------|---------|---------|
| Phase 3 每 2 分钟持续复测 Top M | 时间维度：多个时间点采样，捕获间歇性丢包 | 客户端 1.1 |
| Phase 3 带宽测速 (1MB 下载) | 连接深度：持续的 TCP 流暴露丢包→带宽下降 | 客户端 1.1 |
| 上报原始 `tcp_samples[]` 而非均值 | 保留每次测量的成败，服务端可计算真实的丢包分布 | 客户端 1.4 |
| `ewma_loss_rate` 独立追踪 | 不合并到 success_rate，让丢包成为独立评分维度 | 服务端 2.1, 3.0 Layer2 |
| 时间窗口内的成功率方差 | EWMA 同时追踪 `success_rate` 和它的 `variance`：一个稳定 100% 和时好时坏的 100% 均值，方差可以区分 | 服务端 2.2 |
| 长连接模拟 (Phase 3 可选) | 对候选 Top 5 IP 做一次 10 秒持续小流量下载，模拟真实代理连接的 TCP 行为 | 客户端 1.1 Phase 3 |

---

## 业界算法参考

### 同类项目算法对比

| 项目 | 测速方式 | 排序/评分算法 | 复杂度 |
|------|---------|-------------|--------|
| **XIU2/CloudflareSpeedTest** | TCPing + 下载测速 | 两阶段自然排序：0%丢包组优先 → 组内延迟升序 → 下载速度降序。**无显式加权公式** | 低 |
| **cfnb** | TCP ×7次 + 代理可用性API + curl下载1MB | 三层瀑布淘汰：平均延迟取32候选 → 可用性过滤假通 → 带宽降序取16。**无加权** | 低 |
| **better-cloudflare-ip** | HTTPS curl --resolve ×3 + 顺序下载 | 延迟升序 → 逐个下载 → 首位达标即终止。**无加权** | 低 |
| **Cloudflare 官方 speedtest** | TCP+TLS+下载+上传+jitter | **阈值分段评分**：每项指标落入阈值区间赋固定分，3类场景聚合为 bad/poor/average/good/great | 中 |
| **vps789** | 三网24h TCPing+下载+晚高峰 | 闭源，推断为多维度打分 + 每日淘汰 1/3 | 中 |
| **我们的 PigeonRelay** | TCP ×5取平均 | 5种显式加权公式 + success_rate gate + variance + jitter + countPenalty | 中高 |

**核心发现**：大多数同类项目**不使用复杂加权公式**，靠分层筛选自然得出最优。我们的 5 算法框架在思路上比业界平均先进，真正的问题是**数据质量不够好**（缺 TLS、缺带宽、缺时间衰减、缺百分位）而非算法设计不够复杂。

### Cloudflare 官方阈值评分（可参考）

```
延迟 <10ms  → 20分    下载 >100Mbps → 30分
延迟 10-20  → 10分    下载 50-100   → 20分
延迟 20-50  →  5分    下载 10-50    → 10分
延迟 50-100 →  0分    下载 1-10     →  5分
延迟 100-500→ -10分   下载 <1Mbps   →  0分
延迟 >500  → -20分

丢包 <1%   → 10分    jitter <10ms  → 10分
丢包 1-5%  →  5分    jitter 10-20  →  5分
丢包 5-25% →  0分    jitter 20-100 →  0分
丢包 25-50%→ -10分   jitter >500   → -20分
丢包 >50%  → -20分
```

> 这种"查表法"比我们当前的连续函数更直观、更容易调试。可以考虑用于 Layer 3 带宽加分。



### Cloudflare 扫描检测机制

Cloudflare **不公开固定阈值**，使用多信号动态概率模型：

| 检测层 | 机制 | 响应时间 | 对测速客户端的影响 |
|--------|------|---------|-------------------|
| **IP 信誉评分 (Threat Score 0-100)** | Project Honeypot + 公共情报 + ASN 信誉 + 历史攻击记录 | 实时 | **数据中心/VPS IP 天然低信誉**，容忍阈值远低于家宽 |
| **L3/L4 DDoS (dosd)** | 分析 SYN 速率、TCP 标志位、端口分布、报文速率 | 检测亚秒，缓解 ≤3s | 短时间大量 SYN → 触发 SYN Cookie 挑战 |
| **L7 Bot 评分 (1-99)** | TLS/JA3 指纹、HTTP/2 指纹、行为节奏、Cookie 连续性 | 实时 | **Go/Python 默认 TLS 库指纹极易被识别为 bot** |
| **Error 1015 速率限制** | 超出站点配置的请求阈值 | 软封 15min-1h，递增至 24h | 持续激进 → 封禁时间指数增长 |

关键结论：
- **数据中心 IP 容忍度远低于家宽** — 同一行为 VPS 秒封、家宽无感
- **TLS 指纹是核心信号** — Go `crypto/tls` / Python `requests` 固定 JA3/JA4 指纹瞬间暴露
- **行为节奏 > 绝对数量** — 毫秒级规律间隔是自动化铁证
- **不是简单计数器** — 是 IP 信誉 × 流量模式 × 协议行为 × TLS 指纹的联合概率

### Go TLS 指纹伪装方案

Python 测速客户端和 Go 后端的 TLS 握手测试都需要伪装：

| 库 | 语言 | 说明 |
|----|------|------|
| `refraction-networking/utls` | Go | `crypto/tls` 的 fork，底层 ClientHello 控制（2.3k stars） |
| `bogdanfinn/tls-client` | Go | 类 `net/http.Client` 接口，内置 Chrome/Firefox/Safari 指纹（1.5k stars） |
| `skycheung803/go-bypasser` | Go | 实现 `http.RoundTripper`，可集成 resty/colly，支持 Browser Mode |
| `Noooste/azuretls-client` | Go+Python | 100% Go 实现，支持 JA3/HTTP2/HTTP3，Python CFFI 绑定 |
| `tls-client` (Python) | Python | Python 侧 TLS 指纹伪装，对标 bogdanfinn 方案 |

> **关键原则**：TLS 指纹必须与 User-Agent 匹配。Chrome 指纹配 Chrome UA，Firefox 指纹配 Firefox UA，否则反而成为更明显的异常信号。

### XIU2/CloudflareSpeedTest 核心经验

- **TCPing 模式**不被运营商视为扫描（仅三次握手即关闭）
- **HTTPing 模式**在 VPS 上容易被封 → 降并发至 n=30
- 总测速次数 = IP数量 × 每IP次数 → 这个数字过大是首要原因
- **-tll 参数**过滤 transparent proxy 假低延迟（**仅电信/联通**需要 40-100ms，**移动不要设**，否则会误杀香港 10-20ms 真实节点）
- **-sl 参数**移动用户用它来过滤假蔷 IP（下载速度为 0 但延迟正常）
- 每个 /24 段随机抽 1 个是标准做法（不扫全段）
- **并发模型**：Go channel 做信号量 (semaphore)，`-n` 决定缓冲大小
- **下载测速**：自定义 `DialContext` 绑 IP + `TLSClientConfig.ServerName` 设 SNI，等效 `curl --resolve`
- 排查被限三步法：降 `-t`(测次) → 降 `-n`(并发) → 拆分 IP 文件分批
- Ctrl+C 后网络立刻恢复 = 被实时限制（非永久封锁）

### better-cloudflare-ip (badafans) 核心经验

- 用 `curl --resolve` 绑 IP 做**完整 TLS 握手**，不依赖 ICMP/TCP 探测
- 空路由检测：三次连接全部超时 → 直接丢弃
- DNS 污染缓解：`--resolve` 直连 IP 绕过系统 DNS
- 两阶段递进：并行 RTT 海选 → **顺序**下载测速精选（非并行，降低检测风险）
- 找到第一个达标的 IP 就终止（早停），不浪费资源
- **不向服务器上传节点信息**，隐私保护设计

### vps789 / GetCFipToDns IP 池管理模型

这是目前中文社区最成熟的 IP 池管理方案——**双池两阶段模型**：

```
初选池 (Initial Pool)          监测池 (Monitor Pool)
CloudflareST 批量生成 ──────► 200-500 IP 24h 三网持续监测
                                          │
                                    每日滚动更新:
                                    ① 多维度综合评分
                                    ② 淘汰底部 1/3
                                    ③ 从初选池补充等量新 IP
                                    ④ 优质 IP 持续存活数周
```

| 参数 | 值 | 说明 |
|------|---|------|
| 监测池规模 | 200-500 IP | 每日固定监测 |
| 每日淘汰率 | 1/3 | 比例制而非固定阈值（自适应网络波动） |
| 评估维度 | 延迟/丢包/下载速度/晚高峰表现/HTTP 可达性 |
| 收敛周期 | 3-7 天进入稳态 | 初期快速洗出劣质 IP，后期窄幅竞争 |
| DNS 同步 | 每小时 | 域名 A 记录指向当前最优 IP |
| 故障保护 | 全部不可用时不清空 DNS | 保留现有记录避免彻底断线 |

**淘汰率为何是 1/3**：类似遗传算法的 selection pressure。
- 过高 (50%+) → 收敛太快，陷入局部最优
- 过低 (10%) → 收敛太慢，劣质 IP 长期滞留
- 1/3 → 平均存活 3 天，头部 IP 可存活数周

### cf_auto_bestip (lee1080) 故障转移机制

- 双脚本：`cfst_test.js` 低频测速（每天/每周） + `cf_dns_sync.js` 高频健康检查（每 5 分钟）
- 在岗 IP 通过 `/cdn-cgi/trace` 实时健康检查
- 检测到失效 IP → 自动摘除 DNS 记录 → 从备选池补充
- **全部候选不可用时不清空 DNS**（关键保护）

### cf-knife 高并发扫描器

- 单二进制，跨平台，TLS/HTTP 双模探测
- DPI 绕过分析（TLS ClientHello 分片）
- WARP 扫描、反 MITM 检测
- 参考其 TLS 分片技术用于过墙场景

### cfnb (xinyitang3) 核心经验

- 三重筛选：TCP 延迟 → IP 可用性**二次验证** → 真实带宽测速
- 支持分国家优选（按 CF colo 代码过滤）
- 自动更新 CF DNS 多 IP 轮询
- 微信通知实时反馈

---

## 重构方案

### 总体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     PigeonRelay Server                       │
│                                                             │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │ 上报接收  │  │  任务队列     │  │  权重引擎             │  │
│  │ /report  │  │  /tasks      │  │  (EWMA + 多算法)     │  │
│  └────┬─────┘  └──────┬───────┘  └──────────┬───────────┘  │
│       │               │                      │              │
│       ▼               ▼                      ▼              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              IP 状态表 (ip_stats)                      │   │
│  │  · ewma_latency    · success_rate (EWMA)              │   │
│  │  · jitter          · last_tested_at                   │   │
│  │  · tls_ok_rate     · per_client_stats (JSON)          │   │
│  │  · score_snapshot  · test_priority                    │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
   ┌─────────┐       ┌─────────┐       ┌─────────┐
   │ 客户端A  │       │ 客户端B  │       │ 客户端C  │
   │ Pull任务 │       │ Pull任务 │       │ Pull任务 │
   │ 执行测速 │       │ 执行测速 │       │ 执行测速 │
   │ 上报结果 │       │ 上报结果 │       │ 上报结果 │
   └─────────┘       └─────────┘       └─────────┘
```

### IP 池管理：双池模型

采用业界验证的"初选池 + 监测池"双层架构：

```
初选池 (Initial Pool)                   监测池 (Monitor Pool)
· 全量 /24 段扫描结果                     · 200-500 高价值 IP
· 不定期更新                              · 24h 持续 TCP+TLS 监测
· 存储 ip_address + first_seen           · 每日滚动淘汰底部 1/3
         │                                       │
         └─────────────── 补充 ──────────────────┘
```

| 参数 | 建议值 | 说明 |
|------|--------|------|
| 监测池规模 | 300 IP | 平衡覆盖面与维护成本 |
| 每日淘汰率 | 1/3 | 比例制（非固定阈值），自适应网络整体波动 |
| 评估维度 | TCP 延迟 + TLS 延迟 + 成功率 + jitter + 晚高峰表现 | 多维度综合评分 |
| 收敛周期 | 3-7 天 | 初期快速洗出劣质 IP，后期头部 IP 窄幅竞争 |
| 保护机制 | 全不可用时保留现有记录 | 避免断线 |
| DNS 同步 | 每小时（cfdns 更新 A 记录） | 每次选前 20 IP 做 TLS 二次验证 |

### TLS 指纹伪装

客户端和服务端做 TLS 握手测速时，必须伪装 TLS 指纹避免被 CF 识别为 bot：

| 场景 | 方案 |
|------|------|
| **Python 客户端 TLS 测速** | 使用 `tls-client` Python 库替代 `requests`，伪装 Chrome/Firefox JA3 指纹 |
| **Go 服务端 cfdns tlsCheck** | 使用 `bogdanfinn/tls-client` 或 `refraction-networking/utls`，自定义 Dialer 绑 IP + 伪装指纹 |
| **指纹-UserAgent 匹配** | Chrome 指纹 + Chrome UA，Firefox 指纹 + Firefox UA，必须一一对应 |
| **行为伪装** | TLS 测速间加入随机抖动 (jitter) 延迟 50-200ms 随机，模拟人类操作节奏 |

### 一、客户端重构

#### 1.1 三阶段测速（替代现在的单阶段 TCP）

```
阶段1: TCP RTT 海选 (全量 IP 池)
  └─ 每个 /24 段抽 1 个 IP
  └─ 并发 50（可配置），超时 2s
  └─ 过滤：latency < tll_min → 标记为疑似 transparent proxy
  └─ 筛选出 Top N（默认 200）进入阶段2

阶段2: TLS 握手验证 (Top N)
  └─ 对阶段1的前 N 个 IP 做 TLS Handshake
  └─ 并发 10（低并发，避免触发 IDS），超时 3s
  └─ 测量 TLS 握手时间 = tls_time
  └─ 真实延迟 = TCP RTT + TLS 握手时间
  └─ 过滤：TLS 失败率 > 50% 的 IP 直接淘汰
  └─ 筛选出 Top M（默认 50）进入阶段3

阶段3: 精准抽样 (Top M，高频复测)
  └─ 每 2 分钟对 Top M 做 TCP + TLS 双测
  └─ 上报原始数据（不取平均）
  └─ 优先复测服务端下发的高优先级 IP
```

#### 1.2 反封锁策略

| 策略 | 实现 | 对应 CF 检测信号 |
|------|------|-----------------|
| **降低并发** | 阶段1 n=50、阶段2 n=10（远低于 200 默认） | 降低 SYN 速率 → 减少 L4 dosd 触发 |
| **错峰分片** | IP 池分 N 片，每片间隔 30-60s，不连续轰炸 | 打破规律节奏 → 干扰行为检测 |
| **连接即关** | TCP 握手完成后立即 RST，不保持连接 | 减少 flowtrackd 连接追踪 |
| **TLS 低频** | TLS 验证仅在 Top N 做，不对全量 IP 做 | 减少 TLS 握手频率（TLS 握手比 TCP 更重） |
| **速率限制** | 单 IP 两次测速间隔 ≥ 60s（fast track 除外） | 单 IP 视角不可见扫描行为 |
| **退避机制** | 连续超时 IP → 标记 dead → 冷却 30min 后才重试 | 减少对不可达 IP 的无效连接 |
| **TLS 指纹伪装** | Python 用 `tls-client` 库替代 `requests`，伪装 Chrome JA3 | 降低 L7 Bot 评分 |
| **行为抖动** | 连接间加入 50-200ms 随机延迟 | 破坏"毫秒级规律间隔"特征 |
| **自建测速端点** | 服务端暴露 `/speedtest/health` 做 TLS SNI 目标 | 避免对 CF 官方 IP 高频 TLS 握手 |

#### 1.3 分运营商假蔷过滤

不同运营商对 transparent proxy 的表现不同，不能一刀切：

| 运营商 | 假蔷特征 | 过滤方式 |
|--------|---------|---------|
| **电信/联通** | TCPing 异常低（几十 ms），远超实际延迟 | `tll_min = 40ms`，低于此值且 TLS 失败 → 丢弃 |
| **移动** | TCPing 正常，下载速度为 0 | `sl_min = 1 MB/s`，下载速度为 0 则丢弃 |
| **海外/直连** | 无假蔷 | `tll_min = 0`（不过滤） |

客户端上报时携带 ISP 信息，服务端根据 ISP 类型自动调整过滤策略。

#### 1.4 上报格式（保留原始数据）

```json
{
  "client_id": "uuid",
  "client_info": {
    "isp": "china-telecom",
    "region": "east-china",
    "version": "2.0.0"
  },
  "data": [
    {
      "ip": "104.16.50.1",
      "tcp_samples": [85, 87, 82, 90, 84],
      "tls_samples": [12, 15, 11, 14, 13],
      "tls_ok": 5,
      "tls_fail": 0
    }
  ]
}
```

保留每个样本的原始值，服务端自主计算分布指标。

#### 1.5 任务轮询（Pull 模式）

```python
# 客户端主循环中增加任务拉取
def fetch_tasks():
    r = requests.get(f"{BASE_URL}/api/v1/latency/tasks",
        headers={"Authorization": f"Bearer {LATENCY_TOKEN}"},
        params={"client_id": CLIENT_ID})
    if r.status_code == 200:
        return r.json()  # { tasks: [{ip, priority, action}] }
    return []

# 任务类型
# - "test_ip": 精准测指定 IP（含 TLS）
# - "test_cidr": 测指定 CIDR 段
# - "recheck": 复测之前表现好但最近没数据的 IP
# - "pause": 暂停全量扫描 N 分钟
```

#### 1.6 上报可靠性

- 上报失败 → 写本地 SQLite/journal 持久化
- 下次上报周期合并重试
- 最多保留 3 个周期的积压数据

### 二、服务端接收重构

#### 2.1 IP 状态表 (ip_stats)

```sql
CREATE TABLE ip_stats (
    ip_address TEXT PRIMARY KEY,
    -- EWMA 指标（衰减因子 α=0.3，等效半衰期约 2 个测量周期）
    ewma_latency_ms REAL NOT NULL DEFAULT 0,
    ewma_tls_ms REAL NOT NULL DEFAULT 0,
    ewma_success_rate REAL NOT NULL DEFAULT 0,  -- 0-1, TCP 成功率
    ewma_tls_ok_rate REAL NOT NULL DEFAULT 0,   -- 0-1, TLS 握手成功率
    ewma_loss_rate REAL NOT NULL DEFAULT 0,     -- 0-1, 丢包率独立追踪
    -- 波动指标
    jitter_ms REAL NOT NULL DEFAULT 0,           -- EWMA of |Δ|
    latency_stddev REAL NOT NULL DEFAULT 0,      -- EWMA of variance
    success_rate_variance REAL NOT NULL DEFAULT 0, -- 成功率方差（区分稳定100%和时好时坏100%）
    -- 百分位近似（EWMA 维护）
    p50_latency_ms REAL NOT NULL DEFAULT 0,
    p95_latency_ms REAL NOT NULL DEFAULT 0,
    -- 带宽
    throughput_bytes_per_sec REAL NOT NULL DEFAULT 0,
    -- 时间窗口统计（便捷查询）
    samples_1h INTEGER DEFAULT 0,
    samples_24h INTEGER DEFAULT 0,
    last_tested_at DATETIME,
    last_tls_ok_at DATETIME,
    -- 调度
    test_priority REAL NOT NULL DEFAULT 0,       -- 越大越优先测
    cooldown_until DATETIME,                      -- 冷却期
    -- 分客户端统计（JSON）
    per_client_json TEXT DEFAULT '{}',
    -- 历史峰值
    peak_hour_latency REAL,                       -- 近 7 天高峰最差值
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 2.2 EWMA 滚动更新（替代等权窗口）

```go
const alpha = 0.3 // 衰减因子

func updateEWMA(old, new float64) float64 {
    return alpha*new + (1-alpha)*old
}

// 收到新数据时更新 ip_stats
func (r *Repo) UpdateIPStats(ip string, samples []Sample) error {
    stats := r.GetIPStats(ip)
    for _, s := range samples {
        stats.EWMALatency = updateEWMA(stats.EWMALatency, s.Latency)
        stats.EWMASuccessRate = updateEWMA(stats.EWMASuccessRate, boolToFloat(s.OK))
        // jitter: EWMA of absolute deltas
        delta := math.Abs(s.Latency - prevLatency)
        stats.Jitter = updateEWMA(stats.Jitter, delta)
    }
    // 更新 test_priority
    stats.TestPriority = calcPriority(stats)
    return r.UpsertIPStats(stats)
}
```

半衰期 ≈ ln(2) / α ≈ 2.3 个测量周期。客户端每 10 分钟上报一次 → 约 23 分钟半衰期，意味着 1 小时前的数据权重下降到约 1/6。

#### 2.3 原始记录保留策略

- `latency_records` 保留 7 天（与现状一致）
- 新增 `ip_stats` 表为滚动聚合（常驻，不过期）
- 算法查询优先读 `ip_stats`，仅在需要原始数据（tidal 高峰分析/lowjitter）时回退到 `latency_records`

#### 2.4 客户端质量加权

```go
// 每个客户端有独立的偏差系数
// bias = 该客户端测得延迟 / 全局 EWMA 延迟
// 稳定偏差 > 1.5x 的客户端数据降权
func clientWeight(clientBias float64) float64 {
    if clientBias <= 1.2 {
        return 1.0
    }
    if clientBias >= 2.0 {
        return 0.3
    }
    return 1.0 - (clientBias-1.2)*0.875 // 线性降权 1.0 → 0.3
}
```

#### 2.5 cfdns TLS 验证升级

当前 `cfdns.tlsCheck` 使用 Go 标准库 `crypto/tls`，JA3 指纹固定且易被识别。重构方案：

```go
// 使用 uTLS 伪装 Chrome 指纹做 TLS 握手验证
import (
    "net"
    "time"
    utls "github.com/refraction-networking/utls"
)

func tlsCheckWithFingerprint(ip, sni string) (bool, int) {
    // 自定义 Dialer：直连 IP（等效 curl --resolve）
    dialer := &net.Dialer{Timeout: 3 * time.Second}
    conn, err := dialer.Dial("tcp", ip+":443")
    if err != nil {
        return false, 0
    }
    defer conn.Close()

    // 使用 Chrome 的 TLS ClientHello 指纹
    tlsConn := utls.UClient(conn, &utls.Config{
        ServerName:         sni,
        InsecureSkipVerify: true,
    }, utls.HelloChrome_Auto)

    start := time.Now()
    err = tlsConn.Handshake()
    elapsed := int(time.Since(start).Milliseconds())
    tlsConn.Close()
    
    return err == nil, elapsed
}
```

所有 cfdns 和算法查询中的 TLS 验证点均升级为伪装指纹版本。

#### 2.6 测速能力缺口补齐

对标业界，我们当前和重构后需要新增的测速维度：

| 维度 | 当前 | 重构后 | 来源依据 |
|------|------|--------|---------|
| TCP 延迟 | ✅ 5次取平均 | ✅ 原始样本 + 百分位 (P50/P95) | 所有项目标配 |
| TLS 握手 | ❌ | ✅ Phase 2 原始样本 + `ewma_tls_ok_rate` | better-cloudflare-ip, CF官方 |
| 下载带宽 | ❌ | ✅ Phase 3 1MB 文件下载，上报 `bytes_per_sec` | XIU2, cfnb, CF官方 |
| 丢包率 | ✅ 折算在 rate | ✅ 独立 `ewma_loss_rate`，参与稳定性惩罚 | XIU2 分组排序的核心维度 |
| jitter | ✅ 聚合后算 | ✅ 原始样本 EWMA 滚动 | CF官方 |
| 百分位 P50/P95 | ❌ | ✅ EWMA 维护 approx. percentiles | CF官方 speedtest |

### 三、权重算法重构

#### 3.0 算法简化：三层评分模型

当前 5 算法保留命名和定位，但内部统一到三层结构：

```
FinalScore = Layer1_base × Layer2_penalty − Layer3_bonus
```

**Layer 1: 基础延迟分**
| 算法 | Layer1 公式 | 说明 |
|------|-----------|------|
| fast | P50_latency / min(rate, 0.95) | 中位数不受极端值影响 |
| steady | EWMA_latency / rate | 平滑值更稳定 |
| tidal | peak_lat × 0.6 + offpeak_lat × 0.4 | 不变 |
| composite | P50_latency / rate² | 丢包平方惩罚 |
| lowjitter | EWMA_latency / rate | 与 steady 同底 |

**Layer 2: 稳定性惩罚 (所有算法共用)**
```
penalty = 1.0 + jitter/200 + (P95-P50)/150 + (1-tls_ok_rate)×2 + loss_rate×3
```
- jitter/200：波动惩罚（已有）
- (P95-P50)/150：长尾惩罚（新增，P95 远超 P50 说明不稳定）
- (1-tls_ok_rate)×2：TLS 不可达惩罚（新增，解决"TCP通但代理不通"）
- loss_rate×3：丢包惩罚（已有）

**Layer 3: 带宽加分 (所有算法共用)**
```
bonus = min(throughput_score, 15)  // 上限 15 分，避免带宽主导评分
throughput_score 参考 CF 官方阈值表：
  >50Mbps → 15,  10-50Mbps → 10,  5-10 → 5,  1-5 → 2,  <1 → 0
```
- 仅在 `ewma_tls_ok_rate >= 0.8` 时生效（TLS 都不通的 IP 不配拿带宽加分）
- 带宽数据不足时默认 0，不影响无带宽测速能力的客户端

所有算法改查 `ip_stats` 表（预计算指标），消除 N+1 查询。

#### 3.3 tidal 重构

```sql
-- 不再对每个候选 IP 分别查询！直接从 ip_stats 读取预计算的 peak_hour_latency
SELECT ip_address,
       peak_hour_latency * 0.6 + ewma_latency_ms * 0.4 AS tidal_score
FROM ip_stats
WHERE ewma_success_rate >= 0.6
  AND samples_24h >= 10
  AND last_tested_at >= datetime('now', '-2 hours')
ORDER BY tidal_score ASC
LIMIT 1
```

`peak_hour_latency` 由后台定时任务（每小时）计算一次：

```sql
-- 每小时执行：更新 ip_stats.peak_hour_latency
UPDATE ip_stats SET peak_hour_latency = (
    SELECT AVG(latency_ms) FROM latency_records
    WHERE ip_address = ip_stats.ip_address
      AND recorded_at >= datetime('now', '-7 days')
      AND CAST(strftime('%H', recorded_at) AS INTEGER) BETWEEN 18 AND 23
      AND latency_ms > 0
)
```

#### 3.4 lowjitter 重构

jitter 已存储在 `ip_stats.jitter_ms`（EWMA 滚动计算），不用每次查询时从原始记录算。

#### 3.5 新增：渐进式 success_rate 惩罚

替代硬门槛 60%：

```go
func ratePenalty(rate float64) float64 {
    if rate >= 0.95 { return 1.0 }
    if rate <= 0.4  { return 100.0 } // 基本淘汰
    // 平滑曲线：rate=0.6 → 2.5x, rate=0.8 → 1.3x
    return 1.0 + math.Pow((0.95-rate)*10, 2)
}
```

#### 3.6 新增：客户端感知优选

```go
// 当 domain_mapping 指定了 client_filter 时
// 从 ip_stats.per_client_json 中提取该客户端历史数据
// 用该客户端的偏差系数调整延迟预估值
func clientAdjustedLatency(ip string, clientID string) float64 {
    stats := getIPStats(ip)
    bias := getClientBias(clientID, ip)
    return stats.EWMALatency * bias
}
```

### 四、服务端→客户端指令通道

#### 4.1 GET /api/v1/latency/tasks

客户端轮询间隔：与 fast track 同频（60s）

**请求**：
```
GET /api/v1/latency/tasks?client_id=xxx
Authorization: Bearer <LATENCY_TOKEN>
```

**响应**：
```json
{
  "tasks": [
    {"ip": "104.16.50.1", "action": "test_ip", "priority": 9},
    {"ip": "104.16.0.0/20", "action": "test_cidr", "priority": 5},
    {"action": "recheck_stale", "count": 20},
    {"action": "report_status"}
  ],
  "config": {
    "tcp_concurrency": 50,
    "tls_concurrency": 10,
    "measure_interval_sec": 600,
    "report_interval_sec": 1200,
    "tll_min_ms": 40
  }
}
```

#### 4.2 任务优先级调度逻辑

服务端根据 `ip_stats.test_priority` 生成任务：

```go
func (r *Repo) GenerateTasks(clientID string) []Task {
    // 1. 过期重测：last_tested_at > 30min 且之前表现好的 IP（前 10%）
    // 2. 精准补充：服务端选出的 candidate IP 缺少 TLS 验证 → 下发让客户端做
    // 3. 异常复检：success_rate 突然下降的 IP → 指定客户端精准复测
    // 4. 新段探索：从未测过的 /24 段 → 渐进式探索
    return tasks
}
```

#### 4.3 test_priority 计算公式

```
test_priority = base_score × freshness_decay × exploration_bonus

base_score       = 1 / (1 + ewma_latency/100)
freshness_decay  = max(0.1, 1 - minutes_since_last_test / 60)
exploration_bonus= 1.5 (从未测过)，1.0 (已测过)
```

优先测：低延迟 + 久未更新 + 高价值 IP。

### 五、数据流总览

```
                    ┌─────────────────┐
                    │  客户端测速引擎   │
                    │  ┌───┐ ┌───┐    │
                    │  │TCP│→│TLS│    │
                    │  └───┘ └───┘    │
                    └────────┬────────┘
                             │ POST /report (原始样本)
                             ▼
                    ┌─────────────────┐
                    │  服务端接收层     │
                    │  · 客户端鉴权    │
                    │  · 异常值过滤    │
                    │  · 客户端偏差计算 │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  ip_stats 更新   │
                    │  · EWMA 滚动     │
                    │  · jitter 计算   │
                    │  · priority 更新 │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │ 算法查询  │  │ 任务生成  │  │ CF DNS   │
        │ (O(1))   │  │ (priority)│  │ 更新     │
        └──────────┘  └──────────┘  └──────────┘
                             │
                             ▼
                    GET /tasks (Pull 60s)
                             │
                             ▼
                    ┌─────────────────┐
                    │  客户端接收任务   │
                    │  优先执行高优IP   │
                    └─────────────────┘
```

### 六、实施路线

| 阶段 | 内容 | 改动范围 | 风险 |
|------|------|---------|------|
| **Phase 1** | ip_stats 表 + EWMA 滚动更新 + 算法迁移到查 ip_stats | repository, latency handler | 低：新增表，不删旧逻辑 |
| **Phase 2** | 客户端三阶段测速 + 原始数据上报 + 反封锁参数 | client.py | 中：需测试反封锁效果 |
| **Phase 3** | 任务队列 + 客户端 Pull 指令 + test_priority | repository, handler, client.py | 中：新增 API，需前后端联调 |
| **Phase 4** | 客户端质量加权 + 渐进式 success_rate + 算法优化 | repository | 低：算法层改动 |
| **Phase 5** | 清理旧代码、废弃 tcpping.go、性能压测 | 全项目 | 低 |

### 七、风险与注意事项

1. **TLS 握手 = 真实流量特征**：阶段2 TLS 验证虽然低频（10 并发），但仍比纯 TCP 更易被识别。建议在服务端暴露测速端点（如 `/speedtest/health`）作为 TLS SNI 目标，避免对 CF 官方 IP 做大量 TLS 握手。

2. **客户端偏差系数需要预热期**：新客户端前 24h 偏差系数固定为 1.0，积累足够数据后再计算。

3. **ip_stats 表写入瓶颈**：高频上报场景下，建议使用 `INSERT ... ON CONFLICT DO UPDATE` 批量 upsert，配合 SQLite WAL 模式。

4. **旧客户端兼容**：Phase 2 的新上报格式保留对旧格式的兼容解析，Phase 1 的 ip_stats 更新同时支持新旧两种数据源。

5. **参数可配置**：所有关键参数（α 衰减因子、并发数、阈值）放在 settings 表中，前端 Settings 页面可调。
