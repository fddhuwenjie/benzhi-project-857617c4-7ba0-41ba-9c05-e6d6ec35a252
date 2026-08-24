package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *FileStore) CheckWritable() error {
	file, err := os.CreateTemp(s.root, ".health-*")
	if err != nil {
		return fmt.Errorf("持久化目录不可写: %w", err)
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	if filepath.Dir(name) != s.root {
		return fmt.Errorf("健康检查临时文件路径异常")
	}
	return os.Remove(name)
}
