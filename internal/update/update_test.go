package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/traceonx/link-scope/internal/version"
)

func TestHelpers(t *testing.T) {
	cases := []struct{ img, repo, tag, hub string }{
		{"sunqz/link-scope:latest", "sunqz/link-scope", "latest", "sunqz/link-scope"},
		{"docker.io/sunqz/link-scope:0.2.0", "docker.io/sunqz/link-scope", "0.2.0", "sunqz/link-scope"},
		{"sunqz/link-scope", "sunqz/link-scope", "latest", "sunqz/link-scope"},
		{"registry:5000/a/b@sha256:abc", "registry:5000/a/b", "latest", "registry:5000/a/b"},
		{"alpine:3.20", "alpine", "3.20", "library/alpine"},
	}
	for _, c := range cases {
		r, tg := splitImage(c.img)
		if r != c.repo || tg != c.tag {
			t.Errorf("splitImage(%q)=%q,%q", c.img, r, tg)
		}
		if h := hubRepo(c.img); h != c.hub {
			t.Errorf("hubRepo(%q)=%q", c.img, h)
		}
	}
	if !newer("0.10.0", "0.9.9") || newer("0.2.0", "0.2.0") || !newer("1.0.0", "0.99.0") || newer("0.1.0", "0.1.1") {
		t.Error("版本比较错误")
	}
}

func TestStatusWithoutSocket(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{
			{"name": "latest"}, {"name": "0.3.0"}, {"name": "0.10.1"}, {"name": "sha-abc"},
		}})
	}))
	defer hub.Close()
	t.Setenv("UPDATE_DOCKER_SOCKET", filepath.Join(t.TempDir(), "none.sock"))
	t.Setenv("UPDATE_HUB_API", hub.URL)
	version.Version = "0.2.0"
	m := New(t.TempDir())
	s := m.Status(t.Context())
	if s.Docker.Available || s.Docker.Reason == "" {
		t.Errorf("没有 socket 时应不可用: %+v", s.Docker)
	}
	if s.Latest != "0.10.1" || !s.HasUpdate {
		t.Errorf("最新版本识别错误: %+v", s)
	}
	if _, err := m.Start(t.Context()); err == nil {
		t.Error("不可用时 Start 应报错")
	}
}

func TestAutoSettingsAndRestartState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("UPDATE_DOCKER_SOCKET", filepath.Join(dir, "none.sock"))
	m := New(dir)
	if _, err := m.SetAuto(true, "25:00"); err == nil {
		t.Error("非法时间应报错")
	}
	a, err := m.SetAuto(true, "03:30")
	if err != nil || !a.Enabled || a.Next == "" {
		t.Fatalf("保存自动更新失败: %+v %v", a, err)
	}
	if m2 := New(dir); !m2.autoConfig().Enabled || m2.autoConfig().Time != "03:30" {
		t.Error("自动更新设置未持久化")
	}

	// 模拟：更新过程中进程被替换，新版本启动后应判定成功
	b, _ := json.Marshal(State{Running: true, Phase: "restarting", FromVersion: "0.1.0", StartedAt: now()})
	_ = os.WriteFile(filepath.Join(dir, "update-state.json"), b, 0o644)
	version.Version = "0.2.0"
	m3 := New(dir)
	if m3.state.Running || m3.state.Success == nil || !*m3.state.Success {
		t.Errorf("重启后应判定更新成功: %+v", m3.state)
	}

	b, _ = json.Marshal(State{Running: true, Phase: "pull", FromVersion: "0.2.0", StartedAt: now()})
	_ = os.WriteFile(filepath.Join(dir, "update-state.json"), b, 0o644)
	m4 := New(dir)
	if m4.state.Success == nil || *m4.state.Success {
		t.Errorf("下载中被中断应判定失败: %+v", m4.state)
	}
}
