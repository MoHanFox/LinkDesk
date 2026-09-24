# 回声实验室 Exam · 带用户系统的短链接服务

一个用 Go 写的迷你短链接 SaaS:用户注册登录后管理自己的短链,有每日创建配额,短链可访问跳转并统计点击次数。

- 后端:`Go` + `Gin` + `GORM` + `MySQL`
- 前端:`frontend/` 目录下的纯静态页面(无构建步骤,直接打开即可)

---

# 使用的 AI 工具

> DeepSeek harness + deepseek flash v4.1

---

# 技术栈与选型理由

| 用途 | 选型 | 为什么选它 |
|---|---|---|
| 语言 | Go 1.26 | 考核要求;标准库自带 `crypto/rand`、`net/http`,写 HTTP 服务不需要额外依赖 |
| Web 框架 | Gin v1.12 | 教程指定;路由分组能直接把鉴权挂在一组接口上,不用逐个 handler 写 |
| ORM | GORM v1.31 | 教程指定;`AutoMigrate` 自动建表,省掉手写 DDL;`TranslateError` 能把 MySQL 的重复键错误翻译成 `gorm.ErrDuplicatedKey`,注册接口靠它返回 409 |
| 数据库 | MySQL 26.7 | 持久化要求;唯一索引能直接保证用户名、短码、token 不重复 |
| 配置 | Viper | 一个 `config.yaml` 管住端口、数据库、配额、CORS、密钥有效期,不用改代码 |
| 密码哈希 | `golang.org/x/crypto/bcrypt` | 需求明确要求,且 bcrypt 自带 salt,同一个密码每次哈希结果都不同 |
| 登录凭证 | 随机 token + 会话表 | 需求允许 JWT 或随机 token 二选一。选随机 token 是因为它**能吊销**——删掉会话记录就立刻失效,而 JWT 签发后只能等到过期 |
| 字体/配色 | OPPO Sans 3 + 蓝色主题 | 前端界面风格取自 HaloForum |

---

# 功能

- 注册、登录,密码 bcrypt 哈希后入库
- 登录签发随机 token,写入会话表,带有效期(默认 24 小时)
- 鉴权中间件统一校验 token,未登录返回 401
- 创建短链,后端生成唯一短码,按配置的公开地址拼出 `short_url`
- 查看自己的短链列表(分页)和详情
- 删除自己的短链
- 访问短链 302 跳转,并给点击数 +1
- **数据隔离**:每个人只能看到和操作自己的短链
- **每日创建配额**:每个用户每天最多创建 N 条,超出返回 429
- **CORS 配置化**:允许的跨域来源写在配置里

---

# 快速开始

## 1. 环境要求

- Go 1.26+
- MySQL 8.0+(本项目在 MySQL 26.7 上验证)
- 一个能跑静态页面的方式:直接双击 `frontend/index.html`,或用 IDE 的内置 Web 服务器

## 2. 建库

```sql
CREATE DATABASE linkdesk CHARACTER SET utf8mb4;
```

`AutoMigrate` 出 `users`、`links`、`sessions` 三张表。

## 3. 改配置

编辑项目根目录的 `config.yaml`(**必须放在项目根目录**,程序从这里读):

```yaml
server:
  port: ":8080"

database:
  host: "127.0.0.1"
  port: "3306"
  path: "linkdesk"
  user: "你的数据库账号"
  password: "你的数据库密码"

cors_origins:
  - "http://localhost:63342"    # 前端页面所在地址,按实际改

token_ttl: "24h"
daily_link_quota: 5
public_base_url: "http://localhost:8080"
timezone: "Asia/Shanghai"
```

## 4. 启动后端

```bash
go run .
```

## 5. 打开前端

用 IDE 的内置服务器打开 `frontend/index.html`,或在项目根目录起一个静态服务器:

```bash
cd frontend
python -m http.server 63342
```

> **注意**:`frontend/config.js` 里的 `API_BASE_URL` 指向 `http://localhost:8080`。如果前端页面不在 `http://localhost:63342` 上,记得把实际地址加进 `config.yaml` 的 `cors_origins`,否则浏览器会因为跨域把响应拦掉,前端会显示"无法连接后端"。

---

