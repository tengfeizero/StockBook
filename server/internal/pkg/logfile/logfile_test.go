package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 当天文件应被创建，写入内容可读回，跨天命名符合 huoben-YYYY-MM-DD.log
func TestDailyWrite(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDaily(dir, 14)
	if err != nil {
		t.Fatalf("NewDaily: %v", err)
	}
	defer d.Close()

	if _, err := d.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	name := filepath.Join(dir, "huoben-"+time.Now().Format("2006-01-02")+".log")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读取当天日志失败: %v", err)
	}
	if string(raw) != "hello\n" {
		t.Fatalf("内容不符: %q", string(raw))
	}
	if !strings.HasSuffix(d.String(), filepath.Base(name)) {
		t.Fatalf("String() = %q，期望以 %q 结尾", d.String(), filepath.Base(name))
	}
}

// 超过保留天数的历史日志在启动时清理，保留期内的不动
func TestDailyPrune(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "huoben-2020-01-01.log")
	recent := filepath.Join(dir, "huoben-"+time.Now().AddDate(0, 0, -2).Format("2006-01-02")+".log")
	for _, p := range []string{old, recent} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("准备文件失败: %v", err)
		}
	}

	d, err := NewDaily(dir, 7)
	if err != nil {
		t.Fatalf("NewDaily: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("超期日志应被删除，仍存在: %v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("保留期内日志不应被删除: %v", err)
	}
}
