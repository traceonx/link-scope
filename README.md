# LinkScope · 链接透视

运行在 NAS 上的下载目录清理工具：qBittorrent 下载、MoviePilot 用硬链接整理到媒体库后，总有一些文件因为识别失败、整理失败等原因留在下载目录里白占空间。LinkScope 找出这些**没被媒体库用上的游离文件**，按文件夹列出、统计占用空间，并安全地清理（回收站可恢复）；连接下载器后还能看到做种状态和 PT 站点。单文件、低内存，Docker 一键部署，为飞牛 fnOS 优化。

## 能检测什么

| 检测 | 说明 |
| --- | --- |
| **硬链接** | 每个文件的链接数；指定「查找目录」后列出每个硬链接的具体位置 |
| **未链接** | 只有自己一份的文件，通常是还没整理 |
| **重复副本** | 内容相同、却是独立文件的副本（整理时被复制而不是硬链接），并统计多占的空间 |
| **软链接** | 指向哪里；目标已不存在的标为失效 |
| **STRM** | `.strm` 里写的本地路径是否还存在；远程地址只记录不检测 |
| **做种 / 站点** | 连接 qBittorrent / Transmission 后：每个文件属于哪个种子、是否在做种、来自哪个站点 |
| **站点已删种** | Tracker 返回「未注册 / 种子不存在」等错误的种子 |
| **孤儿文件** | 在下载目录里，但不属于任何种子的文件 |

结果可以**按文件夹**（默认）或**按文件**查看。按文件夹时，下载目录下的每个一级子文件夹是一行，状态为 **待清理**（没有任何文件被整理到媒体库）、**部分游离**（部分文件已整理）或 **已整理**；默认显示「待清理」并按游离大小从大到小排序。点开文件夹可看到其中每个文件，再点文件查看详情。

重复比对有两种方式：**抽样**（默认，只读每个文件的头、中、尾各 1 MB，速度快）和**完整**（读完整个文件，最准确但慢）。只有大小完全相同的文件才会被比对。

## 删除与回收站

详情面板里每个路径都可以单独删除，也可以「删除全部硬链接」或「删除所在文件夹」；列表可勾选批量删除，或删除当前筛选的全部结果；目录浏览里可以删除文件夹。

- **删除前预览**：数量、大小和**实际释放空间**。硬链接要所有路径都删掉才释放空间，只删其中一个路径时会明确提示
- **做种提醒**：连接了下载器时，若文件仍在做种，会提示来自哪些站点，避免 H&R
- **回收站**：默认移到文件所在文件系统顶层的 `.linkscope-trash`（飞牛 Btrfs 下通常是 `/vol2/1000/.linkscope-trash` 这样的用户子卷目录；同盘改名，瞬间完成），左侧导航「回收站」页面可恢复或清空，`LS_TRASH_DAYS` 天后自动清除；勾选「彻底删除」并二次确认可直接删除
- **安全**：只能删除 `LS_ROOTS` 内的内容，不能删除根目录本身；媒体目录只读挂载（`:ro`）或 `LS_DELETE=false` 时不显示删除功能；每次操作记录在 `data/delete.log`

## 站点种子检测

左侧导航「下载器」页面中添加 qBittorrent 或 Transmission（可添加多个），填写 WebUI 地址、账号，以及**路径映射**：下载器也运行在 Docker 中时，它看到的保存路径（如 `/downloads`）与本工具中的路径（如 `/vol1/1000/Downloads`）不同，需要映射后才能对上文件。

保存后，每次扫描结束会自动读取种子（也可在「下载器」里手动「重新读取种子」）：

- 列表新增「种子 / 站点」列，可筛选「做种中 / 已删种 / 孤儿文件」，并按站点筛选
- 统计区显示各站点的文件数与占用空间
- 扫描媒体库、把下载目录作为查找目录时，也会通过硬链接对应到种子

站点按 Tracker 域名识别，常见 PT 站显示中文名，其余显示主域名。

## 部署（飞牛 fnOS）

镜像由 GitHub Actions 发布到 Docker Hub：`sunqz/link-scope`（amd64 / arm64）。

```bash
mkdir -p /vol1/1000/docker/link-scope && cd /vol1/1000/docker/link-scope
# 把 deploy/docker-compose.yml 放到这里，按需修改路径
docker compose up -d
```

打开 `http://NAS地址:8082`。扫描结果和更新记录在同级目录 `./data`，备份或迁移时复制这个目录即可。

> **硬链接只能存在于同一个存储空间内。** 请把整个存储空间（如 `/vol1`）以**相同路径**挂载进容器，不要把下载目录和媒体库分成两个挂载点，否则容器内会被当成不同设备，硬链接识别会出错。

### 一键更新 / 自动更新

compose 中挂载了 `/var/run/docker.sock` 时，点击页面右上角的版本号（或「设置 → 版本与更新」）：

- **一键更新**：拉取当前标签（需为 `latest`）的新镜像，显示下载进度，然后临时启动 watchtower（`--run-once --no-pull`）按原配置重建容器，完成后页面自动刷新
- **自动更新**：每天在设定时间检查，有新版本就自动更新；失败后 6 小时内不重试

版本号旁出现橙点表示有新版本。镜像固定为版本标签（如 `sunqz/link-scope:0.2.0`）时不提供一键更新，改 compose 中的版本号后重新部署即可。

