# oh-my-commic 后端 API 概览

> **单一真相源是 [`docs/openapi.yaml`](./openapi.yaml)（OpenAPI 3.1）。**
> 本文件只是快速概览；端点、字段、状态码以 openapi.yaml 为准。
> 后端契约 E2E（`test/contract`）会用 openapi.yaml 校验每个响应——改任何端点必须同步更新它，否则测试变红。

## 约定
- **所有业务端点前缀 `/api/v1`**（register/login/logout/me、books、characters/scenes、chapters、panels、story 全部）。
- **`GET /api/health` 不版本化**（运维/CI 探活端点，保持稳定路径）。
- 认证用 cookie session：登录后自动 set `session` cookie，前端 `fetch` 必须带 `credentials: 'include'`。
- 图片经 `/media/...` 直接访问（无需 cookie，不在契约内）。开发时 Vite proxy 把 `/api` 和 `/media` 转发到后端。
- 请求体只发**已知字段**（部分接口开启 `DisallowUnknownFields`）。
- 错误统一为 `{"error": string}` 信封。

## 状态码约定
| 码 | 含义 |
|---|---|
| 200 / 201 | 成功 / 创建成功 |
| 400 | 参数或格式错误 |
| 401 | 未登录 |
| 402 | 积分不足（生图/漫画化） |
| 404 | 资源不存在或跨用户访问（不泄露存在性） |
| 429 | 上游限流 |
| 502 | AI/上游服务错误（不泄露上游细节） |
| 504 | 上游超时 |

## 端点速查（详情见 openapi.yaml）
- **认证**：`POST /api/v1/register`、`POST /api/v1/login`、`POST /api/v1/logout`、`GET /api/v1/me`
- **书**：`GET|POST /api/v1/books`、`GET|PUT|DELETE /api/v1/books/{id}`
- **资产**：`POST /api/v1/books/{bookId}/upload`；`GET|POST /api/v1/books/{bookId}/characters`、`PUT|DELETE .../characters/{id}`；`scenes` 同构
- **章节**：`GET|POST /api/v1/books/{bookId}/chapters`、`POST /api/v1/books/{bookId}/cover-chapter`、`GET|DELETE /api/v1/chapters/{id}`、`PUT /api/v1/chapters/{id}/status`
- **分镜**：`GET|PUT /api/v1/chapters/{id}/panels`（PUT 整章替换，后端重排 order）、`PUT /api/v1/panels/{id}`
- **AI**：`POST /api/v1/chapters/{id}/storyboard-chat`（第 1 段对话拆镜）、`POST /api/v1/panels/{id}/process`（第 2 段解析）、`POST /api/v1/panels/{id}/render`（同步生图，消耗积分）

## 数据结构
以 openapi.yaml 的 `components.schemas` 为准（`User` 含 `credits`、`Book`、`Character`、`Scene`、`Chapter`、`Panel`、`ConversationMsg`、错误信封 `Error`）。字段名/类型与 `internal/models` 一致。
