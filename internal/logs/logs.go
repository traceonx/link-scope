// Package logs 收集程序日志：照常输出到标准错误（容器日志），同时在内存中保留最近的日志供页面查看。
package logs

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Entry 是一条日志。
type Entry struct {
	ID    int64  `json:"id"`
	Time  int64  `json:"time"`  // 毫秒时间戳
	Level string `json:"level"` // INFO | WARN | ERROR
	Msg   string `json:"msg"`
}

// Capacity 是内存中保留的日志条数。
const Capacity = 2000

// Buffer 是固定容量的日志环形缓冲，实现 io.Writer 供标准库 log 写入。
type Buffer struct {
	mu      sync.Mutex
	entries []Entry
	next    int64
	now     func() time.Time
}

// Default 是全局日志缓冲。
var Default = New()

func New() *Buffer {
	return &Buffer{now: time.Now}
}

// Install 让标准库 log 同时写入标准错误与 Default。
func Install() {
	log.SetOutput(io.MultiWriter(os.Stderr, Default))
}

// Warnf / Errorf 输出带级别前缀的日志，页面据此着色；普通 log.Printf 视为 INFO。
func Warnf(format string, args ...any)  { log.Printf("[WARN] "+format, args...) }
func Errorf(format string, args ...any) { log.Printf("[ERROR] "+format, args...) }

// stdPrefix 是标准库 log 默认的时间前缀长度（"2006/01/02 15:04:05 "）。
const stdPrefix = len("2006/01/02 15:04:05 ")

func parse(line string) (level, msg string) {
	if len(line) > stdPrefix && line[4] == '/' && line[7] == '/' && line[13] == ':' {
		line = line[stdPrefix:]
	}
	switch {
	case strings.HasPrefix(line, "[ERROR] "):
		return "ERROR", line[8:]
	case strings.HasPrefix(line, "[WARN] "):
		return "WARN", line[7:]
	}
	return "INFO", line
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now().UnixMilli()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.next++
		lv, msg := parse(line)
		b.entries = append(b.entries, Entry{ID: b.next, Time: t, Level: lv, Msg: msg})
	}
	if over := len(b.entries) - Capacity; over > 0 {
		b.entries = append(b.entries[:0:0], b.entries[over:]...)
	}
	return len(p), nil
}

// Since 返回 ID 大于 after 的日志（最多 limit 条，取最新的），以及当前最新的 ID。
func (b *Buffer) Since(after int64, limit int) ([]Entry, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []Entry{}
	for _, e := range b.entries {
		if e.ID > after {
			out = append(out, e)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, b.next
}
