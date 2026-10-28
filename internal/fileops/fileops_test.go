package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (string, *Manager) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "dl", "show"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "media"), 0o755))
	must(os.WriteFile(filepath.Join(root, "dl", "a.mkv"), []byte("aaaa"), 0o644))
	must(os.WriteFile(filepath.Join(root, "dl", "b.mkv"), []byte("bbbbbb"), 0o644))
	must(os.Link(filepath.Join(root, "dl", "a.mkv"), filepath.Join(root, "media", "a.mkv")))
	must(os.WriteFile(filepath.Join(root, "dl", "show", "e1.mkv"), []byte("123"), 0o644))
	must(os.Symlink("/nonexistent", filepath.Join(root, "dl", "bad.lnk")))
	return root, New([]string{root}, filepath.Join(root, "data"), 30, false)
}

func TestCapability(t *testing.T) {
	if ok, _ := New(nil, t.TempDir(), 30, false).Capability(); ok {
		t.Error("没有 LS_ROOTS 时不应允许删除")
	}
	if ok, _ := New([]string{t.TempDir()}, t.TempDir(), 30, true).Capability(); ok {
		t.Error("LS_DELETE=false 时不应允许删除")
	}
}

func TestPreviewFreed(t *testing.T) {
	root, m := setup(t)
	pv, err := m.Preview([]string{filepath.Join(root, "dl", "a.mkv"), filepath.Join(root, "dl", "b.mkv")})
	if err != nil {
		t.Fatal(err)
	}
	// a.mkv 在 media 中还有硬链接，不释放空间；b.mkv 释放 6 字节
	if pv.Freed != 6 || pv.Kept != 1 || pv.TotalSize != 10 || pv.Files != 2 {
		t.Errorf("预览错误: %+v", pv)
	}
	if pv.Items[0].Freed || !pv.Items[1].Freed {
		t.Errorf("单项释放标记错误: %+v", pv.Items)
	}
	// 两个硬链接都选中时释放空间
	pv, _ = m.Preview([]string{filepath.Join(root, "dl", "a.mkv"), filepath.Join(root, "media", "a.mkv")})
	if pv.Freed != 4 || pv.Kept != 0 {
		t.Errorf("同时删除所有链接应释放空间: %+v", pv)
	}
	// 文件夹与其中的文件同时选中时去重
	pv, _ = m.Preview([]string{filepath.Join(root, "dl"), filepath.Join(root, "dl", "b.mkv")})
	if pv.Count != 1 || pv.Dirs != 1 || pv.Files != 3 {
		t.Errorf("文件夹预览错误: %+v", pv)
	}
}

func TestGuards(t *testing.T) {
	root, m := setup(t)
	for _, p := range []string{root, "/etc/passwd", "relative", filepath.Join(root, TrashDir)} {
		if _, _, err := m.resolve(p); err == nil {
			t.Errorf("应拒绝 %s", p)
		}
	}
}

func TestTrashRestorePurge(t *testing.T) {
	root, m := setup(t)
	b := filepath.Join(root, "dl", "b.mkv")
	show := filepath.Join(root, "dl", "show")
	res, done, err := m.Delete([]string{b, show, filepath.Join(root, "dl", "bad.lnk")}, false)
	if err != nil || len(done) != 3 {
		t.Fatalf("移到回收站失败: %+v %v", res, err)
	}
	if _, err := os.Lstat(b); !os.IsNotExist(err) {
		t.Error("原文件应已移走")
	}
	tr := m.Trash()
	if len(tr) != 3 {
		t.Fatalf("回收站条目数 %d", len(tr))
	}
	var bID string
	for _, e := range tr {
		if e.Path == b {
			bID = e.ID
		}
		if e.Path == show && (e.Kind != "dir" || e.Files != 1) {
			t.Errorf("文件夹条目错误: %+v", e)
		}
	}
	rr, restored := m.Restore([]string{bID})
	if len(restored) != 1 || !rr[0].OK {
		t.Fatalf("恢复失败: %+v", rr)
	}
	if data, _ := os.ReadFile(b); string(data) != "bbbbbb" {
		t.Error("恢复后内容不对")
	}
	// 原位置已存在时恢复失败
	_, _, _ = m.Delete([]string{b}, false)
	_ = os.WriteFile(b, []byte("new"), 0o644)
	for _, e := range m.Trash() {
		if e.Path == b {
			if rr, _ := m.Restore([]string{e.ID}); rr[0].OK {
				t.Error("原位置已有文件时不应覆盖")
			}
		}
	}
	m.Purge(nil, true)
	if n := len(m.Trash()); n != 0 {
		t.Errorf("清空后仍有 %d 项", n)
	}
	if log, _ := os.ReadFile(filepath.Join(root, "data", "delete.log")); !strings.Contains(string(log), "移到回收站") {
		t.Error("应写入操作日志")
	}
}

func TestPermanentAndExpire(t *testing.T) {
	root, m := setup(t)
	a := filepath.Join(root, "dl", "a.mkv")
	if _, done, _ := m.Delete([]string{a}, true); len(done) != 1 {
		t.Fatal("永久删除失败")
	}
	if _, err := os.Lstat(a); !os.IsNotExist(err) {
		t.Error("文件应已删除")
	}
	if _, err := os.Stat(filepath.Join(root, "media", "a.mkv")); err != nil {
		t.Error("另一个硬链接不应受影响")
	}
	_, _, _ = m.Delete([]string{filepath.Join(root, "dl", "b.mkv")}, false)
	m.retention = time.Nanosecond
	time.Sleep(1100 * time.Millisecond) // deleted_at 精度为秒
	if n := m.CleanExpired(); n != 1 {
		t.Errorf("过期清理数量 %d", n)
	}
}
