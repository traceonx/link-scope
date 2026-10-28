// Package jobs 管理后台扫描任务：同一时间只运行一个扫描，结果持久化到磁盘。
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/traceonx/link-scope/internal/scanner"
	"github.com/traceonx/link-scope/internal/torrents"
)

// Filters 是结果列表支持的筛选项。
var Filters = []string{"all", "linked", "unlinked", "dup", "symlink", "strm", "broken",
	"torrent", "seeding", "unregistered", "orphan"}

// Fetcher 读取所有下载器的种子；为 nil 或没有下载器时跳过种子检测。
type Fetcher interface {
	HasAny() bool
	Fetch(ctx context.Context) torrents.Snapshot
}

var ErrBusy = errors.New("已有扫描正在进行")

// State 是当前任务状态。
type State struct {
	Status  string  `json:"status"` // idle | running | done | error | cancelled
	Phase   string  `json:"phase,omitempty"`
	Done    int64   `json:"done"`
	Total   int64   `json:"total"`
	Started float64 `json:"started,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type Manager struct {
	mu      sync.RWMutex
	dataDir string
	state   State
	result  *scanner.Result
	counts  map[string]int
	lower   []string // 每条结果路径的小写形式，用于搜索
	cancel  context.CancelFunc
	fetcher Fetcher
}

func New(dataDir string, f Fetcher) *Manager {
	m := &Manager{dataDir: dataDir, state: State{Status: "idle"}, fetcher: f}
	m.load()
	return m
}

func (m *Manager) file() string { return filepath.Join(m.dataDir, "last_scan.json") }

func (m *Manager) load() {
	b, err := os.ReadFile(m.file())
	if err != nil {
		return
	}
	var r scanner.Result
	if json.Unmarshal(b, &r) == nil {
		m.setResult(&r)
	}
}

func (m *Manager) save(r *scanner.Result) {
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		return
	}
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	tmp := m.file() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, m.file())
	}
}

// setResult 需在持有写锁或初始化时调用。
func (m *Manager) setResult(r *scanner.Result) {
	counts := make(map[string]int, len(Filters))
	lower := make([]string, len(r.Items))
	for i := range r.Items {
		counts["all"]++
		for _, t := range r.Items[i].Tags {
			counts[t]++
		}
		lower[i] = strings.ToLower(r.Items[i].Path)
	}
	m.result, m.counts, m.lower = r, counts, lower
}

// Start 启动一次扫描，已有扫描在运行时返回 ErrBusy。
func (m *Manager) Start(o scanner.Options) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Status == "running" {
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.state = State{Status: "running", Phase: "source", Started: float64(time.Now().UnixMilli()) / 1000}
	o.Progress = func(phase string, done, total int64) {
		m.mu.Lock()
		m.state.Phase, m.state.Done, m.state.Total = phase, done, total
		m.mu.Unlock()
	}
	go func() {
		r, err := scanner.Scan(ctx, o)
		if err == nil && m.fetcher != nil && m.fetcher.HasAny() {
			m.mu.Lock()
			m.state.Phase, m.state.Done, m.state.Total = "torrents", 0, 0
			m.mu.Unlock()
			snap := m.fetcher.Fetch(ctx)
			annotate(r, &snap)
		}
		if err == nil {
			m.save(r)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		switch {
		case errors.Is(err, context.Canceled):
			m.state = State{Status: "cancelled"}
		case err != nil:
			m.state = State{Status: "error", Error: err.Error()}
		default:
			m.setResult(r)
			m.state = State{Status: "done"}
		}
		cancel()
	}()
	return nil
}

func (m *Manager) Cancel() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Summary 返回最近一次扫描的汇总；没有结果时为 nil。
func (m *Manager) Summary() *scanner.Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.result == nil {
		return nil
	}
	s := m.result.Summary
	return &s
}

func (m *Manager) Counts() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int, len(Filters))
	for _, f := range Filters {
		out[f] = m.counts[f]
	}
	return out
}

// Query 按筛选项、站点和关键字过滤，返回总数和分页后的条目。limit<=0 表示不分页。
func (m *Manager) Query(filter, site, q string, offset, limit int) (int, []scanner.Item) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.result == nil {
		return 0, []scanner.Item{}
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := []scanner.Item{}
	count := 0
	for i := range m.result.Items {
		it := &m.result.Items[i]
		if filter != "all" && !hasTag(it.Tags, filter) {
			continue
		}
		if q != "" && !strings.Contains(m.lower[i], q) {
			continue
		}
		if site != "" && !hasSite(it, site) {
			continue
		}
		if count >= offset && (limit <= 0 || len(out) < limit) {
			out = append(out, *it)
		}
		count++
	}
	return count, out
}

// RefreshTorrents 重新读取下载器并更新最近一次扫描结果中的种子信息。
func (m *Manager) RefreshTorrents(ctx context.Context) (*scanner.TorrentSummary, error) {
	if m.fetcher == nil || !m.fetcher.HasAny() {
		return nil, errors.New("还没有添加下载器")
	}
	m.mu.RLock()
	has := m.result != nil
	running := m.state.Status == "running"
	m.mu.RUnlock()
	if running {
		return nil, ErrBusy
	}
	if !has {
		return nil, errors.New("还没有扫描结果")
	}
	snap := m.fetcher.Fetch(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.result == nil {
		return nil, errors.New("还没有扫描结果")
	}
	annotate(m.result, &snap)
	m.setResult(m.result)
	m.save(m.result)
	return m.result.Summary.Torrents, nil
}

// Running 是否有扫描正在进行。
func (m *Manager) Running() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Status == "running"
}

// ItemsByPath 返回结果中指定路径的条目（用于删除前检查做种情况）。
func (m *Manager) ItemsByPath(paths []string) []scanner.Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.result == nil {
		return nil
	}
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	var out []scanner.Item
	for _, it := range m.result.Items {
		if set[it.Path] || under(it.Path, paths) {
			out = append(out, it)
		}
	}
	return out
}

// RemovePaths 从结果中去掉已删除的路径。
func (m *Manager) RemovePaths(deleted []string) {
	if len(deleted) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.result == nil {
		return
	}
	removePaths(m.result, deleted)
	m.setResult(m.result)
	m.save(m.result)
}

// MarkStale 标记结果需要重新扫描（例如从回收站恢复了文件）。
func (m *Manager) MarkStale() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.result != nil && !m.result.Summary.Stale {
		m.result.Summary.Stale = true
		m.save(m.result)
	}
}

func hasSite(it *scanner.Item, site string) bool {
	for _, r := range it.Torrents {
		if r.Site == site {
			return true
		}
	}
	return false
}

func hasTag(tags []string, t string) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}
