package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Accounts 是节点的账号体系，封装 MySQL accounts 表操作
type Accounts struct {
	db *sql.DB
}

// NewAccounts 初始化 Accounts，自动建表（幂等）
func NewAccounts(db *sql.DB) (*Accounts, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS accounts (
		id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
		app_uid    VARCHAR(64) NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, fmt.Errorf("create accounts table: %w", err)
	}
	return &Accounts{db: db}, nil
}

// GetOrCreate 查询或创建账号，返回 node_uid（accounts.id），幂等
func (a *Accounts) GetOrCreate(appUID string) (uint64, error) {
	var id uint64
	err := a.db.QueryRow(`SELECT id FROM accounts WHERE app_uid = ?`, appUID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("query account: %w", err)
	}

	res, err := a.db.Exec(`INSERT IGNORE INTO accounts (app_uid) VALUES (?)`, appUID)
	if err != nil {
		return 0, fmt.Errorf("insert account: %w", err)
	}

	lastID, err := res.LastInsertId()
	if err != nil || lastID == 0 {
		// INSERT IGNORE 时另一并发请求已插入，重新查询
		err = a.db.QueryRow(`SELECT id FROM accounts WHERE app_uid = ?`, appUID).Scan(&id)
		return id, err
	}
	return uint64(lastID), nil
}

// InsertOwner 插入运营者账号（激活时调用，幂等）
func (a *Accounts) InsertOwner() (uint64, error) {
	return a.GetOrCreate("__node_owner__")
}

// GetAppUIDs 批量根据 node_uid（id）查询 app_uid，不存在的跳过
func (a *Accounts) GetAppUIDs(nodeUIDs []uint64) (map[uint64]string, error) {
	if len(nodeUIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(nodeUIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(nodeUIDs))
	for i, id := range nodeUIDs {
		args[i] = id
	}
	rows, err := a.db.Query(
		`SELECT id, app_uid FROM accounts WHERE id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint64]string, len(nodeUIDs))
	for rows.Next() {
		var id uint64
		var appUID string
		if err := rows.Scan(&id, &appUID); err != nil {
			return nil, err
		}
		result[id] = appUID
	}
	return result, rows.Err()
}
