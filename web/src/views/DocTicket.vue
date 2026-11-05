<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { docApi, docTypeConfig, statusText, type Doc } from '../api/doc'
import { fenToYuan } from '../utils/money'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const doc = ref<Doc | null>(null)
const shop = ref({ name: '', phone: '', addr: '', footer: '', logo: '' })
// 打印模板（v0.21）：字段显隐，默认全显示
const opt = ref({ showPrice: true, showAmount: true, showPartner: true, showBatch: true })
const printedAt = ref('')

function fmtNow() {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

onMounted(async () => {
  // 单据与抬头并行取：抬头配置在系统设置里，未配置时票头自动省略
  const [d, s]: any[] = await Promise.all([
    docApi.get(Number(route.params.id)),
    request.get('/system/settings').catch(() => ({})),
  ])
  doc.value = d
  shop.value = {
    name: s?.shop_name || '',
    phone: s?.shop_phone || '',
    addr: s?.shop_addr || '',
    footer: s?.ticket_footer || '',
    logo: s?.print_logo || '',
  }
  opt.value = {
    showPrice: s?.ticket_show_price !== false,
    showAmount: s?.ticket_show_amount !== false,
    showPartner: s?.ticket_show_partner !== false,
    showBatch: s?.ticket_show_batch !== false,
  }
  printedAt.value = fmtNow()
})

const title = computed(() => (doc.value ? docTypeConfig[doc.value.doc_type]?.title || doc.value.doc_type : ''))
const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')
const money = (fen: number) => fenToYuan(fen)
const dateText = (d?: string) => (d || '').slice(0, 10)
// 草稿/作废票必须显眼标注，避免被当结算凭证拿走
const abnormal = computed(() => (doc.value ? doc.value.status !== 'confirmed' : false))

const doPrint = () => window.print()
const goA4 = () => doc.value && router.push(`/print/${doc.value.id}`)
</script>

<template>
  <div class="ticket-page" v-if="doc">
    <div class="toolbar no-print">
      <el-button type="primary" @click="doPrint">打印小票</el-button>
      <el-button @click="router.back()">返回</el-button>
      <el-button text @click="goA4">改打 A4 单据</el-button>
      <span class="hint">
        纸张选「58mm × 自动」· 缩放 100% · 边距「无」· 关闭「页眉页脚」
      </span>
    </div>

    <div class="paper">
      <div class="ticket">
        <div class="shop-head">
          <img v-if="shop.logo" class="shop-logo" :src="shop.logo" alt="" />
          <template v-if="shop.name || shop.phone || shop.addr">
            <div class="shop-name" v-if="shop.name">{{ shop.name }}</div>
            <div class="center sub" v-if="shop.phone">电话 {{ shop.phone }}</div>
            <div class="center sub" v-if="shop.addr">{{ shop.addr }}</div>
          </template>
        </div>
        <div class="cut" v-if="shop.logo || shop.name || shop.phone || shop.addr"></div>

        <div class="doc-title">{{ title }}</div>
        <div class="mark" v-if="abnormal">{{ statusText[doc.status] }} · 不作结算凭证</div>

        <div class="kv"><span>单号</span><span class="v">{{ doc.doc_no }}</span></div>
        <div class="kv"><span>日期</span><span class="v">{{ dateText(doc.doc_date) }}</span></div>
        <div class="kv" v-if="opt.showPartner && doc.partner_name">
          <span>{{ doc.partner_type === 'supplier' ? '供应商' : '客户' }}</span>
          <span class="v">{{ doc.partner_name }}</span>
        </div>
        <div class="cut"></div>

        <div class="item" v-for="(it, i) in doc.items" :key="it.id">
          <div class="name">{{ i + 1 }}. {{ it.product_name }}</div>
          <div class="kv" v-if="doc.doc_type === 'transfer'">
            <span>{{ it.location_name || '未分配' }} → {{ it.to_location_name || '未分配' }}</span>
            <span class="v">{{ qtyText(it.quantity) }}</span>
          </div>
          <div class="kv" v-else-if="opt.showPrice">
            <span>{{ qtyText(it.quantity) }} × {{ money(it.price) }}</span>
            <span class="v" v-if="opt.showAmount">{{ money(it.amount) }}</span>
            <span class="v" v-else>{{ qtyText(it.quantity) }}</span>
          </div>
          <div class="kv" v-else>
            <span>{{ qtyText(it.quantity) }}</span>
            <span class="v" v-if="opt.showAmount">{{ money(it.amount) }}</span>
          </div>
          <div class="mini" v-if="opt.showBatch && it.batch_no">批次 {{ it.batch_no }}</div>
        </div>
        <div class="cut"></div>

        <div class="kv"><span>件数</span><span class="v">{{ qtyText(doc.total_qty) }}</span></div>
        <div class="kv grand" v-if="doc.doc_type !== 'transfer' && opt.showAmount">
          <span>合计</span><span class="v">￥{{ money(doc.total_amount) }}</span>
        </div>

        <template v-if="doc.remark">
          <div class="cut"></div>
          <div class="note">备注：{{ doc.remark }}</div>
        </template>

        <div class="cut"></div>
        <div class="center footer" v-if="shop.footer">{{ shop.footer }}</div>
        <div class="center mini">打印时间 {{ printedAt }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ticket-page {
  min-height: 100vh;
  background: #e8e8e8;
  padding: 24px 0 60px;
}
.toolbar {
  width: 380px;
  margin: 0 auto 12px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.hint {
  flex-basis: 100%;
  color: #888;
  font-size: 12px;
  line-height: 1.5;
}
/* 58mm 热敏纸：纸宽 58mm、可打印宽度约 48mm（左右各 4.5mm 撕边不可打） */
.paper {
  width: 58mm;
  margin: 0 auto;
  padding: 4mm 5mm 8mm;
  background: #fff;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
}
.ticket {
  color: #000;
  font-family: 'Courier New', Consolas, monospace;
  font-size: 11px;
  line-height: 1.55;
}
.shop-head {
  text-align: center;
}
.shop-logo {
  max-width: 100%;
  max-height: 12mm;
  margin-bottom: 1mm;
}
.shop-name {
  text-align: center;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 1px;
  margin-bottom: 1mm;
}
.center {
  text-align: center;
}
.sub {
  font-size: 10px;
  line-height: 1.4;
  word-break: break-all;
}
.doc-title {
  text-align: center;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 2px;
  margin: 0 0 1mm;
}
.mark {
  border: 1px solid #000;
  text-align: center;
  font-weight: 700;
  letter-spacing: 1px;
  padding: 1mm 0;
  margin: 1mm 0;
}
.kv {
  display: flex;
  justify-content: space-between;
  gap: 4px;
}
.kv > .v {
  text-align: right;
  word-break: break-all;
}
.item {
  margin-bottom: 1mm;
}
.name {
  word-break: break-all;
}
.mini {
  font-size: 9px;
  line-height: 1.4;
  color: #000;
}
.note {
  word-break: break-all;
}
.grand {
  font-size: 13px;
  font-weight: 700;
}
.footer {
  margin-bottom: 0.5mm;
}
.cut {
  border-top: 1px dashed #000;
  margin: 1.5mm 0;
}

@media print {
  @page {
    size: 58mm auto;
    margin: 0;
  }
  .ticket-page {
    background: #fff;
    padding: 0;
    min-height: 0;
  }
  .no-print {
    display: none !important;
  }
  .paper {
    width: 58mm;
    padding: 2mm 5mm 6mm;
    box-shadow: none;
  }
  .ticket {
    font-size: 11px;
  }
}
</style>
