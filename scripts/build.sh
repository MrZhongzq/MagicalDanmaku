#!/usr/bin/env bash
# 本地交叉编译脚本：零外部依赖，只用 Go 工具链出全平台包。
#
# 与 .goreleaser.yaml 的目标列表和 ldflags 保持一致；正式发版走
# GoReleaser（见 .github/workflows/release.yml），本脚本用于日常验证。
#
# 用法:
#   scripts/build.sh              # 出全部平台
#   scripts/build.sh linux/amd64  # 只出指定平台
set -euo pipefail

cd "$(dirname "$0")/.."

MODULE_DIR="server"
BIN_NAME="magicd"
OUT_DIR="dist"

# 目标平台列表，与 .goreleaser.yaml 一致。
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

# 版本信息：优先用当前 tag，否则用 dev + 短哈希。
VERSION="$(git describe --tags --exact-match 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
  COMMIT="${COMMIT}-dirty"
fi
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

PKG="github.com/MrZhongzq/MagicalDanmaku/server/internal/buildinfo"
LDFLAGS="-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.Date=${DATE}"

# 允许只构建单个平台
if [ $# -gt 0 ]; then
  TARGETS=("$@")
fi

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

echo "版本 ${VERSION} 提交 ${COMMIT}"
echo

failed=0
for target in "${TARGETS[@]}"; do
  goos="${target%%/*}"
  goarch="${target##*/}"

  ext=""
  [ "${goos}" = "windows" ] && ext=".exe"
  out="${OUT_DIR}/${BIN_NAME}_${goos}_${goarch}${ext}"

  printf "%-16s " "${goos}/${goarch}"
  # CGO_ENABLED=0 保证产物是静态二进制，无 libc 依赖。
  if (cd "${MODULE_DIR}" && CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
      go build -trimpath -ldflags "${LDFLAGS}" -o "../${out}" ./cmd/magicd); then
    size="$(du -h "${out}" | cut -f1)"
    echo "OK   ${out} (${size})"
  else
    echo "FAIL"
    failed=1
  fi
done

echo
if [ "${failed}" -ne 0 ]; then
  echo "构建失败" >&2
  exit 1
fi

# 校验和文件，便于分发时核对
(cd "${OUT_DIR}" && sha256sum ./* > checksums.txt 2>/dev/null || shasum -a 256 ./* > checksums.txt)
echo "已生成 ${OUT_DIR}/checksums.txt"
