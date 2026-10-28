package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(r *Result, name string) *Item {
	for i := range r.Items {
		if r.Items[i].Name == name {
			return &r.Items[i]
		}
	}
	return nil
}

func has(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func setup(t *testing.T) (src, media string) {
	root := t.TempDir()
	src = filepath.Join(root, "Downloads")
	media = filepath.Join(root, "Media")
	big := strings.Repeat("x", 4<<20) + "A"

	write(t, filepath.Join(src, "movies", "linked.mkv"), "linked-content")
	write(t, filepath.Join(src, "movies", "alone.mkv"), "alone-content")
	write(t, filepath.Join(src, "movies", "copied.mkv"), big)
	write(t, filepath.Join(src, "movies", "samesize.mkv"), strings.Repeat("x", 4<<20)+"B")
	write(t, filepath.Join(src, "movies", "tiny.nfo"), "n")
	write(t, filepath.Join(media, "电影", "copied (copy).mkv"), big)
	if err := os.Link(filepath.Join(src, "movies", "linked.mkv"), filepath.Join(media, "电影", "linked.mkv")); err != nil {
		t.Fatal(err)
	}
	// 软链接：一个有效，一个失效
	if err := os.Symlink(filepath.Join(src, "movies", "alone.mkv"), filepath.Join(src, "good.lnk")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nonexistent/file.mkv", filepath.Join(src, "bad.lnk")); err != nil {
		t.Fatal(err)
	}
	// STRM：本地有效 / 本地失效 / 远程
	write(t, filepath.Join(src, "ok.strm"), filepath.Join(src, "movies", "alone.mkv")+"\n")
	write(t, filepath.Join(src, "gone.strm"), "/nonexistent/x.mkv")
	write(t, filepath.Join(src, "remote.strm"), "# comment\nhttps://example.com/a.mkv")
	return src, media
}

func TestScan(t *testing.T) {
	src, media := setup(t)
	r, err := Scan(context.Background(), Options{
		Source: src, Targets: []string{media}, Recursive: true, MinSize: 2, DupMode: DupQuick,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary

	if find(r, "tiny.nfo") != nil {
		t.Error("小于 MinSize 的文件不应出现在结果中")
	}
	if s.Total != 4 || s.Linked != 1 || s.Unlinked != 3 {
		t.Errorf("统计错误: total=%d linked=%d unlinked=%d", s.Total, s.Linked, s.Unlinked)
	}

	l := find(r, "linked.mkv")
	if l == nil || !l.Linked || l.Nlink != 2 || len(l.Links) != 1 || !strings.HasSuffix(l.Links[0], "电影/linked.mkv") {
		t.Errorf("硬链接未正确识别: %+v", l)
	}
	if l.Missing == nil || *l.Missing != 0 {
		t.Errorf("missing 应为 0: %+v", l.Missing)
	}
	if s.Saved != int64(len("linked-content")) {
		t.Errorf("saved=%d", s.Saved)
	}

	c := find(r, "copied.mkv")
	if c == nil || c.Linked || !has(c.Tags, "dup") || len(c.Dups) != 1 {
		t.Errorf("重复副本未识别: %+v", c)
	}
	if ss := find(r, "samesize.mkv"); ss == nil || has(ss.Tags, "dup") {
		t.Error("大小相同但内容不同的文件不应判为重复")
	}
	if s.DupGroups != 1 || s.DupWasted != int64(4<<20+1) {
		t.Errorf("dup groups=%d wasted=%d", s.DupGroups, s.DupWasted)
	}

	if g := find(r, "good.lnk"); g == nil || g.Type != "symlink" || g.Broken || !strings.HasSuffix(g.Resolved, "alone.mkv") {
		t.Errorf("有效软链接: %+v", g)
	}
	if b := find(r, "bad.lnk"); b == nil || !b.Broken || !has(b.Tags, "broken") {
		t.Errorf("失效软链接: %+v", b)
	}
	if s.Symlinks != 2 || s.BrokenLinks != 1 {
		t.Errorf("symlinks=%d broken=%d", s.Symlinks, s.BrokenLinks)
	}

	if x := find(r, "ok.strm"); x == nil || x.Broken {
		t.Errorf("有效 STRM: %+v", x)
	}
	if x := find(r, "gone.strm"); x == nil || !x.Broken {
		t.Errorf("失效 STRM: %+v", x)
	}
	if x := find(r, "remote.strm"); x == nil || !x.Remote || x.Broken || x.Target != "https://example.com/a.mkv" {
		t.Errorf("远程 STRM: %+v", x)
	}
	if s.Strm != 3 || s.BrokenStrm != 1 {
		t.Errorf("strm=%d broken=%d", s.Strm, s.BrokenStrm)
	}
}

func TestScanFullAndOff(t *testing.T) {
	src, media := setup(t)
	full, err := Scan(context.Background(), Options{Source: src, Targets: []string{media}, Recursive: true, DupMode: DupFull})
	if err != nil {
		t.Fatal(err)
	}
	if full.Summary.DupGroups != 1 {
		t.Errorf("full 模式 dup groups=%d", full.Summary.DupGroups)
	}
	off, err := Scan(context.Background(), Options{Source: src, Targets: []string{media}, Recursive: true, DupMode: DupOff})
	if err != nil {
		t.Fatal(err)
	}
	if off.Summary.DupGroups != 0 {
		t.Error("off 模式不应检测重复")
	}
}

func TestNonRecursiveAndNoTargets(t *testing.T) {
	src, _ := setup(t)
	r, err := Scan(context.Background(), Options{Source: src, Recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	if find(r, "linked.mkv") != nil {
		t.Error("不递归时不应包含子文件夹中的文件")
	}
	if find(r, "ok.strm") == nil {
		t.Error("应包含顶层的 .strm")
	}
}

func TestCancel(t *testing.T) {
	src, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, Options{Source: src, Recursive: true}); err == nil {
		t.Error("取消后应返回错误")
	}
}

func TestMissingSource(t *testing.T) {
	if _, err := Scan(context.Background(), Options{Source: "/nonexistent-dir-xyz"}); err == nil {
		t.Error("源目录不存在时应报错")
	}
}