# 配置项说明

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `server.port` | `:8080` | HTTP 服务监听端口 |
| `database.host` | `127.0.0.1` | 数据库地址 |
| `database.port` | `3306` | 数据库端口 |
| `database.path` | `linkDesk` | 数据库名 |
| `database.user` | `root` | 数据库账号 |
| `database.password` | — | 数据库密码 |
| `cors_origins` | `[]`(不允许任何跨域) | 允许访问后端的前端地址列表 |
| `token_ttl` | `24h` | 登录 token 有效期 |
| `daily_link_quota` | `5` | 每个用户每天最多创建多少条短链 |
| `public_base_url` | `http://localhost:8080` | 生成 `short_url` 用,短链 = 它 + `/r/` + 短码 |
| `timezone` | `Asia/Shanghai` | 每日配额按哪个时区算"当天"的边界 |

---

# 数据模型

三张表都在 `db/model.go`,启动时自动迁移。

## `users` — 用户

| 字段 | 说明 |
|---|---|
| `id` | 主键 |
| `username` | 唯一索引,3-32 个字符 |
| `password` | **bcrypt 哈希**(60 字符),绝不存明文 |
| `created_at` / `updated_at` / `deleted_at` | 由 `gorm.Model` 提供 |

## `links` — 短链接

| 字段 | 说明 |
|---|---|
| `id` | 主键 |
| `user_id` | 所属用户,建了索引,所有查询都按它过滤 |
| `original_url` | 原始地址,最长 2048 |
| `code` | 短码,唯一索引,6 位随机字母数字 |
| `click_count` | 点击次数,默认 0 |
| `created_at` / `updated_at` / `deleted_at` | 由 `gorm.Model` 提供 |

## `sessions` — 登录会话

| 字段 | 说明 |
|---|---|
| `id` | 主键 |
| `user_id` | 归属用户 |
| `token` | 唯一索引,鉴权时按它反查 |
| `expires_at` | 过期时间,由 `token_ttl` 算出 |
| `created_at` / `updated_at` / `deleted_at` | 由 `gorm.Model` 提供 |

**为什么单独建会话表**:一个用户可能同时有多条会话(电脑、手机、无痕窗口各一条),用一张表存天然支持;而且删掉某条记录就能让那个 token 立刻失效。

---

# API

统一前缀 `/api/v1`,公开跳转接口 `/r/:code` 不带前缀。

| 方法 | 路径 | 鉴权 | 请求 |
|---|---|---|---|
| GET | `/health` | 否 | 无 |
| POST | `/api/v1/auth/register` | 否 | `{"username","password"}` |
| POST | `/api/v1/auth/login` | 否 | `{"username","password"}` |
| GET | `/api/v1/me` | 是 | 无 |
| GET | `/api/v1/links?page=1&page_size=20` | 是 | 查询参数 `page`、`page_size` |
| POST | `/api/v1/links` | 是 | `{"url"}` |
| GET | `/api/v1/links/:id` | 是 | 路径参数 `id` |
| DELETE | `/api/v1/links/:id` | 是 | 路径参数 `id` |
| GET | `/r/:code` | 否 | 路径参数 `code` |

## 统一响应格式

成功响应直接返回数据,不额外包一层 `data`。

错误响应统一格式:

```json
{ "code": "INVALID_CREDENTIALS", "message": "用户名或密码错误" }
```

前端按 `code` 翻译成中文提示。

已经定义的 `code`:`INVALID_CREDENTIALS`、`USERNAME_EXISTS`、`INVALID_URL`、`DAILY_QUOTA_EXCEEDED`、`UNAUTHORIZED`、`NOT_FOUND`、`VALIDATION_FAILED`。

## 状态码

| 状态码 | 场景 |
|---|---|
| `200` | 查询、登录、删除成功 |
| `201` | 注册、创建短链成功 |
| `302` | 短链跳转 |
| `400` | JSON 格式错误或参数校验失败 |
| `401` | 未登录、token 缺失/无效/过期、账号密码错误 |
| `404` | 资源不存在,**或资源不属于当前用户** |
| `409` | 用户名已存在 |
| `429` | 超过每日创建配额 |
| `500` | 服务端错误 |


---

# 接口调用示例

## 健康检查

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

数据库不可用时返回 `500`,不会假报健康。

## 注册

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"tom","password":"123456"}'
# 201 {"code":"OK","message":"注册成功"}
```

用户名重复返回 `409 USERNAME_EXISTS`,长度不合规返回 `400 VALIDATION_FAILED`。

## 登录

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"tom","password":"123456"}'
```

