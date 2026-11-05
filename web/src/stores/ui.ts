import { defineStore } from 'pinia'

// 全局 UI 状态：新手引导气泡的开关（顶栏「?」和手册页都能调起）
export const useUiStore = defineStore('ui', {
  state: () => ({
    tourVisible: false,
  }),
  actions: {
    startTour() {
      this.tourVisible = true
    },
    stopTour() {
      this.tourVisible = false
    },
  },
})
