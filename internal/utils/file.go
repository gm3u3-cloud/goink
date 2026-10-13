package utils

import (
	"os"
	"path/filepath"
)

// AtomicWrite 在目标目录创建权限为 0600 的临时文件，写入并同步后重命名为目标文件。
// 调用方需保证父目录已经存在；失败时清理临时文件。
func AtomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".atomic-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
