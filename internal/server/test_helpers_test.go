package server

import (
	"path/filepath"
	"sync"
	"testing"

	"shop_server/pkg/logger"
)

var initTestLoggerOnce sync.Once

func initTestLogger(t *testing.T) {
	t.Helper()

	initTestLoggerOnce.Do(func() {
		if err := logger.Init(&logger.Config{
			Level:    "error",
			FilePath: filepath.Join(t.TempDir(), "test.log"),
			Console:  false,
			MaxAge:   1,
		}); err != nil {
			t.Fatalf("failed to init test logger: %v", err)
		}
	})
}
