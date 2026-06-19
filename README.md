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
| `SEED_DATA` | `true` | 启动时写入演示账号与关键词 |
| `SCHEDULER_ENABLED` | `true` | 是否启动定时发布扫描器 |
| `XHS_ADAPTER` | `mock` | `mock` 或 `openapi` |
| `XHS_BASE_URL` | `https://ark.xiaohongshu.com` | 小红书 OpenAPI 基础地址 |
| `XHS_ACCESS_TOKEN` | 空 | 开放平台访问令牌 |
| `XHS_DRAFT_ENDPOINT` | 空 | 有草稿权限后配置草稿接口 |
| `XHS_PUBLISH_ENDPOINT` | 空 | 有发布权限后配置发布接口 |

## 小红书开放接口接入

当前代码内置两种适配器：

- `MockXHSAdapter`：默认模式，完整跑通采集、草稿和发布闭环，不访问小红书。
- `XHSOpenAPIAdapter`：开放平台模式，通过 `XHS_ADAPTER=openapi` 开启。

已接入/预留能力：

- 官方素材上传：`POST /api/xhs/materials`，内部调用 `/ark/open_api/v3/common_controller`。
- 草稿保存：通过 `XHS_DRAFT_ENDPOINT` 配置有权限的接口地址。
- 发布笔记：通过 `XHS_PUBLISH_ENDPOINT` 配置有权限的接口地址。
- 关键词笔记搜索：官方公开目录未确认通用接口，当前不会做未授权抓取；建议使用授权数据源、人工导入或申请对应数据权限。

官方参考：

- 小红书开放平台应用类目与权限：https://xiaohongshu.apifox.cn/
- 素材中心上传素材：https://xiaohongshu.apifox.cn/api-24925828
- 小红书分享开放平台：https://agora.xiaohongshu.com/

## 说明

- 小红书真实采集/发布能力通过 `XHSAdapter` 接口预留，默认使用 `MockXHSAdapter`，不会绕过平台登录、验证码或风控。
- AI 文案生成目前是本地规则生成器，保留 `Generator` 接口，后续可替换为大模型服务。
- 采集功能默认生成模拟样本，也支持通过 API 扩展为官方授权接口或人工导入。
