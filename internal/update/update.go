// Package update 实现一键更新（参考 shop-cashback）：
// 容器挂载宿主机 /var/run/docker.sock 后，通过 Docker Engine API 拉取当前标签的新镜像，
// 再临时启动 watchtower（--run-once --no-pull）按原配置重建本容器。
// 本进程会在更新过程中被替换，因此状态写入 DATA_DIR/update-state.json，重启后根据版本号判断结果。
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/traceonx/link-scope/internal/version"
)

const defaultImage = "sunqz/link-scope:latest"

var (
	ErrBusy      = errors.New("更新正在进行中")
	semverRe     = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	mountIDRe    = regexp.MustCompile(`/containers/([0-9a-f]{64})/`)
	ansiRe       = regexp.MustCompile(`\x1B\[[0-?]*[ -/]*[@-~]`)
	updatedRe    = regexp.MustCompile(`Updated=(\d+)`)
	failedRe     = regexp.MustCompile(`Failed=(\d+)`)
	timeOfDayRe  = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	versionTagRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)
)

type Progress struct {
	Percent int   `json:"percent"`
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
	Layers  int   `json:"layers"`
	Done    int   `json:"done"`
}

type State struct {
	Running     bool      `json:"running"`
	Phase       string    `json:"phase,omitempty"` // prepare | pull | recreate | restarting
	Progress    *Progress `json:"progress,omitempty"`
	FromVersion string    `json:"fromVersion,omitempty"`
	StartedAt   string    `json:"startedAt,omitempty"`
	UpdatedAt   string    `json:"updatedAt,omitempty"`
	FinishedAt  string    `json:"finishedAt,omitempty"`
	Success     *bool     `json:"success"`
	Message     string    `json:"message,omitempty"`
	Log         string    `json:"log,omitempty"`
}

// AutoUpdate 自动更新设置：每天在 Time（HH:MM）检查一次。
type AutoUpdate struct {
	Enabled bool   `json:"enabled"`
	Time    string `json:"time"`
	Next    string `json:"next,omitempty"`
}

type DockerStatus struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
	Name      string `json:"name,omitempty"`
	Image     string `json:"image,omitempty"`
}

type Status struct {
	Current      version.Info `json:"current"`
	Latest       string       `json:"latest"`
	LatestError  string       `json:"latestError,omitempty"`
	ImageChanged *bool        `json:"imageChanged"`
	ImageError   string       `json:"imageError,omitempty"`
	HasUpdate    bool         `json:"hasUpdate"`
	AutoUpdate   AutoUpdate   `json:"autoUpdate"`
	Docker       DockerStatus `json:"docker"`
	State        State        `json:"state"`
}

type cached[T any] struct {
	at    time.Time
	key   string
	value T
	err   string
}

type Manager struct {
	mu         sync.Mutex
	dataDir    string
	docker     *dockerClient
	watchtower string
	disabled   bool
	container  string // UPDATE_CONTAINER
	hubAPI     string
	web        *http.Client // 访问 Docker Hub（可走 PROXY_HOST）
	state      State
	lastWrite  time.Time
	auto       AutoUpdate
	latest     *cached[string]
	digest     *cached[*bool]
}

