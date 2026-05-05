<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="mb-8 flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">延迟监控</h1>
        <p class="text-sm text-slate-500 mt-1">CF 边缘 IP 延迟统计与每日精选。</p>
      </div>
    </div>

    <!-- 每日精选 IP -->
    <Card class="mb-8">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-bold text-slate-800">每日精选 Top 20</h2>
        <Badge variant="info">最近 24 小时 · 平均延迟 &lt; 500ms</Badge>
      </div>
      <div v-if="dailyPicksLoading" class="h-16 flex items-center justify-center text-sm text-slate-400 animate-pulse">加载中...</div>
      <div v-else-if="dailyPicks.length === 0" class="text-sm text-slate-400 text-center py-4">暂无数据 — 等待测速客户端上报</div>
      <div v-else class="space-y-1">
        <div v-for="(p, idx) in dailyPicks" :key="p.ip"
             class="flex items-center gap-2 px-3 py-2 bg-slate-50 rounded-lg border border-slate-100 hover:border-primary-200 hover:bg-primary-50/50 transition-colors cursor-pointer text-xs"
             @click="selectedIP = p; loadIPDetail(p.ip)">
          <span class="font-bold text-primary-600 w-5 shrink-0">#{{ idx + 1 }}</span>
          <span class="font-mono font-bold text-slate-700 w-32 shrink-0 truncate">{{ p.ip }}</span>
          <span class="w-12 text-right font-bold shrink-0" :class="p.avg_ms < 50 ? 'text-emerald-600' : p.avg_ms < 150 ? 'text-amber-600' : 'text-slate-500'">{{ p.avg_ms }}ms</span>
          <span class="text-slate-400 w-20 text-right shrink-0">{{ p.min_ms }}-{{ p.max_ms }}ms</span>
          <span class="font-mono w-10 text-right shrink-0" :class="p.rate >= 80 ? 'text-emerald-600' : p.rate >= 50 ? 'text-amber-600' : 'text-red-500'">{{ p.rate }}%</span>
          <span class="w-12 text-right shrink-0" :class="p.stable_rt >= 90 ? 'text-emerald-600' : p.stable_rt >= 70 ? 'text-amber-600' : 'text-red-500'">{{ p.stable_rt }}%稳</span>
          <span v-if="p.unstable > 0" class="text-amber-500 w-16 text-right shrink-0">{{ p.unstable }}次&gt;500</span>
          <span v-else class="text-emerald-500 w-16 text-right shrink-0">稳定</span>
          <span class="text-slate-400 w-10 text-right shrink-0">{{ p.success }}/{{ p.count }}</span>
        </div>
      </div>
    </Card>

    <!-- 延迟图表 -->
    <Card class="mb-8">
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-3">
          <template v-if="selectedIP">
            <Button variant="ghost" size="sm" @click="selectedIP=null; loadLatency(latencySpan)">← 返回统计</Button>
            <h2 class="text-lg font-bold text-slate-800 font-mono">{{ selectedIP }}</h2>
            <Badge variant="primary" dot class="text-xs">24h 详情</Badge>
          </template>
          <template v-else>
            <h2 class="text-lg font-bold text-slate-800">延迟统计</h2>
            <Badge variant="default" dot class="text-xs">P50 · P95 · 均值</Badge>
            <label class="flex items-center gap-1.5 text-xs text-slate-500 cursor-pointer">
              <input type="checkbox" v-model="filterChart" @change="buildChart(cachedRecords)" />
              仅优选 (avg&lt;500ms)
            </label>
          </template>
        </div>
        <div class="flex gap-2">
          <Button size="sm" :variant="latencySpan === 4 ? 'primary' : 'secondary'" @click="selectedIP ? loadIPDetail(selectedIP, 4) : loadLatency(4)">4h</Button>
          <Button size="sm" :variant="latencySpan === 12 ? 'primary' : 'secondary'" @click="selectedIP ? loadIPDetail(selectedIP, 12) : loadLatency(12)">12h</Button>
          <Button size="sm" :variant="latencySpan === 24 ? 'primary' : 'secondary'" @click="selectedIP ? loadIPDetail(selectedIP, 24) : loadLatency(24)">24h</Button>
          <Button size="sm" :variant="latencySpan === 168 ? 'primary' : 'secondary'" @click="selectedIP ? loadIPDetail(selectedIP, 168) : loadLatency(168)">1周</Button>
        </div>
      </div>
      <!-- IP stats bar -->
      <div v-if="selectedIP && ipStats" class="flex gap-4 mb-3 text-xs">
        <span>均值 <b class="text-primary-600">{{ ipStats.avg }}ms</b></span>
        <span>P50 <b class="text-indigo-600">{{ ipStats.p50 }}ms</b></span>
        <span>P95 <b class="text-amber-600">{{ ipStats.p95 }}ms</b></span>
        <span>成功率 <b :class="ipStats.rate >= 80 ? 'text-emerald-600' : 'text-red-500'">{{ ipStats.rate }}%</b></span>
        <span>&gt;500ms <b class="text-amber-600">{{ ipStats.unstable }}次</b></span>
      </div>
      <div v-if="latencyLoading" class="h-80 flex items-center justify-center">
        <div class="animate-pulse text-slate-400">加载数据中...</div>
      </div>
      <div v-else class="h-80 relative">
        <v-chart class="w-full h-full" :option="chartOption" autoresize />
      </div>
    </Card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, ScatterChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, MarkLineComponent } from 'echarts/components'