```json
{
  "token": "KFC_V_ME_50",
  "expires_at": "2026-09-25T15:26:33Z",
  "user": { "id": 1, "username": "tom" }
}
```

账号不存在和密码错误返回**完全一样**的 `401 INVALID_CREDENTIALS`,避免别人拿登录接口试出哪些用户名已注册。

下面示例里的 `$TOKEN` 换成上面返回的 token。

## 当前用户

```bash
curl http://localhost:8080/api/v1/me -H "Authorization: Bearer $TOKEN"
# 200 {"id":1,"username":"tom"}
```

前端刷新页面后靠这个接口确认本地 token 是否还有效。

## 创建短链

```bash
curl -X POST http://localhost:8080/api/v1/links \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://go.dev"}'
```

```json
{
  "id": 1,
  "code": "zpTGl8",
  "original_url": "https://go.dev",
  "short_url": "http://localhost:8080/r/zpTGl8",
  "click_count": 0,
  "created_at": "2026-09-24T15:26:40.018Z"
}
```

- 只接受 `http://` 和 `https://`,`javascript:`、`data:`、`file:` 一律 `400 INVALID_URL`
- 短码由后端生成,撞码会自动换一个重试,不会覆盖已有记录
- `short_url` 由后端按 `public_base_url` 拼,前端不自己拼
- 超过每日配额返回 `429 DAILY_QUOTA_EXCEEDED`

## 短链列表

```bash
curl "http://localhost:8080/api/v1/links?page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "items": [ { "id": 1, "code": "zpTGl8", "...": "..." } ],
  "page": 1,
  "page_size": 20,
  "total": 1
}
```

- `page` 默认 1,`page_size` 默认 20、上限 100
- 按创建时间倒序
- 只返回当前用户的短链
- 没有数据时 `items` 是 `[]`,不是 `null`

## 短链详情 / 删除

```bash
curl http://localhost:8080/api/v1/links/1 -H "Authorization: Bearer $TOKEN"

curl -X DELETE http://localhost:8080/api/v1/links/1 -H "Authorization: Bearer $TOKEN"
# 200 {"message":"删除成功"}
```

别人的短链一律 `404`,不返回 `403`,避免暴露"这条资源存在"。

## 访问短链

```bash
curl -i http://localhost:8080/r/zpTGl8
# HTTP/1.1 302 Found
# Location: https://go.dev/
```

不需要登录。每次访问点击数 +1,短码不存在或已删除返回 `404`。

> 这个接口是给**浏览器直接访问**的,不要用 `fetch()` 当 JSON 接口调。

---

# 鉴权说明

- 登录成功后拿到 `token`,之后所有需要登录的接口都要带上:

```http
Authorization: Bearer <token>
```

- token 通过 URL 参数传递一律不接受(会被写进日志、浏览器历史、Referer)
- token 有有效期(默认 24 小时),过期后返回 `401`
- 校验统一在 `middleware/auth.go` 里做,挂在路由组上,**handler 里没有一行重复的鉴权代码**
- **用户身份只能来自 token**:`user_id` 绝不从请求体或查询参数里取,这是数据隔离的前提
- 以下情况一律返回 `401 UNAUTHORIZED`:没有 `Authorization` 头、格式不是 `Bearer <token>`、token 不存在、token 已过期、token 对应的会话查不到

---

# 每日配额

- 每个用户单独计算,用户之间互不影响
- 每天最多成功创建 `daily_link_quota` 条,默认 5
- 当天边界按 `timezone` 配置的时区算
- **只有成功保存的才计入配额**:参数错误、未登录、URL 不合法、数据库报错都不计
- **删除不会归还配额**:已经用掉的额度当天不恢复
- 超出返回 `429` + `{"code":"DAILY_QUOTA_EXCEEDED"}`
- 后端必须自己校验,不能只靠前端(前端校验能被 `curl` 绕过)
- "查配额 + 写入"两步用锁串起来,避免并发请求同时通过检查

验证方式:把 `daily_link_quota` 改成 `2`,重启服务,同一账号创建第 3 条时会被拒绝。

---

# CORS

后端支持**配置化 CORS**(`cors_origins`),实现在 `middleware/cors.go`,默认使用gin自带的CORS:

