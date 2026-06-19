# RED OPS 小红书内容运营系统

Go + MySQL 实现的内容运营工作台，覆盖关键词采集、热度分析、文案生成、人工审核、驳回重生、账号草稿、定时发布和结果回传。

## 当前电脑快速启动

```bash
cp .env.example .env
./run-local.sh
```

打开：

```text
http://127.0.0.1:8080
```

当前电脑已经安装：

- Go: `/usr/local/go`
- Homebrew: `/usr/local/bin/brew`
- MySQL: Homebrew service `mysql`

如果只想无数据库预览：

```bash
./run-memory.sh
```

当前电脑已配置用户级后台服务：

```bash
launchctl print gui/$(id -u)/com.redops.local
launchctl kickstart -k gui/$(id -u)/com.redops.local
```

运行产物部署在 `~/.redops`，代码仍维护在当前仓库。这样可以避开 macOS 对 `Documents` 目录的后台服务访问限制。

## MySQL 初始化

已经执行过：

```sql
CREATE DATABASE IF NOT EXISTS redops CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER IF NOT EXISTS 'redops'@'localhost' IDENTIFIED BY 'redops';
GRANT ALL PRIVILEGES ON redops.* TO 'redops'@'localhost';
FLUSH PRIVILEGES;
```

启动/停止 MySQL：

```bash
brew services start mysql
brew services stop mysql
```

## Docker 方案

如果安装了 Docker Desktop，也可以直接：

```bash
docker compose up --build
```

服务地址同样是：

```text
http://127.0.0.1:8080
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ADDR` | `:8080` | HTTP 服务监听地址 |
| `APP_STORE` | `mysql` | `mysql` 或 `memory` |
| `MYSQL_DSN` | 见 `.env.example` | MySQL 连接串 |
| `AUTO_MIGRATE` | `true` | 启动时自动建表 |
| `SEED_DATA` | `false` | 启动时写入演示账号与关键词，本项目默认关闭 |
| `SCHEDULER_ENABLED` | `true` | 是否启动定时发布扫描器 |
| `XHS_ADAPTER` | `openapi` | `mock` 或 `openapi` |
| `XHS_BASE_URL` | `https://ark.xiaohongshu.com` | 小红书 OpenAPI 基础地址 |
| `XHS_ACCESS_TOKEN` | 空 | 开放平台访问令牌 |
| `XHS_DRAFT_ENDPOINT` | 空 | 有草稿权限后配置草稿接口 |
| `XHS_PUBLISH_ENDPOINT` | 空 | 有发布权限后配置发布接口 |

## 小红书开放接口接入

当前代码内置两种适配器：

- `MockXHSAdapter`：默认模式，完整跑通采集、草稿和发布闭环，不访问小红书。
- `XHSOpenAPIAdapter`：开放平台模式，通过 `XHS_ADAPTER=openapi` 开启。

已接入/预留能力：

- 授权样本导入：`POST /api/keywords/{id}/import`，支持人工或合规数据源导入笔记指标。
- 网页文本导入：`POST /api/keywords/{id}/import-text`，支持粘贴从小红书网页复制出的标题、链接、点赞、评论、收藏、浏览文本。
- 官方素材上传：`POST /api/xhs/materials`，内部调用 `/ark/open_api/v3/common_controller`。
- 草稿保存：通过 `XHS_DRAFT_ENDPOINT` 配置有权限的接口地址。
- 发布笔记：通过 `XHS_PUBLISH_ENDPOINT` 配置有权限的接口地址。
- 关键词笔记搜索：官方公开目录未确认通用接口，当前不会做未授权抓取；建议使用授权数据源、人工导入或申请对应数据权限。

授权样本导入示例：

```bash
curl -X POST http://127.0.0.1:8080/api/keywords/1/import \
  -H 'Content-Type: application/json' \
  -d '{
    "posts": [{
      "title": "油皮夏天真的别乱叠早C晚A",
      "content_summary": "真实体验、步骤拆解、评论关注适用肤质",
      "url": "https://www.xiaohongshu.com/example-authorized",
      "views": 128000,
      "likes": 8420,
      "comments": 516,
      "favorites": 2104,
      "published_at": "2026-06-18T19:30:00+08:00"
    }]
  }'
```

网页文本导入示例：

```bash
curl -X POST http://127.0.0.1:8080/api/keywords/1/import-text \
  -H 'Content-Type: application/json' \
  -d '{
    "raw_text": "早C晚A真的别乱叠，我脸红了三天\n浏览 12.8万 点赞 8420 评论 516 收藏 2104\nhttps://www.xiaohongshu.com/explore/example"
  }'
```

官方参考：

- 小红书开放平台应用类目与权限：https://xiaohongshu.apifox.cn/
- 素材中心上传素材：https://xiaohongshu.apifox.cn/api-24925828
- 小红书分享开放平台：https://agora.xiaohongshu.com/

## 说明

- 小红书真实采集/发布能力通过 `XHSAdapter` 接口预留，默认使用 `MockXHSAdapter`，不会绕过平台登录、验证码或风控。
- 当前 `.env` 使用 `XHS_ADAPTER=openapi` 和 `SEED_DATA=false`，不会写入演示数据。
- AI 文案生成目前是本地规则生成器，保留 `Generator` 接口，后续可替换为大模型服务。
- 关键词自动采集仅在 mock 模式生成模拟样本；真实模式下请使用官方授权接口、网页文本导入或人工审核导入。
