#!/usr/bin/env bash
# 启动构建好的二进制，确认 /readyz 与 /openapi.json 可用。
# 用法：make smoke（需可达的 PostgreSQL；默认 127.0.0.1:15436/golottery）
set -euo pipefail

binary="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
port="${SMOKE_PORT:-5569}"
base="http://127.0.0.1:${port}"

# 在临时目录里运行：配置会读取当前目录的 .env 并覆盖环境变量
workdir="$(mktemp -d)"
log="${workdir}/golottery.log"
cleanup() {
  [[ -n "${pid:-}" ]] && kill "${pid}" 2>/dev/null && wait "${pid}" 2>/dev/null
  rm -rf "${workdir}"
}
trap cleanup EXIT

(
  cd "${workdir}"
  DATABASE_URL="${SMOKE_DATABASE_URL:-postgres://postgres:secret@127.0.0.1:15436/golottery?sslmode=disable}" \
  SESSION_SECRET="smoke-test-session-secret-0123456789abcdef" \
  APP_HOST=127.0.0.1 \
  APP_PORT="${port}" \
  exec "${binary}"
) >"${log}" 2>&1 &
pid=$!

for _ in $(seq 1 60); do
  if ! kill -0 "${pid}" 2>/dev/null; then
    echo "smoke: 进程提前退出" >&2
    cat "${log}" >&2
    exit 1
  fi
  if curl -fsS "${base}/readyz" >/dev/null 2>&1; then
    curl -fsS "${base}/healthz" >/dev/null
    curl -fsS "${base}/openapi.json" >/dev/null
    echo "smoke: /readyz /healthz /openapi.json 正常"
    exit 0
  fi
  sleep 0.5
done

echo "smoke: 30 秒内 /readyz 未就绪" >&2
cat "${log}" >&2
exit 1
