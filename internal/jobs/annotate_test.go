package jobs

import (
	"testing"

	"github.com/traceonx/link-scope/internal/scanner"
	"github.com/traceonx/link-scope/internal/torrents"
)

func sample() *scanner.Result {
	return &scanner.Result{Items: []scanner.Item{
		{Type: "file", Path: "/dl/a.mkv", Size: 10, Nlink: 2, Linked: true, Links: []string{"/media/a.mkv"}, Tags: []string{"linked"}},
		{Type: "file", Path: "/dl/b.mkv", Size: 20, Nlink: 1, Tags: []string{"unlinked", "dup"}, Dups: []string{"/media/b copy.mkv"}},
		{Type: "file", Path: "/dl/orphan.mkv", Size: 5, Nlink: 1, Tags: []string{"unlinked"}},
		{Type: "file", Path: "/other/x.mkv", Size: 7, Nlink: 1, Tags: []string{"unlinked"}},
		{Type: "symlink", Path: "/dl/l", Tags: []string{"symlink"}},
	}}
}

func TestAnnotate(t *testing.T) {
	r := sample()
	snap := &torrents.Snapshot{FetchedAt: 1, Clients: []torrents.ClientResult{{
		Name: "qb",
		Torrents: []torrents.Torrent{
			{Hash: "h1", SaveDir: "/dl", Files: []string{"/dl/a.mkv"}, State: "seeding"},
			{Hash: "h2", SaveDir: "/dl", Files: []string{"/dl/b.mkv"}, State: "paused", Unregistered: true},
		},
		Sites: []string{"天空 HDSky", "馒头 M-Team"},
	}}}
	annotate(r, snap)
	ts := r.Summary.Torrents
	if ts.Matched != 2 || ts.Seeding != 1 || ts.Unregistered != 1 || ts.Orphans != 1 || ts.OrphanSize != 5 {
		t.Errorf("汇总错误: %+v", ts)
	}
	if len(ts.Sites) != 2 || ts.Sites[0].Site != "馒头 M-Team" {
		t.Errorf("站点统计应按大小排序: %+v", ts.Sites)
	}
	if !hasTag(r.Items[0].Tags, "seeding") || !hasTag(r.Items[1].Tags, "unregistered") || !hasTag(r.Items[2].Tags, "orphan") {
		t.Errorf("标签错误: %+v", r.Items)
	}
	if hasTag(r.Items[3].Tags, "orphan") {
		t.Error("不在下载目录中的文件不应标为孤儿")
	}
	// 重复标注不应累积标签
	annotate(r, snap)
	n := 0
	for _, tg := range r.Items[0].Tags {
		if tg == "seeding" {
			n++
		}
	}
	if n != 1 || len(r.Items[0].Torrents) != 1 {
		t.Errorf("重复标注出错: %+v", r.Items[0])
	}
}

func TestRemovePaths(t *testing.T) {
	r := sample()
	removePaths(r, []string{"/media/a.mkv", "/media/b copy.mkv", "/dl/orphan.mkv"})
	if len(r.Items) != 4 {
		t.Fatalf("条目数 %d", len(r.Items))
	}
	a := r.Items[0]
	if a.Linked || a.Nlink != 1 || len(a.Links) != 0 || !hasTag(a.Tags, "unlinked") {
		t.Errorf("删除硬链接后应变为未链接: %+v", a)
	}
	if b := r.Items[1]; hasTag(b.Tags, "dup") || len(b.Dups) != 0 {
		t.Errorf("副本删除后不应再标为重复: %+v", b)
	}
	if !r.Summary.Stale || r.Summary.Linked != 0 || r.Summary.Total != 3 {
		t.Errorf("汇总错误: %+v", r.Summary)
	}
	// 删除文件夹
	removePaths(r, []string{"/dl"})
	if len(r.Items) != 1 || r.Items[0].Path != "/other/x.mkv" {
		t.Errorf("删除文件夹后结果错误: %+v", r.Items)
	}
}
