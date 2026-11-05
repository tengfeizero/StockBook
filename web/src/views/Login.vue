<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import request from '../utils/request'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const demoLoading = ref(false)
const form = reactive({ tenantCode: localStorage.getItem('lastTenantCode') || '', username: 'admin', password: '' })

interface DemoTpl {
  key: string
  name: string
  scene: string
}
const templates = ref<DemoTpl[]>([
  { key: 'hardware', name: '五金建材店', scene: '五金机电、水暖电料，含散称螺丝与按桶计价的油漆。' },
  { key: 'convenience', name: '便利店', scene: '社区便利店，小包装快消品与冷藏鲜食。' },
  { key: 'fresh', name: '生鲜果蔬店', scene: '社区生鲜，蔬菜水果散称、肉禽水产与短保熟食。' },
  { key: 'wholesale', name: '快消品批发部', scene: '整箱大单批发配送，多级价格与月结对账。' },
])
const demoKey = ref('hardware')
const demoScene = computed(() => templates.value.find((t) => t.key === demoKey.value)?.scene || '')
const demoName = computed(() => templates.value.find((t) => t.key === demoKey.value)?.name || '演示')

onMounted(async () => {
  try {
    const list = (await request.get('/demo/templates')) as DemoTpl[]
    if (Array.isArray(list) && list.length) templates.value = list
  } catch {
    // 登录页拿不到模板清单时用内置的三套兜底
  }
})

async function submit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(form.username, form.password, form.tenantCode.trim())
    localStorage.setItem('lastTenantCode', form.tenantCode.trim())
    router.push('/')
  } finally {
    loading.value = false
  }
}

function goRegister() {
  router.push('/register')
}

// 一键体验演示数据：仅单机版可用（已启用多租户时后端会拒绝）
async function initDemo() {
  await ElMessageBox.confirm(
    `将清空现有全部业务数据（商品/往来单位/单据/库存/资金），并写入一套「${demoName.value}」演示数据（仅保留用户与系统设置）。确定继续？`,
    '一键体验演示数据',
    { type: 'warning', confirmButtonText: '清空并写入演示数据', cancelButtonText: '取消' },
  )
  demoLoading.value = true
  try {
    await request.post('/demo/init', { confirm: 'RESET', template: demoKey.value })
    ElMessage.success(`${demoName.value}演示数据已就绪，请使用 admin/admin123 登录体验`)
  } finally {
    demoLoading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <el-card class="login-card">
      <h2>货本</h2>
      <p class="tagline">一个 exe，一本货账</p>
      <el-form @submit.prevent="submit">
        <el-form-item>
          <el-input v-model="form.tenantCode" placeholder="企业编码（单机版留空）" size="large" clearable />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" show-password />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" native-type="submit">
          登 录
        </el-button>

        <div class="register-row">
          还没有企业账号？<el-link type="primary" :underline="false" @click="goRegister">免费注册试用 →</el-link>
        </div>

        <div class="demo-block">
          <div class="demo-title">单机版体验：选个行业一键写入演示数据</div>
          <el-select v-model="demoKey" size="default" style="width: 100%">
            <el-option v-for="t in templates" :key="t.key" :label="t.name" :value="t.key" />
          </el-select>
          <div class="demo-scene">{{ demoScene }}</div>
          <el-button link type="info" :loading="demoLoading" @click="initDemo">
            写入「{{ demoName }}」演示数据 →
          </el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-light);
}
.login-card {
  width: 380px;
  text-align: center;
}
h2 {
  margin-bottom: 24px;
  font-weight: 600;
}
.tagline {
  margin: -16px 0 20px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  letter-spacing: 1px;
}
.register-row {
  margin-top: 12px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.demo-block {
  margin-top: 22px;
  padding-top: 16px;
  border-top: 1px dashed var(--el-border-color);
}
.demo-title {
  margin-bottom: 10px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.demo-scene {
  min-height: 34px;
  margin: 8px 2px 4px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-placeholder);
  text-align: left;
}
</style>
