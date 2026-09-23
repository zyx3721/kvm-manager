<div align="center">

<h1>KVM Manager</h1>

<p><b>KVM 虚拟化统一管理平台</b> — 宿主机 · 虚拟机 · 快照 · 存储池 · 网络池 · 任务 · 告警</p>

<p><b>简体中文</b> · <a href="README.en.md">English</a></p>

机房里的 KVM 宿主机，日常启停虚机、看资源、拍快照，通常要挨台 SSH 上去敲 `virsh`。  
KVM Manager 把它们收进一套**自托管**的控制台：Go 后端 + React 前端 + PostgreSQL + Redis，  
每台宿主机跑一个轻量 Agent 采集运行态并执行变更，平台侧统一展示、操作、告警与审计。

<p>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?labelColor=1f2937" alt="MIT License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white&labelColor=1f2937" alt="Go 1.25+"></a>
  <a href="https://react.dev/"><img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white&labelColor=1f2937" alt="React 19"></a>
  <img src="https://img.shields.io/badge/PostgreSQL-17-336791?labelColor=1f2937" alt="PostgreSQL 17">
  <img src="https://img.shields.io/badge/Redis-7-DC382D?labelColor=1f2937" alt="Redis 7">
</p>

<p>
  <b><a href="#项目预览">项目预览</a></b> ·
  <a href="#它做什么">它做什么</a> ·
  <a href="#怎么工作">怎么工作</a> ·
  <a href="#技术栈">技术栈</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#部署">部署</a> ·
  <a href="#权限模型">权限</a> ·
  <a href="#数据与安全">安全</a> ·
  <a href="#常见问题">常见问题</a> ·
  <a href="#项目结构">项目结构</a> ·
  <a href="#文档">文档</a>
</p>

</div>

---

KVM Manager 是一个面向 Libvirt/KVM 环境的统一运维控制台：后端（Go 1.25 + 标准库 `net/http`）负责账号权限、任务审计、告警通知与系统配置；每台 KVM 宿主机部署轻量 Agent，通过 `virsh` / `qemu-img` 采集宿主机与虚拟机运行态并执行启停、克隆、迁移、快照等变更；平台数据库只保存自身业务数据，宿主机与虚拟机运行态统一放在 Redis 缓存中，经 API 与 SSE 推送给 React 控制台。

## 项目预览

### 登录

内部系统，没有公开注册。支持本地密码、AD/LDAP 与企业微信扫码登录，找回密码走图形验证码 + 邮箱验证码。

![登录页](.github/images/kvm-manager-login.jpg)

### 控制台首页

仪表盘汇总宿主机与虚拟机运行态、资源使用与最近告警。侧栏按登录用户的权限逐项显示或隐藏，无权操作不会出现在界面上。

![控制台首页](.github/images/kvm-manager-home.jpg)

## 它做什么

- **虚拟机管理** — 虚拟机列表与详情、启动/关机/暂停/强制操作、创建/克隆/迁移/编辑（资源、介质、设备、XML）、VNC 控制台（noVNC 代理）、单机监控曲线；已运行虚拟机支持在预留上限内热扩容 CPU、内存与磁盘。
- **快照管理** — 从 Agent 实时获取快照列表，支持创建、恢复、删除，并在平台侧维护备注与标签。
- **宿主机与 Agent** — 宿主机 CPU/内存/存储/负载运行态与趋势曲线；Agent 登记、连接测试、手动同步，连续同步失败自动标记离线并生成告警，恢复后自动关闭。
- **存储池与网络池** — 存储池容量与卷管理、ISO 上传、卷克隆；网络池与宿主机接口管理，支持 DNS 写入与 NAT/ROUTE/BRIDGE 校验。
- **实时刷新** — 后端按环境变量定时触发全局轻量刷新，前端经 SSE 事件更新页面；手动刷新接口可触发 full 全量任务。
- **任务与审计** — 后台刷新、虚拟机操作等长耗时动作进任务体系并可跟踪进度；关键操作写审计日志，统一运维页面查询。
- **告警与通知** — Agent 离线、资源阈值等活跃告警进通知中心，支持 Webhook、邮件、飞书/企业微信/钉钉机器人与应用通知多媒介推送与恢复通知。
- **用户与权限** — 本地密码、AD/LDAP 与企业微信扫码登录（直连或统一认证中心模式）、找回密码邮件流程、企微账号绑定；内置 `admin` / `operator` / `viewer` 三角色，自定义角色从 34 项权限中勾选，勾选操作权限自动补齐对应查看权限。

