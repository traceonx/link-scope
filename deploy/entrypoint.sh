#!/bin/sh
# 默认以 root 运行，保证能读取 NAS 上所有用户的文件。
# 设置 PUID / PGID 后：修正数据目录权限；若挂载了 docker.sock（一键更新），把该用户加入其所属组；然后降权运行。
set -e
DATA_DIR="${LS_DATA_DIR:-/app/data}"
mkdir -p "$DATA_DIR"

if [ "$(id -u)" = "0" ] && [ -n "$PUID" ] && [ "$PUID" != "0" ]; then
  PGID="${PGID:-$PUID}"
  group=$(getent group "$PGID" | cut -d: -f1)
  if [ -z "$group" ]; then group=linkscope; addgroup -g "$PGID" "$group"; fi
  user=$(getent passwd "$PUID" | cut -d: -f1)
  if [ -z "$user" ]; then user=linkscope; adduser -D -H -u "$PUID" -G "$group" "$user"; fi
  chown -R "$PUID:$PGID" "$DATA_DIR"
  sock="${UPDATE_DOCKER_SOCKET:-/var/run/docker.sock}"
  if [ -S "$sock" ]; then
    gid=$(stat -c %g "$sock")
    sgroup=$(getent group "$gid" | cut -d: -f1)
    if [ -z "$sgroup" ]; then sgroup=dockerhost; addgroup -g "$gid" "$sgroup"; fi
    addgroup "$user" "$sgroup" 2>/dev/null || true
  fi
  exec su-exec "$user" /usr/local/bin/linkscope "$@"
fi

exec /usr/local/bin/linkscope "$@"
