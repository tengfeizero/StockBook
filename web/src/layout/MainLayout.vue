<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import GuideTour from '../components/GuideTour.vue'
import {
  LayoutDashboard, Package, FolderTree, Truck, Users, Boxes,
  ClipboardList, PackagePlus, PackageMinus, Tag, FileText,
  ClipboardCheck, Inbox, Warehouse, TrendingUp, MapPin,
  Settings, Store, Upload, Wallet, ArrowLeftRight, CreditCard, HelpCircle,
} from 'lucide-vue-next'

const router = useRouter()
const auth = useAuthStore()
const ui = useUiStore()

interface MenuItem { path: string; title: string; group: string; icon: any; perm?: string[] }

const can = (perms?: string[]) => !perms || auth.hasPerm(perms)

// 菜单与路由表一致；group 决定侧边栏分组；perm 控制可见性（不填=登录即可见）
const menus: MenuItem[] = [
  { path: '/', title: '首页看板', group: '', icon: LayoutDashboard },
  { path: '/products', title: '商品档案', group: '基础档案', icon: Package, perm: ['archive'] },
  { path: '/categories', title: '商品分类', group: '基础档案', icon: FolderTree, perm: ['archive'] },
  { path: '/suppliers', title: '供应商', group: '基础档案', icon: Truck, perm: ['archive'] },
  { path: '/customers', title: '客户', group: '基础档案', icon: Users, perm: ['archive'] },
  { path: '/opening-stock', title: '期初库存', group: '基础档案', icon: Boxes, perm: ['archive'] },
  { path: '/locations', title: '库位管理', group: '基础档案', icon: MapPin, perm: ['archive'] },
  { path: '/purchase-order', title: '采购订单', group: '采购销售', icon: ClipboardList, perm: ['purchase'] },
  { path: '/purchase-in', title: '采购入库', group: '采购销售', icon: PackagePlus, perm: ['purchase'] },
  { path: '/purchase-return', title: '采购退货', group: '采购销售', icon: PackageMinus, perm: ['purchase'] },
  { path: '/sales-out', title: '销售出库', group: '采购销售', icon: PackageMinus, perm: ['sales'] },
  { path: '/quotation', title: '报价单', group: '采购销售', icon: Tag, perm: ['sales'] },
  { path: '/sales-return', title: '销售退货', group: '采购销售', icon: PackagePlus, perm: ['sales'] },
  { path: '/doc-list', title: '单据查询', group: '采购销售', icon: FileText, perm: ['sales', 'purchase', 'stock'] },
  { path: '/finance', title: '收付款管理', group: '资金管理', icon: Wallet, perm: ['finance'] },
  { path: '/stocktake', title: '盘点', group: '库存作业', icon: ClipboardCheck, perm: ['stock'] },
  { path: '/other-in', title: '其他入库', group: '库存作业', icon: Inbox, perm: ['stock'] },
  { path: '/other-out', title: '其他出库', group: '库存作业', icon: Upload, perm: ['stock'] },
  { path: '/transfer', title: '移库', group: '库存作业', icon: ArrowLeftRight, perm: ['stock'] },
  { path: '/inventory', title: '库存查询', group: '库存作业', icon: Boxes, perm: ['stock', 'sales', 'purchase'] },
  { path: '/reports', title: '报表中心', group: '', icon: TrendingUp, perm: ['reports'] },
  { path: '/system', title: '用户与角色', group: '系统设置', icon: Users, perm: ['system'] },
  { path: '/audit', title: '操作日志', group: '系统设置', icon: FileText, perm: ['system'] },
  { path: '/settings', title: '其他设置', group: '系统设置', icon: Settings },
  { path: '/subscription', title: '订阅与计费', group: '系统设置', icon: CreditCard },
]

// 一级分组图标
const groupIcons: Record<string, any> = {
  '基础档案': Warehouse,
  '采购销售': Store,
  '资金管理': Wallet,
  '库存作业': Boxes,
  '系统设置': Settings,
}

const visibleMenus = computed(() => menus.filter((m) => can(m.perm)))
const topMenus = computed(() => visibleMenus.value.filter((m) => !m.group)) // 一级菜单（首页看板/报表中心）
const groups = computed(() => [...new Set(visibleMenus.value.map((m) => m.group).filter(Boolean))])
const groupItems = (g: string) => visibleMenus.value.filter((x) => x.group === g)

