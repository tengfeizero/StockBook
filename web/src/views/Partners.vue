<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { partnerApi, type PartnerPayload } from '../api/archive'
import ImportDialog from '../components/ImportDialog.vue'
import { fenToYuan, yuanToFen } from '../utils/money'

const props = defineProps<{ kind: 'supplier' | 'customer' }>()

const kindText = () => (props.kind === 'supplier' ? '供应商' : '客户')
const openingField = () => (props.kind === 'supplier' ? '期初应付' : '期初应收')

interface PartnerRow {
  id: number; code: string; name: string; contact: string; phone: string; address: string
  opening_payable?: number; opening_receivable?: number; remark: string
}

const list = ref<PartnerRow[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, page_size: 20, keyword: '' })

const dialog = ref(false)
const importVisible = ref(false)
const isEdit = ref(false)
const saving = ref(false)
const form = reactive({
  id: 0, code: '', name: '', contact: '', phone: '', address: '',
  opening_yuan: 0, remark: '',
})

function openingOf(row: PartnerRow): number {
  return (props.kind === 'supplier' ? row.opening_payable : row.opening_receivable) || 0
}

async function load() {
  loading.value = true
  try {
    const data: any = await partnerApi.list(props.kind, query)
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  Object.assign(form, { id: 0, code: '', name: '', contact: '', phone: '', address: '', opening_yuan: 0, remark: '' })
  dialog.value = true
}

function openEdit(row: PartnerRow) {
  isEdit.value = true
  Object.assign(form, {
    id: row.id, code: row.code, name: row.name, contact: row.contact,
    phone: row.phone, address: row.address, opening_yuan: openingOf(row) / 100, remark: row.remark,
  })
  dialog.value = true
}

async function submit() {
  if (!form.code.trim() || !form.name.trim()) return ElMessage.warning('编码和名称必填')
  const payload: PartnerPayload = {
    code: form.code, name: form.name, contact: form.contact, phone: form.phone,
    address: form.address, opening: yuanToFen(form.opening_yuan), remark: form.remark,
  }
  saving.value = true
  try {
    if (isEdit.value) await partnerApi.update(props.kind, form.id, payload)
    else await partnerApi.create(props.kind, payload)
    ElMessage.success('保存成功')
    dialog.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row: PartnerRow) {
  await ElMessageBox.confirm(`确定删除${kindText()}「${row.name}」？`, '提示', { type: 'warning' })
  await partnerApi.remove(props.kind, row.id)
  ElMessage.success('已删除')
  load()
}

// 供应商/客户共用本组件：路由切换时组件被复用，必须监听 kind 重新加载，
// 否则两个页面会显示同一份列表（切换前那份数据）
watch(() => props.kind, () => {
  query.page = 1
  query.keyword = ''
  load()
})
onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px">
      <el-button type="primary" @click="openCreate">新建{{ kindText() }}</el-button>
      <el-button @click="importVisible = true">批量导入</el-button>
      <el-input v-model="query.keyword" placeholder="名称/编码" style="width: 200px" clearable @keyup.enter="query.page = 1; load()" />
      <el-button @click="query.page = 1; load()">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="code" label="编码" width="110" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="contact" label="联系人" width="100" />
      <el-table-column prop="phone" label="电话" width="130" />
      <el-table-column :label="openingField()" width="110" align="right">
        <template #default="{ row }">¥{{ fenToYuan(openingOf(row)) }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="140" />
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="query.page"
      :page-size="query.page_size"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 12px; justify-content: flex-end"
      @current-change="load"
    />
  </el-card>

    <ImportDialog v-model="importVisible" :kind="props.kind" :title="kindText()" @success="load" />

  <el-dialog v-model="dialog" :title="(isEdit ? '编辑' : '新建') + kindText()" width="520px">
    <el-form label-width="90px">
      <el-form-item label="编码" required><el-input v-model="form.code" /></el-form-item>
      <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
      <el-form-item label="联系人"><el-input v-model="form.contact" /></el-form-item>
      <el-form-item label="电话"><el-input v-model="form.phone" /></el-form-item>
      <el-form-item label="地址"><el-input v-model="form.address" /></el-form-item>
      <el-form-item :label="openingField()">
        <el-input-number v-model="form.opening_yuan" :min="0" :precision="2" style="width: 180px" />
        <span style="margin-left: 8px; color: var(--el-text-color-secondary); font-size: 12px">元</span>
      </el-form-item>
      <el-form-item label="备注"><el-input v-model="form.remark" type="textarea" :rows="2" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
    </template>
  </el-dialog>
</template>
