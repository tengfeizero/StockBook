<script setup lang="ts">
import { useRoute } from 'vue-router'
const route = useRoute()
</script>

<template>
  <el-empty :description="`【${route.meta.title}】模块待开发（见 docs/TODO.md 排期）`" />
</template>
