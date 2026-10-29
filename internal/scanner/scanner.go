// Package scanner 实现链接检测：硬链接、重复文件、软链接、STRM。
package scanner

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	strmMaxSize = 64 * 1024
	sampleSize  = 1 << 20 // 抽样比对：头 / 中 / 尾各 1 MB
	maxErrors   = 100
)

// DupMode 重复文件检测方式。
type DupMode string

const (
	DupOff   DupMode = "off"   // 不检测
	DupQuick DupMode = "quick" // 抽样比对（快）
	DupFull  DupMode = "full"  // 完整比对（慢，最准确）
)

// Options 扫描参数。
type Options struct {
	Source      string
	Targets     []string
	Recursive   bool
	MinSize     int64
	DupMode     DupMode
	Workers     int             // 并发读取目录数
	HashWorkers int             // 并发计算指纹数
	SkipDirs    map[string]bool // 跳过的目录名（如 @eaDir、#recycle）
	Progress    func(phase string, done, total int64)
}

// Item 是结果中的一条记录。
type Item struct {
	Type     string   `json:"type"` // file | symlink | strm
	Name     string   `json:"name"`
	Dir      string   `json:"dir"`
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Mtime    int64    `json:"mtime"`
	Nlink    uint64   `json:"nlink,omitempty"`
	Inode    uint64   `json:"inode,omitempty"`
	Linked   bool     `json:"linked"`
	Links    []string `json:"links,omitempty"`
	Missing  *int     `json:"missing,omitempty"` // 链接中未在目标目录找到的数量（仅指定目标目录时）
	Dups     []string `json:"dups,omitempty"`
	Target   string   `json:"target,omitempty"`
	Resolved string   `json:"resolved,omitempty"`
	Remote   bool     `json:"remote,omitempty"`
	Broken   bool     `json:"broken"`
	Tags     []string `json:"tags"`
	// 种子信息（连接下载器后填充）
	Torrents []TorrentRef `json:"torrents,omitempty"`
}

// TorrentRef 表示文件属于下载器中的某个种子。
type TorrentRef struct {
	Client       string `json:"client"`
	Hash         string `json:"hash"`
	Name         string `json:"name"`
	Site         string `json:"site"`
	State        string `json:"state"` // seeding | downloading | paused | checking | queued | error
	Unregistered bool   `json:"unregistered,omitempty"`
	Message      string `json:"message,omitempty"`
}

// TorrentSummary 是种子检测的汇总。
type TorrentSummary struct {
	FetchedAt    int64        `json:"fetched_at"`
	Clients      []ClientStat `json:"clients"`
	Sites        []SiteStat   `json:"sites"`
	Matched      int          `json:"matched"` // 属于某个种子的文件数
	Seeding      int          `json:"seeding"`
	Unregistered int          `json:"unregistered"`
	Orphans      int          `json:"orphans"`
	OrphanSize   int64        `json:"orphan_size"`
}

type ClientStat struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Torrents int    `json:"torrents"`
}

type SiteStat struct {
	Site         string `json:"site"`
	Files        int    `json:"files"`
	Size         int64  `json:"size"`
	Seeding      int    `json:"seeding"`
	Unregistered int    `json:"unregistered"`
}

// Summary 扫描汇总。
type Summary struct {
	Source       string   `json:"source"`
	Targets      []string `json:"targets"`
	Recursive    bool     `json:"recursive"`
	MinSize      int64    `json:"min_size"`
	DupMode      DupMode  `json:"dup_mode"`
	Total        int      `json:"total"`
	TotalSize    int64    `json:"total_size"`
	LinkedSize   int64    `json:"linked_size"`   // 已链接（已整理）文件的大小
	UnlinkedSize int64    `json:"unlinked_size"` // 未链接（待清理）文件的大小
	Linked       int      `json:"linked"`
	Unlinked     int      `json:"unlinked"`
	Saved        int64    `json:"saved"`
	DupFiles     int      `json:"dup_files"`
	DupGroups    int      `json:"dup_groups"`
	DupWasted    int64    `json:"dup_wasted"`
	Symlinks     int      `json:"symlinks"`
	BrokenLinks  int      `json:"broken_links"`
	Strm         int      `json:"strm"`
	BrokenStrm   int      `json:"broken_strm"`
	ScannedAt    int64    `json:"scanned_at"`
	Duration     float64  `json:"duration"`
	Warnings     []string `json:"warnings"`
	Errors       []string `json:"errors"`
	ErrorCount   int      `json:"error_count"`
	// Stale：扫描后有文件被删除或恢复，统计为估算，建议重新扫描
	Stale    bool            `json:"stale,omitempty"`
	Torrents *TorrentSummary `json:"torrents,omitempty"`
}

