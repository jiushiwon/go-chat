# PLAN · go-ws-sqlite

> **一句话定位**：一个开箱即用的轻量级 WebSocket 实时消息后端
> —— 自管用户体系(SQLite) + 频道广播 + 完整历史回放 + 单二进制部署
>
> **目标用户**：任何想做"聊天室 / 房间系统 / 实时通知 / 群聊 / 简单社交"的开发者
> **启动成本**：5 分钟（下载二进制 + 启动 + 注册账号 + 连 WS）
> **作者**：gang.wang + AI 协作
> **起始日期**：2026-09-16

---

## 1. 为什么做这个

伞仓里现在有几个不同形态的"实时通信"需求：

- `games/lobby` —— 完整对战大厅，gin + 单 goroutine Hub + 内存房间 + 可插拔游戏引擎（重度完整，3072 行 Go）
- `vibecoding-portal` —— 静态站 + 跨域调 humeng 后端（前端式登录）
- `humeng-uniapp-note` —— 微信小程序后端（FastAPI + MongoDB + jx_users）
- `super-shell` / `session-share` —— 各自有零散的"在线状态"诉求

**共性诉求**：都要"一个能说话的后端"，但都不需要也不愿意背上 humeng 的 MongoDB 依赖
或 `games/lobby` 的对战引擎层。

所以 **go-ws-sqlite** 的定位非常克制：

- ✅ 单二进制 + 单 SQLite 文件 + 零外部依赖
- ✅ 自管用户体系（不连任何外部 IdP）
- ✅ 通用频道模型（业务自定义 kind / meta）
- ✅ 消息全量落库 + 可按 seq 回放
- ❌ 不做游戏规则
- ❌ 不做推送 / 邮件 / 第三方登录
- ❌ 不做集群 / 多实例

**预期复用场景**：

| 接入方 | 用法 |
|---|---|
| `games/lobby` | 单机游戏异步上报战绩、邀请好友对战 |
| 未来 H5 小游戏 | 简单在线状态 / 跨设备同步 |
| 任何新项目 | 聊天室 / 房间系统 / 实时通知 |

---

## 2. v0 范围

### 2.1 必须有

- [ ] 用户注册 / 登录 / 改密 / 注销（HTTP REST）
- [ ] 自签 token（落库可吊销，不走 JWT）
- [ ] WS 升级 + 协议信封 `{type, seq?, ts, data}`
- [ ] 频道创建 / 加入 / 退出 / 列表 / 详情（HTTP REST）
- [ ] WS 订阅 / 退订 / 发布 / 历史拉取
- [ ] 全部消息落 SQLite（频道内 seq 单调递增）
- [ ] 健康检查 / 统计 / 日志
- [ ] 配置通过 `.env`（godotenv）
- [ ] 协议文档 `docs/PROTOCOL.md`
- [ ] e2e 冒烟测试脚本（Node，无依赖）
- [ ] README + 启动说明

### 2.2 明确不做

- ❌ 集群 / 多实例（单机足够"轻"）
- ❌ WebRTC / 语音 / 视频
- ❌ OAuth / 第三方登录
- ❌ 推送通知 / 邮件
- ❌ 文件上传 / 大文件消息
- ❌ 前端 SDK（服务端 + 文档，业务方各自实现客户端）
- ❌ Web 管理面板（CLI / curl 足够）

---

## 3. 技术选型

| 维度 | 选择 | 理由 |
|---|---|---|
| 语言 | **Go 1.23+** | 跟 `games/lobby` 同栈，生态成熟 |
| 路由 | **net/http + http.ServeMux**（Go 1.22+ path 模式） | 不用 gin / echo，保持轻 |
| WebSocket | **gorilla/websocket** | `games/lobby` 已验证稳定 |
| SQLite | **`modernc.org/sqlite`** | **纯 Go 无 CGO**，Windows 编译不踩坑 |
| 密码 | **`golang.org/x/crypto/bcrypt`** | 标准做法，cost=10 |
| 限流 | **`golang.org/x/time/rate`** | 标准库配套 |
| 日志 | **`log/slog`** | Go 1.21+ 标准库 |
| 配置 | **`joho/godotenv`** | 跟 `games/lobby` 一致 |
| ID | 16 hex 前缀化（`usr_` / `ch_` / `tok_` / `msg_`） | 易调试 |

### 3.1 为什么不用 JWT

JWT 的一大痛点 = 不可吊销。go-ws-sqlite 的 token 落库 (`tokens` 表)，有 `revoked_at` 字段，
注销即生效，方便做"踢人"、"封号"、"查看活跃 token"等运维场景。

