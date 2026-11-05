<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'

interface UserRow {
  id: number; username: string; display_name: string
  role_id: number; role_name: string; status: string; created_at: string
}
interface RoleRow {
  id: number; name: string; remark: string; permissions: string[]; user_count: number
}

const users = ref<UserRow[]>([])
const roles = ref<RoleRow[]>([])
// 支持 /system?tab=demo 直达（首页新手引导与手册里会用到）
const route = useRoute()
const activeTab = ref((route.query.tab as string) || 'users')

// 模块权限定义（与后端 validPerms 对应）
const permOptions = [
  { code: 'archive', label: '基础档案', desc: '商品/分类/供应商/客户/期初/导入' },
  { code: 'purchase', label: '采购', desc: '采购订单/入库/退货' },
  { code: 'sales', label: '销售', desc: '销售出库/报价/退货/单据查询' },
  { code: 'finance', label: '资金', desc: '应收应付/收付款' },
  { code: 'stock', label: '库存作业', desc: '盘点/其他出入库/库存查询' },
  { code: 'reports', label: '报表', desc: '进销存汇总/销售毛利' },
  { code: 'system', label: '系统管理', desc: '用户/角色/日志/备份/设置' },
]

async function load() {
  users.value = (await request.get('/system/users')) as any
  roles.value = (await request.get('/system/roles')) as any
}

// ---- 用户 ----
const userDialog = ref(false)
const userForm = reactive({ id: 0, username: '', password: '', display_name: '', role_id: 0, status: 'active' })
const isUserEdit = ref(false)

function openUserCreate() {
  isUserEdit.value = false
  Object.assign(userForm, { id: 0, username: '', password: '', display_name: '', role_id: roles.value[0]?.id || 0, status: 'active' })
  userDialog.value = true
}
function openUserEdit(u: UserRow) {
  isUserEdit.value = true
  Object.assign(userForm, { id: u.id, username: u.username, password: '', display_name: u.display_name, role_id: u.role_id, status: u.status })
  userDialog.value = true
}
async function submitUser() {
  if (isUserEdit.value) {
    await request.put(`/system/users/${userForm.id}`, {
      display_name: userForm.display_name, role_id: userForm.role_id, status: userForm.status,
    })
    ElMessage.success('已保存')
  } else {
    await request.post('/system/users', {
      username: userForm.username, password: userForm.password,
      display_name: userForm.display_name, role_id: userForm.role_id,
    })
    ElMessage.success('已创建')
  }
  userDialog.value = false
  load()
}

async function resetPwd(u: UserRow) {
  const { value } = await ElMessageBox.prompt(`为「${u.display_name}」设置新密码（至少6位）：`, '重置密码', {
    inputType: 'password', inputPattern: /^.{6,}$/, inputErrorMessage: '密码至少6位',
  })
  await request.post(`/system/users/${u.id}/password`, { password: value })
  ElMessage.success('已重置')
}

async function toggleStatus(u: UserRow) {
  const target = u.status === 'active' ? 'disabled' : 'active'
  await ElMessageBox.confirm(`确认${target === 'disabled' ? '停用' : '启用'}账号「${u.display_name}」？`, '提示', { type: 'warning' })
  await request.put(`/system/users/${u.id}`, { status: target })
  load()
}

// ---- 角色 ----
const roleDialog = ref(false)
const roleForm = reactive({ id: 0, name: '', remark: '', permissions: [] as string[] })

function openRoleCreate() {
  Object.assign(roleForm, { id: 0, name: '', remark: '', permissions: ['sales', 'stock'] })
  roleDialog.value = true
}
function openRoleEdit(r: RoleRow) {
  Object.assign(roleForm, { id: r.id, name: r.name, remark: r.remark, permissions: [...r.permissions] })
  roleDialog.value = true
}
async function submitRole() {
  await request.post(roleForm.id ? `/system/roles/${roleForm.id}` : '/system/roles', {
    id: roleForm.id, name: roleForm.name, remark: roleForm.remark, permissions: roleForm.permissions,
  })
  ElMessage.success('已保存')
  roleDialog.value = false
  load()
}

