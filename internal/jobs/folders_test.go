package jobs

import (
	"testing"

	"github.com/traceonx/link-scope/internal/scanner"
)

func TestBuildFolders(t *testing.T) {
	r := &scanner.Result{Summary: scanner.Summary{Source: "/dl"}, Items: []scanner.Item{
		{Type: "file", Path: "/dl/MovieA/a.mkv", Size: 10, Linked: true, Tags: []string{"linked"}},
		{Type: "file", Path: "/dl/MovieA/extras/b.mkv", Size: 5, Linked: true, Tags: []string{"linked"}},
		{Type: "file", Path: "/dl/ShowB/e1.mkv", Size: 3, Linked: true, Tags: []string{"linked", "seeding"},
			Torrents: []scanner.TorrentRef{{Site: "天空 HDSky", State: "seeding"}}},
		{Type: "file", Path: "/dl/ShowB/e2.mkv", Size: 3, Tags: []string{"unlinked", "dup"}, Dups: []string{"/m/x"}},
		{Type: "file", Path: "/dl/Old/c.mkv", Size: 7, Tags: []string{"unlinked", "orphan"}},
		{Type: "file", Path: "/dl/single.mkv", Size: 2, Tags: []string{"unlinked"}},
		{Type: "symlink", Path: "/dl/Old/bad", Broken: true, Tags: []string{"symlink", "broken"}},
	}}
	m := &Manager{}
	m.setResult(r)

	counts, all := m.Folders(Q{})
	if len(all) != 4 {
		t.Fatalf("文件夹数 %d: %+v", len(all), all)
	}
	by := map[string]Folder{}
	for _, f := range all {
		by[f.Name] = f
	}
	if f := by["MovieA"]; f.Status != "linked" || f.Files != 2 || f.Size != 15 || f.IsFile {
		t.Errorf("MovieA: %+v", f)
	}
	if f := by["ShowB"]; f.Status != "partial" || f.Linked != 1 || f.Dups != 1 || f.Seeding != 1 || len(f.Sites) != 1 {
		t.Errorf("ShowB: %+v", f)
	}
	if f := by["Old"]; f.Status != "unlinked" || f.Orphans != 1 || f.Broken != 1 {
		t.Errorf("Old: %+v", f)
	}
	if f := by["single.mkv"]; !f.IsFile || f.Status != "unlinked" {
		t.Errorf("single: %+v", f)
	}
	if counts["linked"] != 2 || counts["full"] != 1 || counts["partial"] != 1 || counts["unlinked"] != 2 || counts["seeding"] != 1 {
		t.Errorf("counts: %+v", counts)
	}
	if _, l := m.Folders(Q{Filter: "unlinked"}); len(l) != 2 {
		t.Errorf("未链接筛选: %d", len(l))
	}
	if _, l := m.Folders(Q{Site: "天空 HDSky"}); len(l) != 1 || l[0].Name != "ShowB" {
		t.Errorf("站点筛选: %+v", l)
	}
	if _, l := m.Folders(Q{Text: "movie"}); len(l) != 1 {
		t.Errorf("搜索: %d", len(l))
	}
	if n, items := m.Query(Q{Dir: "/dl/ShowB"}); n != 2 || len(items) != 2 {
		t.Errorf("按文件夹查询文件: %d", n)
	}
	// 结果变化后缓存失效
	m.RemovePaths([]string{"/dl/Old"})
	if _, l := m.Folders(Q{}); len(l) != 3 {
		t.Errorf("删除后文件夹数 %d", len(l))
	}
}
