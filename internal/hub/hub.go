// Package hub 实现单 goroutine 事件循环（参考 games/lobby 同款经验）。
//
// 当前 Phase 1 是占位结构，仅持有 DB 与 config；后续添加：
//   - WS 连接池
//   - 频道订阅 map
//   - 命令 channel（HTTP/WS → Hub 单向）
//   - 派发器（按 message type 路由）
package hub

import (
	"database/sql"

	"go-ws-sqlite/internal/config"
)

type Hub struct {
	db   *sql.DB
	cfg  *config.Config
	done chan struct{}
}

func New(db *sql.DB, cfg *config.Config) *Hub {
	return &Hub{
		db:   db,
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

// Run Phase 1 占位：什么都不做，等用户拍板后填充 select 循环。
func (h *Hub) Run() {
	<-h.done
}

// Stop 通知 Run 退出。
func (h *Hub) Stop() {
	close(h.done)
}