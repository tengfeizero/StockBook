<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { financeApi, methodText, type PartnerBalance, type Payment } from '../api/finance'
import { fenToYuan, yuanToFen } from '../utils/money'

const activeTab = ref<'customer' | 'supplier'>('customer')
const balances = ref<PartnerBalance[]>([])
const loading = ref(false)

// 收付款对话框
const payDialog = ref(false)
const payForm = reactive({ partner_id: 0, partner_name: '', amount: '', method: 'cash', remark: '' })

// 收付款记录
const records = ref<Payment[]>([])
const recTotal = ref(0)
const recQuery = reactive({ page: 1, page_size: 10 })
const recLoading = ref(false)

async function loadBalances() {
  loading.value = true
  try {
    balances.value = (await financeApi.balances(activeTab.value)) || []
  } finally {
    loading.value = false
  }
}

function openPay(row: PartnerBalance) {
  payForm.partner_id = row.partner_id
  payForm.partner_name = row.name
  payForm.amount = ''
  payForm.method = 'cash'
  payForm.remark = ''
  payDialog.value = true
}

async function submitPay() {
  const fen = yuanToFen(payForm.amount)
  if (!fen || fen <= 0) {
    ElMessage.warning('请输入正确金额')
    return
  }
  await financeApi.createPayment({
    kind: activeTab.value === 'customer' ? 'receipt' : 'payment',
    partner_id: payForm.partner_id,
    amount: fen,
    method: payForm.method,
    remark: payForm.remark,
  })
  ElMessage.success('已保存')
  payDialog.value = false
  loadBalances()
  loadRecords()
}

async function loadRecords() {
  recLoading.value = true
  try {
    const data: any = await financeApi.payments({ page: recQuery.page, page_size: recQuery.page_size })
    records.value = data.list
    recTotal.value = data.total
  } finally {
    recLoading.value = false
  }
}

async function voidPayment(row: Payment) {
  const { value } = await ElMessageBox.prompt(`作废 ${row.doc_no}（¥${fenToYuan(row.amount)}），作废后不再计入已收付。请输入原因：`, '作废收付款单', {
    inputPlaceholder: '作废原因（可空）',
    type: 'warning',
  })
  await financeApi.voidPayment(row.id, value || '')
  ElMessage.success('已作废')
  loadBalances()
  loadRecords()
}

const tabTitle = () => (activeTab.value === 'customer' ? '应收余额' : '应付余额')
const settleText = () => (activeTab.value === 'customer' ? '已收款' : '已付款')

function switchTab() {
  loadBalances()
}

onMounted(() => {
  loadBalances()
  loadRecords()
})
</script>

<template>
  <el-card shadow="never">
    <el-tabs v-model="activeTab" @tab-change="switchTab">
      <el-tab-pane label="客户应收" name="customer" />
      <el-tab-pane label="供应商应付" name="supplier" />
    </el-tabs>

    <el-table :data="balances" v-loading="loading" border size="small" :empty-text="'暂无' + (activeTab === 'customer' ? '客户' : '供应商') + '档案'">
      <el-table-column prop="code" label="编码" width="110" />
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column label="期初" width="110" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.opening) }}</template>
      </el-table-column>
      <el-table-column label="单据发生" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.biz) }}</template>
      </el-table-column>
      <el-table-column :label="settleText()" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.settled) }}</template>
      </el-table-column>
      <el-table-column label="余额" width="120" align="right">
        <template #default="{ row }">
          <b :style="{ color: row.balance > 0 ? 'var(--el-color-danger)' : 'var(--el-color-success)' }">¥{{ fenToYuan(row.balance) }}</b>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openPay(row)">{{ activeTab === 'customer' ? '收款' : '付款' }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top: 8px; color: var(--el-text-color-secondary); font-size: 12px">
      {{ tabTitle() }} = 期初 + 单据发生 − {{ settleText() }}；红色表示{{ activeTab === 'customer' ? '客户尚欠' : '尚欠供应商' }}。
    </div>
  </el-card>

  <el-card shadow="never" style="margin-top: 16px">
    <template #header>收付款记录</template>
    <el-table :data="records" v-loading="recLoading" border size="small">
      <el-table-column prop="doc_no" label="单号" width="160" />
      <el-table-column label="类型" width="90">
        <template #default="{ row }">
          <el-tag :type="row.kind === 'receipt' ? 'success' : 'warning'" size="small">{{ row.kind === 'receipt' ? '收款' : '付款' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="partner_name" label="往来单位" min-width="140" />
      <el-table-column label="金额" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.amount) }}</template>
      </el-table-column>
      <el-table-column label="方式" width="90">
        <template #default="{ row }">{{ methodText[row.method] || row.method || '—' }}</template>
      </el-table-column>
      <el-table-column label="日期" width="110">
        <template #default="{ row }">{{ (row.doc_date || '').slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="120" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'confirmed' ? 'success' : 'danger'" size="small">{{ row.status === 'confirmed' ? '有效' : '已作废' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'confirmed'" link type="danger" @click="voidPayment(row)">作废</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="recQuery.page"
      :page-size="recQuery.page_size"
      :total="recTotal"
      layout="total, prev, pager, next"
      style="margin-top: 12px; justify-content: flex-end"
      @current-change="loadRecords"
    />
  </el-card>

  <el-dialog v-model="payDialog" :title="activeTab === 'customer' ? '收款' : '付款' + ' — ' + payForm.partner_name" width="420px">
    <el-form label-width="70px">
      <el-form-item label="金额">
        <el-input v-model="payForm.amount" placeholder="元" type="number" />
      </el-form-item>
      <el-form-item label="方式">
        <el-select v-model="payForm.method" style="width: 100%">
          <el-option label="现金" value="cash" />
          <el-option label="微信" value="wechat" />
          <el-option label="支付宝" value="alipay" />
          <el-option label="银行转账" value="bank" />
        </el-select>
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="payForm.remark" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="payDialog = false">取消</el-button>
      <el-button type="primary" @click="submitPay">保存</el-button>
    </template>
  </el-dialog>
</template>
