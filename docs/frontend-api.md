# oh-my-commic 后端 API 契约（前端对接用）

所有接口前缀 `/api`。认证用 cookie（登录后自动 set `session` cookie）——前端 `fetch` 必须带 `credentials: 'include'`。图片用 `/media/...` 直接访问（无需 cookie）。

开发时前端 Vite 用 proxy 把 `/api` 和 `/media` 转发到后端 `http://localhost:8080`。

> 请求体只发**已知字段**（后端对部分接口开启了 DisallowUnknownFields）。未登录访问受保护接口返回 **401**；跨用户/不存在返回 **404**；AI/上游错误返回 **502**；校验错误 **400**。

## 认证
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| POST | `/api/register` | `{nickname, password}` | 201 |
| POST | `/api/login` | `{nickname, password}` | 200 + set-cookie，返回 `User` |
| POST | `/api/logout` | — | 200 |

## Book
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| GET | `/api/books` | — | `Book[]`（仅自己的） |
| POST | `/api/books` | `{title, style?, summary?}` | 201 `Book`（style 默认 `ghibli`） |
| GET | `/api/books/{id}` | — | `Book` |
| PUT | `/api/books/{id}` | `{title, style, summary}` | `Book` |
| DELETE | `/api/books/{id}` | — | 200 |

## 资产（角色/宠物/场景，均在某本书下）
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| POST | `/api/books/{bookId}/upload` | multipart，字段 `file`（png/jpg/webp，≤5MB） | `{imageUrl}` |
| GET | `/api/books/{bookId}/characters` | — | `Character[]` |
| POST | `/api/books/{bookId}/characters` | `{type, name, gender, age, personality, description, imageUrl}` | 201 `Character` |
| PUT | `/api/books/{bookId}/characters/{id}` | 同上 | `Character` |
| DELETE | `/api/books/{bookId}/characters/{id}` | — | 200 |
| GET/POST/PUT/DELETE | `/api/books/{bookId}/scenes[/{id}]` | `{name, description, imageUrl}` | `Scene` |

> 角色 `type` 取 `"character"`（角色）或 `"pet"`（宠物）。

## 章节
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| GET | `/api/books/{bookId}/chapters` | — | `Chapter[]` |
| POST | `/api/books/{bookId}/chapters` | `{title}` | 201 `Chapter`（status `draft`） |
| GET | `/api/chapters/{id}` | — | `Chapter` |
| PUT | `/api/chapters/{id}/status` | `{status}` | `Chapter`（draft→storyboarding→rendering→done） |

## 分镜
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| GET | `/api/chapters/{id}/panels` | — | `Panel[]`（按 order） |
| PUT | `/api/chapters/{id}/panels` | `Panel[]`（整章替换，后端重排 order 0..n-1） | `Panel[]` |
| PUT | `/api/panels/{id}` | `{caption, characterIds, sceneId, imagePrompt}` | `Panel` |

## AI：对话分镜 & 生图
| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| POST | `/api/chapters/{id}/converse` | `{messages:[{role,content},...]}` | `{reply}` |
| POST | `/api/chapters/{id}/storyboard` | `{messages:[...], panelCount}` | `Panel[]`（落库 + 章节转 storyboarding） |
| POST | `/api/panels/{id}/render` | — | `Panel`（同步等待生图完成，可能 10-60s，imageUrl 落 `/media/{bookId}/..`，status `done`；失败 status `failed`） |

## 数据结构（JSON）
```ts
type User      = { id:number; nickname:string; createdAt:string }
type Book      = { id:number; userId:number; title:string; coverUrl:string; style:string; summary:string; isPublic:boolean; createdAt:string; updatedAt:string }
type Character = { id:number; bookId:number; type:'character'|'pet'; name:string; gender:string; age:string; personality:string; description:string; imageUrl:string }
type Scene     = { id:number; bookId:number; name:string; description:string; imageUrl:string }
type Chapter   = { id:number; bookId:number; order:number; title:string; status:'draft'|'storyboarding'|'rendering'|'done'; createdAt:string }
type Panel     = { id:number; chapterId:number; order:number; caption:string; characterIds:number[]; sceneId:number; imagePrompt:string; imageUrl:string; status:'pending'|'rendering'|'done'|'failed' }
```