func env(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

// New 创建更新管理器，并根据上次保存的状态判断更新结果。
func New(dataDir string) *Manager {
	m := &Manager{
		dataDir:    dataDir,
		docker:     newDockerClient(env("UPDATE_DOCKER_SOCKET", "/var/run/docker.sock")),
		watchtower: env("UPDATE_WATCHTOWER_IMAGE", "containrrr/watchtower:latest"),
		disabled:   os.Getenv("UPDATE_ENABLED") == "false",
		container:  os.Getenv("UPDATE_CONTAINER"),
		hubAPI:     strings.TrimRight(env("UPDATE_HUB_API", "https://hub.docker.com"), "/"),
		auto:       AutoUpdate{Time: "04:00"},
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if p := os.Getenv("PROXY_HOST"); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	m.web = &http.Client{Transport: tr, Timeout: 10 * time.Second}
	m.loadState()
	m.loadAuto()
	return m
}

func (m *Manager) statePath() string { return filepath.Join(m.dataDir, "update-state.json") }
func (m *Manager) autoPath() string  { return filepath.Join(m.dataDir, "update-settings.json") }
func now() string                    { return time.Now().Format(time.RFC3339) }
func boolp(b bool) *bool             { return &b }

func (m *Manager) loadState() {
	b, err := os.ReadFile(m.statePath())
	if err != nil {
		return
	}
	var s State
	if json.Unmarshal(b, &s) != nil {
		return
	}
	m.state = s
	if !s.Running {
		return
	}
	s.Running, s.Phase, s.Progress, s.FinishedAt = false, "", nil, now()
	started, _ := time.Parse(time.RFC3339, s.StartedAt)
	switch {
	case s.FromVersion != "" && s.FromVersion != version.Version:
		s.Success, s.Message = boolp(true), fmt.Sprintf("已从 v%s 更新到 v%s", s.FromVersion, version.Version)
	case m.state.Phase == "restarting":
		s.Success, s.Message = boolp(true), "已使用新镜像重启（当前 v"+version.Version+"）"
	case m.state.Phase == "pull" || m.state.Phase == "prepare":
		s.Success, s.Message = boolp(false), "更新被中断（服务在下载镜像时重启），请重试"
	case time.Since(started) > 15*time.Minute:
		s.Success, s.Message = boolp(false), "更新未完成：服务已重启但版本未变化，请查看 watchtower 日志"
	default:
		return
	}
	m.persistLocked(s, false)
}

// persistLocked 更新状态并写盘；调用方需持有 m.mu（或处于初始化阶段）。
func (m *Manager) persistLocked(s State, throttle bool) {
	if s.Message != "" && s.Message != m.state.Message {
		log.Printf("[一键更新] %s", s.Message)
	}
	s.UpdatedAt = now()
	m.state = s
	if throttle && time.Since(m.lastWrite) < time.Second {
		return
	}
	m.lastWrite = time.Now()
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		return
	}
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		_ = os.WriteFile(m.statePath(), b, 0o644)
	}
}

func (m *Manager) update(fn func(*State), throttle bool) {
	m.mu.Lock()
	s := m.state
	fn(&s)
	m.persistLocked(s, throttle)
	m.mu.Unlock()
}

func (m *Manager) finish(ok bool, msg string) {
	m.update(func(s *State) {
		s.Running, s.Phase, s.Progress, s.FinishedAt, s.Success, s.Message = false, "", nil, now(), boolp(ok), msg
	}, false)
}

// ---- 自动更新设置 ----

func (m *Manager) loadAuto() {
	if b, err := os.ReadFile(m.autoPath()); err == nil {
		var a AutoUpdate
		if json.Unmarshal(b, &a) == nil && timeOfDayRe.MatchString(a.Time) {
			m.auto = a
		}
	}
}

func nextRun(t string) string {
	h, _ := strconv.Atoi(t[:2])
	mi, _ := strconv.Atoi(t[3:])
	n := time.Now()
	next := time.Date(n.Year(), n.Month(), n.Day(), h, mi, 0, 0, n.Location())
	if !next.After(n) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Format(time.RFC3339)
}

func (m *Manager) autoConfig() AutoUpdate {
	m.mu.Lock()
	a := m.auto
	m.mu.Unlock()
	if a.Enabled {
		a.Next = nextRun(a.Time)
	}
	return a
}

