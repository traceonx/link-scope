// Package fileops 实现删除与回收站。
//
// 回收站放在路径所属根目录（LS_ROOTS 之一）下的 .linkscope-trash，移入只是同盘改名：
// 瞬间完成、保留硬链接关系，但空间要等清空回收站后才释放。
package fileops

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TrashDir 是回收站目录名，扫描时始终跳过。
const TrashDir = ".linkscope-trash"

const previewLimit = 200

var idRe = regexp.MustCompile(`^\d{14}-[0-9a-f]{8}$`)

var (
	ErrDisabled = errors.New("删除功能不可用")
	ErrNotFound = errors.New("回收站中没有这一项")
)

type Manager struct {
	mu        sync.Mutex
	roots     []string
	dataDir   string
	retention time.Duration
	disabled  bool
}

func New(roots []string, dataDir string, trashDays int, disabled bool) *Manager {
	if trashDays <= 0 {
		trashDays = 30
	}
	return &Manager{roots: roots, dataDir: dataDir, retention: time.Duration(trashDays) * 24 * time.Hour, disabled: disabled}
}

func (m *Manager) TrashDays() int { return int(m.retention / (24 * time.Hour)) }

func writable(p string) bool { return syscall.Access(p, 2 /* W_OK */) == nil }

// Capability 返回是否可以删除，以及不可用的原因。
func (m *Manager) Capability() (bool, string) {
	if m.disabled {
		return false, "已通过 LS_DELETE=false 关闭删除功能"
	}
	if len(m.roots) == 0 {
		return false, "需要设置 LS_ROOTS（允许操作的目录）才能删除文件"
	}
	for _, r := range m.roots {
		if writable(r) {
			return true, ""
		}
	}
	return false, "媒体目录是只读挂载，如需删除请在 compose 中去掉 :ro"
}

func (m *Manager) logf(format string, args ...any) {
	line := time.Now().Format("2006-01-02 15:04:05 ") + fmt.Sprintf(format, args...)
	log.Print("[删除] " + line[20:])
	_ = os.MkdirAll(m.dataDir, 0o755)
	if f, err := os.OpenFile(filepath.Join(m.dataDir, "delete.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString(line + "\n")
		_ = f.Close()
	}
}

// resolve 规范化路径（只解析上级目录中的软链接，路径本身是软链接时删除的是链接本身），
// 并返回所属根目录。不允许操作根目录本身和回收站。
func (m *Manager) resolve(p string) (real, root string, err error) {
	if !strings.HasPrefix(p, "/") {
		return "", "", fmt.Errorf("需要完整路径: %s", p)
	}
	clean := filepath.Clean(p)
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return "", "", fmt.Errorf("路径不存在: %s", p)
	}
	real = filepath.Join(parent, filepath.Base(clean))
	for _, r := range m.roots {
		if real == r {
			return "", "", fmt.Errorf("不能删除根目录: %s", p)
		}
		if strings.HasPrefix(real, strings.TrimRight(r, "/")+"/") {
			rel := strings.TrimPrefix(real, strings.TrimRight(r, "/")+"/")
			if rel == TrashDir || strings.HasPrefix(rel, TrashDir+"/") {
				return "", "", fmt.Errorf("请在回收站中管理: %s", p)
			}
			return real, r, nil
		}
	}
	return "", "", fmt.Errorf("路径不在允许范围内（LS_ROOTS）: %s", p)
}

// dedupe 去重，并去掉已包含在其他所选文件夹中的路径。
func dedupe(paths []string) []string {
	ps := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" && !seen[p] {
			seen[p] = true
			ps = append(ps, filepath.Clean(p))
		}
	}
	sort.Strings(ps)
	out := ps[:0]
	for _, p := range ps {
		if len(out) > 0 && (p == out[len(out)-1] || strings.HasPrefix(p, strings.TrimRight(out[len(out)-1], "/")+"/")) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ---- 预览 ----

type PreviewItem struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"` // file | dir | symlink
	Size  int64  `json:"size"`
	Files int    `json:"files"`
	Nlink uint64 `json:"nlink,omitempty"`
	Freed bool   `json:"freed"` // 文件：删除后是否释放空间
	Error string `json:"error,omitempty"`
}

