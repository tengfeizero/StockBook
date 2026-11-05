<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useUiStore } from '../stores/ui'

// 新手引导：聚光灯 + 气泡。步骤锚点用 data-guide="xxx" 标记，
// 元素不存在（如不在首页、无报表权限）时自动跳过该步。
interface Step {
  key: string
  sel?: string
  title: string
  desc: string
  placement: 'right' | 'bottom' | 'left' | 'top' | 'center'
}

const STEPS: Step[] = [
  {
    key: 'menu',
    sel: '[data-guide="menu"]',
    title: '功能都在左边的分组里',
    desc: '基础档案 → 采购销售 → 资金管理 → 库存作业 → 系统设置，按干活的顺序排好了。第一次用建议先建商品和期初库存。',
    placement: 'right',
  },
  {
    key: 'quick',
    sel: '[data-guide="quick"]',
    title: '四类高频动作，一步直达',
    desc: '采购入库、销售出库、盘点、收付款——日常 90% 的操作都从这四个按钮进去，不用翻菜单。',
    placement: 'bottom',
  },
  {
    key: 'kpi',
    sel: '[data-guide="kpi"]',
    title: '今天的生意，一眼看完',
    desc: '今日销售额/毛利/单据数、库存总值，以及应收应付欠款。点应收应付可直接跳去收款付款。',
    placement: 'bottom',
  },
  {
    key: 'reports',
    sel: '[data-guide="reports"]',
    title: '月底对账看这里',
    desc: '进销存汇总、销售毛利明细都在「报表中心」，数据来自单据流水，和库存查询永远对得上。',
    placement: 'right',
  },
  {
    key: 'user',
    sel: '[data-guide="user"]',
    title: '企业信息与账号',
    desc: '右上角是企业名、试用/订阅剩余天数和你自己的账号。到期前 7 天会提示，随时可以自助续费。',
    placement: 'bottom',
  },
  {
    key: 'done',
    title: '上手就三件事',
    desc: '① 建商品和期初库存 ② 开第一张销售出库单 ③ 去「其他设置」配好店名和小票抬头。卡住了随时点右上角「?」看使用手册。',
    placement: 'center',
  },
]

const ui = useUiStore()
const router = useRouter()

const list = ref<Step[]>([])
const idx = ref(0)
const rect = ref<DOMRect | null>(null)
const tipRef = ref<HTMLElement | null>(null)
const tipSize = ref({ w: 340, h: 180 })

const active = computed<Step | null>(() => list.value[idx.value] || null)
const isLast = computed(() => idx.value >= list.value.length - 1)

const holeStyle = computed(() => {
  const r = rect.value
  if (!r) return { display: 'none' }
  return {
    left: `${Math.max(r.left - 6, 0)}px`,
    top: `${Math.max(r.top - 6, 0)}px`,
    width: `${r.width + 12}px`,
    height: `${r.height + 12}px`,
  }
})

const tipStyle = computed(() => {
  const step = active.value
  const r = rect.value
  if (!step) return { display: 'none' }
  if (step.placement === 'center' || !r) {
    return { left: '50%', top: '50%', transform: 'translate(-50%, -50%)' }
  }
  const gap = 18
  const w = tipSize.value.w
  const h = tipSize.value.h
  const vw = window.innerWidth
  const vh = window.innerHeight
  let left = 0
  let top = 0
  if (step.placement === 'right') {
    left = r.right + gap
    top = r.top + Math.min(Math.max(r.height / 2 - h / 2, 0), vh - h - 16)
  } else if (step.placement === 'left') {
    left = r.left - w - gap
    top = r.top + Math.min(Math.max(r.height / 2 - h / 2, 0), vh - h - 16)
  } else if (step.placement === 'bottom') {
    left = r.left
    top = r.bottom + gap
  } else {
    left = r.left
    top = r.top - h - gap
  }
  left = Math.min(Math.max(left, 16), vw - w - 16)
  top = Math.min(Math.max(top, 16), vh - h - 16)
  return { left: `${left}px`, top: `${top}px` }
})