// SetAuto 保存自动更新设置。
func (m *Manager) SetAuto(enabled bool, t string) (AutoUpdate, error) {
	if !timeOfDayRe.MatchString(t) {
		return AutoUpdate{}, fmt.Errorf("时间格式应为 HH:MM，例如 04:00")
	}
	m.mu.Lock()
	m.auto = AutoUpdate{Enabled: enabled, Time: t}
	b, _ := json.Marshal(m.auto)
	m.mu.Unlock()
	_ = os.MkdirAll(m.dataDir, 0o755)
	if err := os.WriteFile(m.autoPath(), b, 0o644); err != nil {
		return AutoUpdate{}, err
	}
	if enabled {
		log.Printf("[一键更新] 自动更新已开启（每天 %s）", t)
	} else {
		log.Printf("[一键更新] 自动更新已关闭")
	}
	return m.autoConfig(), nil
}

// RunAutoUpdate 每分钟检查一次，到点且有新版本时自动更新；上次失败后 6 小时内不重试。
func (m *Manager) RunAutoUpdate(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-t.C:
			a := m.autoConfig()
			if !a.Enabled || n.Format("15:04") != a.Time {
				continue
			}
			m.mu.Lock()
			s := m.state
			m.latest, m.digest = nil, nil
			m.mu.Unlock()
			if s.Running {
				continue
			}
			if fin, err := time.Parse(time.RFC3339, s.FinishedAt); err == nil && s.Success != nil && !*s.Success &&
				time.Since(fin) < 6*time.Hour {
				continue
			}
			st := m.Status(ctx)
			if !st.HasUpdate || !st.Docker.Available {
				log.Printf("[一键更新] 自动更新检查：已是最新版本")
				continue
			}
			log.Printf("[一键更新] 自动更新：发现新版本，开始更新")
			_, _ = m.Start(ctx)
		}
	}
}

// ---- 状态查询 ----

func (m *Manager) self(ctx context.Context) (*containerInfo, error) {
	if m.container != "" {
		c, err := m.docker.inspect(ctx, m.container)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, fmt.Errorf("找不到 UPDATE_CONTAINER 指定的容器 %s", m.container)
		}
		return c, nil
	}
	var refs []string
	if b, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		if mm := mountIDRe.FindSubmatch(b); mm != nil {
			refs = append(refs, string(mm[1]))
		}
	}
	if h, err := os.Hostname(); err == nil {
		refs = append(refs, h)
	}
	for _, r := range refs {
		c, err := m.docker.inspect(ctx, r)
		if err != nil {
			return nil, err
		}
		if c != nil {
			return c, nil
		}
	}
	code, b, err := m.docker.api(ctx, "GET", "/containers/json", nil, 0)
	if err != nil {
		return nil, err
	}
	var list []struct {
		Names []string
		Image string
	}
	if code == 200 {
		_ = json.Unmarshal(b, &list)
	}
	var mine []string
	for _, c := range list {
		if strings.Contains(c.Image, "link-scope") && len(c.Names) > 0 {
			mine = append(mine, c.Names[0])
		}
	}
	if len(mine) == 1 {
		return m.docker.inspect(ctx, strings.TrimPrefix(mine[0], "/"))
	}
	return nil, fmt.Errorf("无法识别当前容器，请在 compose 中设置 UPDATE_CONTAINER 为容器名")
}

func (m *Manager) dockerStatus(ctx context.Context) (DockerStatus, *containerInfo) {
	if m.disabled {
		return DockerStatus{Reason: "已通过 UPDATE_ENABLED=false 关闭一键更新"}, nil
	}
	if _, err := os.Stat(m.docker.socket); err != nil {
		return DockerStatus{Reason: "未挂载 Docker socket（" + m.docker.socket +
			"），请在 compose 中加入 - /var/run/docker.sock:/var/run/docker.sock"}, nil
	}
	c, err := m.self(ctx)
	if err != nil {
		reason := err.Error()
		if errors.Is(err, os.ErrPermission) || strings.Contains(reason, "permission denied") {
			reason = "无权访问 Docker socket"
		}
		return DockerStatus{Reason: reason}, nil
	}
	_, tag := splitImage(c.Image)
	if versionTagRe.MatchString(tag) {
		return DockerStatus{Name: c.Name, Image: c.Image,
			Reason: "当前镜像固定为版本标签 " + c.Image + "，一键更新需使用 latest 标签；也可修改 compose 中的版本号后重新部署"}, c
	}
	return DockerStatus{Available: true, Name: c.Name, Image: c.Image}, c
}