`tokens.id` 用 HMAC-SHA256 自签，载荷只有 `token_id`，对应查库拿到完整上下文。

### 3.2 为什么用 modernc/sqlite

`mattn/go-sqlite3` 是 CGO + 需要 gcc，Windows 编译踩过坑；
`modernc.org/sqlite` 是纯 Go 实现，跨平台编译 `CGO_ENABLED=0` 也能跑。
性能略低于 CGO 版本（~10%），但对轻量场景无影响。

---

## 4. 架构

```
                        ┌──────────────────────────────────────┐
HTTP :8088              │        Hub (single goroutine)        │
├─ /healthz            │  ┌─────────────────────────────────┐ │
├─ /api/auth/*         │  │ connections   map[id]*Conn       │ │
├─ /api/channels/*     │  │ users         map[id]*UserCtx    │ │
├─ /api/messages         │  │ channels      map[id]*ChannelCtx │ │
├─ /api/ws              │  │ pendingCmds   chan PendingCmd     │ │
└─ /api/admin/stats     │  └─────────────────────────────────┘ │
                        │            │                        │
                        │            ▼ (Hub 消费 + 派发)     │
WS  :8088/api/ws        │   conn.sendq → ws.WriteMessage     │
                        │            │                        │
                        │            ▼ (SQLite hook)          │
                        │   msg_log → INSERT messages         │
                        └──────────────────────────────────────┘
```

### 4.1 核心组件

| 组件 | 职责 |
|---|---|
| `cmd/server` | 入口：加载配置 → 打开 SQLite → 启动 Hub → 启动 HTTP server |
| `internal/store` | SQLite 封装（连接 + 迁移 + 查询） |
| `internal/auth` | 注册 / 登录 / token 签发 / 校验 / 吊销 |
| `internal/channel` | 频道模型 + 成员管理 + 权限检查 |
| `internal/message` | 消息持久化 + 序号生成 + 历史查询 |
| `internal/protocol` | WS 信封 + 消息类型常量 + 序列化 |
| `internal/ws` | 连接管理 + 心跳 + 重连补偿（按 since_seq） |
| `internal/httpx` | HTTP handler 通用工具（JSON 响应 / 错误码） |

### 4.2 单 goroutine Hub 模式

跟 `games/lobby` 同款经验：

- 所有可变状态只在 Hub goroutine 里访问 → 零锁
- 其他 goroutine（HTTP handler / WS reader / WS writer）通过 channel 跟 Hub 通信
- 优点：简单、可预测、无并发 bug
- 缺点：单核 CPU bound → 但这是"轻量"系统，IO 才是瓶颈

---

## 5. 数据模型（SQLite Schema）

```sql
-- 用户
CREATE TABLE users (
  id            TEXT PRIMARY KEY,          -- usr_<16-hex>
  username      TEXT UNIQUE NOT NULL,      -- 3-32 字符
  password_hash TEXT NOT NULL,             -- bcrypt cost=10
  nickname      TEXT NOT NULL,
  created_at    INTEGER NOT NULL,          -- unix ms
  updated_at    INTEGER NOT NULL,
  last_seen_at  INTEGER,
  status        TEXT NOT NULL DEFAULT 'active'   -- 'active' / 'banned'
);

-- token（落库可吊销，不走 JWT）
CREATE TABLE tokens (
  id           TEXT PRIMARY KEY,           -- tok_<16-hex>
  user_id      TEXT NOT NULL REFERENCES users(id),
  issued_at    INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  revoked_at   INTEGER,                    -- NULL = 有效
  last_used_at INTEGER,
  client_ip    TEXT,
  user_agent   TEXT
);
CREATE INDEX idx_tokens_user    ON tokens(user_id);
CREATE INDEX idx_tokens_expires ON tokens(expires_at);

-- 频道
CREATE TABLE channels (
  id          TEXT PRIMARY KEY,           -- ch_<16-hex>
  kind        TEXT NOT NULL,              -- 'room' / 'dm' / 'broadcast' / 'system'
  name        TEXT NOT NULL,
  owner_id    TEXT REFERENCES users(id),
  created_at  INTEGER NOT NULL,
  meta_json   TEXT,                       -- 业务自定义元数据（JSON）
  is_private  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_channels_kind ON channels(kind);

-- 频道成员
CREATE TABLE channel_members (
  channel_id    TEXT NOT NULL REFERENCES channels(id),
  user_id       TEXT NOT NULL REFERENCES users(id),
  role          TEXT NOT NULL DEFAULT 'member',   -- 'owner' / 'admin' / 'member'
  joined_at     INTEGER NOT NULL,
  last_read_seq INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (channel_id, user_id)
);
CREATE INDEX idx_cm_user ON channel_members(user_id);

-- 消息（全部落库，7 天后清理）
CREATE TABLE messages (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id  TEXT NOT NULL REFERENCES channels(id),
  from_user   TEXT REFERENCES users(id),          -- NULL = 系统消息
  msg_type    TEXT NOT NULL,                       -- 'chat' / 'system' / 'notice' / 业务自定义
  seq         INTEGER NOT NULL,                    -- 频道内单调递增
  payload     TEXT NOT NULL,                       -- JSON
  created_at  INTEGER NOT NULL,
  edited_at   INTEGER,
  deleted_at  INTEGER
);
CREATE INDEX idx_msg_channel_seq ON messages(channel_id, seq);
CREATE INDEX idx_msg_created     ON messages(created_at);

-- 频道内 seq 计数器（用于单调递增分配）
CREATE TABLE channel_seq (
  channel_id TEXT PRIMARY KEY REFERENCES channels(id),
  last_seq   INTEGER NOT NULL DEFAULT 0
);
```

