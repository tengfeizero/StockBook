<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import { scaleApi, type ScaleParseResult, type ScaleRule } from '../api/archive'

// ---- 修改密码 ----
const pwdForm = reactive({ old_password: '', new_password: '', confirm: '' })
async function changePwd() {
  if (pwdForm.new_password.length < 6) {
    ElMessage.warning('新密码至少6位')
    return
  }
  if (pwdForm.new_password !== pwdForm.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  await request.post('/auth/password', { old_password: pwdForm.old_password, new_password: pwdForm.new_password })
  ElMessage.success('密码已修改')
  pwdForm.old_password = pwdForm.new_password = pwdForm.confirm = ''
}

// ---- 业务设置 ----
const settings = reactive({
  allow_negative_stock: false, shelf_life_warn_days: 7, location_block_overflow: false,
  shop_name: '', shop_phone: '', shop_addr: '', ticket_footer: '',
  stocktake_review_enabled: false, stocktake_review_percent: 10, stocktake_review_amount_yuan: 0,
  print_logo: '',
  ticket_show_price: true, ticket_show_amount: true, ticket_show_partner: true, ticket_show_batch: true,
  a4_show_price: true, a4_show_amount: true, a4_show_signs: true,
})
const settingsLoading = ref(false)
async function loadSettings() {
  const data: any = await request.get('/system/settings')
  settings.allow_negative_stock = data.allow_negative_stock
  settings.shelf_life_warn_days = data.shelf_life_warn_days
  settings.location_block_overflow = !!data.location_block_overflow
  settings.shop_name = data.shop_name || ''
  settings.shop_phone = data.shop_phone || ''
  settings.shop_addr = data.shop_addr || ''
  settings.ticket_footer = data.ticket_footer || ''
  settings.stocktake_review_enabled = !!data.stocktake_review_enabled
  settings.stocktake_review_percent = data.stocktake_review_percent ?? 10
  settings.stocktake_review_amount_yuan = (data.stocktake_review_amount || 0) / 100
  settings.print_logo = data.print_logo || ''
  settings.ticket_show_price = data.ticket_show_price !== false
  settings.ticket_show_amount = data.ticket_show_amount !== false
  settings.ticket_show_partner = data.ticket_show_partner !== false
  settings.ticket_show_batch = data.ticket_show_batch !== false
  settings.a4_show_price = data.a4_show_price !== false
  settings.a4_show_amount = data.a4_show_amount !== false
  settings.a4_show_signs = data.a4_show_signs !== false
}
async function saveSettings() {
  settingsLoading.value = true
  try {
    const { stocktake_review_amount_yuan, ...rest } = settings
    await request.put('/system/settings', {
      ...rest,
      stocktake_review_amount: Math.round(stocktake_review_amount_yuan * 100),
    })
    ElMessage.success('已保存')
  } finally {
    settingsLoading.value = false
  }
}

// ---- 打印模板（v0.21）：Logo 上传（canvas 压缩为 data URL） ----
const logoInput = ref<HTMLInputElement>()
function pickLogo() {
  logoInput.value?.click()
}
function onLogoFile(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  ;(e.target as HTMLInputElement).value = ''
  if (!file) return
  if (!file.type.startsWith('image/')) {
    ElMessage.warning('请选择图片文件')
    return
  }
  const img = new Image()
  img.onload = () => {
    // 等比压缩到宽 ≤ 240px（小票/A4 页头足够），PNG 保留透明背景
    const w = Math.min(240, img.width)
    const h = Math.round((img.height * w) / img.width)
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    canvas.getContext('2d')!.drawImage(img, 0, 0, w, h)
    settings.print_logo = canvas.toDataURL('image/png')
  }
  img.src = URL.createObjectURL(file)
}
function clearLogo() {
  settings.print_logo = ''
}
// 预览：打开最近一张已确认单据的 58mm 小票
async function previewTicket() {
  await saveSettings()
  const list: any = await request.get('/docs?page=1&page_size=1&doc_type=sales_out&status=confirmed')
  const id = list?.list?.[0]?.id
  if (!id) {
    ElMessage.warning('还没有已确认的单据，先开一张销售单再预览')
    return
  }
  window.open(`/print/${id}/ticket`, '_blank')
}

// ---- 备份 ----
interface BackupFile { name: string; size: number; time: string }
const backups = ref<BackupFile[]>([])
const backupLoading = ref(false)
async function loadBackups() {
  backups.value = (await request.get('/system/backups')) as any
}
async function createBackup() {
  backupLoading.value = true
  try {
    const b: any = await request.post('/system/backups')
    ElMessage.success(`备份完成：${b.name}`)
    loadBackups()
  } finally {
    backupLoading.value = false
  }
}
function downloadBackup(name: string) {
  // 带鉴权下载：fetch → blob
  import('../utils/request').then(({ default: req }) => {
    req.get(`/system/backups/${name}`, { responseType: 'blob' }).then((blob: any) => {
      const url = URL.createObjectURL(blob as Blob)
      const a = document.createElement('a')
      a.href = url
      a.download = name
      a.click()
      URL.revokeObjectURL(url)
    })
  })
}

// ---- 授权（M5） ----
interface LicenseStatus {
  mode: 'licensed' | 'trial' | 'expired'
  owner?: string
  expires_at?: string
  trial_left: number
  machine_code: string
}
const lic = ref<LicenseStatus | null>(null)
const licFile = ref<File | null>(null)
const licUploading = ref(false)
async function loadLicense() {
  lic.value = (await request.get('/system/license')) as any
}
async function uploadLicense() {
  if (!licFile.value) return
  licUploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', licFile.value)
    lic.value = (await request.post('/system/license', fd)) as any
    ElMessage.success('授权已生效，感谢支持！')
    licFile.value = null
  } finally {
    licUploading.value = false
  }
}

