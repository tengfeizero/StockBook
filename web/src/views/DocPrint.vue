<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { docApi, docTypeConfig, statusText, type Doc } from '../api/doc'
import { fenToYuan } from '../utils/money'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const doc = ref<Doc | null>(null)
// 打印模板（v0.21）：页眉抬头 + 字段显隐，默认全显示
const shop = ref({ name: '', phone: '', addr: '', logo: '' })
const opt = ref({ showPrice: true, showAmount: true, showSigns: true })

onMounted(async () => {
  const [d, s]: any[] = await Promise.all([
    docApi.get(Number(route.params.id)),
    request.get('/system/settings').catch(() => ({})),
  ])
  doc.value = d
  shop.value = {
    name: s?.shop_name || '',
    phone: s?.shop_phone || '',
    addr: s?.shop_addr || '',
    logo: s?.print_logo || '',
  }
  opt.value = {
    showPrice: s?.a4_show_price !== false,
    showAmount: s?.a4_show_amount !== false,
    showSigns: s?.a4_show_signs !== false,
  }
})

// 明细表总列数（随盘点/单价/金额开关变化，供合计行与填充行对齐）
const totalCols = computed(() => {
  if (!doc.value) return 8
  const base = 5 + (doc.value.doc_type === 'stocktake' ? 1 : 0) + 1 // # 编码 名称 批次 库位 [账面] 数量
  return base + (opt.value.showPrice ? 1 : 0) + (opt.value.showAmount ? 1 : 0)
})
const labelCols = computed(() => totalCols.value - 1 - (opt.value.showPrice ? 1 : 0) - (opt.value.showAmount ? 1 : 0))

function goBack() {
  router.back()
}

const title = computed(() => (doc.value ? docTypeConfig[doc.value.doc_type]?.title || doc.value.doc_type : ''))

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')
const dateText = (d?: string) => (d || '').slice(0, 10)

function doPrint() {
  window.print()
}

// 金额转大写（人民币）
function uppercaseAmount(fen: number): string {
  if (fen <= 0) return '零元整'
  const digits = '零壹贰叁肆伍陆柒捌玖'
  const units = ['', '拾', '佰', '仟']
  const bigUnits = ['', '万', '亿']
  let yuan = Math.floor(fen / 100)
  const jiao = Math.floor((fen % 100) / 10)
  const cent = fen % 10
  let s = ''
  let section = 0
  let sectionAll = ''
  let needZero = false
  if (yuan === 0) {
    s = ''
  } else {
    while (yuan > 0) {
      const part = yuan % 10000
      let partStr = ''
      let zero = false
      for (let u = 0, p = part; p > 0; u++, p = Math.floor(p / 10)) {
        const d = p % 10
        if (d === 0) {
          zero = true
        } else {
          partStr = digits[d] + units[u] + (zero && partStr ? '零' : '') + partStr
          zero = false
        }
      }
      sectionAll = partStr + bigUnits[section] + (part < 1000 && part > 0 && section > 0 ? '零' : '') + sectionAll
      yuan = Math.floor(yuan / 10000)
      section++
    }
    s = sectionAll + '元'
    needZero = jiao === 0 && cent > 0
  }
  if (jiao > 0) s += (needZero ? '零' : '') + digits[jiao] + '角'
  else if (cent > 0 && yuan >= 0 && s) s += '零'
  if (cent > 0) s += digits[cent] + '分'
  else if (!s.includes('角') && !s.includes('分')) s += '整'
  return s
}
</script>

