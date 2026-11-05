import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue') },
    // SaaS 注册开户（公开）
    { path: '/register', component: () => import('../views/Register.vue'), meta: { title: '注册企业' } },
    {
      path: '/',
      component: () => import('../layout/MainLayout.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '首页' } },
        // M1 基础档案
        { path: 'products', component: () => import('../views/Products.vue'), meta: { title: '商品档案', perm: ['archive'] } },
        { path: 'categories', component: () => import('../views/Categories.vue'), meta: { title: '商品分类', perm: ['archive'] } },
        { path: 'suppliers', component: () => import('../views/Partners.vue'), props: { kind: 'supplier' as const }, meta: { title: '供应商', perm: ['archive'] } },
        { path: 'customers', component: () => import('../views/Partners.vue'), props: { kind: 'customer' as const }, meta: { title: '客户', perm: ['archive'] } },
        { path: 'opening-stock', component: () => import('../views/OpeningStock.vue'), meta: { title: '期初库存', perm: ['archive'] } },
        { path: 'locations', component: () => import('../views/Locations.vue'), meta: { title: '库位管理', perm: ['archive'] } },
        // M2 核心单据
        { path: 'purchase-order', component: () => import('../views/DocEdit.vue'), props: { docType: 'purchase_order' }, meta: { title: '采购订单', perm: ['purchase'] } },
        { path: 'purchase-in', component: () => import('../views/DocEdit.vue'), props: { docType: 'purchase_in' }, meta: { title: '采购入库', perm: ['purchase'] } },
        { path: 'sales-out', component: () => import('../views/DocEdit.vue'), props: { docType: 'sales_out' }, meta: { title: '销售出库', perm: ['sales'] } },
        { path: 'quotation', component: () => import('../views/DocEdit.vue'), props: { docType: 'quotation' }, meta: { title: '报价单', perm: ['sales'] } },
        { path: 'purchase-return', component: () => import('../views/DocEdit.vue'), props: { docType: 'purchase_return' }, meta: { title: '采购退货', perm: ['purchase'] } },
        { path: 'sales-return', component: () => import('../views/DocEdit.vue'), props: { docType: 'sales_return' }, meta: { title: '销售退货', perm: ['sales'] } },
        { path: 'doc-list', component: () => import('../views/DocList.vue'), meta: { title: '单据查询', perm: ['sales', 'purchase', 'stock'] } },
        { path: 'finance', component: () => import('../views/Finance.vue'), meta: { title: '收付款管理', perm: ['finance'] } },
        { path: 'other-in', component: () => import('../views/DocEdit.vue'), props: { docType: 'other_in' }, meta: { title: '其他入库', perm: ['stock'] } },
        { path: 'other-out', component: () => import('../views/DocEdit.vue'), props: { docType: 'other_out' }, meta: { title: '其他出库', perm: ['stock'] } },
        // M3 库存作业与报表
        { path: 'stocktake', component: () => import('../views/Stocktake.vue'), meta: { title: '盘点', perm: ['stock'] } },
        { path: 'transfer', component: () => import('../views/DocEdit.vue'), props: { docType: 'transfer' }, meta: { title: '移库', perm: ['stock'] } },
        { path: 'inventory', component: () => import('../views/Inventory.vue'), meta: { title: '库存查询', perm: ['stock', 'sales', 'purchase'] } },
        { path: 'reports', component: () => import('../views/Reports.vue'), meta: { title: '报表中心', perm: ['reports'] } },
        // M4 系统
        { path: 'system', component: () => import('../views/System.vue'), meta: { title: '用户与角色', perm: ['system'] } },
        { path: 'audit', component: () => import('../views/AuditLog.vue'), meta: { title: '操作日志', perm: ['system'] } },
        { path: 'settings', component: () => import('../views/Settings.vue'), meta: { title: '系统设置' } },
        // SaaS 订阅与计费（登录即可查看，续费需 system 权限）
        { path: 'subscription', component: () => import('../views/Subscription.vue'), meta: { title: '订阅与计费' } },
        // 使用手册（v0.24）：内容与 docs/用户手册.md 同源
        { path: 'help', component: () => import('../views/Help.vue'), meta: { title: '使用手册' } },
      ],
    },
    // A4 打印页（独立布局，不套侧边栏）
    { path: '/print/:id', component: () => import('../views/DocPrint.vue'), meta: { title: '打印单据' } },
    // 58mm 热敏小票打印页（v0.14）
    { path: '/print/:id/ticket', component: () => import('../views/DocTicket.vue'), meta: { title: '打印小票' } },
    // 盘点差异表（v0.15）：可打印 + 导出 CSV
    { path: '/print/:id/diff', component: () => import('../views/StocktakeDiff.vue'), meta: { title: '盘点差异表' } },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.path !== '/login' && to.path !== '/register' && !auth.isLoggedIn) return '/login'
  // 页面级权限：无权限回首页（菜单已隐藏，防手输地址）
  const perm = to.meta.perm as string[] | undefined
  if (perm && auth.isLoggedIn && auth.permissions.length > 0 && !auth.hasPerm(perm)) return '/'
})

export default router
