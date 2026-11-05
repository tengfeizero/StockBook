<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import request from '../utils/request'

interface AuditRow {
  id: number; username: string; action: string; target: string
  detail: string; ip: string; result: string; created_at: string
}

const list = ref<AuditRow[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, page_size: 20, username: '', action: '' })

async function load() {
  loading.value = true
  try {
    const data: any = await request.get('/system/audit-logs', { params: query })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

const actionColor = (a: string) =>
  a === 'DELETE' ? 'danger' : a === 'LOGIN' ? 'success' : a === 'PUT' ? 'warning' : 'primary'

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px">
      <el-input v-model="query.username" placeholder="操作人" style="width: 160px" clearable @keyup.enter="query.page = 1; load()" />
      <el-select v-model="query.action" placeholder="全部类型" clearable style="width: 130px" @change="query.page = 1; load()">
        <el-option label="登录" value="LOGIN" />
        <el-option label="新增(POST)" value="POST" />
        <el-option label="修改(PUT)" value="PUT" />
        <el-option label="删除(DELETE)" value="DELETE" />
      </el-select>
      <el-button type="primary" @click="query.page = 1; load()">查询</el-button>
      <span style="margin-left: auto; color: var(--el-text-color-secondary); font-size: 12px; align-self: center">
        记录所有登录与数据变更操作（密码已脱敏），保留于本机数据库。
      </span>
    </div>

    <el-table :data="list" v-loading="loading" border size="small">
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ (row.created_at || '').slice(0, 19).replace('T', ' ') }}</template>
      </el-table-column>
      <el-table-column prop="username" label="操作人" width="110" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }">
          <el-tag :type="actionColor(row.action)" size="small">{{ row.action }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target" label="对象" min-width="220" />
      <el-table-column prop="detail" label="内容摘要" min-width="260" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" width="130" />
      <el-table-column label="结果" width="80">
        <template #default="{ row }">
          <el-tag :type="row.result === 'ok' ? 'success' : 'danger'" size="small">{{ row.result === 'ok' ? '成功' : '失败' }}</el-tag>
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
</template>