**它不是**云平台：不做编排调度、镜像市场或多租户，管的是「已有 KVM 宿主机上的虚拟机运维」。

同一台宿主机上的日常运维，两种做法大致是这样：

| 现场 | SSH + virsh | 用 KVM Manager |
| --- | --- | --- |
| 看资源 | 挨台登录敲命令 | 宿主机/虚拟机运行态集中展示 |
| 启停虚机 | 记域名、敲命令 | 页面点击，操作进任务与审计 |
| 拍快照 | 手写快照名易冲突 | 平台统一命名、备注与标签 |
| 谁改的 | 无据可查 | 关键操作全量审计留痕 |
| 出故障 | 事后才发现 | Agent 离线与资源阈值主动告警 |
| 权限 | root 一把梭 | 34 项权限逐接口校验 |

## 怎么工作

```text
        浏览器
           │  http
           ▼
  ┌──────────────────────────────────────┐
  │  Nginx（容器内或宿主机）                │
  │  /            → 前端静态资源（SPA）    │
  │  /api/ · /swagger/ → Go 后端          │
  └──────────────────────────────────────┘
        │                   │
        ▼                   ▼
  React 19 控制台      Go 1.25 后端 :8080
  Vite 静态构建              │
                    ┌────────┼─────────────────┐
                    ▼        ▼                 ▼
              PostgreSQL   Redis        KVM 宿主机 × N
              业务数据     运行态缓存      kvm-agent :9443
              会话/审计    SSE 事件通道     │ virsh / qemu-img
                                          ▼
                                     libvirt 虚拟机
```

