<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import request from '../utils/request'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const form = reactive({
  name: '',
  code: '',
  contact: '',
  phone: '',
  username: 'admin',
  password: '',
  template: 'hardware',
  agree: true,
})

interface DemoTpl {
  key: string
  name: string
  scene: string
}
const templates = ref<DemoTpl[]>([
  { key: 'hardware', name: '五金建材店', scene: '五金机电、水暖电料' },
  { key: 'convenience', name: '便利店', scene: '小包装快消与冷藏鲜食' },
  { key: 'fresh', name: '生鲜果蔬店', scene: '散称蔬菜水果、肉禽水产' },
  { key: '', name: '空白账套', scene: '不写入演示数据，按自己的商品从头建账' },
])
const scene = computed(() => templates.value.find((t) => t.key === form.template)?.scene || '')

onMounted(async () => {
  try {
    const list = (await request.get('/demo/templates')) as DemoTpl[]
    if (Array.isArray(list) && list.length) {
      templates.value = [...list.map((t) => ({ ...t })), { key: '', name: '空白账套', scene: '不写入演示数据，按自己的商品从头建账' }]
    }
  } catch {
    // 用内置兜底
  }
})

// 企业编码：小写字母/数字/短横线，随时可改
function normalizeCode() {
  form.code = form.code.toLowerCase().replace(/[^a-z0-9-]/g, '')
}

async function submit() {
  if (!form.name.trim()) return ElMessage.warning('请填写企业名称')
  if (!form.username.trim()) return ElMessage.warning('请填写管理员账号')
  if (form.password.length < 6) return ElMessage.warning('密码至少 6 位')
  if (!form.agree) return ElMessage.warning('请先勾选同意服务条款')
  loading.value = true
  try {
    const res: any = await request.post('/tenants/register', {
      name: form.name.trim(),
      code: form.code.trim(),
      contact: form.contact.trim(),
      phone: form.phone.trim(),
      username: form.username.trim(),
      password: form.password,
      template: form.template,
    })
    await auth.loginByToken(res)
    ElMessage.success(`开户成功，已进入「${res.tenant_name}」，试用 14 天`)
    router.push('/')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="reg-page">
    <el-card class="reg-card">
      <h2>注册企业账号</h2>
      <p class="sub">开通后即是独立账套，数据与其它企业完全隔离；试用期 14 天，可随时续费。</p>

      <el-form label-width="92px" @submit.prevent="submit">
        <el-form-item label="企业名称" required>
          <el-input v-model="form.name" placeholder="如：三生顺五金（嘉兴店）" maxlength="50" />
        </el-form-item>
        <el-form-item label="企业编码">
          <el-input v-model="form.code" placeholder="留空自动生成，登录时填写" maxlength="30" @input="normalizeCode" />
          <div class="tip">3-30 位小写字母/数字/短横线，注册后不可修改</div>
        </el-form-item>
        <el-form-item label="联系人">
          <el-input v-model="form.contact" maxlength="20" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="form.phone" maxlength="20" />
        </el-form-item>
        <el-form-item label="管理员账号" required>
          <el-input v-model="form.username" maxlength="30" />
        </el-form-item>
        <el-form-item label="登录密码" required>
          <el-input v-model="form.password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="行业模板">
          <el-select v-model="form.template" style="width: 100%">
            <el-option v-for="t in templates" :key="t.key" :label="t.name" :value="t.key" />
          </el-select>
          <div class="tip">{{ scene }}</div>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="form.agree">我已阅读并同意《服务条款》与《隐私政策》</el-checkbox>
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" native-type="submit">
          开通试用
        </el-button>
        <div class="back">
          已有账号？<el-link type="primary" :underline="false" @click="router.push('/login')">返回登录</el-link>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.reg-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-light);
  padding: 32px 0;
}
.reg-card {
  width: 520px;
  padding: 8px 12px;
}
h2 {
  margin-bottom: 6px;
  font-weight: 600;
  text-align: center;
}
.sub {
  margin: 0 0 20px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
  text-align: center;
}
.tip {
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-placeholder);
}
.back {
  margin-top: 14px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  text-align: center;
}
</style>