type Preview struct {
	Items     []PreviewItem `json:"items"` // 最多 200 条
	Count     int           `json:"count"`
	Files     int           `json:"files"`
	Dirs      int           `json:"dirs"`
	Symlinks  int           `json:"symlinks"`
	TotalSize int64         `json:"total_size"`
	Freed     int64         `json:"freed"`     // 永久删除后实际释放的空间
	Kept      int           `json:"kept"`      // 其他位置仍有硬链接、删除后不释放空间的文件数
	KeptSize  int64         `json:"kept_size"` //
	Errors    int           `json:"errors"`
}

type inodeKey struct{ dev, ino uint64 }
type inodeAgg struct {
	size  int64
	nlink uint64
	count uint64
}

func statKey(fi fs.FileInfo) (inodeKey, uint64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return inodeKey{}, 1, false
	}
	return inodeKey{uint64(st.Dev), st.Ino}, uint64(st.Nlink), true //nolint:unconvert
}

// Preview 计算删除影响：数量、大小，以及按 inode 计算的实际释放空间。
func (m *Manager) Preview(paths []string) (*Preview, error) {
	if ok, reason := m.Capability(); !ok {
		return nil, fmt.Errorf("%w：%s", ErrDisabled, reason)
	}
	pv := &Preview{Items: []PreviewItem{}}
	agg := map[inodeKey]*inodeAgg{}
	type fileRef struct {
		idx int
		key inodeKey
	}
	var fileRefs []fileRef
	add := func(fi fs.FileInfo) (inodeKey, uint64) {
		k, n, _ := statKey(fi)
		a := agg[k]
		if a == nil {
			a = &inodeAgg{size: fi.Size(), nlink: n}
			agg[k] = a
		}
		a.count++
		return k, n
	}
	for _, p := range dedupe(paths) {
		pv.Count++
		it := PreviewItem{Path: p}
		real, _, err := m.resolve(p)
		var fi fs.FileInfo
		if err == nil {
			fi, err = os.Lstat(real)
		}
		if err != nil {
			it.Error = err.Error()
			if errors.Is(err, fs.ErrNotExist) {
				it.Error = "文件不存在"
			}
			pv.Errors++
		} else {
			switch {
			case fi.Mode()&fs.ModeSymlink != 0:
				it.Kind = "symlink"
				pv.Symlinks++
			case fi.IsDir():
				it.Kind = "dir"
				pv.Dirs++
				_ = filepath.WalkDir(real, func(_ string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.Type().IsRegular() {
						if info, err := d.Info(); err == nil {
							add(info)
							it.Files++
							it.Size += info.Size()
						}
					}
					return nil
				})
				pv.Files += it.Files
				pv.TotalSize += it.Size
			default:
				it.Kind = "file"
				it.Size = fi.Size()
				it.Files = 1
				k, n := add(fi)
				it.Nlink = n
				pv.Files++
				pv.TotalSize += it.Size
				if len(pv.Items) < previewLimit {
					fileRefs = append(fileRefs, fileRef{len(pv.Items), k})
				}
			}
		}
		if len(pv.Items) < previewLimit {
			pv.Items = append(pv.Items, it)
		}
	}
	for _, a := range agg {
		if a.count >= a.nlink {
			pv.Freed += a.size
		} else {
			pv.Kept++
			pv.KeptSize += a.size
		}
	}
	for _, r := range fileRefs {
		a := agg[r.key]
		pv.Items[r.idx].Freed = a.count >= a.nlink
	}
	return pv, nil
}

// ---- 删除 ----

type Result struct {
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	TrashID string `json:"trash_id,omitempty"`
}