- 只有来源在配置白名单里才回 `Access-Control-Allow-Origin`,**不会返回 `*`**
- 正确处理 `OPTIONS` 预检请求,返回 `204`
- 允许 `Content-Type` 和 `Authorization` 两个请求头(登录后每个请求都带 `Authorization`,少放一个就会被浏览器拦)
- 允许 `GET`、`POST`、`DELETE`、`OPTIONS`
- 带 `Vary: Origin`,避免代理把给 A 站点的响应缓存后发给 B

**前端页面地址变了就要改 `cors_origins`**,否则浏览器会拦掉响应,前端显示"无法连接后端"——注意这句提示是误导,实际是连上了但不让 JS 读。判断方法:F12 看 Console 有没有 `blocked by CORS policy`,或看后端日志里是不是只有 `OPTIONS` 没有真正的请求。

---

# 项目结构

```
EpcExam/
├── main.go                  入口:加载配置 → 连数据库 → 建路由 → 启动
├── config.yaml              配置文件(放项目根目录)
├── config/
│   └── config.go            Viper 读配置 + 默认值
├── db/
│   ├── db.go                连接、AutoMigrate、Ping
│   ├── model.go             User / Link / Session
│   └── db_test.go           数据层测试
├── handler/
│   ├── router.go            路由注册 + /health
│   ├── auth.go              注册 / 登录 / 当前用户
│   └── link.go              创建 / 列表 / 详情 / 删除 / 跳转
├── middleware/
│   ├── auth.go              鉴权中间件
│   └── cors.go              CORS 中间件
└── frontend/                纯静态前端页面
```

# AI 辅助的地方

## 1. 用 AI 找出前端所有的**连接约定**

> 服务地址:`config.js` 里 `API_BASE_URL = "http://localhost:8080"`,即后端默认监听 8080。
>
> 前端用 `fetch`,带 `Authorization: Bearer <token>` 和 `Content-Type: application/json`,所以 8080 必须开 CORS,允许 `Authorization` 头并处理 `OPTIONS` 预检。
>
> 请求超时 12 秒,超时/连不上时前端按 `NETWORK_ERROR` 处理(显示"无法连接后端")。
>
> 错误响应体格式必须是 `{"code": "...", "message": "..."}`;HTTP 状态码决定 401/404 分支。前端认得的 code:`INVALID_CREDENTIALS`、`USERNAME_EXISTS`、`INVALID_URL`、`DAILY_QUOTA_EXCEEDED`、`UNAUTHORIZED`、`NOT_FOUND`、`VALIDATION_FAILED`。

## 2. AI 得出 API 清单

前端只调用一部分接口,完整契约以需求文档为准(例如 `GET /api/v1/links/:id` 前端没用到,但需求要求实现)。

## 3. AI 建立所需的结构体

AI 最初自己写了一套 GORM tag,我改成了内嵌 `gorm.Model`——让它统一提供 `ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`,省掉重复字段,还自带软删除。

## 4. AI 编写和测试 API

接口实现、鉴权中间件、CORS 中间件、数据层测试都由 AI 起草,我逐条跑了验收(注册 → 登录 → 带 token 创建 → 302 跳转 → 点击数 +1 → 超配额被拒 → 数据隔离 → 重启后数据仍在)。

## 5. 我对 AI 产出的调整

- **内嵌 `gorm.Model`**:AI 一开始把 `ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt` 一个个写出来,我改成内嵌 `gorm.Model`,字段一次管全,还自带软删除。
- **把 `model` 包合并进 `db` 包**:AI 建议数据模型单独成一个包(为了避免 `db` 和 `handler` 互相 import)。我把模型直接放进 `db` 包——`db` 不 import `handler`,同样不会有循环依赖,少一个包更好找。
- **端口和数据库地址走配置**:AI 最初把 `r.Run(":8080")` 和 `tcp(127.0.0.1:3306)` 写死在代码里,我改成读 `config.yaml` 的 `server.port` 和 `database.host`/`database.port`。
- **加了一把 `sync.Mutex` 想防重复注册**:后来发现唯一性本来就由数据库唯一索引保证,那把锁挡不住多实例部署,还白白串行化所有注册请求,删掉了。(现在只在"查配额 + 写入"这个真正需要串行的场景里用锁。)
