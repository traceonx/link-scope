# LinkScope · 链接透视

运行在 NAS 上的轻量链接检测工具：一眼看清哪些文件已硬链接、链接到了哪里、哪些还没整理、哪些被重复复制占用空间，以及失效的软链接和 STRM。单文件、低内存，Docker 一键部署，为飞牛 fnOS 优化。

## 能检测什么

| 检测 | 说明 |
| --- | --- |
| **硬链接** | 每个文件的链接数；指定「查找目录」后列出每个硬链接的具体位置 |
| **未链接** | 只有自己一份的文件，通常是还没整理 |
| **重复副本** | 内容相同、却是独立文件的副本（整理时被复制而不是硬链接），并统计多占的空间 |
| **软链接** | 指向哪里；目标已不存在的标为失效 |
| **STRM** | `.strm` 里写的本地路径是否还存在；远程地址只记录不检测 |

重复比对有两种方式：**抽样**（默认，只读每个文件的头、中、尾各 1 MB，速度快）和**完整**（读完整个文件，最准确但慢）。只有大小完全相同的文件才会被比对。

## 部署（飞牛 fnOS）

```bash
mkdir -p /vol1/1000/docker/linkscope && cd /vol1/1000/docker/linkscope
# 把仓库里的 docker-compose.yml 放到这里，按需修改路径
docker compose up -d
```

打开 `http://NAS地址:8090`。

> **硬链接只能存在于同一个存储空间内。** 请把整个存储空间（如 `/vol1`）以**相同路径**挂载进容器，不要把下载目录和媒体库分成两个挂载点，否则容器内会被当成不同设备，硬链接识别会出错。

### 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `LS_PORT` | `8090` | 监听端口 |
| `LS_ROOTS` | 空（不限制） | 允许扫描的根目录，逗号分隔，例如 `/vol1,/vol2` |
| `LS_DEFAULT_SOURCE` | 空 | 页面默认的检测文件夹 |
| `LS_DEFAULT_TARGETS` | 空 | 页面默认的查找目录，逗号分隔 |
| `LS_DATA_DIR` | `/config` | 扫描结果保存位置（重启后仍可查看上次结果） |
| `LS_AUTH_USER` / `LS_AUTH_PASS` | 空 | 同时设置后启用登录保护 |
| `LS_WORKERS` | CPU 数×2（4–16） | 并发读取目录数；机械硬盘可调小 |
| `LS_HASH_WORKERS` | `2` | 并发比对重复文件数 |
| `LS_SKIP_DIRS` | `@eaDir,#recycle,.Trash-1000,lost+found` | 跳过的目录名 |

## 开发

需要 Go 1.24+ 和 Node 22+。

```bash
# 后端
go test ./...
LS_ROOTS=/your/path go run ./cmd/linkscope

# 前端（另开终端，/api 会代理到 8090）
cd web && npm install && npm run dev

# 打包成单个可执行文件
cd web && npm run build && cd .. && go build -o linkscope ./cmd/linkscope
```

目录结构：

```
cmd/linkscope/      程序入口
internal/scanner/   扫描核心（硬链接 / 重复 / 软链接 / STRM）
internal/jobs/      后台任务与结果持久化
internal/server/    HTTP 接口（Go 标准库 net/http）
internal/config/    环境变量配置
web/                前端（Vue 3 + Vite + TypeScript + Naive UI），构建后嵌入程序
scripts/            独立的命令行检测脚本（Python，无需部署）
```

### 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/config` | 版本、允许的根目录、默认路径 |
| GET | `/api/browse?path=` | 列出子文件夹 |
| POST | `/api/scan` | 开始扫描 `{source, targets, recursive, min_size_mb, dup_mode}` |
| POST | `/api/scan/cancel` | 取消扫描 |
| GET | `/api/status` | 进度和上次扫描汇总 |
| GET | `/api/result?filter=&q=&offset=&limit=` | 分页结果；filter：all / linked / unlinked / dup / symlink / strm / broken |
| GET | `/api/export.csv?filter=&q=` | 导出 CSV（Excel 可直接打开） |

## 性能

在 2 核云主机（热缓存）上实测：20 万个源文件 + 10 万个查找目录文件，不比对重复约 1.4 秒；所有文件大小都相同、全部参与抽样比对的极端情况约 4.2 秒。真实 NAS 上，冷启动时主要受硬盘读取速度限制。

可自行复现：`LS_BENCH_N=200000 LS_BENCH_DUP=off go test -run TestLargeScan -v ./internal/scanner/`
