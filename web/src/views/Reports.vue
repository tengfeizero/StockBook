<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { reportApi, type SummaryRow, type ProfitResult } from '../api/report'
import { fenToYuan } from '../utils/money'
import { docTypeConfig } from '../api/doc'

const activeTab = ref('summary')
const loading = ref(false)
const range = reactive({ date_from: monthStart(), date_to: today() })

const summaryRows = ref<SummaryRow[]>([])
const profitData = ref<ProfitResult | null>(null)

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')

function today() {
  return new Date().toISOString().slice(0, 10)
}
function monthStart() {
  const d = new Date()
  return new Date(d.getFullYear(), d.getMonth(), 1).toISOString().slice(0, 10)
}

const summaryTotals = reactive({ opening_val: 0, in_val: 0, out_val: 0, closing_val: 0 })

async function loadSummary() {
  loading.value = true
  try {
    summaryRows.value = (await reportApi.summary(range)) || []
    summaryTotals.opening_val = summaryRows.value.reduce((s, r) => s + r.opening_val, 0)
    summaryTotals.in_val = summaryRows.value.reduce((s, r) => s + r.in_val, 0)
    summaryTotals.out_val = summaryRows.value.reduce((s, r) => s + r.out_val, 0)
    summaryTotals.closing_val = summaryRows.value.reduce((s, r) => s + r.closing_val, 0)
  } finally {
    loading.value = false
  }
}

async function loadProfit() {
  loading.value = true
  try {
    profitData.value = (await reportApi.profit(range)) || null
  } finally {
    loading.value = false
  }
}

function switchTab() {
  if (activeTab.value === 'summary') loadSummary()
  else loadProfit()
}

onMounted(loadSummary)
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
      <el-date-picker v-model="range.date_from" type="date" placeholder="开始日期" value-format="YYYY-MM-DD" style="width: 150px" />
      <span>至</span>
      <el-date-picker v-model="range.date_to" type="date" placeholder="结束日期" value-format="YYYY-MM-DD" style="width: 150px" />
      <el-button type="primary" @click="switchTab()">查询</el-button>
    </div>

    <el-tabs v-model="activeTab" @tab-change="switchTab">
      <el-tab-pane label="进销存汇总" name="summary" />
      <el-tab-pane label="销售毛利" name="profit" />
    </el-tabs>

    <!-- 进销存汇总 -->
    <template v-if="activeTab === 'summary'">
      <el-table :data="summaryRows" v-loading="loading" border show-summary :summary-method="() => ['', '', '合计', '', '', '', '¥' + fenToYuan(summaryTotals.in_val), '', '¥' + fenToYuan(summaryTotals.out_val), '', '¥' + fenToYuan(summaryTotals.closing_val)]">
        <el-table-column prop="code" label="编码" width="100" />
        <el-table-column prop="name" label="商品" min-width="130" />
        <el-table-column label="期初数量" width="95" align="right">
          <template #default="{ row }">{{ qtyText(row.opening_qty) }}</template>
        </el-table-column>
        <el-table-column label="期初金额" width="105" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.opening_val) }}</template>
        </el-table-column>
        <el-table-column label="入库数量" width="95" align="right">
          <template #default="{ row }">{{ qtyText(row.in_qty) }}</template>
        </el-table-column>
        <el-table-column label="入库金额" width="105" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.in_val) }}</template>
        </el-table-column>
        <el-table-column label="出库数量" width="95" align="right">
          <template #default="{ row }">{{ qtyText(row.out_qty) }}</template>
        </el-table-column>
        <el-table-column label="出库成本" width="105" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.out_val) }}</template>
        </el-table-column>
        <el-table-column label="期末数量" width="95" align="right">
          <template #default="{ row }">
            <b>{{ qtyText(row.closing_qty) }}</b>
          </template>
        </el-table-column>
        <el-table-column label="期末金额" width="105" align="right">
          <template #default="{ row }">
            <b>¥{{ fenToYuan(row.closing_val) }}</b>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 8px; color: var(--el-text-color-secondary); font-size: 12px">
        按库存流水聚合：期初 + 入库 − 出库 = 期末；单据作废后流水自动回冲，报表随之变化。
      </div>
    </template>

    <!-- 销售毛利 -->
    <template v-else-if="profitData">
      <el-descriptions :column="3" border size="small" style="margin-bottom: 12px">
        <el-descriptions-item label="销售收入">¥{{ fenToYuan(profitData.total.revenue) }}</el-descriptions-item>
        <el-descriptions-item label="销售成本">¥{{ fenToYuan(profitData.total.cost) }}</el-descriptions-item>
        <el-descriptions-item label="毛利">
          <b :style="{ color: profitData.total.profit >= 0 ? 'var(--el-color-success)' : 'var(--el-color-danger)' }">
            ¥{{ fenToYuan(profitData.total.profit) }}
          </b>
        </el-descriptions-item>
      </el-descriptions>
      <el-table :data="profitData.rows" v-loading="loading" border size="small" empty-text="该区间无销售单据">
        <el-table-column prop="doc_no" label="单号" width="160" />
        <el-table-column prop="doc_date" label="日期" width="110" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">{{ docTypeConfig[row.doc_type]?.title || row.doc_type }}</template>
        </el-table-column>
        <el-table-column prop="partner" label="客户" min-width="130" />
        <el-table-column label="收入" width="110" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.revenue) }}</template>
        </el-table-column>
        <el-table-column label="成本" width="110" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.cost) }}</template>
        </el-table-column>
        <el-table-column label="毛利" width="110" align="right">
          <template #default="{ row }">
            <span :style="{ color: row.profit >= 0 ? 'var(--el-color-success)' : 'var(--el-color-danger)' }">¥{{ fenToYuan(row.profit) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </template>
  </el-card>
</template>
