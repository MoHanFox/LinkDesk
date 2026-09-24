# 使用的AI工具

> Deepseek hardness + deepseek flashv4.1

# 技术栈
- Golang
- Http: Gin
- 数据库: Gorm、MySQL

# 项目说明

使用的API

| 方法   | 路径                                | 鉴权 | 请求                        |
|--------|-------------------------------------|------|-----------------------------|
| GET    | /health                             | 否   | 无                          |
| POST   | /api/v1/auth/register               | 否   | {"username","password"}     |
| POST   | /api/v1/auth/login                  | 否   | {"username","password"}     |
| GET    | /api/v1/me                          | 是   | 无                          |
| GET    | /api/v1/links?page=1&page_size=100  | 是   | 查询参数 page、page_size    |
| POST   | /api/v1/links                       | 是   | {"url"}                     |
| DELETE | /api/v1/links/{id}                  | 是   | 路径参数 id                 |
| GET    | /r/{code}                           | 否   | 路径参数 code               |

# AI 辅助的地方
- 使用**AI**找出前端所有的**连接约定**
> 连接约定
> 
> 服务地址：config.js 里 API_BASE_URL = "http://localhost:8080"，即后端默认监听 8080。
>
> 前端用 fetch，带 Authorization: Bearer <token> 和 Content-Type: application/json，所以 8080 必须开 CORS，允许 Authorization 头并处理 OPTIONS 预检。
>
> 请求超时 12 秒，超时/连不上时前端按 NETWORK_ERROR 处理（显示"无法连接后端"）。
>
> 错误响应体格式必须是 {"code": "...", "message": "..."}；HTTP 状态码决定 401/404 分支。前端认得的 code：INVALID_CREDENTIALS、USERNAME_EXISTS、INVALID_URL、DAILY_QUOTA_EXCEEDED、UNAUTHORIZED、NOT_FOUND、VALIDATION_FAILED。
- AI 得出API
- AI 建立所需的结构体 (AI在这里自己创建了gorm的json，我改成了gorm.Model)
