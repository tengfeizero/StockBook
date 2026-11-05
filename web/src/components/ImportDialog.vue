<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import type { UploadFile, UploadUserFile } from 'element-plus'
import { importApi, type ImportResult } from '../api/archive'

const props = defineProps<{ kind: 'products' | 'supplier' | 'customer'; title: string }>()
const emit = defineEmits<{ success: [] }>()

const visible = defineModel<boolean>({ default: false })
const fileList = ref<UploadUserFile[]>([])
const uploading = ref(false)
const result = ref<ImportResult | null>(null)

function downloadTemplate() {
  importApi.template(props.kind).then((blob) => {
    const url = URL.createObjectURL(blob as unknown as Blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${props.title}导入模板.xlsx`
    a.click()
    URL.revokeObjectURL(url)
  })
}

async function submit() {
  const uf = fileList.value[0] as UploadFile | undefined
  if (!uf?.raw) return ElMessage.warning('请先选择 Excel 文件')
  uploading.value = true
  result.value = null
  try {
    const fn = props.kind === 'products' ? importApi.products : (f: File) => importApi.partners(props.kind as 'supplier' | 'customer', f)
    result.value = (await fn(uf.raw as File)) as any
    if ((result.value as any).success > 0) emit('success')
  } finally {
    uploading.value = false
  }
}

function onChange(_file: any, files: UploadUserFile[]) {
  fileList.value = files.slice(-1)
}
</script>

<template>
  <el-dialog v-model="visible" :title="`${title}批量导入`" width="560px">
    <el-alert type="info" :closable="false" style="margin-bottom: 16px">
      <template #title>
        请按模板格式填写；编码重复的行将跳过并提示。
        <el-button link type="primary" @click="downloadTemplate">下载导入模板</el-button>
      </template>
    </el-alert>
    <el-upload drag :auto-upload="false" :limit="1" accept=".xlsx,.xls" :on-change="onChange" :file-list="fileList">
      <el-icon style="font-size: 40px; color: var(--el-text-color-placeholder)"><UploadFilled /></el-icon>
      <div>拖拽文件到这里，或点击选择 .xlsx 文件</div>
    </el-upload>

    <div v-if="result" style="margin-top: 16px">
      <el-alert
        :type="result.errors.length === 0 ? 'success' : 'warning'"
        :closable="false"
        :title="`成功导入 ${result.success} 条，跳过 ${result.skipped} 条，失败 ${result.errors.length} 条`"
      />
      <div v-if="result.errors.length" class="err-list">
        <div v-for="(e, i) in result.errors" :key="i" class="err-item">第 {{ e.row }} 行：{{ e.msg }}</div>
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
      <el-button type="primary" :loading="uploading" @click="submit">开始导入</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.err-list {
  margin-top: 8px;
  max-height: 160px;
  overflow-y: auto;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 8px 12px;
}
.err-item {
  font-size: 12px;
  color: var(--el-color-danger);
  line-height: 1.8;
}
</style>
