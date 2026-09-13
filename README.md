# Sub-Manager (Go)

个人自建的代理订阅聚合平台 —— Go 重写版，功能对齐 [Sub-Manager (Java/Spring Boot)](../Sub-Manager)，
追求更小的内存占用、更快的启动速度、更小的部署镜像。

从多个机场拉取订阅链接，本地缓存 → 交给 subconverter 合并渲染成不同格式的"组合订阅" →
通过固定 URL 对外发布给 Clash / sing-box 等下游客户端。

设计边界：**本平台不实现任何协议解析**，所有协议识别、格式转换、规则融合全部由独立的
subconverter 容器完成。

---

## 技术栈

Go 1.23+ · 标准库 `net/http`（Go 1.22+ 的 `ServeMux` method+pattern 路由，无第三方 web 框架）·
`html/template`（标准库，零依赖）· SQLite（`modernc.org/sqlite`，纯 Go 驱动，无需 cgo）·
手写 repository（无 ORM）· DaisyUI + Tailwind（CSS 静态编译产物，embed 进二进制）+ htmx + Alpine.js

对比 Java 版：**没有** Spring / JPA / Thymeleaf / Gradle 前端构建任务；DB 从 H2 换成 SQLite；
所有 CSS 资源 embed 进单一二进制，容器镜像 ~27MB（对比 Java 版 JRE 基础镜像 200MB+）。

## 数据模型

| 表 | 说明 |
|---|---|
| `airport` | 机场配置 + 本地缓存的原始订阅内容（`cached_raw_content`, `cached_userinfo`）+ 拉取健康状态 |
| `profile` | 对外发布的组合订阅 = `{slug, target_format, external_config, cached_content}` |
| `profile_airports` | Profile ↔ Airport 的多对多选择关系（空 = 该 profile 使用所有启用机场） |
| `app_settings` | 单行全局配置（刷新间隔 / 各类超时 / 服务地址），可在 `/admin/settings` 运行时修改 |

## 主要路由

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/profile/{slug}` | 对外订阅发布端点（`Cache-Control: no-store`；聚合返回 `Subscription-Userinfo`） |
| GET | `/internal/airports/{id}/raw` | 内部端点：把本地缓存的机场原始订阅原样返回给 subconverter |
| GET/POST | `/admin/airports` `/admin/profiles` `/admin/settings` | 管理页 |
| POST | `/admin/refresh` | 立即全部刷新（拉全部启用机场 → 合成全部启用 profile） |
| GET | `/demo/airport/{name}` | 造假机场端点，方便本地测试多源合成（`SUBMANAGER_DEMO_ENABLED=false` 关闭） |

## 本地开发

```bash
go run ./cmd/submanager
```

默认监听 `:8080`，SQLite 文件在 `./data/submanager.db`。打开 <http://localhost:8080/admin>。
默认没有鉴权（同 Java 版：管理页鉴权交给前面的 Cloudflare Access）。

需要 subconverter 才能真跑合成：

```bash
docker compose up -d subconverter
```

然后在 `/admin/settings` 把 **Internal Base URL** 改成 `http://host.docker.internal:8080`，
subconverter 才能回调到本机跑的进程。

## Docker 部署

```bash
docker compose up -d --build
```

- 多阶段构建：`golang:1.25-bookworm` 编译 → `gcr.io/distroless/static-debian12:nonroot` 运行
  （无 shell、无包管理器，攻击面小；最终镜像 ~27MB）
- 依赖通过 `vendor/` 目录随仓库提交，构建时不需要访问 Go module proxy（离线可构建）
- 应用以 `nonroot` 用户运行，只监听容器内 `127.0.0.1:8080`（compose 里用 `127.0.0.1:8080:8080` 绑定），
  前面挂 `cloudflared` 走 Cloudflare Tunnel 暴露
- SQLite 数据文件持久化在 named volume `submanager-go-data:/app/data`

## 运行时可配置项（`/admin/settings`）

| 字段 | 默认 | 说明 |
|---|---|---|
| `refreshIntervalSeconds` | 3600 | 一轮完整刷新的间隔（30 - 86400 秒），改完立即重新排程生效，不需要重启 |
| `subconverterTimeoutSeconds` | 30 | 调用 subconverter 的 HTTP 超时（1 - 600 秒） |
| `airportFetchTimeoutSeconds` | 20 | 拉取单个机场原始订阅的 HTTP 超时（1 - 600 秒） |
| `subconverterBaseUrl` | `http://subconverter:25500` | subconverter 服务地址 |
| `internalBaseUrl` | `http://app:8080` | subconverter 回调本平台的前缀 |
| `publicBaseUrl` | *(空)* | 复制订阅 URL 按钮用的域名前缀，可空 |
| `internalSecret` | *(空)* | `/internal/airports/*/raw` 的可选访问 token |

## 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `SUBMANAGER_DB_PATH` | `data/submanager.db`（容器内 `/app/data/submanager.db`） | SQLite 文件路径 |
| `SUBMANAGER_DEMO_ENABLED` | `true` | 是否启用 `/demo/airport/{name}` 造假机场端点 |

## 前端资源重新生成（改模板后）

`internal/web/static/css/{daisyui.css,tailwind.css}` 已编译好并 embed 进二进制，日常开发不需要
任何构建步骤。只有新增了模板里没出现过的 Tailwind utility class 时才需要重新生成：

```bash
curl -fsSL -o /tmp/tailwindcss \
  https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.13/tailwindcss-macos-arm64
chmod +x /tmp/tailwindcss
echo '@tailwind base;@tailwind components;@tailwind utilities;' > /tmp/tw-src.css
/tmp/tailwindcss -c tailwind.config.js -i /tmp/tw-src.css \
  -o internal/web/static/css/tailwind.css --minify
```

## 和 Java 版的已知差异（简化点）

- Flash 提示消息用短生命周期 cookie 实现（`Max-Age=10s`，读取后立即清除），而非服务端 session；
  行为上等价于 Spring 的 `RedirectAttributes` 一次性属性
- 未实现自定义 404/500 错误页（管理页路由打错直接走 Go 默认的纯文本 404），公开端点
  （`/profile/*`、`/internal/*`）本来就是空 body 404，行为一致
- 没有 H2 console 等价物（SQLite 没有内置 web 控制台），需要检查数据直接用 `sqlite3` CLI 打开
  `data/submanager.db`
