// LinkScope · 链接透视：NAS 上的硬链接、重复文件、软链接、STRM 检测工具。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/traceonx/link-scope/internal/config"
	"github.com/traceonx/link-scope/internal/jobs"
	"github.com/traceonx/link-scope/internal/server"
	"github.com/traceonx/link-scope/web"
)

// version 在构建时通过 -ldflags "-X main.version=..." 注入。
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "显示版本")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg := config.Load()
	jm := jobs.New(cfg.DataDir)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(cfg, jm, web.FS(), version).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("LinkScope %s 已启动，监听 %s", version, cfg.Addr)
		if len(cfg.Roots) > 0 {
			log.Printf("允许扫描的目录: %v", cfg.Roots)
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	jm.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