type TrashEntry struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Size      int64  `json:"size"`
	Files     int    `json:"files"`
	Root      string `json:"root"`
	DeletedAt int64  `json:"deleted_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}

func dirStats(p string) (int64, int) {
	var size int64
	var files int
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
				files++
			}
		}
		return nil
	})
	return size, files
}

// Delete 删除路径；permanent=false 时移到回收站。返回每条结果与成功删除的路径。
func (m *Manager) Delete(paths []string, permanent bool) ([]Result, []string, error) {
	if ok, reason := m.Capability(); !ok {
		return nil, nil, fmt.Errorf("%w：%s", ErrDisabled, reason)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []Result
	var done []string
	for _, p := range dedupe(paths) {
		r := Result{Path: p}
		real, root, err := m.resolve(p)
		if err == nil {
			if permanent {
				err = m.remove(real)
			} else {
				r.TrashID, err = m.toTrash(real, root)
			}
		}
		if err != nil {
			r.Error = err.Error()
			m.logf("失败 %s：%v", p, err)
		} else {
			r.OK = true
			done = append(done, real)
			if permanent {
				m.logf("永久删除 %s", real)
			} else {
				m.logf("移到回收站 %s（%s）", real, r.TrashID)
			}
		}
		results = append(results, r)
	}
	return results, done, nil
}

func (m *Manager) remove(real string) error {
	fi, err := os.Lstat(real)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errors.New("文件不存在")
		}
		return err
	}
	if fi.IsDir() {
		return os.RemoveAll(real)
	}
	return os.Remove(real)
}

func (m *Manager) toTrash(real, root string) (string, error) {
	fi, err := os.Lstat(real)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", errors.New("文件不存在")
		}
		return "", err
	}
	e := TrashEntry{ID: newID(), Path: real, Name: filepath.Base(real), Root: root, DeletedAt: time.Now().Unix()}
	switch {
	case fi.Mode()&fs.ModeSymlink != 0:
		e.Kind = "symlink"
	case fi.IsDir():
		e.Kind = "dir"
		e.Size, e.Files = dirStats(real)
	default:
		e.Kind, e.Size, e.Files = "file", fi.Size(), 1
	}
	dir := filepath.Join(root, TrashDir, e.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("无法创建回收站目录：%v", err)
	}
	if err := os.Rename(real, filepath.Join(dir, e.Name)); err != nil {
		_ = os.Remove(dir)
		if errors.Is(err, syscall.EXDEV) {
			return "", errors.New("文件与回收站不在同一个存储空间，无法移入回收站，请选择永久删除")
		}
		return "", err
	}
	b, _ := json.Marshal(e)
	_ = os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644)
	return e.ID, nil
}

// ---- 回收站 ----

func (m *Manager) entryDir(id string) (string, *TrashEntry, error) {
	if !idRe.MatchString(id) {
		return "", nil, ErrNotFound
	}
	for _, r := range m.roots {
		dir := filepath.Join(r, TrashDir, id)
		b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
		if err != nil {
			continue
		}
		var e TrashEntry
		if json.Unmarshal(b, &e) == nil {
			return dir, &e, nil
		}
	}
	return "", nil, ErrNotFound
}

// Trash 列出回收站内容（新的在前）。
func (m *Manager) Trash() []TrashEntry {
	out := []TrashEntry{}
	for _, r := range m.roots {
		des, err := os.ReadDir(filepath.Join(r, TrashDir))
		if err != nil {
			continue
		}
		for _, d := range des {
			if !idRe.MatchString(d.Name()) {
				continue
			}
			if _, e, err := m.entryDir(d.Name()); err == nil {
				e.ExpiresAt = e.DeletedAt + int64(m.retention/time.Second)
				out = append(out, *e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// Restore 恢复到原位置；原位置已有同名文件时失败。返回结果与恢复后的路径。
func (m *Manager) Restore(ids []string) ([]Result, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []Result
	var restored []string
	for _, id := range ids {
		r := Result{TrashID: id}
		dir, e, err := m.entryDir(id)
		if err == nil {
			r.Path = e.Path
			if _, statErr := os.Lstat(e.Path); statErr == nil {
				err = errors.New("原位置已存在同名文件或文件夹")
			} else if err = os.MkdirAll(filepath.Dir(e.Path), 0o755); err == nil {
				if err = os.Rename(filepath.Join(dir, e.Name), e.Path); err == nil {
					_ = os.RemoveAll(dir)
				}
			}
		}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.OK = true
			restored = append(restored, e.Path)
			m.logf("从回收站恢复 %s", e.Path)
		}
		results = append(results, r)
	}
	return results, restored
}

// Purge 永久删除回收站中的条目；all=true 时清空全部。
func (m *Manager) Purge(ids []string, all bool) []Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	if all {
		ids = ids[:0]
		for _, e := range m.Trash() {
			ids = append(ids, e.ID)
		}
	}
	var results []Result
	for _, id := range ids {
		r := Result{TrashID: id}
		dir, e, err := m.entryDir(id)
		if err == nil {
			r.Path = e.Path
			err = os.RemoveAll(dir)
		}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.OK = true
			m.logf("清除回收站 %s", e.Path)
		}
		results = append(results, r)
	}
	return results
}

// CleanExpired 清除超过保留天数的条目。
func (m *Manager) CleanExpired() int {
	var expired []string
	limit := time.Now().Add(-m.retention).Unix()
	for _, e := range m.Trash() {
		if e.DeletedAt < limit {
			expired = append(expired, e.ID)
		}
	}
	if len(expired) == 0 {
		return 0
	}
	m.Purge(expired, false)
	return len(expired)
}
