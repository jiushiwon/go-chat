# go-ws-sqlite

> 一个开箱即用的轻量级 WebSocket 实时消息后端
> —— 自管用户体系（SQLite）+ 频道广播 + 完整历史回放 + 单二进制部署

**当前阶段**：Phase 1 — 骨架（[docs/PLAN.md](./docs/PLAN.md) 是设计源）

---

## 它要解决什么问题

你在做一个产品，里面需要"实时"：

- 聊天室、群聊、私聊
- 房间系统（语音房、协作房间、观战房）
- 实时通知（新消息、状态变更、系统广播）
- 简单的社交互动（关注、在线状态、打招呼）

传统的做法要么重、要么绑死。要么上一整套腾讯 IM / 融云 / Firebase（注册复杂、SDK 入侵、收费），
要么从零写一套 WS（手写协议、心跳、重连、广播、鉴权、历史……三五天起步）。

**`go-ws-sqlite` 走中间路线**：给你一套**开箱即用、零外部依赖、单二进制**的后端，
自带用户体系和消息历史，**协议透明**（任何语言前端都能连），规模从 1 人到几千人在线都撑得住。

> 我们**比不上腾讯 IM** —— 没有音视频、没有消息推送、没有内容审核、没有全球加速、没有 SLA。
> 但**简单场景能用就行** —— 5 分钟部署、20 行前端代码接入、不用再操心"消息丢了怎么办"。

### 谁该用 / 谁不该用

| ✅ 该用 | ❌ 不该用 |
|---|---|
| 想做个内部工具 / 个人项目 / MVP / Demo | 已有腾讯 IM / 融云 / Firebase 等成熟方案 |
| 不希望被某个第三方 SDK 绑死 | 需要音视频通话 / 屏幕共享 |
| 想要**完全可控**的用户体系和消息存储 | 需要消息推送（APNs / 离线 push） |
| 想给 H5 / 小程序 / 桌面端 / 移动端统一一套后端 | 单机撑不住、需要多实例分布式部署 |
| 不想 / 不能装 MongoDB / Redis / MySQL | 团队规模 100+ 在线、需要 SLA |

---

## 它不是什么

它**不是**：

- ❌ 腾讯 IM / 融云 / Firebase 的替代品（这些是商业级服务）
- ❌ Socket.IO（那是 Node 生态 + 浏览器优先）
- ❌ Matrix / XMPP（那是去中心化的联邦协议）
- ❌ Centrifugo（那是 Go 但重，要 Redis）
- ❌ `games/lobby`（那是**完整对战大厅**，3072 行 Go + 游戏引擎）
- ❌ `humeng-uniapp-note` 后端（那是**微信小程序后端**，绑 MongoDB）

---

## 核心特性

- ✅ **自带用户体系** —— 注册 / 登录 / 改密 / 注销,bcrypt 密码哈希
- ✅ **token 落库可吊销**（不像 JWT 一旦签发就管不了）
- ✅ **频道广播** —— 任意业务可定义频道类型（room / dm / broadcast / system / game ...）
- ✅ **全部消息落库** —— 7 天保留 + 按 seq 回放,断线重连零丢失
- ✅ **完整历史** —— `since_seq` 拉断线期间的所有消息
- ✅ **WebSocket + HTTP 双协议** —— REST 管用户/频道/历史,WS 管实时收发
- ✅ **零外部依赖** —— 单二进制 + 单 SQLite 文件 = 5 分钟部署
- ✅ **跨平台编译** —— `CGO_ENABLED=0` 一行产出 Linux/macOS/Windows 二进制
- ✅ **协议透明** —— 客户端 SDK 不绑架你,任何语言前端都能连

---

## 技术选型（为什么是这些）

> 这一节讲选型理由，方便你 review / 将来有需求时调整。

### 语言：Go 1.23+

- **编译快**：几百毫秒出二进制，改完即跑
- **单二进制部署**：目标机器不需要装 Go runtime
- **并发原生**：goroutine + channel 写 WS 服务是教科书级体验
- **生态成熟**：标准库自带 `net/http`、`crypto/bcrypt`、`log/slog`、`database/sql`

### HTTP 路由：`net/http` + `http.ServeMux`（Go 1.22+ path 模式）

- **不引入 gin / echo / chi**：标准库 1.22+ 自带 path 模式（`mux.HandleFunc("GET /api/auth/login", ...)`），够用
- **轻**：少一个依赖、编译产物更小、二进制更小
- **零学习成本**：业务方读代码就是标准库
- **如果将来复杂**：再切到 chi/echo 也只是改 `cmd/server/main.go` 一处

