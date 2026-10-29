package jobs

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/traceonx/link-scope/internal/scanner"
)

// FolderFilters 是文件夹视图支持的筛选项。
// linked：含已链接文件（全部或部分）；full：全部链接；partial：部分链接；unlinked：没有任何已链接文件。
var FolderFilters = []string{"all", "linked", "full", "partial", "unlinked", "dup", "seeding", "unregistered", "orphan"}

// Folder 是检测目录下一个一级子文件夹（或直接放在检测目录下的单个文件）的汇总。
type Folder struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	IsFile       bool     `json:"is_file"` // 直接位于检测目录下的文件
	Status       string   `json:"status"`  // linked（全部链接） | partial | unlinked
	Files        int      `json:"files"`
	Linked       int      `json:"linked"`
	Unlinked     int      `json:"unlinked"`
	Size         int64    `json:"size"`
	LinkedSize   int64    `json:"linked_size"`
	Dups         int      `json:"dups"`
	Symlinks     int      `json:"symlinks"`
	Broken       int      `json:"broken"`
	Torrents     int      `json:"torrents"` // 属于种子的文件数
	Seeding      int      `json:"seeding"`
	Unregistered int      `json:"unregistered"`
	Orphans      int      `json:"orphans"`
	Sites        []string `json:"sites"`
	Mtime        int64    `json:"mtime"` // 最近修改的文件时间
}

func (f *Folder) tags() []string {
	var t []string
	if f.Linked > 0 {
		t = append(t, "linked")
	}
	switch f.Status {
	case "linked":
		t = append(t, "full")
	case "partial":
		t = append(t, "partial")
	default:
		t = append(t, "unlinked")
	}
	if f.Dups > 0 {
		t = append(t, "dup")
	}
	if f.Seeding > 0 {
		t = append(t, "seeding")
	}
	if f.Unregistered > 0 {
		t = append(t, "unregistered")
	}
	if f.Orphans > 0 {
		t = append(t, "orphan")
	}
	return t
}

// buildFolders 按检测目录的一级子项分组汇总。
func buildFolders(r *scanner.Result) []Folder {
	src := strings.TrimRight(r.Summary.Source, "/")
	idx := map[string]int{}
	var out []Folder
	sites := map[string]map[string]bool{}
	for i := range r.Items {
		it := &r.Items[i]
		rel, ok := strings.CutPrefix(it.Path, src+"/")
		if !ok {
			continue
		}
		top, _, nested := strings.Cut(rel, "/")
		key := filepath.Join(src, top)
		j, ok := idx[key]
		if !ok {
			j = len(out)
			idx[key] = j
			out = append(out, Folder{Name: top, Path: key, IsFile: !nested, Sites: []string{}})
			sites[key] = map[string]bool{}
		}
		f := &out[j]
		if it.Mtime > f.Mtime {
			f.Mtime = it.Mtime
		}
		switch it.Type {
		case "symlink", "strm":
			if it.Type == "symlink" {
				f.Symlinks++
			}
			if it.Broken {
				f.Broken++
			}
			continue
		}
		f.Files++
		f.Size += it.Size
		if it.Linked {
			f.Linked++
			f.LinkedSize += it.Size
		} else {
			f.Unlinked++
		}
		if len(it.Dups) > 0 {
			f.Dups++
		}
		if len(it.Torrents) > 0 {
			f.Torrents++
		}
		for _, t := range it.Tags {
			switch t {
			case "seeding":
				f.Seeding++
			case "unregistered":
				f.Unregistered++
			case "orphan":
				f.Orphans++
			}
		}
		for _, t := range it.Torrents {
			if !sites[key][t.Site] {
				sites[key][t.Site] = true
				f.Sites = append(f.Sites, t.Site)
			}
		}
	}
	keep := out[:0]
	for _, f := range out {
		if f.Files == 0 && f.Symlinks == 0 && f.Broken == 0 {
			continue
		}
		switch {
		case f.Files > 0 && f.Linked == f.Files:
			f.Status = "linked"
		case f.Linked > 0:
			f.Status = "partial"
		default:
			f.Status = "unlinked"
		}
		sort.Strings(f.Sites)
		keep = append(keep, f)
	}
	sort.Slice(keep, func(i, j int) bool { return strings.ToLower(keep[i].Name) < strings.ToLower(keep[j].Name) })
	return keep
}

// Folders 返回各筛选项的数量和过滤后的文件夹列表。
func (m *Manager) Folders(o Q) (map[string]int, []Folder) {
	counts := make(map[string]int, len(FolderFilters))
	for _, f := range FolderFilters {
		counts[f] = 0
	}
	m.mu.Lock()
	if m.result != nil && m.folders == nil {
		m.folders = buildFolders(m.result)
	}
	all := m.folders
	m.mu.Unlock()

	filter := o.Filter
	if filter == "" {
		filter = "all"
	}
	q := strings.ToLower(strings.TrimSpace(o.Text))
	out := []Folder{}
	for i := range all {
		f := &all[i]
		tags := f.tags()
		counts["all"]++
		for _, t := range tags {
			counts[t]++
		}
		if filter != "all" && !hasTag(tags, filter) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(f.Name), q) {
			continue
		}
		if o.Site != "" && !hasTag(f.Sites, o.Site) {
			continue
		}
		out = append(out, *f)
	}
	return counts, out
}