async function deleteRole(r: RoleRow) {
  await ElMessageBox.confirm(`删除角色「${r.name}」？`, '提示', { type: 'warning' })
  await request.delete(`/system/roles/${r.id}`)
  ElMessage.success('已删除')
  load()
}

const permText = (perms: string[]) =>
  perms.includes('*') ? '全部权限' : perms.map((p) => permOptions.find((o) => o.code === p)?.label || p).join('、')

// ---- 演示数据 ----
interface DemoTpl {
  key: string
  name: string
  scene: string
  detail: string
  shop: string
  products: number
  sales_docs: number
}
const demoTemplates = ref<DemoTpl[]>([])
const demoKey = ref('hardware')
const demoSelected = computed(() => demoTemplates.value.find((t) => t.key === demoKey.value))
const demoLoading = ref(false)

async function loadDemoTemplates() {
  try {
    const list = (await request.get('/demo/templates')) as DemoTpl[]
    if (Array.isArray(list) && list.length) {
      demoTemplates.value = list
      if (!list.some((t) => t.key === demoKey.value)) demoKey.value = list[0].key
    }
  } catch {
    // 模板清单拿不到时保留空列表（页面只显示说明，不给点重置）
  }
}

async function initDemo() {
  const tpl = demoSelected.value
  await ElMessageBox.confirm(
    `将清空现有全部业务数据（商品/往来单位/单据/库存/资金），并重新写入一套「${tpl?.name || '五金建材店'}」演示数据（仅保留用户、角色与系统设置）。确定继续？`,
    '重置为演示数据',
    { type: 'warning', confirmButtonText: '清空并写入演示数据', cancelButtonText: '取消' },
  )
  demoLoading.value = true
  try {
    const sum: any = await request.post('/system/demo-data', { confirm: 'RESET', template: demoKey.value })
    ElMessage.success(
      `${sum.template_name}演示数据已就绪：商品 ${sum.products}、单据 ${sum.purchase_docs + sum.sales_docs + sum.other_docs}、收付款 ${sum.payments}`,
    )
  } finally {
    demoLoading.value = false
  }
}

onMounted(() => {
  load()
  loadDemoTemplates()
})
</script>