import VChart from 'vue-echarts'

use([CanvasRenderer, LineChart, ScatterChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, MarkLineComponent])

import { api } from '../api.js'
import Card from '../components/ui/Card.vue'
import Badge from '../components/ui/Badge.vue'
import Button from '../components/ui/Button.vue'

const latencySpan = ref(24)
const latencyLoading = ref(true)
const chartOption = ref({})
const dailyPicks = ref([])
const dailyPicksLoading = ref(true)
const filterChart = ref(true)
const cachedRecords = ref([])
const chartLimit = 30
const selectedIP = ref(null)
const ipStats = ref(null)

onMounted(() => {
  loadLatency(24)
  fetchDailyPicks()
})

async function loadIPDetail(ip, hours) {
  selectedIP.value = ip
  latencySpan.value = hours || 24
  latencyLoading.value = true
  try {
    const records = await api.latencyByIP(ip, hours || 24)
    buildIPChart(records)
  } catch { window.$toast?.error('加载失败') }
  finally { latencyLoading.value = false }
}

function buildIPChart(records) {
  if (!records.length) { chartOption.value = { series: [] }; ipStats.value = null; return }
  const vals = records.map(r => r.latency_ms).sort((a,b) => a-b)
  const success = vals.filter(v => v > 0)
  if (!success.length) { chartOption.value = { series: [] }; ipStats.value = null; return }
  const avg = Math.round(success.reduce((s,v)=>s+v,0)/success.length)
  const p50 = success[Math.floor(success.length*0.5)]
  const p95 = success[Math.floor(success.length*0.95)]
  const unstable = vals.filter(v => v > 500).length
  const rate = Math.round(success.length*100/vals.length)
  ipStats.value = { avg, p50: p50||0, p95: p95||0, unstable, rate }

  const scatterData = records.map(r => {
    const t = new Date(r.recorded_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'})
    return [t, r.latency_ms]
  })

  chartOption.value = {
    tooltip: { trigger: 'axis' },
    legend: { data: ['延迟↓', '均值→'], bottom: 0, itemWidth: 16, itemHeight: 2 },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '5%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false },
    yAxis: { type: 'value', name: 'ms', splitLine: { lineStyle: { type: 'dashed' } } },
    dataZoom: [{ type: 'inside', start: 0, end: 100 }],
    series: [
      { name: '延迟↓', type: 'scatter', symbolSize: 3, data: scatterData,
        itemStyle: { color: (p) => p.value[1] > 500 ? '#ef4444' : p.value[1] > 0 ? '#6366f1' : '#d4d4d8' } },
      { name: '均值→', type: 'line', smooth: true, symbol: 'none', data: scatterData,
        lineStyle: { color: '#f59e0b', width: 1, type: 'dashed' },
        markLine: { symbol: ['none','none'], label: { show: false },
          data: [{ yAxis: 500, itemStyle: { color: '#f59e0b' } }, { yAxis: 1200, itemStyle: { color: '#ef4444' } }] } }
    ]
  }
}

async function fetchDailyPicks() {
  dailyPicksLoading.value = true
  try {
    dailyPicks.value = await api.latencyDailyPicks()
  } catch {
    dailyPicks.value = []
  } finally {
    dailyPicksLoading.value = false
  }
}

async function loadLatency(hours) {
  latencySpan.value = hours
  latencyLoading.value = true
  try {
    cachedRecords.value = await api.latencyData(hours)
    buildChart(cachedRecords.value)
  } catch (e) {
    window.$toast?.error('加载延迟失败', e.message)
  } finally {
    latencyLoading.value = false
  }
}

function buildChart(records) {
  // Collect per-IP series and filter
  const seriesData = {}
  records.forEach(r => {
    if (!seriesData[r.ip_address]) seriesData[r.ip_address] = []
    seriesData[r.ip_address].push(r.latency_ms)
  })

  let selectedIPs = Object.keys(seriesData)
  if (filterChart.value) {
    selectedIPs = selectedIPs
      .map(ip => ({ ip, avg: seriesData[ip].reduce((s, v) => s + v, 0) / seriesData[ip].length }))
      .filter(x => x.avg < 500)
      .sort((a, b) => a.avg - b.avg)
      .slice(0, chartLimit)
      .map(x => x.ip)
  }

  // Group by time bucket, compute P50/P95/AVG across selected IPs
  const timeBuckets = {}
  records.forEach(r => {
    if (!selectedIPs.includes(r.ip_address)) return
    const t = new Date(r.recorded_at)
    const key = t.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    if (!timeBuckets[key]) timeBuckets[key] = { ts: t.getTime(), vals: [] }
    timeBuckets[key].vals.push(r.latency_ms)
  })

  const sorted = Object.entries(timeBuckets).sort((a, b) => a[1].ts - b[1].ts)
  if (sorted.length === 0) { chartOption.value = { series: [] }; return }

  function pct(sortedVals, n) {
    const i = Math.ceil(sortedVals.length * n / 100) - 1
    return sortedVals[Math.max(0, i)]
  }

  const avgData = [], p50Data = [], p95Data = []
  sorted.forEach(([time, bucket]) => {
    const sortedVals = bucket.vals.sort((a, b) => a - b)
    const avg = Math.round(sortedVals.reduce((s, v) => s + v, 0) / sortedVals.length)
    avgData.push([time, avg])
    p50Data.push([time, pct(sortedVals, 50)])
    p95Data.push([time, pct(sortedVals, 95)])
  })

  chartOption.value = {
    tooltip: {
      trigger: 'axis',
      formatter: (params) => {
        const t = params[0]?.axisValue || ''
        let s = `<b>${t}</b><br/>`
        params.forEach(p => { s += `${p.marker} ${p.seriesName}: ${p.value[1]}ms<br/>` })
        return s
      }
    },
    legend: { data: ['P95', '中位数', '均值'], bottom: 0, itemWidth: 16, itemHeight: 2 },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '5%', containLabel: true },
    xAxis: { type: 'category', boundaryGap: false },
    yAxis: { type: 'value', name: 'ms', splitLine: { lineStyle: { type: 'dashed' } } },
    dataZoom: [{ type: 'inside', start: 0, end: 100 }],
    series: [
      {
        name: 'P95', type: 'line', smooth: true, symbol: 'none', data: p95Data,
        lineStyle: { color: '#f59e0b', width: 1, type: 'dashed' },
        areaStyle: { color: 'rgba(245,158,11,0.06)' },
        markLine: {
          symbol: ['none', 'none'], label: { show: false },
          data: [{ yAxis: 500, itemStyle: { color: '#f59e0b', type: 'dashed' } }, { yAxis: 1200, itemStyle: { color: '#ef4444', type: 'dashed' } }]
        }
      },
      { name: '中位数', type: 'line', smooth: true, symbol: 'none', data: p50Data, lineStyle: { color: '#6366f1', width: 2 } },
      { name: '均值', type: 'line', smooth: true, symbol: 'none', data: avgData, lineStyle: { color: '#0ea5e9', width: 1.5, type: 'dotted' } },
    ]
  }
}

</script>
