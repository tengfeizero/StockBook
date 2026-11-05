// 金额工具：数据库/后端一律存"分"(int)，界面展示"元"
export function fenToYuan(fen: number | null | undefined): string {
  if (fen === null || fen === undefined) return '0.00'
  return (fen / 100).toFixed(2)
}

export function fenToYuanSymbol(fen: number | null | undefined): string {
  return '¥' + fenToYuan(fen)
}

// 界面输入"元"（字符串）→ 存储分，四舍五入到分
export function yuanToFen(yuan: string | number): number {
  const n = typeof yuan === 'string' ? parseFloat(yuan) : yuan
  if (isNaN(n)) return 0
  return Math.round(n * 100)
}
