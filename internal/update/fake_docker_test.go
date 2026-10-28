package update

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/traceonx/link-scope/internal/version"
)

// fakeDocker 模拟 Docker Engine API 的最小子集。
type fakeDocker struct {
	mu         sync.Mutex
	newImageID string // 拉取后镜像的 ID
	curImageID string // 当前容器使用的镜像 ID
	created    map[string]any
	started    bool
	haveWT     bool
}

func (f *fakeDocker) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.45", "Arch": "amd64"})
	})
	mux.HandleFunc("/v1.45/containers/linkscope/json", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Name": "/linkscope", "Image": f.curImageID, "Config": map[string]string{"Image": "sunqz/link-scope:latest"},
		})
	})
	mux.HandleFunc("/v1.45/images/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if strings.Contains(r.URL.Path, "watchtower") && !f.haveWT {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Id": f.newImageID, "RepoDigests": []string{"sunqz/link-scope@sha256:old"}})
	})
	mux.HandleFunc("POST /v1.45/images/create", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("fromImage"), "watchtower") {
			f.mu.Lock()
			f.haveWT = true
			f.mu.Unlock()
		}
		fl := w.(http.Flusher)
		for _, l := range []string{
			`{"status":"Pulling fs layer","id":"a"}`,
			`{"status":"Downloading","id":"a","progressDetail":{"current":50,"total":100}}`,
			`{"status":"Download complete","id":"a"}`,
			`{"status":"Pull complete","id":"a"}`,
		} {
			fmt.Fprintln(w, l)
			fl.Flush()
		}
	})
	mux.HandleFunc("POST /v1.45/containers/create", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.created = body
		f.mu.Unlock()
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]string{"Id": "updater1"})
	})
	mux.HandleFunc("POST /v1.45/containers/updater1/start", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.started = true
		f.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/v1.45/containers/updater1/logs", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "time=x level=info msg=\"Session done\" Failed=0 Scanned=1 Updated=1 notify=no\n")
	})
	return mux
}

func startFake(t *testing.T, f *fakeDocker) string {
	sock := filepath.Join(t.TempDir(), "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("无法创建 unix socket:", err)
	}
	srv := &http.Server{Handler: f.handler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return sock
}

func waitDone(t *testing.T, m *Manager) State {
	for i := 0; i < 200; i++ {
		m.mu.Lock()
		s := m.state
		m.mu.Unlock()
		if !s.Running {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("更新未结束")
	return State{}
}

func TestUpdateFlow(t *testing.T) {
	f := &fakeDocker{newImageID: "sha256:new", curImageID: "sha256:old"}
	t.Setenv("UPDATE_DOCKER_SOCKET", startFake(t, f))
	t.Setenv("UPDATE_CONTAINER", "linkscope")
	version.Version = "0.2.0"
	m := New(t.TempDir())

	ds, _ := m.dockerStatus(t.Context())
	if !ds.Available || ds.Name != "linkscope" {
		t.Fatalf("应可用: %+v", ds)
	}
	if _, err := m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	s := waitDone(t, m)
	if s.Success == nil || !*s.Success || !strings.Contains(s.Message, "容器已重建") {
		t.Fatalf("更新结果错误: %+v", s)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.started || !f.haveWT {
		t.Error("应拉取 watchtower 并启动更新容器")
	}
	cmd, _ := json.Marshal(f.created["Cmd"])
	if !strings.Contains(string(cmd), `"--no-pull","linkscope"`) {
		t.Errorf("watchtower 参数错误: %s", cmd)
	}
}

func TestUpdateAlreadyLatest(t *testing.T) {
	f := &fakeDocker{newImageID: "sha256:same", curImageID: "sha256:same"}
	t.Setenv("UPDATE_DOCKER_SOCKET", startFake(t, f))
	t.Setenv("UPDATE_CONTAINER", "linkscope")
	m := New(t.TempDir())
	if _, err := m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	s := waitDone(t, m)
	if s.Success == nil || !*s.Success || !strings.Contains(s.Message, "已是最新") {
		t.Fatalf("应提示已是最新: %+v", s)
	}
	if f.started {
		t.Error("镜像未变化时不应重建容器")
	}
}

func TestPinnedTagUnavailable(t *testing.T) {
	f := &fakeDocker{}
	sock := startFake(t, f)
	t.Setenv("UPDATE_DOCKER_SOCKET", sock)
	t.Setenv("UPDATE_CONTAINER", "linkscope")
	m := New(t.TempDir())
	// 改写镜像为固定版本标签
	c, _ := m.docker.inspect(t.Context(), "linkscope")
	if c == nil {
		t.Fatal("inspect 失败")
	}
	if _, tag := splitImage("sunqz/link-scope:0.2.0"); !versionTagRe.MatchString(tag) {
		t.Error("版本标签识别错误")
	}
}
