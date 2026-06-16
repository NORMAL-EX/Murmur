# Murmur

自托管的实时聊天室:**公共频道 + 一对一私信 + AI 机器人 + 完整管理后台**。

前端使用 **Vite 7 + React 19 + TypeScript + Tailwind CSS v4 + [coss-ui](https://coss.com/ui)(基于 Base UI)**;
后端使用 **Go + Gin + GORM + 纯 Go SQLite + gorilla/websocket + JWT**。

---

## ✨ 功能

- **账号体系**:注册 / 登录 / 登出(JWT),可开关注册、可开启注册审核;个人资料(昵称、简介、头像上传)
- **三级角色 + 机器人**:`super_admin` / `admin` / `user` / `bot`,权限逐级收紧,所有鉴权在后端
- **公共频道**:内置「大厅」,管理员可增删改、设只读 / 置顶;历史消息向上滚动分页加载
- **实时通信**:WebSocket 集中 Hub,新消息即时推送,在线状态与「正在输入」提示
- **私信(DM)**:任意两个用户一对一会话,会话列表、未读计数、离线消息补拉
- **@提及**:解析 `@用户名`,高亮可点击跳转资料,产生提及通知与未读红点;输入 `@` 弹出成员补全
- **消息能力**:纯文本 + 基础 Markdown 渲染、表情回应、本人可编辑/软删、管理员可删任意消息、频道内搜索
- **AI 机器人**:`@机器人` 即答。从频道最近消息按**字符上限(默认 5000)**回溯取上下文,调用 OpenAI 兼容接口;
  全部参数后台可配(开关、Base URL、Key、模型、人设、温度、max tokens、上下文上限、冷却、是否含私信、名称/头像),
  API Key 加密存储且不回传明文,失败有友好兜底
- **发送频率限制**:全局默认 + 按角色默认 + **每个用户单独覆盖**(-1 继承 / 0 不限 / N 每分钟),
  后端滑动窗口强制执行,超限返回需等待秒数,前端禁用发送并倒计时
- **管理后台 `/admin`**:仪表盘、用户管理(封禁/解封、角色、单独频率)、注册审核、频道管理、
  AI 设置(含「测试连通性」)、站点设置(标题/描述/公告/注册开关/私信开关/消息长度/默认主题等,即时生效)、审计日志
- **深浅色三态主题**(浅色 / 深色 / 跟随系统),全站 100% coss-ui 组件,按钮默认 `outline`

---

## 🧱 目录结构

```
.
├── web/              # 前端 (Vite + React + coss-ui)
│   └── src/
│       ├── components/ui/    # coss-ui 组件(原样搬运自 Cloud-PE-Website)
│       ├── components/chat/  # 聊天相关组件
│       ├── components/layout/
│       ├── contexts/         # Theme / Auth / Settings / Chat
│       ├── hooks/useChatSocket.ts
│       ├── pages/            # 登录/注册/聊天/资料/admin
│       └── lib/              # api 客户端、类型、工具
├── server/           # 后端 (Go)
│   ├── main.go
│   ├── config/ models/ db/ settings/ auth/ cryptox/ ratelimit/
│   ├── hub/          # WebSocket Hub + 消息/私信/机器人逻辑
│   ├── ai/           # OpenAI 兼容客户端
│   ├── middleware/   # 鉴权与角色中间件
│   ├── handlers/     # REST + WS 处理器
│   └── .env.example
├── Dockerfile
└── docker-compose.yml
```

---

## 🚀 本地开发

需要 **Node ≥ 20** 与 **Go ≥ 1.24**。

### 1) 启动后端

```bash
cd server
cp .env.example .env          # 按需修改 JWT_SECRET、超级管理员账号等
go run .                      # 监听 http://localhost:8080
```

首次启动会自动迁移数据库并 seed:超级管理员(取自 `.env`)、机器人账号、默认「大厅」频道、默认设置。

### 2) 启动前端

```bash
cd web
npm install
npm run dev                   # http://localhost:5173,自动代理 /api /ws /uploads 到 :8080
```

浏览器打开 **http://localhost:5173**,用 `.env` 里的超级管理员账号登录(默认 `admin` / `admin12345`)。

### 生产构建(单服务)

```bash
cd web && npm install && npm run build     # 产出 web/dist
cd ../server && go run .                    # Go 会自动托管 ../web/dist 并提供 API/WS
# 打开 http://localhost:8080
```

---

## 🐳 Docker 一键部署

```bash
docker compose up -d --build
# 打开 http://localhost:8080
```

数据(SQLite + 上传文件)持久化在 `murmur-data` 卷的 `/data`。
**部署前请修改 `docker-compose.yml` 中的 `JWT_SECRET` 和超级管理员密码。**

---

## ⚙️ 环境变量(`server/.env`)

| 变量 | 说明 | 默认 |
| --- | --- | --- |
| `PORT` | 监听端口 | `8080` |
| `JWT_SECRET` | JWT 签名密钥(请改为长随机串) | `murmur-dev-secret-change-me` |
| `SUPER_ADMIN_USERNAME` | 首启 seed 的超级管理员用户名 | `admin` |
| `SUPER_ADMIN_PASSWORD` | 首启 seed 的超级管理员密码 | `admin12345` |
| `DB_PATH` | SQLite 文件路径 | `./data/murmur.db` |
| `UPLOAD_DIR` | 头像上传目录(对外 `/uploads`) | `./uploads` |
| `SETTINGS_ENC_KEY` | 加密密钥设置(如 AI Key)的密钥;留空则从 `JWT_SECRET` 派生 | 派生 |
| `STATIC_DIR` | 前端构建产物目录 | `../web/dist` 等 |

> AI 相关配置(是否启用、Base URL、API Key、模型、人设、温度等)在**后台「AI 设置」**中配置并持久化到数据库,无需写入 `.env`。

---

## 🔌 API 概览

REST(前缀 `/api`)+ 单个 WebSocket `/ws`(JWT 经 `?token=` 鉴权,承载
`chat_message / dm_message / mention / presence / typing / reaction / message_update / message_delete / error`)。

```
POST /api/auth/register|login    POST /api/auth/logout    GET/PATCH /api/me    POST /api/me/avatar
GET  /api/channels   POST /api/channels(admin)   PATCH|DELETE /api/channels/:id(admin)
GET  /api/channels/:id/messages   GET /api/channels/:id/search
PATCH|DELETE /api/messages/:id    POST /api/messages/:id/reactions
GET  /api/dm/conversations   GET /api/dm/:userId/messages   POST /api/dm/:userId   POST /api/dm/:userId/read
GET  /api/mentions   POST /api/mentions/:id/read   POST /api/mentions/read-all
GET  /api/users   GET /api/users/:id   GET /api/settings(公开)
GET  /api/admin/stats|users|registrations|settings|ai|audit
PATCH /api/admin/users/:id   DELETE /api/admin/users/:id(super)
POST /api/admin/registrations/:id/approve|reject   PUT /api/admin/settings   POST /api/admin/ai/test
```

---

## ✅ 验收对照

1. 前端可起、深浅色三态切换 + 弹出菜单与 Cloud-PE 一致;全站 coss-ui、按钮默认 outline
2. 注册 →(可选审核)→ 登录 → 大厅发消息,另一浏览器实时收到
3. 私信互发、未读计数
4. @某人高亮+通知;@机器人取最近 ≤5000 字上下文并实时回复
5. 后台改 AI 配置/系统提示词即时生效,「测试连通性」可用
6. 后台给成员设低频率后超频被拦、前端按钮倒计时
7. super_admin 改站点标题即时生效、开关注册/审核/封禁/任免管理员;普通用户进不了后台、直接打后台 API 被拒