// ---- 秤码规则（v0.11） ----
const rules = ref<ScaleRule[]>([])
const ruleLoading = ref(false)
const ruleDialog = ref(false)
const ruleSaving = ref(false)

const emptyRule = (): Partial<ScaleRule> => ({
  name: '', brand: '', model: '', code_len: 13, prefixes: '22',
  plu_start: 3, plu_len: 5,
  amount_start: 8, amount_len: 5, amount_decimals: 2,
  weight_start: 0, weight_len: 0, weight_decimals: 3,
  check_type: 'mod10', enabled: true, is_default: false, sort: 0, remark: '',
})
const ruleForm = reactive<Partial<ScaleRule>>(emptyRule())

// 数据段类型：金额 / 重量 / 两者都有（18 位码）
const segType = computed<string>({
  get: () => (ruleForm.amount_start ? (ruleForm.weight_start ? 'both' : 'amount') : 'weight'),
  set: (v: string) => {
    if (v === 'amount') {
      Object.assign(ruleForm, { amount_start: 8, amount_len: 5, amount_decimals: 2, weight_start: 0, weight_len: 0 })
    } else if (v === 'weight') {
      Object.assign(ruleForm, { weight_start: 8, weight_len: 5, weight_decimals: 3, amount_start: 0, amount_len: 0 })
    } else {
      Object.assign(ruleForm, {
        amount_start: 8, amount_len: 5, amount_decimals: 2,
        weight_start: 13, weight_len: 5, weight_decimals: 3,
      })
    }
  },
})

async function loadRules() {
  ruleLoading.value = true
  try {
    rules.value = ((await scaleApi.list()) as any) || []
  } finally {
    ruleLoading.value = false
  }
}

function openRule(r?: ScaleRule) {
  Object.assign(ruleForm, emptyRule(), r ? { ...r } : {})
  ruleDialog.value = true
}

async function saveRule() {
  if (!ruleForm.name || !ruleForm.name.trim()) return ElMessage.warning('请填写规则名称')
  if (!ruleForm.amount_start && !ruleForm.weight_start) return ElMessage.warning('金额段与重量段至少配置一项')
  ruleSaving.value = true
  try {
    const { id, ...payload } = ruleForm as any
    if (id) await scaleApi.update(id, payload)
    else await scaleApi.create(payload)
    ElMessage.success('规则已保存')
    ruleDialog.value = false
    loadRules()
  } finally {
    ruleSaving.value = false
  }
}

async function removeRule(r: ScaleRule) {
  await ElMessageBox.confirm(`确定删除规则「${r.name}」？删除后该码制将无法解析。`, '删除确认', { type: 'warning' })
  await scaleApi.remove(r.id)
  ElMessage.success('已删除')
  loadRules()
}

