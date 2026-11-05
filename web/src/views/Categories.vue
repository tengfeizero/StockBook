<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { categoryApi } from '../api/archive'

interface Category { id: number; name: string; parent_id: number; sort: number }

const list = ref<Category[]>([])
const loading = ref(false)
const dialog = ref(false)
const isEdit = ref(false)
const form = reactive({ id: 0, name: '', parent_id: 0, sort: 0 })

async function load() {
  loading.value = true
  try {
    list.value = (await categoryApi.list()) as any
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  Object.assign(form, { id: 0, name: '', parent_id: 0, sort: 0 })
  dialog.value = true
}

function openEdit(row: Category) {
  isEdit.value = true
  Object.assign(form, row)
  dialog.value = true
}

async function submit() {
  if (!form.name.trim()) return ElMessage.warning('请输入分类名称')
  if (isEdit.value) {
    await categoryApi.update(form.id, { name: form.name, parent_id: form.parent_id, sort: form.sort })
  } else {
    await categoryApi.create({ name: form.name, parent_id: form.parent_id, sort: form.sort })
  }
  ElMessage.success('保存成功')
  dialog.value = false
  load()
}

async function remove(row: Category) {
  await ElMessageBox.confirm(`确定删除分类「${row.name}」？`, '提示', { type: 'warning' })
  await categoryApi.remove(row.id)
  ElMessage.success('已删除')
  load()
}

const parentName = (id: number) => list.value.find((c) => c.id === id)?.name || '—'

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px">
      <el-button type="primary" @click="openCreate">新建分类</el-button>
    </div>
    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="分类名称" min-width="150" />
      <el-table-column label="上级分类" width="140">
        <template #default="{ row }">{{ parentName(row.parent_id) }}</template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialog" :title="isEdit ? '编辑分类' : '新建分类'" width="420px">
    <el-form label-width="80px">
      <el-form-item label="名称" required>
        <el-input v-model="form.name" maxlength="50" />
      </el-form-item>
      <el-form-item label="上级分类">
        <el-select v-model="form.parent_id" clearable style="width: 100%">
          <el-option v-for="c in list.filter((x) => x.parent_id === 0 && x.id !== form.id)" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="排序">
        <el-input-number v-model="form.sort" :min="0" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" @click="submit">保存</el-button>
    </template>
  </el-dialog>
</template>
