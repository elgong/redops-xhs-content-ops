#!/usr/bin/env bash
set -euo pipefail

PROFILE_DIR="${XHS_WEB_PROFILE_DIR:-$HOME/.redops/xhs-browser-profile}"
REMOTE_PORT="${XHS_WEB_REMOTE_PORT:-9222}"
mkdir -p "$PROFILE_DIR"

if [[ -d "/Applications/Google Chrome.app" ]]; then
  open -na "Google Chrome" --args --user-data-dir="$PROFILE_DIR" --remote-debugging-port="$REMOTE_PORT" --no-first-run --no-default-browser-check "https://www.xiaohongshu.com"
elif [[ -d "/Applications/Microsoft Edge.app" ]]; then
  open -na "Microsoft Edge" --args --user-data-dir="$PROFILE_DIR" --remote-debugging-port="$REMOTE_PORT" --no-first-run --no-default-browser-check "https://www.xiaohongshu.com"
else
  echo "未找到 Google Chrome 或 Microsoft Edge，请先安装浏览器。"
  exit 1
fi

echo "已打开小红书登录浏览器，远程调试端口 $REMOTE_PORT。完成扫码登录后，可以回到 RED OPS 点击关键词采集。"
