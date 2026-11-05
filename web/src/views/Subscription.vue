<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import request from '../utils/request'

const auth = useAuthStore()

interface Plan {
  code: string
  name: string
  price_monthly: number
  max_users: number
  max_products: number
  features: string
}
interface Current {
  standalone?: boolean
  code?: string
  name?: string
  plan_code?: string
  plan_name?: string
  price_monthly?: number
  trial?: boolean
  expire_at?: string
  days_left?: number
  expired?: boolean
  max_users?: number
  user_count?: number
  product_count?: number
  max_products?: number
}
interface Bill {
  id: number
  bill_no: string
  plan_name: string
  months: number
  amount: number
  status: string
  period_start: string
  period_end: string
  operator: string
  created_at: string
}

const loading = ref(false)
const cur = ref<Current>({})
const plans = ref<Plan[]>([])
const bills = ref<Bill[]>([])
const selected = ref('')
const months = ref(12)
const submitting = ref(false)

const yuan = (fen?: number) => `¥${((fen || 0) / 100).toFixed(2)}`
const day = (s?: string) => (s ? new Date(s).toLocaleDateString('zh-CN') : '-')
const canPay = computed(() => auth.hasPerm(['system']))

const expireTag = computed(() => {
  if (cur.value.standalone) return null
  if (cur.value.expired) return { type: 'danger' as const, text: '已到期（只读）' }
  const d = cur.value.days_left ?? 0
  if (d <= 7) return { type: 'warning' as const, text: `剩 ${d} 天` }
  return { type: 'success' as const, text: `剩 ${d} 天` }
})

const amount = computed(() => {
  const p = plans.value.find((x) => x.code === selected.value)
  return (p?.price_monthly || 0) * months.value
})

async function load() {
  loading.value = true
  try {
    plans.value = ((await request.get('/tenants/plans')) as Plan[]) || []
    cur.value = (await request.get('/tenants/current')) as Current
    if (cur.value.standalone) return
    bills.value = ((await request.get('/tenants/bills')) as Bill[]) || []
    selected.value = cur.value.plan_code || plans.value[0]?.code || ''
  } finally {
    loading.value = false
  }
}

async function subscribe() {
  if (!selected.value) return
  const plan = plans.value.find((p) => p.code === selected.value)
  if (!plan) return
  await ElMessageBox.confirm(
    `确认购买「${plan.name}」${months.value} 个月，金额 ${yuan(amount.value)}？\n（当前为演示支付：确认后立即生效并记账单，接入微信/支付宝后改为在线支付）`,
    '确认续费',
    { type: 'warning', confirmButtonText: '确认支付', cancelButtonText: '取消' },
  )
  submitting.value = true
  try {
    await request.post('/tenants/subscribe', { plan_code: selected.value, months: months.value })
    ElMessage.success('续费成功，套餐已生效')
    await load()
    await auth.loadProfile()
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-loading="loading">
    <el-alert
      v-if="cur.standalone"
      type="info"
      :closable="false"
      show-icon
      title="当前为单机版部署：使用授权文件计费，无需订阅"
      style="margin-bottom: 16px"
    >
      如需多企业独立账套、在线续费与用量额度，请使用 SaaS 部署方式。
    </el-alert>

    <template v-else>
      <el-row :gutter="16">
        <el-col :span="10">
          <el-card>
            <template #header>
              <div class="card-head">
                <span>当前订阅</span>
                <el-tag v-if="expireTag" :type="expireTag.type" size="small">{{ expireTag.text }}</el-tag>
              </div>
            </template>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="企业">{{ cur.name }}（{{ cur.code }}）</el-descriptions-item>
              <el-descriptions-item label="套餐">
                {{ cur.plan_name }}
                <el-tag v-if="cur.trial" size="small" type="info" style="margin-left: 6px">试用</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="到期时间">{{ day(cur.expire_at) }}</el-descriptions-item>
              <el-descriptions-item label="用户数">
                {{ cur.user_count }} / {{ cur.max_users || '不限' }}
              </el-descriptions-item>
              <el-descriptions-item label="商品数">
                {{ cur.product_count }} / {{ cur.max_products || '不限' }}
              </el-descriptions-item>
            </el-descriptions>
          </el-card>
        </el-col>
        <el-col :span="14">
          <el-card>
            <template #header><span>续费 / 升级</span></template>
            <el-radio-group v-model="selected" class="plan-group">
              <el-radio v-for="p in plans" :key="p.code" :value="p.code" border class="plan-item">
                <div class="plan-name">
                  {{ p.name }}
                  <span class="plan-price">{{ p.price_monthly ? yuan(p.price_monthly) + '/月' : '免费' }}</span>
                </div>
                <div class="plan-desc">{{ p.features.split('\n').join(' · ') }}</div>
              </el-radio>
            </el-radio-group>
            <div class="buy-row">
              <span>购买月数</span>
              <el-input-number v-model="months" :min="1" :max="24" size="default" />
              <span class="total">合计 <b>{{ yuan(amount) }}</b></span>
              <el-button type="primary" :loading="submitting" :disabled="!canPay" @click="subscribe">
                立即续费
              </el-button>
            </div>
            <div v-if="!canPay" class="no-perm">仅管理员（system 权限）可操作续费</div>
          </el-card>
        </el-col>
      </el-row>

      <el-card style="margin-top: 16px">
        <template #header><span>账单记录</span></template>
        <el-table :data="bills" size="small" empty-text="暂无账单">
          <el-table-column prop="bill_no" label="账单号" min-width="180" />
          <el-table-column prop="plan_name" label="套餐" width="100" />
          <el-table-column prop="months" label="月数" width="70" align="right" />
          <el-table-column label="金额" width="110" align="right">
            <template #default="{ row }">{{ yuan(row.amount) }}</template>
          </el-table-column>
          <el-table-column label="服务期" min-width="200">
            <template #default="{ row }">{{ day(row.period_start) }} ~ {{ day(row.period_end) }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'paid' ? 'success' : 'warning'">
                {{ row.status === 'paid' ? '已支付' : '待支付' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="operator" label="操作人" width="100" />
        </el-table>
      </el-card>
    </template>
  </div>
</template>

<style scoped>
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.plan-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.plan-item {
  width: 100%;
  height: auto;
  margin-right: 0;
  padding: 8px 12px;
  align-items: flex-start;
}
.plan-name {
  font-weight: 600;
}
.plan-price {
  margin-left: 8px;
  font-weight: 400;
  color: var(--el-color-primary);
}
.plan-desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  white-space: normal;
}
.buy-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  font-size: 13px;
}
.total b {
  color: var(--el-color-danger);
  font-size: 16px;
}
.no-perm {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
</style>
