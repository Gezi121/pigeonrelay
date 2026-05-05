# CF IP 优选算法设计

## 核心原则

**所有算法共享硬门槛**：`success_rate >= 60%`。成功率低于 60% 的 IP 直接排除，无论延迟多低。

**评分通用公式**：`score = f(latency) / g(success_rate) + h(volatility)`

三个维度权重不同，但 `success_rate` 对所有算法都是强约束。

## 通用指标

| 指标 | 计算 | 说明 |
|------|------|------|
| success_rate | `COUNT(latency>0) / COUNT(*)` | TCP 握手成功率 |
| stddev | `SQRT(AVG(lat²) - AVG(lat)²)` | 延迟标准差，衡量波动 |
| unstable | `COUNT(latency>200)` | >200ms 次数 |

---

## 算法

### 1. 极速（fast）

**目标**：选延迟最低又能保证基本可靠的 IP。

**公式**：`score = AVG(latency) / min(success_rate, 0.95)`

- 成功率 100% → score = AVG，纯粹按延迟排
- 成功率 80% → score = AVG / 0.8，延迟被放大 25%
- 成功率 60% → score = AVG / 0.6，延迟放大 67%
- 成功率 < 60% → 直接淘汰

**SQL**：
```sql
SELECT ip_address,
  CAST(AVG(l) / MIN(1.0*cnt_ok/cnt_all, 0.95) AS INTEGER) AS score
FROM (
  SELECT ip_address,
    CASE WHEN latency_ms>0 THEN latency_ms ELSE NULL END AS l,
    COUNT(CASE WHEN latency_ms>0 THEN 1 END) OVER(PARTITION BY ip_address) AS cnt_ok,
    COUNT(*) OVER(PARTITION BY ip_address) AS cnt_all
  FROM latency_records WHERE recorded_at>=datetime('now','-3h')
)
GROUP BY ip_address
HAVING 1.0*cnt_ok/cnt_all >= 0.6
ORDER BY score ASC LIMIT 1
```

**适用**：游戏加速、实时语音、视频通话。

---

### 2. 稳如磐石（steady）

**目标**：最稳定、波动最小。宁愿延迟高一点，不用忽快忽慢的 IP。

**公式**：
```
score = AVG(latency) / success_rate + STDDEV × 0.8
```

| 场景 | AVG | rate | STDDEV | score |
|------|-----|------|--------|-------|
| 理想 | 80 | 100% | 5 | 84 |
| 轻微丢包 | 80 | 80% | 10 | 108 |
| 严重波动 | 80 | 90% | 80 | 153 |
| 又丢又跳 | 80 | 70% | 100 | 194 |

**波动惩罚系数 0.8**：100ms 的跳变相当于延迟增加 80ms。

**SQL**：
```sql
SELECT ip_address,
  CAST(AVG(l)/(1.0*cnt_ok/cnt_all) +
    SQRT(MAX(0,AVG(l*l)-AVG(l)*AVG(l)))*0.8 AS INTEGER) AS score
FROM (SELECT ip_address, CASE WHEN latency_ms>0 THEN latency_ms END AS l)
FROM latency_records WHERE recorded_at>=datetime('now','-3h')
GROUP BY ip_address
HAVING 1.0*COUNT(l>0)/COUNT(*) >= 0.6
ORDER BY score ASC LIMIT 1
```

**适用**：大文件下载、API 调用、持续连接。

---

### 3. 时间潮汐（tidal）

**目标**：基于**历史**晚高峰的真实表现选 IP。不是根据当前时间加权，而是回看过去数据中，哪些 IP 在晚高峰（18:00-24:00）表现更好。

**机制**：
1. 过去 24h 的所有记录按时间拆成两组
2. 晚高峰组（18:00-24:00）：权重 2.0
3. 非高峰组（其他时段）：权重 0.5
4. 加权平均延迟 × 成功率的逆

