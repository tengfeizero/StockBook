<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { docApi, type Doc, type DocItem } from '../api/doc'
import { fenToYuan } from '../utils/money'

const route = useRoute()
const router = useRouter()
const doc = ref<Doc | null>(null)
const showAll = ref(false) // 默认只看差异项；可切换显示全部已盘行

onMounted(async () => {
  doc.value = (await docApi.get(Number(route.params.id))) as any
})

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')
const dateText = (d?: string) => (d || '').slice(0, 10)

interface Row {
  it: DocItem
  book: number
  actual: number
  diff: number
  unitCost: number
  diffAmount: number
  legacy: boolean // 旧盘点单未记录账面数
}

// 升级前的盘点单没有 book_qty：全部行账面都为 0 时判定为历史单（新单不可能整单账面全是 0）
const legacyDoc = computed(() => {
  const items = doc.value?.items || []
  return items.length > 0 && items.every((it) => !it.book_qty) && items.some((it) => it.quantity > 0)
})

const rows = computed<Row[]>(() => {
  const legacy = legacyDoc.value
  const items = doc.value?.items || []
  return items.map((it) => {
    const book = it.book_qty || 0
    const actual = it.quantity
    const diff = actual - book
    const unitCost = it.cost || it.price || 0
    return {
      it, book, actual, diff, unitCost, legacy,
      diffAmount: Math.round((Math.abs(diff) * unitCost) / 1000),
    }
  })
})

const diffRows = computed(() => rows.value.filter((r) => !r.legacy && r.diff !== 0))
const viewRows = computed(() => (showAll.value ? rows.value.filter((r) => !r.legacy) : diffRows.value))

const stat = computed(() => {
  let up = 0, down = 0, upAmt = 0, downAmt = 0
  for (const r of diffRows.value) {
    if (r.diff > 0) { up++; upAmt += r.diffAmount } else { down++; downAmt += r.diffAmount }
  }
  return { up, down, upAmt, downAmt, net: upAmt - downAmt }
})

const isLocationScope = computed(() => doc.value?.scope === 'location')
const isRecheck = computed(() => !!doc.value?.ref_doc_id)

function diffText(r: Row) {
  if (r.diff === 0) return '0'
  return (r.diff > 0 ? '+' : '') + qtyText(r.diff)
}
function diffRate(r: Row) {
  if (r.legacy) return '—'
  if (r.book === 0) return r.diff === 0 ? '—' : '新增'
  return ((r.diff / r.book) * 100).toFixed(1) + '%'
}

