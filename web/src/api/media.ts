// 图片地址工具：后端返回的 imageUrl/coverUrl 可能是相对的 /media/.. 路径，
// 直接原样返回即可（Vite proxy 会把 /media 转发到后端）。空值返回 ''。
export function mediaUrl(url: string | undefined | null): string {
  if (!url) return ''
  return url
}