onMounted(() => {
  // 刷新会话时同步最新权限/角色
  if (auth.isLoggedIn) auth.loadProfile().catch(() => {})
  // 首次登录落到首页时自动走一遍新手引导（看过的不再打扰）
  if (localStorage.getItem('jxc_guide_done') !== '1' && router.currentRoute.value.path === '/') {
    setTimeout(() => ui.startTour(), 900)
  }
})

// 顶栏「?」：重新看引导 / 打开使用手册
function onHelp(cmd: string) {
  if (cmd === 'tour') {
    if (router.currentRoute.value.path !== '/') router.push('/')
    setTimeout(() => ui.startTour(), 350)
  } else {
    router.push('/help')
  }
}

const daysLeft = computed(() => auth.daysLeft)
const expireType = computed(() => {
  const d = daysLeft.value ?? 99
  if (d <= 0) return 'danger'
  if (d <= 7) return 'warning'
  return 'success'
})

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <el-container style="height: 100vh">
    <el-aside width="208px">
      <div class="logo">
        <el-icon :size="20" color="var(--el-color-primary)"><Store /></el-icon>
        <span>云进销存</span>
      </div>
      <el-menu router :default-active="router.currentRoute.value.path" class="menu" data-guide="menu">
        <el-menu-item v-for="m in topMenus" :key="m.path" :index="m.path" :data-guide="m.path === '/reports' ? 'reports' : undefined">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.title }}</span>
        </el-menu-item>
        <el-sub-menu v-for="g in groups" :key="g" :index="g">
          <template #title>
            <el-icon><component :is="groupIcons[g]" /></el-icon>
            <span>{{ g }}</span>
          </template>
          <el-menu-item v-for="m in groupItems(g)" :key="m.path" :index="m.path">
            <el-icon><component :is="m.icon" /></el-icon>
            <span>{{ m.title }}</span>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="tenant">
          <el-icon :size="15"><Store /></el-icon>
          <span class="tenant-name">{{ auth.tenant?.name || '单机版' }}</span>
          <el-tag v-if="auth.tenant && daysLeft !== null" size="small" :type="expireType" class="tenant-tag">
            {{ daysLeft === 0 ? '已到期' : `剩 ${daysLeft} 天` }}
          </el-tag>
          <el-button v-if="auth.tenant && daysLeft !== null && daysLeft <= 7" link type="primary" size="small" @click="router.push('/subscription')">
            去续费
          </el-button>
        </div>
        <div class="right">
          <el-dropdown trigger="click" @command="onHelp">
            <el-button text circle title="帮助与引导">
              <el-icon :size="17"><HelpCircle /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="tour">快速上手（1 分钟引导）</el-dropdown-item>
                <el-dropdown-item command="manual">使用手册 / 常见问题</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-dropdown>
            <span class="user" data-guide="user">
              <el-avatar :size="28" style="background: var(--el-color-primary)">{{ (auth.displayName || '用')[0] }}</el-avatar>
              <span style="margin-left: 8px">{{ auth.displayName || '用户' }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="router.push('/help')">使用手册</el-dropdown-item>
                <el-dropdown-item @click="router.push('/subscription')">订阅与计费</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main><router-view /></el-main>
    </el-container>
    <GuideTour />
  </el-container>
</template>

<style scoped>
.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-weight: 600;
  font-size: 16px;
  letter-spacing: 2px;
  border-bottom: 1px solid var(--el-border-color-light);
}
.menu {
  border-right: none;
  height: calc(100vh - 56px);
  overflow-y: auto;
}
.menu :deep(.el-menu-item),
.menu :deep(.el-sub-menu__title) {
  height: 42px;
  line-height: 42px;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
}
.tenant {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.tenant-name {
  font-weight: 500;
}
.tenant-tag {
  margin-left: 2px;
}
.user {
  display: flex;
  align-items: center;
  cursor: pointer;
  outline: none;
}
.right {
  display: flex;
  align-items: center;
  gap: 6px;
}
.el-aside {
  border-right: 1px solid var(--el-border-color-light);
}
</style>
