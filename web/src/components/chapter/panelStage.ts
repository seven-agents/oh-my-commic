import type { Panel } from '../../api/types'

// 判断某一格是否已经过第2段“逐格解析”。
// 未解析：结构字段全空（旁白/地点/出图提示词都为空）。
export function isPanelProcessed(panel: Panel): boolean {
  return (
    panel.caption.trim() !== '' ||
    panel.location.trim() !== '' ||
    panel.imagePrompt.trim() !== ''
  )
}

// 出图门控：未解析的格不能生成图（会画出没内容的通用图）。
export function canRenderPanel(panel: Panel): boolean {
  return isPanelProcessed(panel)
}
