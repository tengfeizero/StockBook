<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { reportApi, type DashboardStats } from '../api/report'
import { docTypeConfig, statusText } from '../api/doc'
import { fenToYuan } from '../utils/money'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { PackagePlus, PackageMinus, ClipboardCheck, Wallet } from 'lucide-vue-next'

const router = useRouter()
const auth = useAuthStore()
const ui = useUiStore()
const stats = ref<DashboardStats | null>(null)
const loading = ref(true)

// 全新账套（还没开过单）时给一条上手提示，开过单就不再出现
const showStarter = computed(() => !!stats.value && stats.value.recent_docs.length === 0)

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')
const typeTitle = (t: string) => docTypeConfig[t]?.title || t
const statusType = (s: string) => (s === 'confirmed' ? 'success' : s === 'voided' ? 'danger' : s === 'review' ? 'warning' : 'info')

const quickActions = [
  { label: '采购入库', icon: PackagePlus, path: '/purchase-in' },
  { label: '销售出库', icon: PackageMinus, path: '/sales-out' },
  { label: '盘点', icon: ClipboardCheck, path: '/stocktake' },
  { label: '收付款', icon: Wallet, path: '/finance' },
]

// 近 14 天趋势柱状图（纯 SVG，无图表库依赖）
const trend = computed(() => {
  const pts = stats.value?.sales_trend || []
  if (!pts.length) return null
  const max = Math.max(...pts.map((p) => p.sales), 1)
  const bars = pts.map((p) => {
    const h = p.sales > 0 ? Math.max(Math.round((p.sales / max) * 100), 2) : 2
    return {
      date: p.date,
      label: p.date.slice(5).replace('-', '/'), // MM/DD
      height: h,
      amount: p.sales,
      isToday: p.date === new Date().toISOString().slice(0, 10),
    }
  })
  return { bars, max, total: pts.reduce((s, p) => s + p.sales, 0) }
})
const fmtYuan = (v: number) => (v / 100).toLocaleString('zh-CN', { maximumFractionDigits: 0 })