---

## 6. HTTP API

### 6.1 鉴权

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/auth/register` | `{username, password, nickname?}` → `{token, user}` |
| `POST` | `/api/auth/login` | `{username, password}` → `{token, user}` |
| `POST` | `/api/auth/logout` | 吊销当前 token |
| `GET` | `/api/auth/me` | 当前用户（`Authorization: Bearer <token>`） |
| `POST` | `/api/auth/password` | 改密 `{old_password, new_password}` |

### 6.2 频道

| Method | Path | 说明 |
|---|---|---|
| `POST` | `/api/channels` | `{kind, name, members?:[user_id...], is_private?, meta?}` |
| `GET` | `/api/channels` | 我加入的频道 |
| `GET` | `/api/channels/:id` | 频道详情 + 成员列表 |
| `POST` | `/api/channels/:id/members` | 加成员（admin） |
| `DELETE` | `/api/channels/:id/members/:user_id` | 踢人（admin） |
| `GET` | `/api/channels/:id/messages?since_seq=N&limit=50` | 历史消息 |

### 6.3 运维

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/healthz` | 健康检查 |
| `GET` | `/api/admin/stats` | 在线连接数 / 频道数 / 消息数 |

### 6.4 错误码约定

```json
{ "code": 4001, "message": "username 已存在", "details": {} }
```

| 区间 | 含义 |
|---|---|
| 0 | 成功 |
| 4xxx | 客户端错误（参数错误 / 未授权 / 冲突） |
| 5xxx | 服务端错误（数据库 / 内部异常） |
| 1xxx | 鉴权错误细分（4001 用户名冲突 / 4002 密码错 / 4003 token 无效…） |

---

## 7. WS 协议

### 7.1 信封

```json
{
  "type": "message",
  "seq":  42,
  "ts":   1700000000000,
  "data": { ... }
}
```

### 7.2 消息类型

**客户端 → 服务端**：

| type | data 字段 | 说明 |
|---|---|---|
| `subscribe` | `{channel_id}` | 订阅频道 |
| `unsubscribe` | `{channel_id}` | 退订 |
| `publish` | `{channel_id, msg_type, payload}` | 在频道发消息 |
| `history` | `{channel_id, since_seq, limit?}` | 拉历史（断线补偿） |
| `ping` | `{}` | 心跳（30s 周期） |

**服务端 → 客户端**：

| type | data 字段 | 说明 |
|---|---|---|
| `welcome` | `{user_id, server_time}` | 连接建立 |
| `ready` | `{user_id, channels:[{id, last_seq}]}` | 鉴权完成，附带已订阅频道 |
| `subscribed` | `{channel_id, last_seq}` | 订阅成功 |
| `unsubscribed` | `{channel_id}` | 退订成功 |
| `message` | `{channel_id, from, msg_type, seq, payload, ts}` | 频道消息 |
| `history` | `{channel_id, messages:[...]}` | 历史响应 |
| `pong` | `{}` | 心跳应答 |
| `error` | `{code, message, ref_seq?}` | 错误 |

### 7.3 鉴权

WS 升级 URL：

```
GET /api/ws?token=<token_id>
```

- `token_id` 从 `Authorization: Bearer` 头或 `?token=` 拿
- 校验：存在 + 未过期 + 未吊销 → 允许升级
- 升级成功后服务端立刻推 `welcome` + `ready`

### 7.4 心跳