- **谁负责什么** — 控制台读取运行态缓存与业务数据做展示；启停、克隆、迁移、快照等变更由后端校验权限、创建任务并下发 Agent 执行，完成后写审计。
- **数据放哪** — PostgreSQL 只保存用户、会话、Agent 登记、任务、审计、告警、系统配置等项目自身数据；宿主机与虚拟机运行态在 Redis 缓存中维护，可由 Agent 随时重建。
- **敏感信息** — Agent Token 加密落库，认证配置中的 Secret 接口返回时以 `hasSecret` 标记脱敏；仓库不提交 Token、密码与真实主机名。
- **对外暴露** — 只有健康检查、登录、认证方式列表、企业微信跳转与回调、找回密码流程、品牌信息接口可匿名访问，其余接口一律要求 `Authorization: Bearer <token>`。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端语言 | Go 1.25+（标准库 `net/http` 路由） |
| 数据库 | PostgreSQL 17（[pgx/v5](https://github.com/jackc/pgx) 驱动） |
| 缓存与事件 | Redis 7（[go-redis/v9](https://github.com/redis/go-redis)，运行态缓存 + SSE 事件通道） |
| 认证与加密 | 不透明会话 Token + PostgreSQL Session、AD/LDAP（go-ldap/ldap v3）、企业微信 OAuth 扫码登录（直连 / 统一认证中心 SSO）、bcrypt 口令散列、Agent Token 加密存储 |
| API 文档 | swag + http-swagger（Swagger UI） |
| 前端框架 | React 19 + TypeScript 5.9 + Vite 7（SPA 静态构建） |
| 样式与组件 | Tailwind CSS v4、lucide-react、sonner |
| 图表 | Recharts |
| 运行时打包 | Docker（Nginx + Supervisor 多进程） |
| 宿主机采集 | kvm-agent（`virsh` / `qemu-img`，Bearer Token 鉴权） |

## 快速开始

本地开发需要 **Go 1.25+**、**Node.js 20+**、**PostgreSQL 17** 与 **Redis 7**。

```bash
git clone https://github.com/zyx3721/kvm-manager.git
cd kvm-manager
```

**准备数据库**

```bash
createdb kvm_manager   # 或使用 deploy/docker-compose.yml 中的 PostgreSQL / Redis 服务
```

**后端**

```bash
cd backend
go mod download
cp .env.example .env      # 按需修改数据库、Redis 连接与密钥
go run ./cmd/server
```

后端默认监听 `http://localhost:8080`，首次启动自动执行数据库迁移并创建默认管理员 `admin / 123456`。

**前端**（另开一个终端）

```bash
cd frontend
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，开发服务器会把 `/api` 反向代理到 `http://localhost:8080`。

打开 `http://localhost:5173`，使用 `admin / 123456` 登录，**登录后立刻改密码**。Swagger 在 `http://localhost:8080/swagger/index.html`。

在 KVM 宿主机上部署 Agent 并回到控制台「Agent 管理」登记后，即可查看宿主机与虚拟机。Agent 的配置与启动见 [《完整手册》2.5 节](docs/manual.md)。

## 部署

只保留两种方式：**Docker 部署**（推荐）与 **Release 二进制部署**。源码编译、Agent 部署与 systemd 直跑的完整过程见 [《完整手册》](docs/manual.md)。

### 方式一：Docker 部署

使用仓库自带的 Compose 编排，一次拉起 PostgreSQL、Redis 与应用三个容器；应用镜像内置 Go 后端、Nginx 与前端静态资源，由 Supervisor 管理多进程，对外只暴露 80 端口：

```bash
git clone https://github.com/zyx3721/kvm-manager.git && cd kvm-manager/deploy
cp .env.example .env      # 至少修改 DB_PASSWORD、JWT_SECRET 与 REDIS_PASSWORD
docker compose up -d
```

数据全部落在 `deploy/` 下的本地目录：

```text
deploy/
├── KVMData/     # 应用运行日志
├── PgSqlData/   # PostgreSQL 数据
└── RedisData/   # Redis 数据（AOF 持久化）
```

核心环境变量（完整清单见 `.env.example` 与 [《完整手册》](docs/manual.md)）：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `JWT_SECRET` | `change-me-in-production` | 会话令牌签名密钥，生产环境必须显式设置为足够随机的长字符串 |
| `JWT_EXPIRE_HOURS` | `12` | 登录会话有效期（小时），会话签发时固定到期时间，注销后立即失效 |
| `SERVER_MODE` | `release` | 运行模式；`release` 下启动页不回显调试验证码 |
| `DB_HOST` / `DB_PORT` | `postgres` / `5432` | PostgreSQL 连接地址 |
| `DB_NAME` / `DB_USER` / `DB_PASSWORD` | `kvm-manager` / `postgres` / `123456ok!` | 数据库与账号 |
| `REDIS_ADDR` / `REDIS_PASSWORD` | `redis:6379` / `123456` | Redis 连接地址与密码 |
| `RUNTIME_SYNC_INTERVAL` | `30s` | 全局运行态轻量刷新间隔 |
| `RUNTIME_DEEP_SYNC_INTERVAL` | `10m` | 全量深同步间隔 |
| `METRIC_RETENTION_DAYS` | `30` | 指标样本保留天数 |

服务管理：

```bash
docker compose ps                     # 查看运行状态
docker logs -f kvm-manager            # 查看实时日志
docker compose restart kvm-manager    # 重启

# 升级到新镜像：拉取后重建应用容器（数据保留在本地目录）
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/kvm-manager:latest
docker compose up -d --force-recreate kvm-manager
```

**最后一步**：在各台 KVM 宿主机上部署 `kvm-agent` 并登记到控制台「Agent 管理」，步骤见下文二进制部署第 6 步或 [《完整手册》2.5 节](docs/manual.md)。

### 方式二：Release 二进制部署

前往 [GitHub Releases](https://github.com/zyx3721/kvm-manager/releases) 页面，按自己的 CPU 架构下载对应压缩包，再按下面步骤校验、解压、配置、启动。

**下载哪些包**

| 用途 | 下载文件 |
| --- | --- |
| 后端服务（Linux x86_64） | `kvm-manager_<版本>_linux_amd64.tar.gz` |
| 后端服务（Linux ARM64，鲲鹏、飞腾等） | `kvm-manager_<版本>_linux_arm64.tar.gz` |
| KVM 宿主机 Agent（x86_64 宿主机） | `kvm-agent_<版本>_linux_amd64.tar.gz` |
| KVM 宿主机 Agent（ARM64 宿主机） | `kvm-agent_<版本>_linux_arm64.tar.gz` |
| 前端界面（必选） | `kvm-manager-frontend_<版本>.tar.gz` |
| 校验和 | 同名 `*_checksums.txt` |

后端包内是 `kvm-manager` 可执行文件、`.env.example` 与 `README.txt`；前端包是 Vite 静态构建产物（`index.html` 与 `assets/`）；Agent 包内是 `kvm-agent` 可执行文件。二进制无外部运行时依赖，下载后可直接运行。

**1. 校验并解压**

```bash
VERSION=1.1.6
mkdir -p /data/kvm-manager && cd /data/kvm-manager
sha256sum -c kvm-manager_${VERSION}_checksums.txt

mkdir -p backend frontend
tar -xzf kvm-manager_${VERSION}_linux_amd64.tar.gz -C backend --strip-components=1
tar -xzf kvm-manager-frontend_${VERSION}.tar.gz -C frontend
```

得到的目录结构：

```text
/data/kvm-manager/
├── backend/
│   ├── kvm-manager      # 后端二进制
│   ├── .env.example
│   └── .env             # 第 2 步创建
└── frontend/
    ├── index.html
    └── assets/          # 浏览器静态资源
```

**2. 配置并启动后端**

```bash
cd /data/kvm-manager/backend
cp .env.example .env
vim .env               # 至少设置 DB_*、REDIS_* 与 JWT_SECRET
./kvm-manager
```

后端默认监听 `:8080`，首次启动自动执行数据库迁移（含企业微信认证表）并创建默认管理员 `admin / 123456`。需要常驻时交给 systemd：

```ini
# /etc/systemd/system/kvm-manager.service
[Unit]
Description=KVM Manager Backend
After=network.target postgresql.service redis.service

[Service]
Type=simple
WorkingDirectory=/data/kvm-manager/backend
ExecStart=/data/kvm-manager/backend/kvm-manager
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now kvm-manager
```

**命令行参数与版本查询**

后端支持命令行参数临时覆盖 `.env` 配置（显式参数优先于环境变量），并支持查询构建版本：

```bash
./kvm-manager -v              # 查看版本信息（版本号、commit、构建时间、Go 版本）
./kvm-manager -h              # 查看全部命令行参数

# 示例：临时覆盖监听端口与会话有效期启动
./kvm-manager -server-port 9090 -jwt-expire-hours 24
```

常用参数与对应环境变量：`-server-host` / `-server-port` / `-server-mode`（`SERVER_*`）、`-db-host` / `-db-port` / `-db-name` / `-db-user` / `-db-password` / `-db-sslmode`（`DB_*`）、`-jwt-secret` / `-jwt-expire-hours`（`JWT_*`）、`-redis-addr` / `-redis-password` / `-redis-db`（`REDIS_*`）以及各运行态刷新与保留天数参数，完整清单以 `-h` 输出为准。

**3. 部署前端静态资源（Nginx）**

前端是 Vite 构建的纯 SPA，交给 Nginx 托管静态文件并把 API 反代到后端即可，无需 Node.js：

```nginx
server {
    listen 80;
    server_name your-domain.com;
    client_max_body_size 50g;    # ISO 上传等大文件场景

    root /data/kvm-manager/frontend;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # SSE 事件通道：关闭缓冲，保证前端实时收到刷新事件
    location = /api/events {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 1h;
        proxy_send_timeout 1h;
        add_header X-Accel-Buffering no;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
    }

    location /swagger/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }

    location = /health {
        proxy_pass http://127.0.0.1:8080/api/health;
    }
}
```

**4. 部署 KVM 宿主机 Agent**

在每台 KVM 宿主机上（需可执行 `virsh`，通常即 libvirt 所在机器）：

```bash
VERSION=1.1.6
mkdir -p /data/kvm-agent && cd /data/kvm-agent
sha256sum -c kvm-agent_${VERSION}_checksums.txt
tar -xzf kvm-agent_${VERSION}_linux_amd64.tar.gz --strip-components=1
```

配置环境变量后启动（Token 需与控制台「Agent 管理」登记时一致）：

```bash
export AGENT_HOST=0.0.0.0
export AGENT_PORT=9443
export AGENT_TOKEN=请替换为足够随机的长字符串
export LIBVIRT_URI=qemu:///system
./kvm-agent
```

回到控制台「Agent 管理」登记该宿主机（地址 + Token），连接测试通过后即可查看宿主机与虚拟机。

**5. 访问**

同 Docker 方式：控制台 `http://your-host/`（`admin / 123456`）、接口文档 `/swagger/index.html`、健康检查 `/health`。

## 权限模型

接口按角色权限逐个校验，无权限返回 403。权限 Key 形如 `模块.资源.操作`，共 34 项，勾选任一操作权限会自动补齐其对应的查看权限。

| 身份 | 默认权限 |
| --- | --- |
| 内置角色 `admin` | 全部 34 项 |
| 内置角色 `operator` | 日常运维操作：宿主机、虚拟机、快照、存储池、网络池、Agent 等资源操作与查看；不含系统配置修改、删除与强制操作等高风险权限 |
| 内置角色 `viewer` | 全部只读：各资源查看与监控，不含任何写操作 |
| 自定义角色 | 从 34 项中勾选；系统配置拆分为基础配置、用户配置、认证配置、通知配置四组独立的查看/管理权限 |

按模块划分（完整清单见 [《完整手册》](docs/manual.md)）：

- **资源运维** — `dashboard.read`、`hosts.*`、`host.interfaces.*`、`vms.*`、`snapshots.*`、`storage.*`、`network.*`、`agents.*`
- **运维页面** — `operations.read`、`alerts.read` / `alerts.manage`
- **系统配置** — `settings.{base|users|auth|notifications}.read` / `.manage`

## 数据与安全

- **先改默认密码** — 首次部署后立即修改 `admin` 的默认口令。
- **显式设置 JWT 密钥** — 生产环境必须把 `JWT_SECRET` 设置为足够随机的长字符串；重设后全部已签发会话立即失效。
- **启用 HTTPS** — 生产环境通过 Nginx 配置证书，参考 [《完整手册》](docs/manual.md)。
- **保护数据目录** — `deploy/` 下的 `PgSqlData/`、`RedisData/`、`KVMData/` 内含数据库、缓存持久化与运行日志，目录权限只授予运行服务的账号；`.env` 已在 `.gitignore` 中排除。
- **收紧 Redis** — Redis 承载运行态缓存与 SSE 事件通道，必须设置 `requirepass`，不要裸奔在内网。
- **Agent Token** — 每台宿主机 Agent 使用独立强随机 Token，平台侧加密存储；泄露后在「Agent 管理」重置即可。
- **审计与告警** — 关键操作全量写审计日志，Agent 离线与资源阈值自动生成活跃告警并支持多媒介推送，建议接入值班可用的通知媒介。

## API 文档

后端集成 Swagger/OpenAPI，启动后即可查看在线接口文档：

- **Swagger UI**：`http://localhost:8080/swagger/index.html`
- **健康检查**：`GET /api/health`、`GET /health`（容器内 Nginx）

无需认证的接口只有：`POST /api/auth/login`、`GET /api/auth/providers`、企业微信认证接口（`/api/auth/wecom/*`）、找回密码相关接口、`GET /api/public/base-config` 与 `GET /api/health`；其余接口均需在请求头携带 `Authorization: Bearer <token>`。

按模块分组的完整接口清单（认证、仪表盘、宿主机、虚拟机、快照、存储池、网络池、任务、审计、告警、通知、系统配置、Agent API）见 [《完整手册》第九章](docs/manual.md)。

修改接口后，在 `backend/` 目录执行以下命令同步 Swagger 产物：

```bash
swag init -g cmd/server/main.go -o docs
```

## 数据库

PostgreSQL 17，默认库名 `kvm-manager`。后端启动时自动按文件名顺序执行 `backend/pkg/database/migrations/` 下的迁移脚本并记录在 `schema_migrations` 表，重复启动自动跳过。

| 分组 | 说明 |
| --- | --- |
| 用户与会话 | `users`、`sessions`、角色权限与用户群组相关表 |
| 资源登记 | `agents`（KVM 宿主机 Agent 连接信息与状态） |
| 任务与审计 | `tasks`、`audit_logs`、`history` 相关表 |
| 告警与通知 | `alerts`、`alert_notification_deliveries`、`notifications`、通知媒介配置表 |
| 系统配置 | `system_settings`（基础配置、通知媒介、认证提供方） |
| 企业微信认证 | `auth_providers`、`auth_login_states`（OAuth 一次性 state）、`auth_wecom_bindings`（企微账号绑定） |
| 指标样本 | 宿主机与虚拟机指标趋势表（按 `METRIC_RETENTION_DAYS` 保留） |

平台**不创建**宿主机、虚拟机、快照资源表——此类运行态数据由 Agent 随时重新获取，统一放在 Redis。每张表的字段与用途见 [《完整手册》](docs/manual.md)。

## 常见问题

**怎么启用企业微信扫码登录？**

在企业微信管理后台「应用管理」创建自建应用，记录 AgentID 与 Secret；在应用的「网页授权及 JS-SDK」中将本系统访问域名配置为可信域名，并在「企业可信 IP」中加入本服务出口 IP。再到「系统配置 → 认证配置 → 企业微信」填写企业 ID（corpid）、应用 AgentID 与应用 Secret 并启用（回调地址前缀可留空，按当前访问地址自动推断）。用户先用账号密码登录，在右上角菜单「绑定企微」扫码完成账号关联，之后即可在登录页选择企业微信扫码登录。登录页选择企业微信后默认在页面内内嵌渲染扫码二维码，扫码确认后页面内完成登录，配置异常时自动回退为整页跳转。

若企业内部已部署统一认证中心（wecom-auth-center），可在企业微信认证配置中将「认证方式」切换为「统一认证中心」，填写认证中心地址、应用标识与应用密钥；认证中心侧为本系统配置 `domain`（本系统外部访问地址）与 `callback_path`（`/api/auth/wecom/sso/callback`）即可，多个内部系统可共用同一套企微应用配置，绑定与登录交互保持一致。统一认证中心模式下内嵌二维码需认证中心登录页允许被本系统 iframe 嵌入（调整 X-Frame-Options / CSP frame-ancestors），未允许时登录页自动回退整页跳转。

**Agent 一直显示离线怎么办？**

按顺序排查：宿主机上 Agent 进程是否存活（`systemctl status kvm-agent` 或进程列表）；控制台「Agent 管理」登记的地址与端口能否连通；Agent 的 `AGENT_TOKEN` 是否与登记一致；后端到 Agent 的网络是否放通 `AGENT_PORT`。连续同步失败达到「Agent 离线失败次数」阈值后会标记离线并生成告警，恢复同步后自动关闭。更多排查手段见 [Agent 命令超时与临时目录说明](docs/agent-command-timeout-and-temp-dirs.md)。

**忘记 admin 密码怎么办？**

本地账号走登录页「忘记密码」流程（图形验证码 + 邮箱验证码，需先在通知媒介中启用邮件的找回密码用途并为 admin 配置邮箱）；或由其他管理员在「系统配置 → 用户配置」中重置该账号密码。

**虚拟机控制台（noVNC）打不开怎么办？**

控制台经后端代理到宿主机 VNC 端口，需保证浏览器能访问平台、平台后端能访问宿主机 VNC 监听地址；宿主机侧 `virsh` 查询虚拟机图形监听配置（VNC Splice/监听地址）是否允许远程连接。详见 [《完整手册》](docs/manual.md)。

**修改接口或采集逻辑后要注意什么？**

接口变更需同步 Swagger 注解并在 `backend/` 下执行 `swag init -g cmd/server/main.go -o docs`；采集口径与刷新边界的变化需同步 `docs/` 下的采集与刷新文档，详见 [《完整手册》](docs/manual.md)。

## 项目结构

```text
kvm-manager/
├── agent/                         # 部署在 KVM 宿主机上的 Agent
│   ├── api/router/                # Agent HTTP 路由、处理函数和控制台代理
│   ├── cmd/agent/                 # Agent 启动入口
│   ├── config/                    # Agent 环境变量配置加载
│   └── internal/                  # Agent 内部 KVM 操作与鉴权能力
│       ├── kvm/                   # virsh 采集、接口、控制台、动作、热扩容、介质、解析、克隆与存储池校验等实现
│       └── security/              # Agent Bearer Token 鉴权
├── backend/                       # Go 后端控制中心
│   ├── api/router/                # HTTP API 路由、处理函数、中间件、Swagger 注解与文档模型
│   ├── cmd/server/                # 后端启动入口
│   ├── config/                    # 后端环境变量配置加载
│   ├── docs/                      # 后端 Swagger/OpenAPI 生成文件
│   ├── internal/                  # 后端内部领域、仓储和业务服务
│   │   ├── domain/                # 后端领域模型
│   │   ├── repository/            # PostgreSQL 仓储、查询和保留清理
│   │   └── service/               # 用户认证、通知和实时同步等业务服务
│   ├── pkg/                       # 后端基础设施与可复用能力
│   │   ├── agent/                 # 访问 KVM Agent 的客户端与 VM 相关请求模型
│   │   ├── database/              # PostgreSQL 连接与初始化迁移
│   │   └── tokencrypto/           # Agent Token 加密存储
│   └── .env.example               # 环境变量模板
├── deploy/                        # Docker Compose、Nginx、Supervisor 和容器入口配置
├── docs/                          # 完整手册、采集说明与前端控件行为文档
├── frontend/                      # React 前端应用（Vite SPA）
│   └── src/
│       ├── app/                   # 应用入口组件与全局路由
│       ├── components/            # 跨业务域复用组件（布局、启动页、统一下拉等）
│       ├── features/              # 页面功能模块，按业务域组织页面、组件、类型与工具
│       └── lib/                   # API 客户端、认证、格式化、刷新事件与主题工具
├── .github/workflows/             # GitHub Actions 构建与发布流水线
├── verchanglog/                   # 版本更新日志
├── AGENTS.md                      # 项目开发规范（AI 助手协作约定）
├── LICENSE
├── README.md                      # 中文说明（本文件）
└── docs/manual.md                 # 完整手册（含全量接口清单、环境变量与部署细节）
```

## 文档

| 先看这个 | 再往下 |
| --- | --- |
| [快速开始](#快速开始) | 本地起后端与前端，默认账号与端口 |
| [部署](#部署) | Docker 与 Release 二进制两条路径、环境变量、反向代理 |
| [权限模型](#权限模型) | 34 项权限怎么分组、内置角色各有什么 |
| [完整手册](docs/manual.md) | 全量接口清单、环境变量、本地源码部署、Agent 细节与历史版本 |
| [虚拟机采集口径](docs/vm-info-collection.md) | 虚拟机字段来源、采集命令与计算回退策略 |
| [宿主机采集口径](docs/host-info-collection.md) | 宿主机字段来源、资源使用率与趋势口径 |
| [刷新功能说明](docs/frontend-refresh-functions.md) | 自动刷新、手动刷新、SSE 事件与各页面刷新入口 |
| [操作日志覆盖](docs/operation-log-coverage.md) | 任务、审计、告警与通知投递的覆盖范围 |

## 版本历史

| 版本 | 发布日期 | 更新日志 |
| --- | --- | --- |
| v1.2.4 | 2026-09-23 | [verchanglog/v1.2.4.md](verchanglog/v1.2.4.md) |
| v1.2.3 | 2026-09-23 | [verchanglog/v1.2.3.md](verchanglog/v1.2.3.md) |
| v1.2.2 | 2026-09-22 | [verchanglog/v1.2.2.md](verchanglog/v1.2.2.md) |
| v1.2.1 | 2026-09-22 | [verchanglog/v1.2.1.md](verchanglog/v1.2.1.md) |
| v1.2.0 | 2026-09-20 | [verchanglog/v1.2.0.md](verchanglog/v1.2.0.md) |
| v1.1.6 | 2026-06-11 | [verchanglog/v1.1.6.md](verchanglog/v1.1.6.md) |
| v1.1.5 | 2026-06-09 | [verchanglog/v1.1.5.md](verchanglog/v1.1.5.md) |
| v1.1.4 | 2026-06-08 | [verchanglog/v1.1.4.md](verchanglog/v1.1.4.md) |
| v1.1.3 | 2026-06-08 | [verchanglog/v1.1.3.md](verchanglog/v1.1.3.md) |
| v1.1.2 | 2026-06-08 | [verchanglog/v1.1.2.md](verchanglog/v1.1.2.md) |
| v1.1.1 | 2026-06-08 | [verchanglog/v1.1.1.md](verchanglog/v1.1.1.md) |
| v1.1.0 | 2026-06-07 | [verchanglog/v1.1.0.md](verchanglog/v1.1.0.md) |
| v1.0.0 | 2026-06-07 | [verchanglog/v1.0.0.md](verchanglog/v1.0.0.md) |

各版本的构建产物与发布说明见 [GitHub Releases](https://github.com/zyx3721/kvm-manager/releases)。

## 致谢

感谢以下开源项目与技术社区：

- [jackc/pgx](https://github.com/jackc/pgx) — PostgreSQL 高性能驱动
- [redis/go-redis](https://github.com/redis/go-redis) — Redis 客户端
- [go-ldap/ldap](https://github.com/go-ldap/ldap) — LDAP 认证支持
- [swaggo/swag](https://github.com/swaggo/swag) — Swagger 文档生成工具
- [novnc/noVNC](https://github.com/novnc/noVNC) — 浏览器 VNC 控制台
- React、Vite 与 Tailwind CSS — 前端构建体系

## 许可证

本项目采用 [MIT License](LICENSE) 开源协议，可自由使用、复制、修改、合并、发布、分发、再许可与销售，只需在所有副本或重要部分中保留版权声明与许可声明。

## 联系方式

- **Email**：416685476@qq.com
- **GitHub Issues**：[zyx3721/kvm-manager/issues](https://github.com/zyx3721/kvm-manager/issues)
- **项目主页**：[github.com/zyx3721/kvm-manager](https://github.com/zyx3721/kvm-manager)

---

**⭐ 如果这个项目对您有帮助，欢迎 Star 支持！**