function csvCell(v: unknown) {
  const s = v === null || v === undefined ? '' : String(v)
  return /[",\r\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
}

function exportCsv() {
  if (!doc.value) return
  const d = doc.value
  const head = [
    '单号', '日期', '盘点范围', '货位', '商品编码', '商品名称', '单位',
    '账面数', '实盘数', '差异', '差异率', '成本单价', '差异金额', '备注',
  ]
  const body = viewRows.value.map((r, i) => [
    i === 0 ? d.doc_no : '', i === 0 ? dateText(d.doc_date) : '',
    i === 0 ? (isLocationScope.value ? '按货架' : '全仓') : '',
    isLocationScope.value ? r.it.location_name || '未分配' : '',
    r.it.product_code, r.it.product_name, '',
    qtyText(r.book), qtyText(r.actual), diffText(r), diffRate(r),
    (r.unitCost / 100).toFixed(2),
    (r.diffAmount / 100).toFixed(2),
    r.it.remark || '',
  ])
  const tail = [[
    '合计', '', '', '', '', '', '',
    '', '', `${diffRows.value.length} 项差异`,
    `盘盈 ${stat.value.up} 项 / 盘亏 ${stat.value.down} 项`,
    '', (stat.value.net / 100).toFixed(2), '',
  ]]
  const csv = [head, ...body, ...tail].map((cols) => cols.map(csvCell).join(',')).join('\r\n')
  // BOM 让 Excel 正确识别 UTF-8 中文
  const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `盘点差异表_${d.doc_no}_${dateText(d.doc_date)}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

const doPrint = () => window.print()
</script>

<template>
  <div class="print-page" v-if="doc">
    <div class="toolbar no-print">
      <el-button type="primary" @click="doPrint">打印差异表</el-button>
      <el-button @click="exportCsv">导出 CSV（Excel）</el-button>
      <el-button @click="router.back()">返回</el-button>
      <el-checkbox v-model="showAll" style="margin-left: 4px">显示全部已盘行（含账实相符）</el-checkbox>
      <span class="hint">A4 纵向 · 边距「无」</span>
    </div>

    <div class="sheet">
      <div class="head">
        <h1>盘点差异表</h1>
        <div class="sub">
          {{ isLocationScope ? '按货架盘点' : '全仓盘点' }}
          <template v-if="isRecheck"> · 复盘单（关联初盘）</template>
          <template v-if="doc.status === 'review'"> · 待主管审批（库存未调整）</template>
          <template v-else-if="doc.status !== 'confirmed'"> · {{ doc.status === 'voided' ? '已作废' : '草稿未生效' }}</template>
        </div>
      </div>

      <table class="meta">
        <tr>
          <td><label>单 号</label>{{ doc.doc_no }}</td>
          <td><label>日 期</label>{{ dateText(doc.doc_date) }}</td>
          <td><label>差异项</label>{{ diffRows.length }} 项（共盘 {{ rows.length }} 项）</td>
        </tr>
      </table>

      <table class="items">
        <thead>
          <tr>
            <th style="width: 34px">#</th>
            <th style="width: 96px">商品编码</th>
            <th>商品名称</th>
            <th v-if="isLocationScope" style="width: 96px">货位</th>
            <th style="width: 74px">账面数</th>
            <th style="width: 74px">实盘数</th>
            <th style="width: 74px">差异</th>
            <th style="width: 62px">差异率</th>
            <th style="width: 74px">成本单价</th>
            <th style="width: 84px">差异金额</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in viewRows" :key="r.it.id">
            <td>{{ i + 1 }}</td>
            <td>{{ r.it.product_code }}</td>
            <td>{{ r.it.product_name }}</td>
            <td v-if="isLocationScope">{{ r.it.location_name || '未分配' }}</td>
            <td class="num">{{ qtyText(r.book) }}</td>
            <td class="num">{{ qtyText(r.actual) }}</td>
            <td class="num" :class="{ up: r.diff > 0, down: r.diff < 0 }">{{ diffText(r) }}</td>
            <td class="num">{{ diffRate(r) }}</td>
            <td class="num">{{ fenToYuan(r.unitCost) }}</td>
            <td class="num">{{ fenToYuan(r.diffAmount) }}</td>
          </tr>
          <tr v-if="viewRows.length === 0">
            <td :colspan="isLocationScope ? 10 : 9" class="empty">账实相符，无差异项</td>
          </tr>
        </tbody>
        <tfoot>
          <tr>
            <td colspan="4">合计</td>
            <td colspan="3" class="num">盘盈 {{ stat.up }} 项 / 盘亏 {{ stat.down }} 项</td>
            <td></td>
            <td class="num">差异金额</td>
            <td class="num">{{ fenToYuan(stat.net) }}</td>
          </tr>
        </tfoot>
      </table>

      <div class="summary">
        <span>盘盈数量合计 <b>{{ stat.up }} 项</b>，金额 ¥{{ fenToYuan(stat.upAmt) }}</span>
        <span>盘亏数量合计 <b>{{ stat.down }} 项</b>，金额 ¥{{ fenToYuan(stat.downAmt) }}</span>
        <span>净差异 <b :class="stat.net >= 0 ? 'up' : 'down'">¥{{ fenToYuan(stat.net) }}</b></span>
      </div>

      <div v-if="legacyDoc" class="warn">
        提示：本单为旧版本盘点单，未记录账面数，差异无法还原。建议对该范围重新盘点一次。
      </div>

      <div v-if="doc.remark" class="remark">备注：{{ doc.remark }}</div>

      <div class="signs">
        <span>盘点人：__________</span>
        <span>复盘人：__________</span>
        <span>负责人：__________</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.print-page {
  min-height: 100vh;
  background: #e8e8e8;
  padding: 24px 0 60px;
}
.toolbar {
  max-width: 794px;
  margin: 0 auto 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.hint {
  color: #888;
  font-size: 12px;
  margin-left: auto;
}
.sheet {
  width: 794px;
  min-height: 1000px;
  margin: 0 auto;
  background: #fff;
  padding: 40px 48px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  font-size: 13px;
  color: #111;
}
.head {
  text-align: center;
  margin-bottom: 18px;
}
.head h1 {
  font-size: 24px;
  letter-spacing: 8px;
  margin: 0 0 6px;
}
.sub {
  color: #666;
  font-size: 12px;
}
table.meta {
  width: 100%;
  margin-bottom: 12px;
}
table.meta td {
  padding: 4px 6px;
  font-size: 13px;
}
table.meta label {
  color: #666;
  margin-right: 8px;
}
table.items {
  width: 100%;
  border-collapse: collapse;
}
table.items th,
table.items td {
  border: 1px solid #333;
  padding: 5px 7px;
  text-align: left;
}
table.items th {
  background: #f2f2f2;
  font-weight: 600;
}
.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.up {
  color: #1e7e34;
}
.down {
  color: #c0392b;
}
.empty {
  text-align: center;
  color: #888;
  padding: 18px 0;
}
tfoot td {
  font-weight: 700;
  background: #fafafa;
}
.summary {
  display: flex;
  gap: 26px;
  margin-top: 14px;
  font-size: 13px;
}
.remark,
.warn {
  margin-top: 10px;
  font-size: 12px;
}
.warn {
  color: #a15c00;
  border-left: 3px solid #e6a23c;
  background: #fdf6ec;
  padding: 6px 10px;
}
.signs {
  display: flex;
  justify-content: space-between;
  margin-top: 70px;
  color: #333;
}

@media print {
  @page {
    size: A4 portrait;
    margin: 0;
  }
  .print-page {
    background: #fff;
    padding: 0;
  }
  .no-print {
    display: none !important;
  }
  .sheet {
    box-shadow: none;
    width: 210mm;
    min-height: 297mm;
    padding: 16mm 15mm;
  }
}
</style>