func newer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(strings.SplitN(pb[i], "-", 2)[0])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func (m *Manager) latestVersion(ctx context.Context, repo string) (string, string) {
	m.mu.Lock()
	c := m.latest
	m.mu.Unlock()
	if c != nil && c.key == repo && time.Since(c.at) < 10*time.Minute {
		return c.value, c.err
	}
	nc := &cached[string]{at: time.Now(), key: repo}
	ns, name, _ := strings.Cut(repo, "/")
	u := fmt.Sprintf("%s/v2/namespaces/%s/repositories/%s/tags?page_size=100&ordering=last_updated", m.hubAPI, ns, name)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := m.web.Do(req)
	if err == nil {
		defer resp.Body.Close()
		var v struct {
			Results []struct{ Name string } `json:"results"`
		}
		if resp.StatusCode != 200 {
			err = fmt.Errorf("Docker Hub HTTP %d", resp.StatusCode)
		} else if err = json.NewDecoder(resp.Body).Decode(&v); err == nil {
			for _, t := range v.Results {
				if semverRe.MatchString(t.Name) && (nc.value == "" || newer(t.Name, nc.value)) {
					nc.value = t.Name
				}
			}
		}
	}
	if err != nil {
		nc.err = "无法访问 Docker Hub（" + err.Error() + "），可设置 PROXY_HOST 或 UPDATE_HUB_API"
	}
	m.mu.Lock()
	m.latest = nc
	m.mu.Unlock()
	return nc.value, nc.err
}

// remoteChanged 通过本机守护进程比对远程镜像摘要（走 NAS 已配置的镜像加速 / 代理）。
func (m *Manager) remoteChanged(ctx context.Context, image string) (*bool, string) {
	m.mu.Lock()
	c := m.digest
	m.mu.Unlock()
	if c != nil && c.key == image && time.Since(c.at) < 10*time.Minute {
		return c.value, c.err
	}
	nc := &cached[*bool]{at: time.Now(), key: image}
	err := func() error {
		_, digests, ok, err := m.docker.imageInfo(ctx, image)
		if err != nil || !ok {
			return fmt.Errorf("读取本地镜像失败")
		}
		code, b, err := m.docker.api(ctx, "GET", "/distribution/"+image+"/json", nil, 20*time.Second)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("查询远程镜像失败：HTTP %d", code)
		}
		var v struct{ Descriptor struct{ Digest string } }
		if json.Unmarshal(b, &v) != nil || v.Descriptor.Digest == "" {
			return fmt.Errorf("远程镜像摘要为空")
		}
		changed := true
		for _, d := range digests {
			if strings.HasSuffix(d, "@"+v.Descriptor.Digest) {
				changed = false
			}
		}
		nc.value = &changed
		return nil
	}()
	if err != nil {
		nc.err = err.Error()
	}
	m.mu.Lock()
	m.digest = nc
	m.mu.Unlock()
	return nc.value, nc.err
}

// Status 返回版本与更新状态。
func (m *Manager) Status(ctx context.Context) Status {
	ds, _ := m.dockerStatus(ctx)
	image := ds.Image
	if image == "" {
		image = defaultImage
	}
	var (
		wg             sync.WaitGroup
		latest, latErr string
		changed        *bool
		imgErr         string
	)
	wg.Add(1)
	go func() { defer wg.Done(); latest, latErr = m.latestVersion(ctx, hubRepo(image)) }()
	if ds.Available {
		wg.Add(1)
		go func() { defer wg.Done(); changed, imgErr = m.remoteChanged(ctx, image) }()
	}
	wg.Wait()
	m.mu.Lock()
	st := m.state
	m.mu.Unlock()
	return Status{
		Current: version.Get(), Latest: latest, LatestError: latErr, ImageChanged: changed, ImageError: imgErr,
		HasUpdate:  (latest != "" && newer(latest, version.Version)) || (changed != nil && *changed),
		AutoUpdate: m.autoConfig(), Docker: ds, State: st,
	}
}