<template>
  <div class="print-page" v-if="doc">
    <div class="toolbar no-print">
      <el-button type="primary" @click="doPrint()">打印</el-button>
      <el-button @click="goBack()">返回</el-button>
      <span class="hint">A4 纵向 · 打印时请选择「无边距」</span>
    </div>

    <div class="sheet">
      <div class="letterhead" v-if="shop.logo || shop.name || shop.phone || shop.addr">
        <img v-if="shop.logo" class="lh-logo" :src="shop.logo" alt="" />
        <div class="lh-text">
          <div class="lh-name" v-if="shop.name">{{ shop.name }}</div>
          <div class="lh-sub" v-if="shop.phone || shop.addr">
            <template v-if="shop.phone">电话：{{ shop.phone }}</template>
            <template v-if="shop.phone && shop.addr"> · </template>
            <template v-if="shop.addr">{{ shop.addr }}</template>
          </div>
        </div>
      </div>

      <div class="doc-head">
        <h1>{{ title }}</h1>
        <div class="status-line">
          单据状态：{{ statusText[doc.status] }}
          <template v-if="doc.status === 'review'">（待主管审批，库存尚未调整）</template>
          <template v-if="doc.status === 'voided'">（作废原因：{{ doc.void_reason || '—' }}）</template>
        </div>
      </div>

      <table class="meta">
        <tr>
          <td><label>单 号</label>{{ doc.doc_no }}</td>
          <td><label>日 期</label>{{ dateText(doc.doc_date) }}</td>
          <td><label>{{ doc.partner_type === 'supplier' ? '供应商' : '客户' }}</label>{{ doc.partner_name || '—' }}</td>
        </tr>
      </table>

      <table class="items">
        <thead>
          <tr>
            <th style="width: 36px">#</th>
            <th style="width: 100px">编码</th>
            <th>商品名称</th>
            <th style="width: 70px">批次</th>
            <th style="width: 90px">库位</th>
            <th v-if="doc.doc_type === 'stocktake'" style="width: 80px">账面数</th>
            <th style="width: 80px">{{ doc.doc_type === 'stocktake' ? '实盘数' : '数量' }}</th>
            <th v-if="opt.showPrice" style="width: 90px">单价</th>
            <th v-if="opt.showAmount" style="width: 100px">金额</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(it, i) in doc.items" :key="it.id">
            <td>{{ i + 1 }}</td>
            <td>{{ it.product_code }}</td>
            <td>{{ it.product_name }}</td>
            <td>{{ it.batch_no || '—' }}</td>
            <td>
              <template v-if="doc?.doc_type === 'transfer'">
                {{ it.location_name || '未分配' }} → {{ it.to_location_name || '未分配' }}
              </template>
              <template v-else>{{ it.location_name || '—' }}</template>
            </td>
            <td v-if="doc.doc_type === 'stocktake'">{{ qtyText(it.book_qty) }}</td>
            <td>{{ qtyText(it.quantity) }}</td>
            <td v-if="opt.showPrice">{{ fenToYuan(it.price) }}</td>
            <td v-if="opt.showAmount">{{ fenToYuan(it.amount) }}</td>
          </tr>
          <tr v-for="i in Math.max(0, 6 - (doc.items?.length || 0))" :key="'pad' + i" class="pad-row">
            <td :colspan="totalCols">&nbsp;</td>
          </tr>
        </tbody>
        <tfoot>
          <tr>
            <td :colspan="labelCols">合计</td>
            <td>{{ qtyText(doc.total_qty) }}</td>
            <td v-if="opt.showPrice"></td>
            <td v-if="opt.showAmount">¥{{ fenToYuan(doc.total_amount) }}</td>
          </tr>
        </tfoot>
      </table>

      <div class="uppercase" v-if="opt.showAmount">大写金额：{{ uppercaseAmount(doc.total_amount) }}</div>
      <div v-if="doc.remark" class="remark">备注：{{ doc.remark }}</div>

      <div class="signs" v-if="opt.showSigns">
        <span>制单：__________</span>
        <span>审核：__________</span>
        <span>客户/供应商签字：__________</span>
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
}
.hint {
  color: #888;
  font-size: 12px;
  margin-left: auto;
}
.sheet {
  width: 794px;
  min-height: 1050px;
  margin: 0 auto;
  background: #fff;
  padding: 48px 56px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  font-size: 13px;
  color: #111;
}
.letterhead {
  display: flex;
  align-items: center;
  gap: 14px;
  border-bottom: 2px solid #333;
  padding-bottom: 10px;
  margin-bottom: 16px;
}
.lh-logo {
  max-height: 44px;
  max-width: 160px;
}
.lh-name {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 1px;
}
.lh-sub {
  font-size: 12px;
  color: #555;
  margin-top: 2px;
}
.doc-head {
  text-align: center;
  margin-bottom: 20px;
}
.doc-head h1 {
  font-size: 26px;
  letter-spacing: 12px;
  margin: 0 0 6px;
}
.status-line {
  color: #666;
  font-size: 12px;
}
table.meta {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 14px;
}
table.meta td {
  padding: 4px 8px;
  font-size: 13px;
}
table.meta label {
  color: #666;
  margin-right: 10px;
}
table.items {
  width: 100%;
  border-collapse: collapse;
}
table.items th,
table.items td {
  border: 1px solid #333;
  padding: 6px 8px;
  text-align: left;
}
table.items th {
  background: #f2f2f2;
  font-weight: 600;
}
table.items td:nth-child(n + 5),
table.items th:nth-child(n + 5) {
  text-align: right;
}
.pad-row td {
  border: 1px solid #333;
}
tfoot td {
  font-weight: 700;
  border: 1px solid #333;
  padding: 6px 8px;
}
.uppercase {
  margin-top: 12px;
  font-size: 13px;
}
.remark {
  margin-top: 6px;
  color: #333;
}
.signs {
  display: flex;
  justify-content: space-between;
  margin-top: 80px;
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