### WebSocket：`github.com/gorilla/websocket`

- **实战验证**：`games/lobby` 已用，扛过 76 项 e2e 冒烟
- **生态成熟**：文档全、示例多、社区活跃
- **轻**：单文件包，不带乱七八糟的依赖

### 数据库驱动：`modernc.org/sqlite`（纯 Go，无 CGO）

- **跨平台编译不踩坑**：`mattn/go-sqlite3` 是 CGO，Windows 上需要 gcc 编译
- **性能**：纯 Go 比 CGO 慢 ~10%，轻量场景完全无感
- **WAL 模式 + foreign_keys(ON)**：开箱即用

### 密码：`golang.org/x/crypto/bcrypt`（cost=10）

- **工业标准**：撞库成本极高，是 OAuth/OWASP 的推荐
- **简单 API**：`bcrypt.GenerateFromPassword` + `CompareHashAndPassword`
- **cost=10**：注册 ~80ms，登录 ~80ms,用户感知不到

### Token：**自签落库**，不走 JWT

- **JWT 痛点**：签出去就收不回来
- **本方案**：`tokens.id` 是 16 hex，HMAC-SHA256 自签 + 落库 + `revoked_at` 字段
- **好处**：踢人 / 封号 / 看活跃 token 全部一行 SQL
- **代价**：每次请求多一次 DB 查,但有索引,微秒级

### 日志：`log/slog`（Go 1.21+ 标准库）

- **结构化日志**：JSON 输出，方便日志系统采集
- **不要 logrus / zap**：标准库够用

### 配置：`joho/godotenv`

- **.env 习惯**：跟 `games/lobby` 一致
- **失败 fallback**：环境变量优先、`.env` 兜底

### **没选**的（也说说为什么）

| 不选 | 原因 |
|---|---|
| Gin / Echo / Chi | 标准库 `net/http` 1.22+ 够用,少一个依赖 |
| `mattn/go-sqlite3` | CGO + 需要 gcc，跨平台编译踩坑 |
| JWT | 不可吊销,本场景要可吊销 |
| Redis / MySQL / Postgres | 多一个服务 = 多一个运维负担,单机 SQLite 够 |
| Socket.IO | 浏览器优先,且协议闭源,不如 WS 直连 |
| 前端 SDK | 协议透明更好,业务方各自实现 30 行客户端 |

---

## 架构

```
                     ┌──────────────────────────────────────┐
HTTP :8088           │        Hub (single goroutine)        │
├─ /healthz         │  ┌─────────────────────────────────┐ │
├─ /api/auth/*      │  │ connections   map[id]*Conn       │ │
├─ /api/channels/*  │  │ channels      map[id]*ChannelCtx  │ │
├─ /api/messages    │  │ pendingCmds   chan PendingCmd     │ │
├─ /api/ws          │  └─────────────────────────────────┘ │
└─ /api/admin/stats │            │                        │
                     │            ▼ (Hub 消费 + 派发)     │
WS  :8088/api/ws     │   conn.sendq → ws.WriteMessage     │
                     │            │                        │
                     │            ▼ (SQLite hook)          │
                     │   msg_log → INSERT messages         │
                     └──────────────────────────────────────┘
```

**单 goroutine Hub 模式** —— 所有可变状态只在 Hub goroutine 里访问,**零锁**。
HTTP handler / WS reader / WS writer 通过 channel 跟 Hub 通信。
优点：简单、可预测、无并发 bug。代价：单核 CPU bound —— 但 WS 系统 IO 才是瓶颈。

---

## 快速开始（5 分钟）

### 1. 编译

```bash
cd go-ws-sqlite
go build -o bin/server.exe ./cmd/server      # Windows
# 或
go build -o bin/server ./cmd/server          # Linux / macOS
```

### 2. 配置

```bash
cp .env.example .env
# 默认配置即可：端口 8088、SQLite 路径 ./data/go-ws-sqlite.db
```

### 3. 启动

```bash
./bin/server.exe
# 看到：
# INFO msg="starting go-ws-sqlite" port=8088 mode=release db=./data/go-ws-sqlite.db
# INFO msg="sqlite ready" path=./data/go-ws-sqlite.db
# INFO msg=listening addr=http://127.0.0.1:8088
```

### 4. 注册账号

