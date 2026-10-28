package update

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// dockerClient 通过 unix socket 调用 Docker Engine API。
type dockerClient struct {
	socket string
	http   *http.Client
	prefix string // 例如 /v1.45
	arch   string
}

func newDockerClient(socket string) *dockerClient {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}
	return &dockerClient{socket: socket, http: &http.Client{Transport: tr}}
}

func (d *dockerClient) do(ctx context.Context, method, path string, body any, timeout time.Duration) (int, []byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, rd)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}

// api 调用带版本前缀的接口；首次调用时读取守护进程的 API 版本与架构。
func (d *dockerClient) api(ctx context.Context, method, path string, body any, timeout time.Duration) (int, []byte, error) {
	if d.prefix == "" {
		code, b, err := d.do(ctx, "GET", "/version", nil, 10*time.Second)
		if err != nil {
			return 0, nil, err
		}
		var v struct{ ApiVersion, Arch string }
		if code != 200 || json.Unmarshal(b, &v) != nil || v.ApiVersion == "" {
			return 0, nil, fmt.Errorf("无法读取 Docker 版本：HTTP %d", code)
		}
		d.prefix, d.arch = "/v"+v.ApiVersion, v.Arch
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return d.do(ctx, method, d.prefix+path, body, timeout)
}

// stream 发起流式请求，逐行回调 JSON；idle 时间内没有任何数据视为卡住。
func (d *dockerClient) stream(ctx context.Context, method, path string, idle time.Duration, onLine func(map[string]any)) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+d.prefix+path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	timer := time.AfterFunc(idle, cancel)
	defer timer.Stop()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		timer.Reset(idle)
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			onLine(m)
		}
	}
	if err := sc.Err(); err != nil {
		if ctx.Err() != nil {
			return resp.StatusCode, fmt.Errorf("长时间没有下载进度，请检查 NAS 的网络或 Docker 镜像加速配置")
		}
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

type containerInfo struct {
	Name  string
	Image string // 容器配置中的镜像名（如 sunqz/link-scope:latest）
	ID    string // 当前使用的镜像 ID
}

func (d *dockerClient) inspect(ctx context.Context, ref string) (*containerInfo, error) {
	code, b, err := d.api(ctx, "GET", "/containers/"+url.PathEscape(ref)+"/json", nil, 0)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, nil
	}
	var v struct {
		Name   string
		Image  string
		Config struct{ Image string }
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &containerInfo{Name: strings.TrimPrefix(v.Name, "/"), Image: v.Config.Image, ID: v.Image}, nil
}

func (d *dockerClient) imageInfo(ctx context.Context, ref string) (id string, repoDigests []string, ok bool, err error) {
	code, b, err := d.api(ctx, "GET", "/images/"+url.PathEscape(ref)+"/json", nil, 0)
	if err != nil {
		return "", nil, false, err
	}
	if code != 200 {
		return "", nil, false, nil
	}
	var v struct {
		ID          string `json:"Id"`
		RepoDigests []string
	}
	err = json.Unmarshal(b, &v)
	return v.ID, v.RepoDigests, true, err
}

// splitImage 拆分镜像名为仓库和标签。
func splitImage(ref string) (repo, tag string) {
	ref = strings.SplitN(ref, "@", 2)[0]
	i := strings.LastIndex(ref, ":")
	if i > strings.LastIndex(ref, "/") {
		return ref[:i], ref[i+1:]
	}
	return ref, "latest"
}

// hubRepo 返回 Docker Hub 上的 namespace/name。
func hubRepo(image string) string {
	repo, _ := splitImage(image)
	repo = strings.TrimPrefix(repo, "docker.io/")
	repo = strings.TrimPrefix(repo, "index.docker.io/")
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	return repo
}
