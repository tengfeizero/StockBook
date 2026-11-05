import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

// 统一响应格式：{ code: 0, msg: "ok", data: ... }
export interface ApiResponse<T = any> {
  code: number
  msg: string
  data: T
}

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

request.interceptors.response.use(
  (resp) => {
    // 二进制响应（如模板下载）直接返回
    if (resp.config.responseType === 'blob') return resp.data
    const body = resp.data as ApiResponse
    if (body.code !== 0) {
      if (body.code === 1002) {
        // 未登录/登录过期
        localStorage.removeItem('token')
        router.push('/login')
      }
      ElMessage.error(body.msg || `错误 ${body.code}`)
      return Promise.reject(body)
    }
    return body.data
  },
  (err) => {
    ElMessage.error(err.message || '网络错误')
    return Promise.reject(err)
  }
)

export default request
