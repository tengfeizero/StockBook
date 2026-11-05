package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Daily 按天切分的日志写入器：跨天自动换文件，构造时清理超期文件。
// 服务方式运行时没有控制台，日志必须落盘，否则出问题无从排查。
type Daily struct {
	dir  string
	mu   sync.Mutex
	day  string
	file *os.File
}

// NewDaily 在 dir 下创建/追加当天的 huoben-YYYY-MM-DD.log，keepDays<=0 表示不清理。
func NewDaily(dir string, keepDays int) (*Daily, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	d := &Daily{dir: dir}
	if keepDays > 0 {
		d.prune(keepDays)
	}
	if _, err := d.open(); err != nil {
		return nil, err
	}
	return d, nil
}

// Write 实现 io.Writer，供 log / gin 使用。
func (d *Daily) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	f, err := d.open()
	if err != nil {
		return 0, err
	}
	return f.Write(p)
}

// Close 关闭当前文件。
func (d *Daily) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file != nil {
		err := d.file.Close()
		d.file = nil
		return err
	}
	return nil
}

// open 返回当天文件句柄，跨天时自动滚动。调用方需持有锁（构造期除外）。
func (d *Daily) open() (*os.File, error) {
	day := time.Now().Format("2006-01-02")
	if d.file != nil && d.day == day {
		return d.file, nil
	}
	if d.file != nil {
		_ = d.file.Close()
		d.file = nil
	}
	f, err := os.OpenFile(filepath.Join(d.dir, "huoben-"+day+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	d.file, d.day = f, day
	return f, nil
}

// prune 删除超过保留天数的历史日志；失败不影响启动。
func (d *Daily) prune(keepDays int) {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "huoben-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		day, err := time.Parse("2006-01-02", strings.TrimSuffix(strings.TrimPrefix(name, "huoben-"), ".log"))
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			_ = os.Remove(filepath.Join(d.dir, name))
		}
	}
}

// String 便于日志里打一行当前文件路径。
func (d *Daily) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return filepath.Join(d.dir, "huoben-"+d.day+".log")
}