// Result 扫描结果。
type Result struct {
	Summary Summary `json:"summary"`
	Items   []Item  `json:"items"`
}

type errList struct {
	mu    sync.Mutex
	items []string
	count int
}

func (e *errList) add(s string) {
	e.mu.Lock()
	e.count++
	if len(e.items) < maxErrors {
		e.items = append(e.items, s)
	}
	e.mu.Unlock()
}

type inodeRec struct {
	path  string
	size  int64
	paths []string
	src   bool
}

// Scan 执行一次扫描。ctx 取消时返回 context.Canceled。
func Scan(ctx context.Context, o Options) (*Result, error) {
	started := time.Now()
	if o.DupMode == "" {
		o.DupMode = DupQuick
	}
	if o.DupMode != DupOff && o.DupMode != DupQuick && o.DupMode != DupFull {
		return nil, fmt.Errorf("未知的重复检测方式: %s", o.DupMode)
	}
	if o.Workers <= 0 {
		o.Workers = 8
	}
	if o.HashWorkers <= 0 {
		o.HashWorkers = 2
	}
	progress := o.Progress
	if progress == nil {
		progress = func(string, int64, int64) {}
	}

	source, err := filepath.Abs(o.Source)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(source); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("文件夹不存在: %s", source)
	}
	srcDev, err := devOf(source)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(o.Targets))
	for _, t := range o.Targets {
		if t = strings.TrimSpace(t); t != "" {
			if a, err := filepath.Abs(t); err == nil {
				targets = append(targets, a)
			}
		}
	}

	var errs errList
	var warnings []string
	var counter atomic.Int64

	// 1. 扫描源目录
	var mu sync.Mutex
	var srcFiles, srcLinks, srcStrm []entry
	walk(ctx, source, o.Recursive, o.Workers, o.SkipDirs, func(e entry) {
		n := counter.Add(1)
		if n%500 == 0 {
			progress("source", n, 0)
		}
		mu.Lock()
		switch {
		case e.link:
			srcLinks = append(srcLinks, e)
		case strings.HasSuffix(strings.ToLower(e.path), ".strm") && e.size <= strmMaxSize:
			srcStrm = append(srcStrm, e)
		case e.size >= o.MinSize:
			srcFiles = append(srcFiles, e)
		}
		mu.Unlock()
	}, errs.add)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	progress("source", counter.Load(), 0)

	// 2. 建立索引
	index := map[inodeKey][]string{}
	seen := make(map[string]bool, len(srcFiles))
	srcSizes := map[int64]bool{}
	inodes := map[inodeKey]*inodeRec{}
	for i := range srcFiles {
		e := &srcFiles[i]
		seen[e.path] = true
		if e.nlink > 1 {
			index[e.key()] = append(index[e.key()], e.path)
		}
		if e.size > 0 {
			srcSizes[e.size] = true
			r := inodes[e.key()]
			if r == nil {
				r = &inodeRec{path: e.path, size: e.size, src: true}
				inodes[e.key()] = r
			}
			r.paths = append(r.paths, e.path)
		}
	}

	// 3. 扫描目标目录：硬链接位置 + 重复候选
	counter.Store(0)
	for _, t := range targets {
		fi, err := os.Stat(t)
		if err != nil || !fi.IsDir() {
			warnings = append(warnings, "目标目录不存在: "+t)
			continue
		}
		if d, err := devOf(t); err == nil && d != srcDev {
			warnings = append(warnings, t+" 与源目录不在同一个存储空间，二者之间不可能有硬链接（仍会查找重复副本）")
		}
		var tFiles []entry
		walk(ctx, t, true, o.Workers, o.SkipDirs, func(e entry) {
			n := counter.Add(1)
			if n%500 == 0 {
				progress("targets", n, 0)
			}
			if e.link {
				return
			}
			mu.Lock()
			tFiles = append(tFiles, e)
			mu.Unlock()
		}, errs.add)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for i := range tFiles {
			e := &tFiles[i]
			if seen[e.path] {
				continue
			}
			seen[e.path] = true
			k := e.key()
			if _, ok := index[k]; ok {
				index[k] = append(index[k], e.path)
			}
			if o.DupMode != DupOff && srcSizes[e.size] {
				r := inodes[k]
				if r == nil {
					r = &inodeRec{path: e.path, size: e.size}
					inodes[k] = r
				}
				r.paths = append(r.paths, e.path)
			}
		}
	}
	if len(targets) > 0 {
		progress("targets", counter.Load(), 0)
	}

	// 4. 重复检测：大小相同、inode 不同 → 比对内容指纹
	dupOf := map[inodeKey][]string{}
	var dupGroups int
	var dupWasted int64
	if o.DupMode != DupOff {
		bySize := map[int64][]inodeKey{}
		for k, r := range inodes {
			bySize[r.size] = append(bySize[r.size], k)
		}
		var cands [][]inodeKey
		var total int64
		for _, ks := range bySize {
			if len(ks) < 2 {
				continue
			}
			hasSrc := false
			for _, k := range ks {
				hasSrc = hasSrc || inodes[k].src
			}
			if hasSrc {
				sort.Slice(ks, func(i, j int) bool { return inodes[ks[i]].path < inodes[ks[j]].path })
				cands = append(cands, ks)
				total += int64(len(ks))
			}
		}
		fps, err := fingerprints(ctx, cands, inodes, o.DupMode, o.HashWorkers, total, progress)
		if err != nil {
			return nil, err
		}
		for _, ks := range cands {
			byFp := map[string][]inodeKey{}
			var order []string
			for _, k := range ks {
				fp := fps[k]
				if fp == "" {
					continue
				}
				if _, ok := byFp[fp]; !ok {
					order = append(order, fp)
				}
				byFp[fp] = append(byFp[fp], k)
			}
			for _, fp := range order {
				g := byFp[fp]
				if len(g) < 2 {
					continue
				}
				hasSrc := false
				for _, k := range g {
					hasSrc = hasSrc || inodes[k].src
				}
				if !hasSrc {
					continue
				}
				dupGroups++
				dupWasted += inodes[g[0]].size * int64(len(g)-1)
				for _, k := range g {
					var others []string
					for _, o2 := range g {
						if o2 != k {
							others = append(others, inodes[o2].paths...)
						}
					}
					dupOf[k] = others
				}
			}
		}
	}

	// 5. 汇总
	s := Summary{
		Source: source, Targets: targets, Recursive: o.Recursive, MinSize: o.MinSize, DupMode: o.DupMode,
		ScannedAt: started.Unix(), Warnings: warnings,
	}
	if s.Targets == nil {
		s.Targets = []string{}
	}
	if s.Warnings == nil {
		s.Warnings = []string{}
	}
	items := make([]Item, 0, len(srcFiles)+len(srcLinks)+len(srcStrm))
	for i := range srcFiles {
		e := &srcFiles[i]
		var others []string
		for _, p := range index[e.key()] {
			if p != e.path {
				others = append(others, p)
			}
		}
		it := Item{
			Type: "file", Name: filepath.Base(e.path), Dir: filepath.Dir(e.path), Path: e.path,
			Size: e.size, Mtime: e.mtime, Nlink: e.nlink, Inode: e.ino, Linked: e.nlink > 1,
			Links: others, Dups: dupOf[e.key()],
		}
		if len(targets) > 0 {
			m := int(e.nlink) - 1 - len(others)
			if m < 0 {
				m = 0
			}
			it.Missing = &m
		}
		if it.Linked {
			it.Tags = []string{"linked"}
			s.Linked++
			s.LinkedSize += e.size
			s.Saved += e.size * int64(e.nlink-1)
		} else {
			it.Tags = []string{"unlinked"}
			s.UnlinkedSize += e.size
		}
		if len(it.Dups) > 0 {
			it.Tags = append(it.Tags, "dup")
			s.DupFiles++
		}
		s.TotalSize += e.size
		items = append(items, it)
	}
	s.Total = len(srcFiles)
	s.Unlinked = s.Total - s.Linked
	s.DupGroups, s.DupWasted = dupGroups, dupWasted

	for i := range srcLinks {
		e := &srcLinks[i]
		raw, _ := os.Readlink(e.path)
		it := Item{Type: "symlink", Name: filepath.Base(e.path), Dir: filepath.Dir(e.path), Path: e.path,
			Mtime: e.mtime, Target: raw, Tags: []string{"symlink"}}
		if fi, err := os.Stat(e.path); err != nil {
			it.Broken = true
			it.Tags = append(it.Tags, "broken")
			s.BrokenLinks++
			if filepath.IsAbs(raw) {
				it.Resolved = raw
			} else {
				it.Resolved = filepath.Join(filepath.Dir(e.path), raw)
			}
		} else {
			if fi.Mode().IsRegular() {
				it.Size = fi.Size()
			}
			if r, err := filepath.EvalSymlinks(e.path); err == nil {
				it.Resolved = r
			}
		}
		items = append(items, it)
	}
	s.Symlinks = len(srcLinks)

	for i := range srcStrm {
		e := &srcStrm[i]
		target := readStrm(e.path)
		it := Item{Type: "strm", Name: filepath.Base(e.path), Dir: filepath.Dir(e.path), Path: e.path,
			Size: e.size, Mtime: e.mtime, Target: target, Tags: []string{"strm"}}
		if strings.Contains(target, "://") {
			it.Remote = true
		} else if target == "" || !exists(target) {
			it.Broken = true
			it.Tags = append(it.Tags, "broken")
			s.BrokenStrm++
		}
		items = append(items, it)
	}
	s.Strm = len(srcStrm)

	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	s.Errors = errs.items
	if s.Errors == nil {
		s.Errors = []string{}
	}
	s.ErrorCount = errs.count
	s.Duration, _ = strconv.ParseFloat(strconv.FormatFloat(time.Since(started).Seconds(), 'f', 2, 64), 64)
	return &Result{Summary: s, Items: items}, nil
}