**公式**：
```
peak_score     = AVG(latency_peak)     / rate_peak
offpeak_score  = AVG(latency_offpeak)  / rate_offpeak
score = peak_score × 0.6 + offpeak_score × 0.4
```

高峰 60% 权重，平时 40%。如果有 IP 高峰期炸了（高峰数据缺失或极差），peak_score 会非常高，直接排到末尾。

**SQL**（Go 端实现，分两次查询）：
```sql
-- peak
SELECT AVG(CASE WHEN l>0 THEN l END)/(1.0*COUNT(l>0)/COUNT(*))
FROM latency_records
WHERE ip_address=? AND recorded_at>=datetime('now','-24h')
  AND CAST(strftime('%H',recorded_at) AS INTEGER) BETWEEN 18 AND 23

-- offpeak  
SELECT AVG(CASE WHEN l>0 THEN l END)/(1.0*COUNT(l>0)/COUNT(*))
FROM latency_records
WHERE ip_address=? AND recorded_at>=datetime('now','-24h')
  AND CAST(strftime('%H',recorded_at) AS INTEGER) NOT BETWEEN 18 AND 23
```

**适用**：晚高峰主力服务器节点。

---

### 4. 综合复合（composite）

**目标**：同时优化速度、可靠性、一致性。默认通用算法。

**公式**：
```
score = AVG(latency) / success_rate² + STDDEV × 0.3
```

三项惩罚：
1. `AVG` — 延迟惩罚（线性）
2. `/ rate²` — 丢包平方惩罚（80%→1.56×，60%→2.78×）
3. `+ σ×0.3` — 波动微罚

**SQL**：
```sql
SELECT ip_address,
  CAST(AVG(l)/(1.0*cnt_ok/cnt_all)/(1.0*cnt_ok/cnt_all) +
    SQRT(MAX(0,AVG(l*l)-AVG(l)*AVG(l)))*0.3 AS INTEGER) AS score
FROM latency_records
WHERE recorded_at>=datetime('now','-3h')
GROUP BY ip_address
HAVING 1.0*COUNT(l>0)/COUNT(*)>=0.6
ORDER BY score ASC LIMIT 1
```

**适用**：没特殊偏好时的默认选择。

---

### 5. 低抖动（lowjitter）🆕

**目标**：延迟一致性最高。每 5 分钟测一次，相邻两次之间跳变尽可能小。

**公式**：
```
jitter = AVG(|latency_i - latency_{i-1}|)  -- 相邻跳变绝对值平均
score = AVG(latency) / success_rate + jitter × 1.5
```

跳变惩罚 1.5×：一次 100ms 的跳变 = 评分 +150。

**SQL**（Go 端实现，SQLite 不支持窗口函数 LAG）：
```sql
SELECT ip_address, recorded_at,
  CASE WHEN latency_ms>0 THEN latency_ms ELSE NULL END AS l
FROM latency_records
WHERE ip_address=? AND recorded_at>=datetime('now','-3h')
ORDER BY recorded_at ASC
```
Go 端遍历计算 jitter。

**适用**：VoIP、直播推流、FPS 游戏。

---

## 对比总结

| 算法 | 延迟敏感 | 丢包惩罚 | 波动惩罚 | 时间感知 | 最佳场景 |
|------|---------|---------|---------|---------|---------|
| fast | ★★★ | ★★ | - | - | 游戏、通话 |
| steady | ★ | ★★★ | ★★★ | - | 下载、API |
| tidal | ★★ | ★★ | - | ★★★ | 晚高峰 |
| composite | ★★ | ★★★ | ★★ | - | 默认通用 |
| lowjitter | ★★ | ★★ | ★★ | - | 直播、FPS |

---

## 前端展示

本地优选看板，选择算法后刷新 IP，展示指标卡片：

```
IP: 104.16.50.1  算法: 极速  评分: 85
延迟 85ms | 成功率 98% | σ 12ms | 不稳 2次/3h
```
