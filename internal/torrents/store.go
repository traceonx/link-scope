package torrents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store 保存下载器配置（DATA_DIR/downloaders.json），并负责读取所有下载器的种子。
type Store struct {
	mu      sync.Mutex
	file    string
	list    []Downloader
	refresh sync.Mutex
}

func NewStore(dataDir string) *Store {
	s := &Store{file: filepath.Join(dataDir, "downloaders.json")}
	if b, err := os.ReadFile(s.file); err == nil {
		_ = json.Unmarshal(b, &s.list)
	}
	return s
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s.list, "", "  ")
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // 含密码，仅本用户可读
		return err
	}
	return os.Rename(tmp, s.file)
}

// List 返回下载器列表（不含密码）。
func (s *Store) List() []Downloader {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Downloader, len(s.list))
	for i, d := range s.list {
		d.Password = ""
		if d.Mappings == nil {
			d.Mappings = []Mapping{}
		}
		out[i] = d
	}
	return out
}

func (s *Store) HasAny() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.list {
		if d.Enabled {
			return true
		}
	}
	return false
}

func validate(d *Downloader) error {
	d.Name = strings.TrimSpace(d.Name)
	d.URL = strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if d.Name == "" {
		return errors.New("请填写名称")
	}
	if d.Type != "qbittorrent" && d.Type != "transmission" {
		return errors.New("类型只能是 qbittorrent 或 transmission")
	}
	if !strings.HasPrefix(d.URL, "http://") && !strings.HasPrefix(d.URL, "https://") {
		return errors.New("地址需以 http:// 或 https:// 开头")
	}
	ms := d.Mappings[:0]
	for _, m := range d.Mappings {
		m.From, m.To = strings.TrimSpace(m.From), strings.TrimSpace(m.To)
		if m.From == "" && m.To == "" {
			continue
		}
		if !strings.HasPrefix(m.From, "/") || !strings.HasPrefix(m.To, "/") {
			return errors.New("路径映射需填写以 / 开头的完整路径")
		}
		ms = append(ms, m)
	}
	d.Mappings = ms
	return nil
}

// Save 新增或修改下载器；修改时密码留空表示不变。
func (s *Store) Save(d Downloader) (Downloader, error) {
	if d.Mappings == nil {
		d.Mappings = []Mapping{}
	}
	if err := validate(&d); err != nil {
		return Downloader{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		d.ID = hex.EncodeToString(b)
		s.list = append(s.list, d)
	} else {
		found := false
		for i := range s.list {
			if s.list[i].ID == d.ID {
				if d.Password == "" {
					d.Password = s.list[i].Password
				}
				s.list[i] = d
				found = true
			}
		}
		if !found {
			return Downloader{}, errors.New("下载器不存在")
		}
	}
	if err := s.save(); err != nil {
		return Downloader{}, err
	}
	d.Password = ""
	return d, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return s.save()
		}
	}
	return errors.New("下载器不存在")
}

// Test 测试连接；id 不为空且密码留空时使用已保存的密码。
func (s *Store) Test(ctx context.Context, d Downloader) (int, error) {
	if d.Mappings == nil {
		d.Mappings = []Mapping{}
	}
	if err := validate(&d); err != nil {
		return 0, err
	}
	if d.ID != "" && d.Password == "" {
		s.mu.Lock()
		for _, x := range s.list {
			if x.ID == d.ID {
				d.Password = x.Password
			}
		}
		s.mu.Unlock()
	}
	c, err := newClient(&d)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ts, err := c.torrents(ctx)
	return len(ts), err
}

// Snapshot 是一次读取所有下载器的结果。
type Snapshot struct {
	FetchedAt int64
	Clients   []ClientResult
}

type ClientResult struct {
	Name     string
	Error    string
	Torrents []Torrent
	Sites    []string // 与 Torrents 对应的站点名
}

// Fetch 并发读取所有启用的下载器。
func (s *Store) Fetch(ctx context.Context) Snapshot {
	s.refresh.Lock()
	defer s.refresh.Unlock()
	s.mu.Lock()
	var list []Downloader
	for _, d := range s.list {
		if d.Enabled {
			list = append(list, d)
		}
	}
	s.mu.Unlock()
	snap := Snapshot{FetchedAt: time.Now().Unix(), Clients: make([]ClientResult, len(list))}
	var wg sync.WaitGroup
	for i := range list {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d := list[i]
			r := ClientResult{Name: d.Name}
			c, err := newClient(&d)
			if err == nil {
				cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				r.Torrents, err = c.torrents(cctx)
				cancel()
			}
			if err != nil {
				r.Error = fmt.Sprint(err)
			}
			for _, t := range r.Torrents {
				r.Sites = append(r.Sites, siteOf(t.Trackers))
			}
			snap.Clients[i] = r
		}(i)
	}
	wg.Wait()
	return snap
}

// RemovedTorrent 是一个已从下载器移除的种子。
type RemovedTorrent struct {
	Client string `json:"client"`
	Name   string `json:"name"`
}

// RemoveResult 是 RemoveByPaths 的结果。
type RemoveResult struct {
	Removed []RemovedTorrent `json:"removed"`
	Partial int              `json:"partial"` // 只删了部分文件、因此保留的种子数
	Errors  []string         `json:"errors"`
}

func underAny(p string, dirs []string) bool {
	for _, d := range dirs {
		d = strings.TrimRight(d, "/")
		if p == d || strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}

// RemoveByPaths 从所有启用的下载器中移除文件已被删除的种子（不删除文件）。
// 只有种子的全部文件都在 deleted 中（或已不存在）时才移除；只删了一部分的种子保留，避免误停仍在用的种子。
func (s *Store) RemoveByPaths(ctx context.Context, deleted []string) RemoveResult {
	res := RemoveResult{Removed: []RemovedTorrent{}, Errors: []string{}}
	if len(deleted) == 0 {
		return res
	}
	clean := make([]string, len(deleted))
	for i, p := range deleted {
		clean[i] = filepath.Clean(p)
	}
	s.mu.Lock()
	var list []Downloader
	for _, d := range s.list {
		if d.Enabled {
			list = append(list, d)
		}
	}
	s.mu.Unlock()

	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range list {
		wg.Add(1)
		go func(d Downloader) {
			defer wg.Done()
			fail := func(err error) {
				mu.Lock()
				res.Errors = append(res.Errors, fmt.Sprintf("%s：%v", d.Name, err))
				mu.Unlock()
			}
			c, err := newClient(&d)
			if err != nil {
				fail(err)
				return
			}
			cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			ts, err := c.torrents(cctx)
			if err != nil {
				fail(err)
				return
			}
			var hashes []string
			var removed []RemovedTorrent
			partial := 0
			for _, t := range ts {
				hit, all := false, true
				for _, f := range t.Files {
					f = filepath.Clean(f)
					if underAny(f, clean) {
						hit = true
					} else if pathExists(f) {
						all = false
					}
				}
				if !hit {
					continue
				}
				if !all {
					partial++
					continue
				}
				hashes = append(hashes, t.Hash)
				removed = append(removed, RemovedTorrent{Client: d.Name, Name: t.Name})
			}
			if len(hashes) > 0 {
				if err := c.remove(cctx, hashes); err != nil {
					fail(err)
					removed = nil
				}
			}
			mu.Lock()
			res.Removed = append(res.Removed, removed...)
			res.Partial += partial
			mu.Unlock()
		}(list[i])
	}
	wg.Wait()
	return res
}
