// 中文数字：1–10 用「一…十」，超过用阿拉伯数字兜底。
const CN_DIGITS = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十']

export function cnNumeral(n: number): string {
  if (n >= 1 && n <= 10) return CN_DIGITS[n - 1]
  return String(n)
}
