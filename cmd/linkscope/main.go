// LinkScope · 链接透视：NAS 上的硬链接、重复文件、软链接、STRM 检测工具。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/traceonx/link-scope/internal/config"
	"github.com/traceonx/link-scope/internal/fileops"
	"github.com/traceonx/link-scope/internal/jobs"
	"github.com/traceonx/link-scope/internal/logs"
	"github.com/traceonx/link-scope/internal/server"
	"github.com/traceonx/link-scope/internal/torrents"
	"github.com/traceonx/link-scope/internal/update"
	"github.com/traceonx/link-scope/internal/version"
	"github.com/traceonx/link-scope/web"
)

func main() {
	showVersion := flag.Bool("version", false, "显示版本")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version)
		return
	}
	logs.Install()
	log.Println(version.Banner())

	cfg := config.Load()
	dl := torrents.NewStore(cfg.DataDir)
	jm := jobs.New(cfg.DataDir, dl)
	um := update.New(cfg.DataDir)
	ops := fileops.New(cfg.Roots, cfg.DataDir, cfg.TrashDays, cfg.DeleteDisabled)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go um.RunAutoUpdate(ctx)
	go func() { // 每小时清理过期的回收站条目
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			if n := ops.CleanExpired(); n > 0 {
				log.Printf("回收站：已清除 %d 个过期条目", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(cfg, jm, um, ops, dl, web.FS()).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("监听 %s，数据目录 %s", cfg.Addr, cfg.DataDir)
		if len(cfg.Roots) > 0 {
			log.Printf("允许扫描的目录: %v", cfg.Roots)
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	jm.Cancel()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
}
