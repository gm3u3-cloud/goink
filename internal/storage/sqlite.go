package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Open 打开 SQLite 数据库，启用 WAL 模式和 sqlite-vec 向量扩展。
//
//	dsn: SQLite 文件路径，如 "/data/my-novel/novel.db"
func Open(dsn string, log *slog.Logger) (*gorm.DB, error) {
	// 启用 sqlite-vec 扩展（全局注册，所有 mattn/go-sqlite3 连接自动可用）
	sqlite_vec.Auto()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: &gormSlogAdapter{log: log},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("storage: failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)            // SQLite 写串行，单连接最安全
	sqlDB.SetMaxIdleConns(1)            // 至少保留一个空闲连接
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接最长存活 1 小时

	if err := enableWAL(sqlDB); err != nil {
		return nil, fmt.Errorf("storage: failed to enable WAL: %w", err)
	}

	log.Info("SQLite opened", "dsn", dsn)
	return db, nil
}

// enableWAL 开启 WAL 模式，支持一写多读并发。
func enableWAL(db *sql.DB) error {
	var journal string
	if err := db.QueryRow("PRAGMA journal_mode=WAL").Scan(&journal); err != nil {
		return err
	}
	if journal != "wal" {
		return fmt.Errorf("expected journal_mode=wal, got %s", journal)
	}
	return nil
}

// Close 关闭数据库并清理 sqlite-vec 全局状态。
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("storage: failed to get sql.DB: %w", err)
	}

	sqlite_vec.Cancel()

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("storage: failed to close database: %w", err)
	}
	return nil
}

// Ping 检查数据库连接是否正常。
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("storage: failed to get sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("storage: ping failed: %w", err)
	}
	return nil
}

// gormSlogAdapter 将 slog 适配为 GORM 的 logger 接口。
type gormSlogAdapter struct {
	log *slog.Logger
}

func (a *gormSlogAdapter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	// 日志级别由 slog 的 HandlerOptions.Level 控制，此处不做二次过滤
	return a
}

func (a *gormSlogAdapter) Info(_ context.Context, msg string, args ...any) {
	a.log.Info(msg, args...)
}

func (a *gormSlogAdapter) Warn(_ context.Context, msg string, args ...any) {
	a.log.Warn(msg, args...)
}

func (a *gormSlogAdapter) Error(_ context.Context, msg string, args ...any) {
	a.log.Error(msg, args...)
}

func (a *gormSlogAdapter) Trace(_ context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	attrs := []any{
		"elapsed", elapsed,
		"rows", rows,
		"sql", sql,
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		a.log.Warn("GORM query error", attrs...)
		return
	}
	a.log.Debug("GORM query", attrs...)
}