// 测试解析
const testCode = ref('')
const testing = ref(false)
const testResult = ref<ScaleParseResult | null>(null)
async function runTest() {
  const code = testCode.value.trim()
  if (!code) return ElMessage.warning('请输入或扫描秤码')
  testing.value = true
  try {
    testResult.value = (await scaleApi.parse(code)) as any
  } finally {
    testing.value = false
  }
}

const qtyText = (r: ScaleParseResult) => (r.quantity / 1000).toFixed(3).replace(/\.?0+$/, '')
function segSummary(r: ScaleRule) {
  const parts: string[] = []
  if (r.amount_start) parts.push(`金额 ${r.amount_start}-${r.amount_start + r.amount_len - 1} 位/${r.amount_decimals} 小数`)
  if (r.weight_start) parts.push(`重量 ${r.weight_start}-${r.weight_start + r.weight_len - 1} 位/${r.weight_decimals} 小数`)
  return parts.join('，')
}

onMounted(() => {
  loadSettings()
  loadBackups()
  loadLicense()
  loadRules()
})
</script>

<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="12">
        <el-card shadow="never">
          <template #header>修改密码</template>
          <el-form label-width="90px" style="max-width: 380px">
            <el-form-item label="当前密码">
              <el-input v-model="pwdForm.old_password" type="password" show-password />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input v-model="pwdForm.new_password" type="password" show-password placeholder="至少6位" />
            </el-form-item>
            <el-form-item label="确认新密码">
              <el-input v-model="pwdForm.confirm" type="password" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="changePwd">保存新密码</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" style="margin-top: 16px">
          <template #header>业务规则设置</template>
          <el-form label-width="150px" v-loading="settingsLoading">
            <el-form-item label="允许负库存销售">
              <el-switch v-model="settings.allow_negative_stock" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                开启后库存不足仍可销售（小店先卖后补单的场景）
              </span>
            </el-form-item>
            <el-form-item label="临期预警天数">
              <el-input-number v-model="settings.shelf_life_warn_days" :min="1" :max="90" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                批次商品到期前 N 天预警（M3 后续接入预警展示）
              </span>
            </el-form-item>
            <el-form-item label="库位超容时禁止作业">
              <el-switch v-model="settings.location_block_overflow" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                关闭（默认）只在库位页提醒；开启后入库/移入超过库位容量直接拦截
              </span>
            </el-form-item>
            <el-form-item label="盘点差异需主管审批">
              <el-switch v-model="settings.stocktake_review_enabled" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                开启后，盘点差异超过下列任一阈值的盘点单不直接调账，转「待审批」，主管通过后才调账
              </span>
            </el-form-item>
            <el-form-item label="差异率阈值">
              <el-input-number v-model="settings.stocktake_review_percent" :min="0" :max="100" :disabled="!settings.stocktake_review_enabled" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                |差异| ÷ 账面数 超过该百分比即触发；0 = 不按差异率
              </span>
            </el-form-item>
            <el-form-item label="差异金额阈值(元)">
              <el-input-number v-model="settings.stocktake_review_amount_yuan" :min="0" :precision="2" :step="50" :disabled="!settings.stocktake_review_enabled" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">
                按仓库均价估算的差异金额 ≥ 该值即触发；0 = 不按金额
              </span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveSettings">保存设置</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" style="margin-top: 16px">
          <template #header>
            小票抬头（58mm 热敏小票）
            <el-tooltip content="打印小票时打印在票头，留空则不打印该行" placement="top">
              <span style="color: var(--el-text-color-secondary); font-size: 12px; margin-left: 8px">留空即不显示</span>
            </el-tooltip>
          </template>
          <el-form label-width="150px" v-loading="settingsLoading">
            <el-form-item label="店铺名称">
              <el-input v-model="settings.shop_name" maxlength="24" show-word-limit placeholder="如：三生顺五金机电" />
            </el-form-item>
            <el-form-item label="联系电话">
              <el-input v-model="settings.shop_phone" maxlength="24" placeholder="如：0573-8888 6666" />
            </el-form-item>
            <el-form-item label="店铺地址">
              <el-input v-model="settings.shop_addr" maxlength="40" placeholder="如：嘉兴市南湖区建国北路 128 号" />
            </el-form-item>
            <el-form-item label="票尾寄语">
              <el-input v-model="settings.ticket_footer" maxlength="30" placeholder="如：谢谢惠顾，欢迎再来" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveSettings">保存设置</el-button>
            </el-form-item>
          </el-form>
        </el-card>
        <el-card shadow="never" style="margin-top: 16px">
          <template #header>
            打印模板（58mm 小票 + A4 单据）
            <el-tooltip content="控制打印页出现哪些内容；改成送货单等不需要金额的场景时，关闭金额即可" placement="top">
              <span style="color: var(--el-text-color-secondary); font-size: 12px; margin-left: 8px">字段显隐即时生效</span>
            </el-tooltip>
          </template>
          <el-form label-width="150px" v-loading="settingsLoading">
            <el-form-item label="打印 Logo">
              <div class="logo-box">
                <img v-if="settings.print_logo" :src="settings.print_logo" class="logo-preview" alt="logo" />
                <div v-else class="logo-empty">未设置</div>
                <div class="logo-ops">
                  <el-button size="small" @click="pickLogo">{{ settings.print_logo ? '更换图片' : '上传图片' }}</el-button>
                  <el-button v-if="settings.print_logo" size="small" @click="clearLogo">清除</el-button>
                  <div style="margin-top: 6px; color: var(--el-text-color-secondary); font-size: 12px">
                    自动压缩到 240px 宽，建议透明底 PNG<br />小票票头与 A4 单据页眉都会打印
                  </div>
                </div>
              </div>
              <input ref="logoInput" type="file" accept="image/*" style="display: none" @change="onLogoFile" />
            </el-form-item>
            <el-divider content-position="left" style="margin: 8px 0">58mm 小票</el-divider>
            <el-form-item label="显示单价"><el-switch v-model="settings.ticket_show_price" /></el-form-item>
            <el-form-item label="显示金额与合计">
              <el-switch v-model="settings.ticket_show_amount" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">送货单场景可关闭</span>
            </el-form-item>
            <el-form-item label="显示客户名称"><el-switch v-model="settings.ticket_show_partner" /></el-form-item>
            <el-form-item label="显示批次号"><el-switch v-model="settings.ticket_show_batch" /></el-form-item>
            <el-divider content-position="left" style="margin: 8px 0">A4 单据</el-divider>
            <el-form-item label="显示单价列"><el-switch v-model="settings.a4_show_price" /></el-form-item>
            <el-form-item label="显示金额列与大写">
              <el-switch v-model="settings.a4_show_amount" />
              <span style="margin-left: 10px; color: var(--el-text-color-secondary); font-size: 12px">关闭即成送货单</span>
            </el-form-item>
            <el-form-item label="显示落款签字行"><el-switch v-model="settings.a4_show_signs" /></el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveSettings">保存设置</el-button>
              <el-button v-if="settings.shop_name" @click="previewTicket">预览小票效果</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card shadow="never">
          <template #header>授权信息</template>
          <div v-if="lic" v-loading="licUploading">
            <el-descriptions :column="1" border size="small" style="margin-bottom: 14px">
              <el-descriptions-item label="授权状态">
                <el-tag v-if="lic.mode === 'licensed'" type="success">已授权{{ lic.owner ? '：' + lic.owner : '' }}</el-tag>
                <el-tag v-else-if="lic.mode === 'trial'" type="warning">试用期（剩余 {{ lic.trial_left }} 天）</el-tag>
                <el-tag v-else type="danger">已过期（只能查看数据与导入授权）</el-tag>
              </el-descriptions-item>
              <el-descriptions-item v-if="lic.expires_at" label="授权到期">{{ lic.expires_at }}</el-descriptions-item>
              <el-descriptions-item label="本机机器码">
                <span style="font-family: monospace; user-select: all">{{ lic.machine_code }}</span>
              </el-descriptions-item>
            </el-descriptions>
            <el-alert
              v-if="lic.mode !== 'licensed'" type="info" :closable="false" show-icon
              title="把本机机器码发给软件提供方，获取 license.key 文件后在此导入"
              style="margin-bottom: 12px"
            />
            <el-upload :auto-upload="false" :limit="1" accept=".key,.json" :on-change="(f: any) => (licFile = f.raw)" style="display: inline-block">
              <el-button>选择 license.key 文件</el-button>
            </el-upload>
            <el-button type="primary" style="margin-left: 10px" :disabled="!licFile" :loading="licUploading" @click="uploadLicense">
              导入授权
            </el-button>
          </div>
        </el-card>

        <el-card shadow="never" style="margin-top: 16px">
          <template #header>
            <div style="display: flex; justify-content: space-between; align-items: center">
              <span>数据备份</span>
              <el-button type="primary" size="small" :loading="backupLoading" @click="createBackup">立即备份</el-button>
            </div>
          </template>
          <el-alert
            type="info" :closable="false" show-icon
            title="每天首次启动服务会自动备份一次；重要操作前建议手动备份。下载后请妥善保管。"
            style="margin-bottom: 12px"
          />
          <el-table :data="backups" border size="small" empty-text="暂无备份">
            <el-table-column prop="name" label="文件名" min-width="180" />
            <el-table-column label="大小" width="90" align="right">
              <template #default="{ row }">{{ (row.size / 1024).toFixed(0) }} KB</template>
            </el-table-column>
            <el-table-column prop="time" label="备份时间" width="160" />
            <el-table-column label="操作" width="90">
              <template #default="{ row }">
                <el-button link type="primary" @click="downloadBackup(row.name)">下载</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" style="margin-top: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>秤码规则（电子秤标签解析）</span>
          <el-button type="primary" size="small" @click="openRule()">新增规则</el-button>
        </div>
      </template>

      <el-alert
        type="info" :closable="false" show-icon style="margin-bottom: 12px"
        title="散称商品称重后，电子秤打印的 EAN-13 店内码里含「商品 PLU + 重量或金额」。配置规则后，开单时用扫码枪扫标签即可自动带出商品与数量。不同品牌/型号只需新增一条规则，无需改程序。"
      />

      <el-table :data="rules" border size="small" v-loading="ruleLoading" empty-text="暂无规则">
        <el-table-column label="规则名称" min-width="200">
          <template #default="{ row }">
            <span>{{ row.name }}</span>
            <el-tag v-if="row.is_default" size="small" type="success" style="margin-left: 6px">默认</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="品牌 / 型号" min-width="160">
          <template #default="{ row }">
            <div>{{ row.brand || '—' }}</div>
            <div style="font-size: 12px; color: var(--el-text-color-secondary)">{{ row.model }}</div>
          </template>
        </el-table-column>
        <el-table-column label="码制" width="150">
          <template #default="{ row }">{{ row.code_len }} 位 · 前缀 {{ row.prefixes || '不限' }}</template>
        </el-table-column>
        <el-table-column label="PLU 段" width="110">
          <template #default="{ row }">第 {{ row.plu_start }} 位 × {{ row.plu_len }}</template>
        </el-table-column>
        <el-table-column label="金额 / 重量段" min-width="230">
          <template #default="{ row }">{{ segSummary(row) }}</template>
        </el-table-column>
        <el-table-column label="校验" width="90">
          <template #default="{ row }">{{ row.check_type === 'mod10' ? '模10' : '不校验' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openRule(row)">编辑</el-button>
            <el-button link type="danger" @click="removeRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-divider content-position="left">测试解析</el-divider>
      <el-row :gutter="16">
        <el-col :span="10">
          <el-input v-model="testCode" placeholder="扫描秤标签或手动输入条码，回车解析" clearable @keyup.enter="runTest">
            <template #append><el-button :loading="testing" @click="runTest">解析</el-button></template>
          </el-input>
          <div style="margin-top: 8px; font-size: 12px; color: var(--el-text-color-secondary)">
            可用示例：顶尖 13 位金额码 <code>2200001025257</code>（PLU=00001 散装螺丝，金额 ¥25.25）
          </div>
        </el-col>
        <el-col :span="14">
          <template v-if="testResult">
            <el-alert
              v-if="testResult.matched" type="success" :closable="false" show-icon
              :title="`识别成功：${testResult.product_name}（${testResult.product_code}）`"
            />
            <el-alert v-else type="warning" :closable="false" show-icon :title="testResult.reason || '未识别'" />
            <el-descriptions v-if="testResult.matched" :column="2" border size="small" style="margin-top: 8px">
              <el-descriptions-item label="命中规则">{{ testResult.rule_name }}</el-descriptions-item>
              <el-descriptions-item label="PLU">{{ testResult.plu }}</el-descriptions-item>
              <el-descriptions-item label="带出数量">{{ qtyText(testResult) }} {{ testResult.unit }}</el-descriptions-item>
              <el-descriptions-item label="条码金额">
                {{ testResult.amount ? '¥' + (testResult.amount / 100).toFixed(2) : '—' }}
              </el-descriptions-item>
              <el-descriptions-item label="数量来源" :span="2">
                {{ testResult.source === 'weight' ? '条码重量段（最准确）' : '按商品默认售价由金额反算，可在单据中修改' }}
              </el-descriptions-item>
            </el-descriptions>
          </template>
          <div v-else style="color: var(--el-text-color-placeholder); font-size: 13px; line-height: 32px">
            解析结果会显示在这里：命中哪条规则、商品、带出数量。
          </div>
        </el-col>
      </el-row>
    </el-card>
  </div>

  <el-dialog v-model="ruleDialog" :title="ruleForm.id ? '编辑秤码规则' : '新增秤码规则'" width="760px">
    <el-form label-width="110px">
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="规则名称"><el-input v-model="ruleForm.name" placeholder="如：大华 TM-15 13位重量码" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="品牌"><el-input v-model="ruleForm.brand" placeholder="如：大华 / 寺冈" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="适用型号"><el-input v-model="ruleForm.model" placeholder="如：TM-15 / LS6" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="条码位数"><el-input-number v-model="ruleForm.code_len" :min="4" :max="40" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="前缀">
            <el-input v-model="ruleForm.prefixes" placeholder="如 22 或 21,22，空=不限" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="校验方式">
            <el-select v-model="ruleForm.check_type" style="width: 100%">
              <el-option label="EAN 模10校验（推荐）" value="mod10" />
              <el-option label="不校验" value="none" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="PLU 起始位"><el-input-number v-model="ruleForm.plu_start" :min="1" :max="30" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="PLU 长度"><el-input-number v-model="ruleForm.plu_len" :min="1" :max="10" /></el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="数据段类型">
            <el-radio-group v-model="segType">
              <el-radio-button value="amount">金额码</el-radio-button>
              <el-radio-button value="weight">重量码</el-radio-button>
              <el-radio-button value="both">金额 + 重量（18 位）</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <template v-if="ruleForm.amount_start">
          <el-col :span="8">
            <el-form-item label="金额起始位"><el-input-number v-model="ruleForm.amount_start" :min="1" :max="40" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="金额长度"><el-input-number v-model="ruleForm.amount_len" :min="1" :max="10" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="金额小数位"><el-input-number v-model="ruleForm.amount_decimals" :min="0" :max="4" /></el-form-item>
          </el-col>
        </template>
        <template v-if="ruleForm.weight_start">
          <el-col :span="8">
            <el-form-item label="重量起始位"><el-input-number v-model="ruleForm.weight_start" :min="1" :max="40" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="重量长度"><el-input-number v-model="ruleForm.weight_len" :min="1" :max="10" /></el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="重量小数位"><el-input-number v-model="ruleForm.weight_decimals" :min="0" :max="4" /></el-form-item>
          </el-col>
        </template>
        <el-col :span="8">
          <el-form-item label="排序"><el-input-number v-model="ruleForm.sort" :min="0" /></el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="启用"><el-switch v-model="ruleForm.enabled" /></el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="设为默认"><el-switch v-model="ruleForm.is_default" /></el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="备注">
            <el-input v-model="ruleForm.remark" type="textarea" :rows="2" placeholder="记录该码制的来源、注意事项" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <el-alert
      type="info" :closable="false" show-icon
      title="位数位置从 1 开始计数。13 位码常见布局：2 位前缀 + 5 位 PLU + 5 位金额/重量 + 1 位校验。"
    />
    <template #footer>
      <el-button @click="ruleDialog = false">取消</el-button>
      <el-button type="primary" :loading="ruleSaving" @click="saveRule">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.logo-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.logo-preview {
  max-width: 120px;
  max-height: 60px;
  border: 1px dashed var(--el-border-color);
  border-radius: 4px;
  padding: 4px;
  background: #fff;
}
.logo-empty {
  width: 120px;
  height: 60px;
  border: 1px dashed var(--el-border-color);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.logo-ops {
  line-height: 1.5;
}
</style>