function measure() {
  // 量完气泡实际尺寸再定位，避免小屏溢出
  if (tipRef.value) {
    const box = tipRef.value.getBoundingClientRect()
    if (box.width && (Math.abs(box.width - tipSize.value.w) > 1 || Math.abs(box.height - tipSize.value.h) > 1)) {
      tipSize.value = { w: box.width, h: box.height }
    }
  }
  const step = active.value
  if (!step?.sel) {
    rect.value = null
    return
  }
  const el = document.querySelector(step.sel)
  rect.value = el ? el.getBoundingClientRect() : null
}

function go(i: number) {
  if (i < 0 || i >= list.value.length) return
  idx.value = i
  rect.value = null
  nextTick(() => {
    measure()
    requestAnimationFrame(measure)
  })
}

function finish() {
  localStorage.setItem('jxc_guide_done', '1')
  ui.stopTour()
}

function openHelp() {
  finish()
  router.push('/help')
}

function onKey(e: KeyboardEvent) {
  if (!ui.tourVisible) return
  if (e.key === 'Escape') finish()
  else if (e.key === 'ArrowRight' || e.key === 'Enter') isLast.value ? finish() : go(idx.value + 1)
  else if (e.key === 'ArrowLeft') go(idx.value - 1)
}

watch(
  () => ui.tourVisible,
  (v) => {
    if (!v) return
    // 只保留页面上真实存在的锚点，避免权限不足时指向空气
    list.value = STEPS.filter((s) => !s.sel || !!document.querySelector(s.sel))
    idx.value = 0
    rect.value = null
    nextTick(() => {
      measure()
      requestAnimationFrame(measure)
    })
  },
)

onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('resize', measure)
  window.addEventListener('scroll', measure, true)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', measure)
  window.removeEventListener('scroll', measure, true)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="ui.tourVisible && active" class="tour">
      <!-- 遮罩：挡住点击，聚光灯用超大 box-shadow 挖洞 -->
      <div class="tour-mask" @click.stop></div>
      <div class="tour-hole" :style="holeStyle"></div>

      <div ref="tipRef" class="tour-tip" :style="tipStyle">
        <div class="tour-head">
          <span class="tour-step">{{ idx + 1 }} / {{ list.length }}</span>
          <span class="tour-close" @click="finish">✕</span>
        </div>
        <div class="tour-title">{{ active.title }}</div>
        <div class="tour-desc">{{ active.desc }}</div>
        <div class="tour-actions">
          <el-button link size="small" @click="finish">跳过</el-button>
          <div style="flex: 1"></div>
          <el-button v-if="idx > 0" size="small" @click="go(idx - 1)">上一步</el-button>
          <el-button v-if="!isLast" type="primary" size="small" @click="go(idx + 1)">下一步</el-button>
          <template v-else>
            <el-button size="small" @click="openHelp">看手册</el-button>
            <el-button type="primary" size="small" @click="finish">开始使用</el-button>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.tour-mask {
  position: fixed;
  inset: 0;
  z-index: 3000;
  background: transparent;
  cursor: default;
}
.tour-hole {
  position: fixed;
  z-index: 3001;
  border-radius: 8px;
  border: 2px solid var(--el-color-primary);
  box-shadow: 0 0 0 100vmax rgba(0, 0, 0, 0.55);
  pointer-events: none;
  transition: all 0.18s ease;
}
.tour-tip {
  position: fixed;
  z-index: 3002;
  width: 340px;
  max-width: calc(100vw - 32px);
  background: var(--el-bg-color);
  border-radius: 10px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.18);
  padding: 14px 16px 12px;
}
.tour-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.tour-step {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.tour-close {
  cursor: pointer;
  font-size: 13px;
  color: var(--el-text-color-placeholder);
  padding: 0 2px;
}
.tour-close:hover {
  color: var(--el-text-color-regular);
}
.tour-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 6px;
  color: var(--el-text-color-primary);
}
.tour-desc {
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}
.tour-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}
</style>