// ---- 执行更新 ----

// Start 开始一键更新（后台执行）。
func (m *Manager) Start(ctx context.Context) (State, error) {
	m.mu.Lock()
	running := m.state.Running
	m.mu.Unlock()
	if running {
		return State{}, ErrBusy
	}
	ds, c := m.dockerStatus(ctx)
	if !ds.Available || c == nil {
		return State{}, errors.New(ds.Reason)
	}
	m.mu.Lock()
	if m.state.Running {
		m.mu.Unlock()
		return State{}, ErrBusy
	}
	m.persistLocked(State{Running: true, Phase: "prepare", FromVersion: version.Version, StartedAt: now(),
		Message: "正在准备更新"}, false)
	st := m.state
	m.mu.Unlock()
	go func() {
		if err := m.run(context.Background(), c.Name, c.Image); err != nil {
			m.finish(false, "更新失败："+err.Error())
		}
	}()
	return st, nil
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

type layer struct {
	current, total int64
	done           bool
}

// pull 通过守护进程拉取镜像，按层汇总下载 / 解压进度。
func (m *Manager) pull(ctx context.Context, ref string, onProgress func(Progress)) error {
	repo, tag := splitImage(ref)
	layers := map[string]*layer{}
	var order []string
	var pullErr string
	code, err := m.docker.stream(ctx, "POST",
		"/images/create?fromImage="+url.QueryEscape(repo)+"&tag="+url.QueryEscape(tag), 3*time.Minute,
		func(line map[string]any) {
			if e, ok := line["error"].(string); ok {
				pullErr = e
			}
			id, _ := line["id"].(string)
			if id == "" || onProgress == nil {
				return
			}
			st, _ := line["status"].(string)
			l := layers[id]
			if l == nil {
				if !strings.Contains("Pulling fs layer|Waiting|Downloading|Download complete|Extracting|Pull complete|Already exists|Verifying Checksum", st) {
					return
				}
				l = &layer{}
				layers[id] = l
				order = append(order, id)
			}
			pd, _ := line["progressDetail"].(map[string]any)
			switch {
			case st == "Downloading" && num(pd["total"]) > 0:
				l.total, l.current = num(pd["total"]), num(pd["current"])
			case st == "Download complete" || st == "Pull complete" || st == "Already exists":
				l.current = l.total
				l.done = l.done || st != "Download complete"
			case st == "Extracting" && num(pd["total"]) > 0:
				if l.total == 0 {
					l.total = num(pd["total"])
				}
				l.current = l.total
			}
			var p Progress
			for _, k := range order {
				x := layers[k]
				p.Total += x.total
				p.Current += x.current
				if x.done {
					p.Done++
				}
			}
			p.Layers = len(order)
			pct := float64(p.Done) / float64(p.Layers) * 15
			if p.Total > 0 {
				pct += float64(p.Current) / float64(p.Total) * 85
			}
			p.Percent = min(99, int(pct))
			onProgress(p)
		})
	if err != nil {
		return err
	}
	if code != 200 || pullErr != "" {
		if pullErr == "" {
			pullErr = "HTTP " + strconv.Itoa(code)
		}
		return fmt.Errorf("拉取 %s 失败：%s", ref, pullErr)
	}
	return nil
}

func (m *Manager) run(ctx context.Context, container, image string) error {
	m.update(func(s *State) {
		s.Phase, s.Progress, s.Message = "pull", &Progress{}, "正在下载新镜像"
	}, false)
	mb := func(n int64) string { return strconv.FormatFloat(float64(n)/1048576, 'f', 1, 64) }
	if err := m.pull(ctx, image, func(p Progress) {
		m.update(func(s *State) {
			s.Progress = &p
			s.Message = "正在下载新镜像"
			if p.Total > 0 {
				s.Message = "正在下载新镜像 " + mb(p.Current) + " / " + mb(p.Total) + " MB"
			}
		}, true)
	}); err != nil {
		return err
	}

	// 当前容器使用的镜像与刚拉取的镜像相同，则无需重建
	cur, err := m.docker.inspect(ctx, container)
	if err != nil || cur == nil {
		return fmt.Errorf("读取当前容器信息失败")
	}
	latestID, _, _, _ := m.docker.imageInfo(ctx, image)
	if latestID != "" && cur.ID == latestID {
		m.finish(true, "已是最新镜像，无需更新（如刚推送代码，请等待 GitHub Actions 构建完成后重试）")
		return nil
	}

	m.update(func(s *State) {
		s.Phase, s.Progress, s.Message = "recreate", &Progress{Percent: 100}, "新镜像已下载，正在准备重建容器"
	}, false)
	if _, _, ok, _ := m.docker.imageInfo(ctx, m.watchtower); !ok {
		if err := m.pull(ctx, m.watchtower, nil); err != nil {
			return err
		}
	}
	code, b, err := m.docker.api(ctx, "POST", "/containers/create?name=link-scope-updater-"+strconv.FormatInt(time.Now().Unix(), 10),
		map[string]any{
			"Image": m.watchtower,
			// --no-pull：镜像已由本服务拉取，watchtower 只负责用本地新镜像按原配置重建容器
			"Cmd": []string{"--run-once", "--cleanup", "--no-pull", container},
			"Tty": true,
			"Env": []string{"DOCKER_API_VERSION=" + strings.TrimPrefix(m.docker.prefix, "/v")},
			"HostConfig": map[string]any{
				"AutoRemove": true,
				"Binds":      []string{m.docker.socket + ":/var/run/docker.sock"},
			},
		}, 0)
	if err != nil {
		return err
	}
	if code != 201 {
		return fmt.Errorf("创建更新容器失败：HTTP %d %s", code, strings.TrimSpace(string(b)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(b, &created)
	m.update(func(s *State) { s.Phase, s.Message = "restarting", "正在重建容器，服务即将重启" }, false)
	code, b, err = m.docker.api(ctx, "POST", "/containers/"+created.ID+"/start", nil, 0)
	if err != nil {
		return err
	}
	if code != 204 && code != 304 {
		return fmt.Errorf("启动更新容器失败：HTTP %d %s", code, strings.TrimSpace(string(b)))
	}

	// 正常情况下本容器会被停止替换，下面执行不到；若 watchtower 失败则记录日志
	_, b, _ = m.docker.api(ctx, "GET", "/containers/"+created.ID+"/logs?stdout=1&stderr=1&follow=1&tail=200", nil, 10*time.Minute)
	lines := strings.Split(strings.TrimSpace(ansiRe.ReplaceAllString(string(b), "")), "\n")
	if len(lines) > 15 {
		lines = lines[len(lines)-15:]
	}
	logText := strings.Join(lines, "\n")
	atoi := func(re *regexp.Regexp) int {
		if mm := re.FindStringSubmatch(logText); mm != nil {
			n, _ := strconv.Atoi(mm[1])
			return n
		}
		return 0
	}
	updated, failed := atoi(updatedRe), atoi(failedRe)
	msg := "watchtower 未执行更新，请查看下方日志"
	switch {
	case failed > 0:
		msg = "重建容器失败，请查看下方日志"
	case updated > 0:
		msg = "容器已重建，服务即将重启"
	}
	m.update(func(s *State) {
		s.Running, s.Phase, s.FinishedAt, s.Success, s.Log, s.Message = false, "", now(), boolp(failed == 0 && updated > 0), logText, msg
	}, false)
	return nil
}
