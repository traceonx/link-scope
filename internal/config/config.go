// Package config 从环境变量（前缀 LS_）读取配置。
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	Addr           string   // 监听地址，默认 :8090
	Roots          []string // 允许扫描的根目录；为空表示不限制
	DefaultSource  string
	DefaultTargets []string
	DataDir        string // 扫描结果保存目录
	AuthUser       string
	AuthPass       string
	Workers        int
	HashWorkers    int
	SkipDirs       map[string]bool
}

func list(name, def string) []string {
	v, ok := os.LookupEnv(name)
	if !ok {
		v = def
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func str(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func num(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return def
}

// Load 读取配置。
func Load() *Config {
	c := &Config{
		Addr:           str("LS_ADDR", ":"+str("LS_PORT", "8090")),
		DefaultSource:  str("LS_DEFAULT_SOURCE", ""),
		DefaultTargets: list("LS_DEFAULT_TARGETS", ""),
		DataDir:        str("LS_DATA_DIR", str("DATA_DIR", "./data")),
		AuthUser:       os.Getenv("LS_AUTH_USER"),
		AuthPass:       os.Getenv("LS_AUTH_PASS"),
		Workers:        num("LS_WORKERS", min(16, max(4, runtime.NumCPU()*2))),
		HashWorkers:    num("LS_HASH_WORKERS", 2),
		SkipDirs:       map[string]bool{},
	}
	for _, r := range list("LS_ROOTS", "") {
		if real, err := filepath.EvalSymlinks(r); err == nil {
			r = real
		}
		c.Roots = append(c.Roots, filepath.Clean(r))
	}
	for _, d := range list("LS_SKIP_DIRS", "@eaDir,#recycle,.Trash-1000,lost+found") {
		c.SkipDirs[d] = true
	}
	return c
}

// AuthEnabled 是否启用了登录保护。
func (c *Config) AuthEnabled() bool { return c.AuthUser != "" && c.AuthPass != "" }