- 客户端 30s 一次发 `ping`
- 服务端收到立即回 `pong`（不查 DB）
- 服务端 60s 没收到 `ping` → 主动断开（让客户端重连 + 走 history 补偿）

---

## 8. 配置（.env）

```env
# 服务
APP_PORT=8088
APP_MODE=release                # debug / release
APP_LOG_LEVEL=info              # debug / info / warn / error

# 数据库
DB_PATH=./data/go-ws-sqlite.db
DB_BUSY_TIMEOUT_MS=5000

# 安全
TOKEN_TTL_HOURS=720             # 30 天
PASSWORD_BCRYPT_COST=10
RATE_LIMIT_PER_MIN=600          # 单连接每分钟最多 600 条消息

# 清理
MESSAGE_RETENTION_DAYS=7        -- 消息保留天数
CLEANUP_INTERVAL_MINUTES=60     -- GC 周期
```

---

## 9. 部署

### 9.1 本地

```bash
git clone <this repo>           # 伞仓内路径 go-ws-sqlite/
cd go-ws-sqlite
cp .env.example .env            # 调整配置
go build -o bin/server ./cmd/server
./bin/server
```

### 9.2 跨平台编译

```bash
# Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/server-linux ./cmd/server

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/server-macos-arm64 ./cmd/server

# Windows
go build -o bin/server.exe ./cmd/server
```

### 9.3 反向代理

```nginx
# nginx 示例
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

## 10. 验证

> **v0 不写自动化测试**（用户 2026-09-16 明确）。
> 验收姿势：编译通过 + 启动后 SQLite 迁移成功 + 手工 curl / wscat 验接口。
> 后续若项目成熟再补单元测试 / e2e。

### 10.1 编译验证

```bash
go vet ./...
go build -o bin/server ./cmd/server    # 必须 0 warning 0 error
```

### 10.2 启动验证

启动后看 `logs/server.log`（或 stdout）应有：

```
INFO msg="starting go-ws-sqlite" port=8088 mode=release db=./data/go-ws-sqlite.db
INFO msg="sqlite ready" path=./data/go-ws-sqlite.db
INFO msg=listening addr=http://127.0.0.1:8088
```

### 10.3 手工 curl 清单（按 Phase 1→2 顺序）

```bash
# 鉴权
curl -X POST http://127.0.0.1:8088/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secret","nickname":"Alice"}'

# 频道
curl -X POST http://127.0.0.1:8088/api/channels \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"kind":"room","name":"八卦局"}'

# 历史
curl "http://127.0.0.1:8088/api/channels/<id>/messages?since_seq=0&limit=20" \
  -H "Authorization: Bearer <token>"

# WS（用 wscat 或 websocat）
wscat -c "ws://127.0.0.1:8088/api/ws?token=<token>"
```

---

## 11. 路线图

### Phase 1 — 骨架（M1 / 本周）

- [x] PLAN.md
- [x] go.mod / cmd/server 入口（最小 healthz）
- [x] store 封装 + 迁移
- [ ] auth 注册 / 登录 / token
- [ ] WS 升级 + ping/pong
- [ ] channel 创建 / 订阅 / publish
- [ ] message 落库 + seq 单调

### Phase 2 — 完善（M2）

- [ ] history（since_seq）
- [ ] 限流
- [ ] GC（消息保留天数）
- [ ] admin stats
- [ ] README + PROTOCOL.md

### Phase 3 — 接入（M3，待评估）

- [ ] `games/lobby` 接入：单机游戏上报战绩
- [ ] 邀请好友对战（如果需要）
- [ ] 多实例部署（redis pubsub 桥接？）—— 仅当单机撑不住时

---

## 12. 风险与回滚

| 风险 | 处理 |
|---|---|
| SQLite 写并发瓶颈 | 单实例 + WAL 模式 + 写批量合并；撑不住再说 |
| 单 goroutine Hub CPU 瓶颈 | IO 为主，目前估算 1000 连接 / 5000 msg/s 没问题 |
| bcrypt cost=10 拖慢注册 | 改为 cost=10 注册 / cost=4 历史（生产前再调） |
| 端口冲突 | 默认 8088，避开 8080 / 9090 / 8081 |
| 时区 | 全用 unix ms，不存时区 |

---

## 13. 不在本次范围

- ❌ 多实例 / 集群
- ❌ WebRTC
- ❌ 文件上传 / 大消息
- ❌ 推送通知
- ❌ 管理面板（CLI 足够）
- ❌ 前端 SDK（业务方各自实现）
- ❌ 自动化测试（单元测试 / e2e）—— 用户明确"先有文档和相关代码即可"

---

*最后更新：2026-09-16*