func fingerprints(ctx context.Context, cands [][]inodeKey, inodes map[inodeKey]*inodeRec, mode DupMode,
	workers int, total int64, progress func(string, int64, int64)) (map[inodeKey]string, error) {
	jobs := make(chan inodeKey)
	out := make(map[inodeKey]string, total)
	var mu sync.Mutex
	var done atomic.Int64
	var wg sync.WaitGroup
	progress("dups", 0, total)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 4<<20) // 每个 worker 复用一块缓冲区，避免频繁分配
			for k := range jobs {
				r := inodes[k]
				fp, _ := fingerprint(ctx, r.path, r.size, mode, buf)
				mu.Lock()
				out[k] = fp
				mu.Unlock()
				progress("dups", done.Add(1), total)
			}
		}()
	}
loop:
	for _, ks := range cands {
		for _, k := range ks {
			select {
			case jobs <- k:
			case <-ctx.Done():
				break loop
			}
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return out, nil
}

// fingerprint 计算文件内容指纹；quick 模式只读头 / 中 / 尾各 1 MB。
// buf 至少需要 sampleSize 大小。
func fingerprint(ctx context.Context, path string, size int64, mode DupMode, buf []byte) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	fmt.Fprintf(h, "%d:", size)
	if mode == DupFull || size <= sampleSize*3 {
		for {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			n, err := f.Read(buf)
			h.Write(buf[:n])
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", err
			}
		}
	} else {
		b := buf[:sampleSize]
		for _, off := range []int64{0, size/2 - sampleSize/2, size - sampleSize} {
			n, err := f.ReadAt(b, off)
			if err != nil && !errors.Is(err, io.EOF) {
				return "", err
			}
			h.Write(b[:n])
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readStrm(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, strmMaxSize))
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\uFEFF"))
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