```bash
curl -X POST http://127.0.0.1:8088/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secret123","nickname":"Alice"}'

# 返回：
# {"code":0,"data":{"token":"tok_xxx...","user":{"id":"usr_xxx...","username":"alice","nickname":"Alice"}}}
```

### 5. 创建频道

```bash
TOKEN=...   # 上一步拿到的 token
curl -X POST http://127.0.0.1:8088/api/channels \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"kind":"room","name":"八卦局"}'

# 返回：
# {"code":0,"data":{"id":"ch_xxx...","name":"八卦局","owner_id":"usr_xxx..."}}
```

### 6. 连 WebSocket

```bash
wscat -c "ws://127.0.0.1:8088/api/ws?token=$TOKEN"

# 连上后立刻收到：
# < {"type":"welcome","data":{"user_id":"usr_xxx...","server_time":...}}
# < {"type":"ready","data":{"user_id":"usr_xxx...","channels":[{"id":"ch_xxx...","last_seq":0}]}}

# 订阅频道：
# > {"type":"subscribe","data":{"channel_id":"ch_xxx..."}}

# 发消息：
# > {"type":"publish","data":{"channel_id":"ch_xxx...","msg_type":"chat","payload":{"text":"hi"}}}
```

完整 API 见 [docs/PLAN.md](./docs/PLAN.md)。

---

## 30 行 JS 客户端示例

```js
// 浏览器 / Node 通用（Node 需 ws 包）
const ws = new WebSocket(`ws://127.0.0.1:8088/api/ws?token=${TOKEN}`);

ws.onmessage = (e) => {
  const env = JSON.parse(e.data);
  if (env.type === 'message') {
    console.log(`[${env.data.channel_id}] ${env.data.from}: ${env.data.payload.text}`);
  }
};

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe',
    data: { channel_id: 'ch_xxx...' },
  }));
};

// 发消息
function send(text) {
  ws.send(JSON.stringify({
    type: 'publish',
    data: { channel_id: 'ch_xxx...', msg_type: 'chat', payload: { text } },
  }));
}
```

---

## 项目结构

```
go-ws-sqlite/
├── docs/
│   └── PLAN.md                   # 纲领（设计 + 数据模型 + API + 协议 + 路线图）
├── cmd/server/                   # 入口
│   └── main.go
├── internal/
│   ├── auth/                     # 用户注册 / 登录 / token
│   ├── channel/                  # 频道模型 + 成员管理
│   ├── message/                  # 消息持久化 + seq
│   ├── ws/                       # WS 连接管理 + 心跳
│   ├── store/                    # SQLite 封装 + 迁移
│   ├── protocol/                 # WS 协议信封 + 类型常量
│   └── httpx/                    # HTTP 通用工具
├── data/                         # SQLite 文件目录（运行时生成）
├── bin/                          # 编译产物
├── .env.example
├── go.mod
└── README.md
```

---

## 部署

### 本地

```bash
go build -o bin/server ./cmd/server
./bin/server
```

### 跨平台编译

```bash
# Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/server-linux ./cmd/server

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/server-macos-arm64 ./cmd/server

# Windows
go build -o bin/server.exe ./cmd/server
```

### 反向代理（生产建议 nginx 终止 TLS）

```nginx
location /api/ws {
    proxy_pass http://127.0.0.1:8088;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header X-Real-IP $remote_addr;
    proxy_read_timeout 600s;     # WS 长连接
}
```

---

## 路线图

- **Phase 1 — 骨架**：✅ PLAN + 入口 + SQLite + 数据模型（**当前**）
- **Phase 2 — 完整**：鉴权 / WS 升级 / 频道 / 消息 / 历史 / 限流 / GC / admin stats
- **Phase 3 — 接入**：`games/lobby` 单机游戏上报战绩 / 邀请对战 / 多实例部署（按需）

详细见 [docs/PLAN.md §11](./docs/PLAN.md#11-路线图)。

---

## 文档

- [docs/PLAN.md](./docs/PLAN.md) — 项目纲领（**先读**）
  - §5 数据模型
  - §6 HTTP API
  - §7 WS 协议
  - §8 配置项
  - §9 部署
  - §10 验证（v0 不写自动化测试,靠编译 + 启动 + 手工 curl）
  - §11 路线图

---

## 验收姿势（v0 阶段）

按用户偏好,**当前不写自动化测试**。验收靠三件：

1. **`go build` 通过** —— 编译无 error / warning
2. **启动后 SQLite 迁移成功** —— 6 张表 + 6 索引
3. **手工 curl + wscat 验接口** —— 见上文"快速开始"

---

## License

内部项目。