### 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `LS_ROOTS` | 空（不限制） | 允许扫描的根目录，逗号分隔，例如 `/vol1,/vol2` |
| `LS_DEFAULT_SOURCE` | 空 | 页面默认的检测文件夹 |
| `LS_DEFAULT_TARGETS` | 空 | 页面默认的查找目录，逗号分隔 |
| `LS_AUTH_USER` / `LS_AUTH_PASS` | 空 | 同时设置后启用登录保护（`/api/health` 除外） |
| `LS_WORKERS` | CPU 数×2（4–16） | 并发读取目录数；机械硬盘可调小 |
| `LS_HASH_WORKERS` | `2` | 并发比对重复文件数 |
| `LS_SKIP_DIRS` | `@eaDir,#recycle,.Trash-1000,lost+found` | 跳过的目录名 |
| `LS_PORT` / `LS_DATA_DIR` | `8090` / `/app/data` | 容器内端口与数据目录，一般不用改 |
| `UPDATE_CONTAINER` | 自动识别 | 一键更新时识别本容器，需与 `container_name` 一致 |
| `UPDATE_ENABLED` | `true` | 设为 `false` 关闭一键更新 |
| `PROXY_HOST` | 空 | Docker Hub 版本查询走的代理，如 `http://192.168.5.199:7890` |
| `UPDATE_HUB_API` | `https://hub.docker.com` | Docker Hub 接口的反向代理地址（国内无法直连时） |
| `PUID` / `PGID` | 空（root） | 设置后降权运行；默认 root 以便读取所有用户的文件 |
| `LS_DELETE` | `true` | 设为 `false` 关闭删除功能 |
| `LS_TRASH_DAYS` | `30` | 回收站保留天数 |

### 发布新版本

修改 `VERSION`、`web/package.json` 的 version 并在 `CHANGELOG.md` 顶部记录，推送到 `main` 即发布 `latest`、版本号与 `sha-xxxxxxx` 标签；推送 `v*` 标签额外发布语义化版本标签。需在仓库 Secrets 中配置 `DOCKER_USERNAME` 与 `DOCKER_PASSWORD`（Docker Hub Access Token）。

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

# 本地构建镜像并运行（http://localhost:8082）
docker compose up -d --build
```

目录结构：

```
cmd/linkscope/      程序入口
internal/scanner/   扫描核心（硬链接 / 重复 / 软链接 / STRM）
internal/jobs/      后台任务与结果持久化
internal/server/    HTTP 接口（Go 标准库 net/http）
internal/fileops/   删除与回收站
internal/torrents/  下载器（qBittorrent / Transmission）与站点识别
internal/update/    一键更新 / 自动更新（docker.sock + watchtower）
internal/version/   版本信息（构建时注入）
internal/config/    环境变量配置
deploy/             Dockerfile、entrypoint.sh、docker-compose.yml
web/                前端（Vue 3 + Vite + TypeScript + Naive UI），构建后嵌入程序
scripts/            独立的命令行检测脚本（Python，无需部署）
```

### 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查与版本（不需要登录） |
| GET | `/api/config` | 版本、允许的根目录、默认路径 |
| GET / POST | `/api/update` | 更新状态 / 开始一键更新 |
| PUT | `/api/update/auto` | 自动更新设置 `{enabled, time: "04:00"}` |
| GET | `/api/browse?path=` | 列出子文件夹 |
| POST | `/api/scan` | 开始扫描 `{source, targets, recursive, min_size_mb, dup_mode}` |
| POST | `/api/scan/cancel` | 取消扫描 |
| GET | `/api/status` | 进度和上次扫描汇总 |
| GET | `/api/result?filter=&site=&q=&dir=&sort=&order=&offset=&limit=` | 分页结果；filter：all / linked / unlinked / dup / symlink / strm / broken / torrent / seeding / unregistered / orphan |
| GET | `/api/export.csv?filter=&site=&q=&dir=` | 导出 CSV（Excel 可直接打开） |
| GET | `/api/folders?filter=&site=&q=&sort=&order=&offset=&limit=`、`/api/folders.csv` | 文件夹视图；filter：all / linked / full / partial / unlinked / dup / seeding / unregistered / orphan |
| POST | `/api/delete/preview`、`/api/delete` | 删除预览 / 删除 `{paths}`、`{filter, site, q, dir}` 或 `{view: "folders", filter, site, q}`，`permanent` |
| GET / POST | `/api/trash`、`/api/trash/restore`、`/api/trash/purge` | 回收站列表 / 恢复 / 永久删除 |
| GET / POST / DELETE | `/api/downloaders`、`/api/downloaders/test`、`/api/downloaders/{id}` | 下载器配置 |
| POST | `/api/torrents/refresh` | 重新读取种子并更新结果 |

## 性能

在 2 核云主机（热缓存）上实测：20 万个源文件 + 10 万个查找目录文件，不比对重复约 1.4 秒；所有文件大小都相同、全部参与抽样比对的极端情况约 4.2 秒。真实 NAS 上，冷启动时主要受硬盘读取速度限制。

可自行复现：`LS_BENCH_N=200000 LS_BENCH_DUP=off go test -run TestLargeScan -v ./internal/scanner/`