onMounted(async () => {
  try {
    stats.value = (await reportApi.dashboard()) as any
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-loading="loading">
    <!-- 新手上手提示（只在还没开过单时出现） -->
    <div v-if="showStarter" class="starter">
      <div class="starter-text">
        <div class="starter-title">第一次使用？3 分钟就能开第一单</div>
        <div class="starter-sub">建商品 → 录期初库存 → 开一张销售出库单。想先看完整效果，也可以载入演示数据。</div>
      </div>
      <div class="starter-actions">
        <el-button size="small" type="primary" @click="ui.startTour()">看引导</el-button>
        <el-button size="small" @click="router.push('/help')">使用手册</el-button>
        <el-button v-if="auth.hasPerm(['system'])" size="small" text @click="router.push('/system?tab=demo')">
          载入演示数据
        </el-button>
      </div>
    </div>

    <!-- 快捷入口 -->
    <div class="quick" data-guide="quick" style="display: flex; gap: 12px; margin-bottom: 16px">
      <el-button
        v-for="a in quickActions" :key="a.path" size="large" plain
        style="flex: 1; height: 56px; font-size: 15px"
        @click="router.push(a.path)"
      >
        <el-icon style="margin-right: 6px"><component :is="a.icon" /></el-icon>
        {{ a.label }}
      </el-button>
    </div>

    <!-- 指标卡 -->
    <el-row :gutter="16" data-guide="kpi">
      <el-col :span="4">
        <el-card shadow="never">
          <div class="stat-label">今日销售额</div>
          <div class="stat-value" style="color: var(--el-color-primary)">
            ¥{{ stats ? fenToYuan(stats.today_sales) : '--' }}
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never">
          <div class="stat-label">今日毛利</div>
          <div class="stat-value" style="color: var(--el-color-success)">
            ¥{{ stats ? fenToYuan(stats.today_profit) : '--' }}
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never">
          <div class="stat-label">今日单据</div>
          <div class="stat-value">{{ stats ? stats.today_docs : '--' }} <span class="stat-unit">张</span></div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never">
          <div class="stat-label">库存总值</div>
          <div class="stat-value">¥{{ stats ? fenToYuan(stats.inventory_val) : '--' }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" style="cursor: pointer" @click="router.push('/finance')">
          <div class="stat-label">应收欠款</div>
          <div class="stat-value" style="color: var(--el-color-danger)">
            ¥{{ stats ? fenToYuan(stats.receivable) : '--' }}
          </div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" style="cursor: pointer" @click="router.push('/finance')">
          <div class="stat-label">应付欠款</div>
          <div class="stat-value" style="color: var(--el-color-warning)">
            ¥{{ stats ? fenToYuan(stats.payable) : '--' }}
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 销售趋势 -->
    <el-card shadow="never" style="margin-top: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>近 14 天销售趋势</span>
          <span v-if="trend" style="color: var(--el-text-color-secondary); font-size: 13px">
            合计 ¥{{ fmtYuan(trend.total) }}
          </span>
        </div>
      </template>
      <div v-if="trend" class="trend">
        <div v-for="(b, i) in trend.bars" :key="i" class="trend-col">
          <div class="trend-amount">{{ b.amount > 0 ? fmtYuan(b.amount) : '' }}</div>
          <div class="trend-bar-wrap">
            <div
              class="trend-bar"
              :class="{ today: b.isToday }"
              :style="{ height: b.height + '%' }"
              :title="`${b.date}：¥${fmtYuan(b.amount)}`"
            ></div>
          </div>
          <div class="trend-date" :class="{ today: b.isToday }">{{ b.label }}</div>
        </div>
      </div>
      <el-empty v-else description="暂无销售数据" :image-size="60" />
    </el-card>

    <el-row :gutter="16" style="margin-top: 16px">
      <!-- 最近单据 -->
      <el-col :span="14">
        <el-card shadow="never">
          <template #header>
            <div style="display: flex; justify-content: space-between; align-items: center">
              <span>最近单据</span>
              <el-button link type="primary" @click="router.push('/doc-list')">全部单据 →</el-button>
            </div>
          </template>
          <el-table
            v-if="stats && stats.recent_docs.length" :data="stats.recent_docs"
            border size="small" style="cursor: pointer"
            @row-click="(row: any) => router.push({ path: '/doc-list', query: { keyword: row.doc_no } })"
          >
            <el-table-column prop="doc_no" label="单号" width="180" />
            <el-table-column label="类型" width="100">
              <template #default="{ row }">{{ typeTitle(row.doc_type) }}</template>
            </el-table-column>
            <el-table-column prop="partner_name" label="往来单位" min-width="130" />
            <el-table-column label="金额" width="110" align="right">
              <template #default="{ row }">¥{{ fenToYuan(row.total_amount) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)" size="small">{{ statusText[row.status] || row.status }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="还没有单据" :image-size="60" />
        </el-card>
      </el-col>

      <!-- 库存预警 -->
      <el-col :span="10">
        <el-card shadow="never">
          <template #header>
            <div style="display: flex; justify-content: space-between; align-items: center">
              <span>库存预警（{{ stats?.low_stock_count || 0 }}）</span>
              <el-button link type="primary" @click="router.push('/inventory')">去库存查询</el-button>
            </div>
          </template>
          <el-table v-if="stats && stats.low_stock.length" :data="stats.low_stock" border size="small">
            <el-table-column prop="code" label="编码" width="100" />
            <el-table-column prop="name" label="商品" min-width="130" />
            <el-table-column label="当前/安全" width="120" align="right">
              <template #default="{ row }">{{ qtyText(row.quantity) }} / {{ qtyText(row.safe_stock) }}</template>
            </el-table-column>
            <el-table-column label="缺口" width="80" align="right">
              <template #default="{ row }">
                <b style="color: var(--el-color-danger)">{{ qtyText(row.safe_stock - row.quantity) }}</b>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-else description="暂无库存预警" :image-size="60" />
          <template v-if="stats && stats.location_alerts && stats.location_alerts.length">
            <el-divider style="margin: 10px 0" />
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px">
              <span style="font-size: 13px">货位容量预警（{{ stats.location_alerts.length }}）</span>
              <el-button link type="primary" size="small" @click="router.push('/locations')">去库位管理</el-button>
            </div>
            <div
              v-for="a in stats.location_alerts" :key="a.id"
              style="display: flex; justify-content: space-between; align-items: center; font-size: 12px; padding: 3px 0"
            >
              <span>{{ a.name }}</span>
              <span>
                <el-tag size="small" :type="a.status === 'over' || a.status === 'full' ? 'danger' : 'warning'">
                  {{ a.status_text }}
                </el-tag>
                <span v-if="a.capacity" style="margin-left: 8px; color: var(--el-text-color-secondary)">
                  {{ qtyText(a.used_qty) }}{{ a.used_unit === 'L' ? 'L' : a.used_unit === 'kg' ? 'kg' : '' }} / {{ qtyText(a.capacity) }}{{ a.used_unit === 'L' ? 'L' : a.used_unit === 'kg' ? 'kg' : '' }}（{{ a.usage_rate }}%）
                </span>
              </span>
            </div>
          </template>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.stat-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin-bottom: 8px;
}
.stat-value {
  font-size: 22px;
  font-weight: 600;
  white-space: nowrap;
}
.stat-unit {
  font-size: 13px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
.trend {
  display: flex;
  align-items: stretch;
  gap: 6px;
  height: 200px;
  padding: 0 8px;
}
.trend-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 0;
}
.trend-amount {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  height: 16px;
  white-space: nowrap;
}
.trend-bar-wrap {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}
.trend-bar {
  width: 60%;
  max-width: 36px;
  min-height: 2px;
  background: var(--el-color-primary-light-5);
  border-radius: 3px 3px 0 0;
  transition: height 0.4s ease;
}
.trend-bar.today {
  background: var(--el-color-primary);
}
.trend-date {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-top: 6px;
  white-space: nowrap;
}
.trend-date.today {
  color: var(--el-color-primary);
  font-weight: 600;
}
.starter {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  margin-bottom: 16px;
  border: 1px solid var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
  border-radius: 8px;
}
.starter-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-color-primary);
  margin-bottom: 3px;
}
.starter-sub {
  font-size: 12.5px;
  color: var(--el-text-color-regular);
}
.starter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}
</style>
