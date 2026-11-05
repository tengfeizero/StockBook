import { defineStore } from 'pinia'
import request from '../utils/request'

export interface TenantInfo {
  code: string
  name: string
  plan_code?: string
  expire_at?: string
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    displayName: localStorage.getItem('displayName') || '',
    roleName: localStorage.getItem('roleName') || '',
    permissions: JSON.parse(localStorage.getItem('permissions') || '[]') as string[],
    tenant: JSON.parse(localStorage.getItem('tenant') || 'null') as TenantInfo | null,
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    // 是否拥有任一权限（"*" 通配）
    hasPerm: (s) => (anyOf: string[]) => {
      if (s.permissions.includes('*')) return true
      return anyOf.some((p) => s.permissions.includes(p))
    },
    // 订阅剩余天数（未登录租户 / 单机版返回 null）
    daysLeft: (s): number | null => {
      if (!s.tenant?.expire_at) return null
      const diff = new Date(s.tenant.expire_at).getTime() - Date.now()
      return Math.max(0, Math.ceil(diff / 86400000))
    },
  },
  actions: {
    async login(username: string, password: string, tenantCode = '') {
      const data: any = await request.post('/auth/login', {
        username,
        password,
        tenant_code: tenantCode,
      })
      this.applyToken(data)
      await this.loadProfile()
    },
    // 注册开户后直接登录
    async loginByToken(resp: any) {
      this.applyToken(resp)
      await this.loadProfile()
    },
    applyToken(data: any) {
      this.token = data.token
      this.displayName = data.display_name || ''
      localStorage.setItem('token', this.token)
      localStorage.setItem('displayName', this.displayName)
      if (data.tenant_code) {
        this.tenant = {
          code: data.tenant_code,
          name: data.tenant_name || '',
          plan_code: data.plan_code,
          expire_at: data.expire_at,
        }
      }
    },
    async loadProfile() {
      const data: any = await request.get('/auth/me')
      this.displayName = data.display_name || this.displayName
      this.roleName = data.role_name || ''
      this.permissions = data.permissions || []
      this.tenant = data.tenant || null
      localStorage.setItem('displayName', this.displayName)
      localStorage.setItem('roleName', this.roleName)
      localStorage.setItem('permissions', JSON.stringify(this.permissions))
      localStorage.setItem('tenant', JSON.stringify(this.tenant))
    },
    logout() {
      this.token = ''
      this.displayName = ''
      this.roleName = ''
      this.permissions = []
      this.tenant = null
      localStorage.removeItem('token')
      localStorage.removeItem('displayName')
      localStorage.removeItem('roleName')
      localStorage.removeItem('permissions')
      localStorage.removeItem('tenant')
    },
  },
})
