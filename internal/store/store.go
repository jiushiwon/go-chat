// Package store 封装 SQLite 连接 + 迁移 + 通用查询工具。
//
// 选型 modernc.org/sqlite（纯 Go 无 CGO），跨平台编译不踩坑。
// 当前不启用 ORM，直接用 SQL；表结构在 PLAN.md §5。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Open 打开 SQLite 数据库。
//
// 行为：
//   - 父目录不存在 → 自动创建
//   - WAL 模式 + synchronous=NORMAL（轻量场景的标配）
//   - busy_timeout 由配置注入
func Open(path string, busyTimeoutMs int) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(%d)&_pragma=foreign_keys(ON)",
		path, busyTimeoutMs,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写多读，限制连接数避免锁竞争
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// Migrate 执行 schema 迁移（当前用纯 CREATE TABLE IF NOT EXISTS，无版本号）。
//
// 后续若 schema 演进，引入迁移表 + 版本号。
func Migrate(db *sql.DB) error {
	stmts := []string{
		// ---- users ----
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			username      TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			nickname      TEXT NOT NULL,
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL,
			last_seen_at  INTEGER,
			status        TEXT NOT NULL DEFAULT 'active'
		)`,

		// ---- tokens ----
		`CREATE TABLE IF NOT EXISTS tokens (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL REFERENCES users(id),
			issued_at    INTEGER NOT NULL,
			expires_at   INTEGER NOT NULL,
			revoked_at   INTEGER,
			last_used_at INTEGER,
			client_ip    TEXT,
			user_agent   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_user    ON tokens(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tokens_expires ON tokens(expires_at)`,

		// ---- channels ----
		`CREATE TABLE IF NOT EXISTS channels (
			id          TEXT PRIMARY KEY,
			kind        TEXT NOT NULL,
			name        TEXT NOT NULL,
			owner_id    TEXT REFERENCES users(id),
			created_at  INTEGER NOT NULL,
			meta_json   TEXT,
			is_private  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_kind ON channels(kind)`,

		// ---- channel_members ----
		`CREATE TABLE IF NOT EXISTS channel_members (
			channel_id    TEXT NOT NULL REFERENCES channels(id),
			user_id       TEXT NOT NULL REFERENCES users(id),
			role          TEXT NOT NULL DEFAULT 'member',
			joined_at     INTEGER NOT NULL,
			last_read_seq INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (channel_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cm_user ON channel_members(user_id)`,

		// ---- messages ----
		`CREATE TABLE IF NOT EXISTS messages (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id  TEXT NOT NULL REFERENCES channels(id),
			from_user   TEXT REFERENCES users(id),
			msg_type    TEXT NOT NULL,
			seq         INTEGER NOT NULL,
			payload     TEXT NOT NULL,
			created_at  INTEGER NOT NULL,
			edited_at   INTEGER,
			deleted_at  INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_msg_channel_seq ON messages(channel_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_msg_created     ON messages(created_at)`,

		// ---- channel_seq（频道内 seq 计数器）----
		`CREATE TABLE IF NOT EXISTS channel_seq (
			channel_id TEXT PRIMARY KEY REFERENCES channels(id),
			last_seq   INTEGER NOT NULL DEFAULT 0
		)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate stmt failed: %w\nSQL: %s", err, s)
		}
	}
	return nil
}