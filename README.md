# Sub-Manager

[中文](#中文) | [English](#english)

---

<a name="中文"></a>
## 中文

### 简介

Sub-Manager 是一个自建的代理订阅聚合与分发服务。它可以从多个机场拉取订阅并缓存在本地，通过内置的调度机制配合独立的 subconverter 服务将节点合并、过滤并转换为不同客户端（如 Clash、sing-box、Surge、Quantumult X 等）所需的配置格式，并对外提供固定的订阅链接。

**核心特性**：
- **组合订阅发布**：自定义 Profile，可灵活勾选参与合并的机场与目标输出格式。
- **故障容灾降级**：本地持久化缓存原始订阅；若某个上游机场临时无法连接，合成时会自动沿用上一次拉取的有效缓存，避免客户端订阅整体断供。
- **流量信息聚合**：自动解析各机场的 `Subscription-Userinfo`，汇总已用/总流量，并取最早的过期时间，统一返回给客户端。
- **动态定时调度**：支持后台修改全量刷新周期，修改后实时重置调度器生效，无需重启。
- **轻量易维护**：内置 Web 管理面板，单二进制配合 SQLite，开箱即用。

---

### 系统架构

Sub-Manager 采用“管理缓存 + 独立转换”的分层架构：

```
+----------------------------------------------------------------+
|                        Sub-Manager                             |
|                                                                |
|  +-------------------+              +-----------------------+  |
|  |   Web 管理面板    |              |     定时刷新调度器    |  |
|  +-------------------+              +-----------------------+  |
|           |                                     |              |
|           v                                     v              |
|  +----------------------------------------------------------+  |
|  |                 抓取与缓存引擎 (SQLite)                  |  |
|  +----------------------------------------------------------+  |
|           |                                     ^              |
|           | 内部端点暴露缓存                   | 回传合成结果  |
|           | (/internal/airports/:id/raw)        |              |
+-----------|-------------------------------------|--------------+
            |                                     |
            v                                     |
+----------------------------------------------------------------+
|                     subconverter 服务                          |
|  (负责协议识别、节点去重/重命名、规则组注入及下游格式渲染)     |
+----------------------------------------------------------------+
            |
            v
下游客户端 (Clash / sing-box / Surge 等) <-- [GET /profile/{slug}]
```

1. **拉取与缓存**：定时或手动从上游机场获取原始订阅与流量信息，存入本地数据库。
2. **合并与合成**：Sub-Manager 将参与合并的机场以内部缓存接口形式拼接为参数调用 subconverter，subconverter 转换完成后将配置存入 Profile 缓存。
3. **分发服务**：下游客户端请求 `/profile/{slug}` 时，直接由本地缓存高速返回配置及汇总后的流量头。

---

### 部署方法

#### 1. Docker Compose 部署（推荐）

项目自带 `docker-compose.yml`，包含 `app` 与 `subconverter` 双容器服务。

1. 克隆或下载代码至服务器：
   ```bash
   git clone https://github.com/Dreamscape315/Sub-Manager-go.git
   cd Sub-Manager-go
   ```

2. 启动服务：
   ```bash
   docker compose up -d
   ```

3. 访问管理后台：
   打开浏览器访问 `http://<服务器IP>:8080/admin`。

> 数据默认持久化在 Docker Volume `submanager-go-data`（映射容器内 `/app/data`）。

#### 2. 本地直接运行

1. 启动配套的 subconverter 容器：
   ```bash
   docker compose up -d subconverter
   ```

2. 本地启动应用：
   ```bash
   go run ./cmd/submanager
   ```
   默认监听 `http://localhost:8080`，数据库自动生成于 `./data/submanager.db`。

3. **注意**：进入 `/admin/settings`，将 **Internal Base URL** 改为 `http://host.docker.internal:8080`，以便 Docker 中的 subconverter 容器能够正确回调本机的缓存端点。

---

### 环境变量与配置项

#### 环境变量

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | 服务监听端口 |
| `SUBMANAGER_DB_PATH` | `data/submanager.db` | SQLite 数据库文件存储路径 |
| `SUBMANAGER_DEMO_ENABLED` | `true` | 是否启用 `/demo/airport/{name}` 模拟测试机场端点 |

#### 后台配置项（可在 `/admin/settings` 动态修改）

| 配置项 | 默认值 | 说明 |
|---|---|---|
| 刷新间隔 | `3600` 秒 | 周期性全量刷新的时间间隔，修改后即刻生效 |
| Subconverter 超时 | `30` 秒 | 请求 subconverter 的 HTTP 超时时间 |
| 机场拉取超时 | `20` 秒 | 拉取单个机场订阅的 HTTP 超时时间 |
| Subconverter 地址 | `http://subconverter:25500` | subconverter 服务的访问地址 |
| Internal Base URL | `http://app:8080` | subconverter 回调拉取本地缓存的本服务地址 |
| Public Base URL | *(空)* | 管理后台一键复制订阅链接的前缀域名 |
| Internal Secret | *(空)* | 内部原始缓存接口的可选鉴权 Token |

---

<a name="english"></a>
## English

### Overview

Sub-Manager is a self-hosted proxy subscription aggregator and distribution service. It pulls and caches subscription links from multiple providers locally, delegates node merging, filtering, and conversion to an independent subconverter service for target formats (e.g., Clash, sing-box, Surge, Quantumult X), and serves them through persistent endpoints.

**Key Features**:
- **Profile Aggregation**: Custom Profiles allow flexible selection of participating providers and output target formats.
- **Graceful Degradation**: Subscription data is cached locally; if an upstream provider goes temporarily offline, the last known good cache is retained during synthesis to prevent client-side disruption.
- **Userinfo Aggregation**: Automatically parses `Subscription-Userinfo` from providers, sums upload/download/total bandwidth, and extracts the earliest expiry date.
- **Dynamic Scheduler**: Runtime-configurable refresh interval that updates immediately without restarting the service.
- **Lightweight & Self-contained**: Built-in web administration dashboard backed by SQLite.

---

### Architecture

Sub-Manager separates orchestration and caching from protocol conversion:

```
+----------------------------------------------------------------+
|                        Sub-Manager                             |
|                                                                |
|  +-------------------+              +-----------------------+  |
|  |     Admin UI      |              |   Refresh Scheduler   |  |
|  +-------------------+              +-----------------------+  |
|           |                                     |              |
|           v                                     v              |
|  +----------------------------------------------------------+  |
|  |              Fetch & Cache Engine (SQLite)               |  |
|  +----------------------------------------------------------+  |
|           |                                     ^              |
|           | Internal Raw Endpoints              | Return       |
|           | (/internal/airports/:id/raw)        | Synthesized  |
+-----------|-------------------------------------|--------------+
            |                                     |
            v                                     |
+----------------------------------------------------------------+
|                     subconverter Service                       |
|  (Protocol conversion, deduplication, rule sets, rendering)    |
+----------------------------------------------------------------+
            |
            v
Downstream Clients (Clash / sing-box / Surge) <-- [GET /profile/{slug}]
```

1. **Fetch & Cache**: Retrieves raw subscriptions and metadata from providers on a schedule or manually, saving them into SQLite.
2. **Synthesis**: Exposes cached content via internal endpoints (`/internal/airports/{id}/raw`) to subconverter, which merges and formats the output into the Profile cache.
3. **Distribution**: Serves `/profile/{slug}` directly from the local cache with aggregated user info headers for fast and resilient delivery.

---

### Deployment

#### 1. Docker Compose (Recommended)

The repository provides a complete `docker-compose.yml` including both the application and `subconverter`.

1. Clone or download the repository:
   ```bash
   git clone https://github.com/Dreamscape315/Sub-Manager-go.git
   cd Sub-Manager-go
   ```

2. Start the services:
   ```bash
   docker compose up -d
   ```

3. Open the admin panel:
   Navigate to `http://<server-ip>:8080/admin` in your browser.

> Data is stored in the Docker named volume `submanager-go-data` (mounted at `/app/data`).

#### 2. Local Run

1. Start subconverter via Docker:
   ```bash
   docker compose up -d subconverter
   ```

2. Run the application:
   ```bash
   go run ./cmd/submanager
   ```
   The service listens on `http://localhost:8080` by default and creates the database at `./data/submanager.db`.

3. **Note**: In `/admin/settings`, set **Internal Base URL** to `http://host.docker.internal:8080` so the Dockerized subconverter can reach the local host's internal endpoints.

---

### Configuration

#### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port to listen on |
| `SUBMANAGER_DB_PATH` | `data/submanager.db` | Path to the SQLite database file |
| `SUBMANAGER_DEMO_ENABLED` | `true` | Enable `/demo/airport/{name}` mock provider endpoint |

#### Runtime Settings (`/admin/settings`)

| Setting | Default | Description |
|---|---|---|
| Refresh Interval | `3600` sec | Interval between full refresh cycles (applies immediately) |
| Subconverter Timeout | `30` sec | HTTP timeout when calling subconverter |
| Provider Fetch Timeout | `20` sec | HTTP timeout when pulling single provider subscriptions |
| Subconverter Base URL | `http://subconverter:25500` | Address of the subconverter service |
| Internal Base URL | `http://app:8080` | Callback URL for subconverter to reach Sub-Manager cache |
| Public Base URL | *(empty)* | Domain prefix used by the copy-link buttons |
| Internal Secret | *(empty)* | Optional secret token guarding the internal raw cache endpoint |