<template>
  <el-card shadow="never">
    <el-tabs v-model="activeTab">
      <el-tab-pane label="用户" name="users" />
      <el-tab-pane label="角色权限" name="roles" />
      <el-tab-pane label="演示数据" name="demo" />
    </el-tabs>

    <!-- 用户 -->
    <template v-if="activeTab === 'users'">
      <div style="margin-bottom: 12px">
        <el-button type="primary" @click="openUserCreate">新建用户</el-button>
      </div>
      <el-table :data="users" border>
        <el-table-column prop="username" label="登录名" width="130" />
        <el-table-column prop="display_name" label="姓名" width="130" />
        <el-table-column prop="role_name" label="角色" width="130" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
              {{ row.status === 'active' ? '正常' : '已停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ (row.created_at || '').slice(0, 16).replace('T', ' ') }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="220">
          <template #default="{ row }">
            <el-button link type="primary" @click="openUserEdit(row)">编辑</el-button>
            <el-button link type="warning" @click="resetPwd(row)">重置密码</el-button>
            <el-button link :type="row.status === 'active' ? 'danger' : 'success'" @click="toggleStatus(row)">
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <!-- 角色 -->
    <template v-else-if="activeTab === 'roles'">
      <div style="margin-bottom: 12px">
        <el-button type="primary" @click="openRoleCreate">新建角色</el-button>
        <span style="margin-left: 12px; color: var(--el-text-color-secondary); font-size: 12px">
          调整角色权限后，该角色用户下一次操作即时生效。
        </span>
      </div>
      <el-table :data="roles" border>
        <el-table-column prop="name" label="角色" width="140" />
        <el-table-column prop="remark" label="说明" min-width="160" />
        <el-table-column label="权限" min-width="240">
          <template #default="{ row }">{{ permText(row.permissions) }}</template>
        </el-table-column>
        <el-table-column prop="user_count" label="用户数" width="90" align="center" />
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-button link type="primary" @click="openRoleEdit(row)">编辑</el-button>
            <el-button v-if="row.user_count === 0" link type="danger" @click="deleteRole(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <!-- 演示数据 -->
    <template v-else-if="activeTab === 'demo'">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="此操作会清空现有业务数据并写入演示数据，正式使用前请先备份数据。"
        style="margin-bottom: 16px"
      />
      <div style="margin-bottom: 12px">
        <span style="margin-right: 12px; color: var(--el-text-color-regular)">体验行业</span>
        <el-radio-group v-model="demoKey">
          <el-radio-button v-for="t in demoTemplates" :key="t.key" :value="t.key">{{ t.name }}</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="demoSelected" class="demo-scene">{{ demoSelected.scene }}</div>
      <el-descriptions v-if="demoSelected" :column="1" border style="max-width: 720px; margin-bottom: 20px">
        <el-descriptions-item label="门店">{{ demoSelected.shop }}</el-descriptions-item>
        <el-descriptions-item label="档案数据">{{ demoSelected.detail }}</el-descriptions-item>
        <el-descriptions-item label="业务数据">
          期初建账 + 近两个月 {{ demoSelected.sales_docs }} 张销售出库、采购入库、盘点/其他出入库/报价单/采购订单，部分收付款形成应收应付余额
        </el-descriptions-item>
        <el-descriptions-item label="保留内容">用户、角色、系统设置、授权、操作日志</el-descriptions-item>
      </el-descriptions>
      <el-button type="danger" plain :loading="demoLoading" :disabled="!demoSelected" @click="initDemo">
        一键重置为演示数据
      </el-button>
    </template>
  </el-card>

  <!-- 用户对话框 -->
  <el-dialog v-model="userDialog" :title="isUserEdit ? '编辑用户' : '新建用户'" width="440px">
    <el-form label-width="80px">
      <el-form-item label="登录名">
        <el-input v-model="userForm.username" :disabled="isUserEdit" />
      </el-form-item>
      <el-form-item v-if="!isUserEdit" label="初始密码">
        <el-input v-model="userForm.password" type="password" show-password placeholder="至少6位" />
      </el-form-item>
      <el-form-item label="姓名">
        <el-input v-model="userForm.display_name" />
      </el-form-item>
      <el-form-item label="角色">
        <el-select v-model="userForm.role_id" style="width: 100%">
          <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="isUserEdit" label="状态">
        <el-radio-group v-model="userForm.status">
          <el-radio value="active">正常</el-radio>
          <el-radio value="disabled">停用</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="userDialog = false">取消</el-button>
      <el-button type="primary" @click="submitUser">保存</el-button>
    </template>
  </el-dialog>

  <!-- 角色对话框 -->
  <el-dialog v-model="roleDialog" :title="roleForm.id ? '编辑角色' : '新建角色'" width="480px">
    <el-form label-width="70px">
      <el-form-item label="角色名">
        <el-input v-model="roleForm.name" />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="roleForm.remark" />
      </el-form-item>
      <el-form-item label="权限">
        <div style="width: 100%">
          <el-checkbox
            :model-value="roleForm.permissions.includes('*')"
            @change="roleForm.permissions = $event ? ['*'] : []"
          >全部权限（管理员）</el-checkbox>
          <div v-if="!roleForm.permissions.includes('*')">
            <el-checkbox v-for="o in permOptions" :key="o.code" v-model="roleForm.permissions" :value="o.code">
              {{ o.label }}<span style="color: var(--el-text-color-secondary); font-size: 12px; margin-left: 4px">{{ o.desc }}</span>
            </el-checkbox>
          </div>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="roleDialog = false">取消</el-button>
      <el-button type="primary" @click="submitRole">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.demo-scene {
  max-width: 720px;
  margin-bottom: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}
</style>
