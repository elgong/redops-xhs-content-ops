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

## 说明

- 小红书真实采集/发布能力通过 `XHSAdapter` 接口预留，默认使用 `MockXHSAdapter`，不会绕过平台登录、验证码或风控。
- AI 文案生成目前是本地规则生成器，保留 `Generator` 接口，后续可替换为大模型服务。
- 采集功能默认生成模拟样本，也支持通过 API 扩展为官方授权接口或人工导入。
