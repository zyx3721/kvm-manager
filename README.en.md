<div align="center">

<h1>KVM Manager</h1>

<p><b>Unified KVM virtualization management</b> — hosts · VMs · snapshots · storage pools · network pools · tasks · alerts</p>

<p><a href="README.md">简体中文</a> · <b>English</b></p>

Day-to-day KVM housekeeping — starting VMs, checking resources, taking snapshots — usually means SSH-ing into every host and typing `virsh` by hand.  
KVM Manager pulls it into one **self-hosted** console: a Go backend, a React frontend, PostgreSQL and Redis.  
A lightweight agent on each KVM host collects runtime state and applies changes, while the platform gives you a single place to view, operate, alert and audit.

<p>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?labelColor=1f2937" alt="MIT License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white&labelColor=1f2937" alt="Go 1.25+"></a>
  <a href="https://react.dev/"><img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white&labelColor=1f2937" alt="React 19"></a>
  <img src="https://img.shields.io/badge/PostgreSQL-17-336791?labelColor=1f2937" alt="PostgreSQL 17">
  <img src="https://img.shields.io/badge/Redis-7-DC382D?labelColor=1f2937" alt="Redis 7">
</p>

<p>
  <b><a href="#preview">Preview</a></b> ·
  <a href="#what-it-does">What it does</a> ·
  <a href="#how-it-works">How it works</a> ·
  <a href="#tech-stack">Tech stack</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#deployment">Deployment</a> ·
  <a href="#permission-model">Permissions</a> ·
  <a href="#data-and-security">Security</a> ·
  <a href="#faq">FAQ</a> ·
  <a href="#repository-layout">Layout</a> ·
  <a href="#documentation">Docs</a>
</p>

</div>

---

KVM Manager is a unified operations console for Libvirt/KVM environments: the backend (Go 1.25 + the standard-library `net/http` router) owns accounts, permissions, tasks, auditing, alerting and system settings; a lightweight agent on every KVM host collects host and VM runtime state via `virsh` / `qemu-img` and applies changes such as start/stop, clone, migrate and snapshot; the database stores only the platform's own data, while host and VM runtime state lives in a Redis cache served to the React console over the API and SSE.

## Preview

### Sign in

An internal system with no public registration. Local passwords, AD/LDAP and WeCom (WeChat Work) QR-code sign-in are supported, with password recovery over a captcha plus an email code.

![Sign-in page](.github/images/kvm-manager-login.jpg)

### Console home

A dashboard summary of host and VM runtime state, resource usage and recent alerts. The sidebar shows or hides each entry based on the signed-in user's permissions, so actions you cannot perform never appear.

![Console home](.github/images/kvm-manager-home.jpg)

## What it does

- **VM management** — VM list and details, start/stop/pause/force operations, create/clone/migrate/edit (resources, media, devices, XML), noVNC console proxying, per-VM monitoring charts; running VMs support hot-expanding CPU, memory and existing disks within their reserved headroom.
- **Snapshot management** — live snapshot lists from the agent, create/restore/delete, plus platform-side notes and tags.
- **Hosts and agents** — host CPU/memory/storage/load runtime state and trend charts; agent registration, connection tests and manual sync. Repeated sync failures mark the agent offline and raise an alert, which clears automatically once syncing recovers.
- **Storage and network pools** — storage pool capacity and volume management, ISO upload, volume cloning; network pools and host interface management with DNS writes and NAT/ROUTE/BRIDGE validation.
- **Real-time refresh** — the backend triggers a global lightweight refresh on a schedule, and the frontend updates pages through SSE events; manual refresh endpoints can still trigger a full sync.
- **Tasks and audit** — long-running actions such as background refreshes and VM operations go through the task system with trackable progress; key operations are written to the audit log and browsable in a unified operations page.
- **Alerts and notifications** — active alerts (agent offline, resource thresholds) land in the notification center and can push to webhooks, email, Feishu/WeCom/DingTalk robots and app notifications, including recovery notices.
- **Users and permissions** — local passwords, AD/LDAP and WeCom QR-code sign-in (direct or unified auth-center mode), an email password-recovery flow and WeCom account binding; three built-in roles (`admin` / `operator` / `viewer`), custom roles picking from 34 permissions, and picking any manage permission automatically grants its read counterpart.

**It is not** a cloud platform: no scheduling, no image marketplace, no multi-tenancy. It manages *day-to-day VM operations on the KVM hosts you already have*.

The same daily routine, two ways of running it:

| Today | SSH + virsh | With KVM Manager |
| --- | --- | --- |
| Check resources | Log in host by host | Host and VM runtime state in one place |
| Start/stop VMs | Remember names, type commands | Click in the console; operations become tasks and audit entries |
| Take snapshots | Hand-written snapshot names | Platform-managed naming, notes and tags |
| Who changed what | No trail | Key operations fully audited |
| When things break | Found after the fact | Agent offline and threshold alerts raised proactively |
| Permissions | Root for everyone | 34 permissions checked per request |

## How it works

```text
        Browser
           │  http
           ▼
  ┌──────────────────────────────────────┐
  │  Nginx (in the container or on host) │
  │  /            → frontend assets (SPA)│
  │  /api/ · /swagger/ → Go backend      │
  └──────────────────────────────────────┘
        │                   │
        ▼                   ▼
  React 19 console    Go 1.25 backend :8080
  Vite static build         │
                    ┌────────┼─────────────────┐
                    ▼        ▼                 ▼
              PostgreSQL   Redis        KVM hosts × N
              business     runtime       kvm-agent :9443
              data/sessions  cache/SSE    │ virsh / qemu-img
                                          ▼
                                     libvirt VMs
```

- **Who does what** — the console renders runtime state and business data; start/stop, clone, migrate and snapshot changes are authorized by the backend, turned into tasks and dispatched to the agent, then audited.
- **Where data lives** — PostgreSQL keeps only the platform's own data: users, sessions, agent registrations, tasks, audit, alerts and settings. Host and VM runtime state lives in Redis and can always be rebuilt from the agents.
- **Secrets** — agent tokens are stored encrypted, and authentication secrets are redacted from API responses with a `hasSecret` flag; the repository holds no tokens, no passwords and no real hostnames.
- **Exposure** — only health checks, sign-in, the provider list, WeCom authorize/callback, the password-recovery flow and branding endpoints are anonymous; everything else requires `Authorization: Bearer <token>`.

## Tech stack

| Layer | Choice |
| --- | --- |
| Backend | Go 1.25+ (standard-library `net/http` router) |
| Database | PostgreSQL 17 ([pgx/v5](https://github.com/jackc/pgx)) |
| Cache and events | Redis 7 ([go-redis/v9](https://github.com/redis/go-redis), runtime cache + SSE channel) |
| Auth and crypto | Opaque session tokens + PostgreSQL sessions, AD/LDAP (go-ldap/ldap v3), WeCom OAuth QR sign-in (direct / unified auth-center SSO), bcrypt password hashing, encrypted agent tokens |
| API docs | swag + http-swagger (Swagger UI) |
| Frontend | React 19 + TypeScript 5.9 + Vite 7 (static SPA build) |
| Styling and UI | Tailwind CSS v4, lucide-react, sonner |
| Charts | Recharts |
| Runtime packaging | Docker (Nginx + Supervisor multi-process) |
| Host collection | kvm-agent (`virsh` / `qemu-img`, Bearer token auth) |

## Quick start

Local development needs **Go 1.25+**, **Node.js 20+**, **PostgreSQL 17** and **Redis 7**.

```bash
git clone https://github.com/zyx3721/kvm-manager.git
cd kvm-manager
```

**Prepare the database**

```bash
createdb kvm_manager   # or use the PostgreSQL / Redis services from deploy/docker-compose.yml
```

**Backend**

```bash
cd backend
go mod download
cp .env.example .env      # adjust database, Redis and secrets as needed
go run ./cmd/server
```

The backend listens on `http://localhost:8080` by default. On first start it runs the database migrations automatically and creates the default admin `admin / 123456`.

**Frontend** (in another terminal)

```bash
cd frontend
npm install
npm run dev
```

The frontend runs on `http://localhost:5173` by default and proxies `/api` to `http://localhost:8080`.

Open `http://localhost:5173`, sign in with `admin / 123456`, and **change the password right away**. Swagger lives at `http://localhost:8080/swagger/index.html`.

Deploy an agent on each KVM host and register it under "Agent Management" in the console to see hosts and VMs. Agent setup is described in [the full manual, section 2.5](docs/manual.md).

## Deployment

Two paths only: **Docker deployment** (recommended) and **Release binary deployment**. Source builds, agent details and plain systemd runs are covered in [the full manual](docs/manual.md).

### Option 1: Docker deployment

The bundled Compose file brings up PostgreSQL, Redis and the app. The app image contains the Go backend, Nginx and the frontend static assets, managed by Supervisor with only port 80 exposed:

```bash
git clone https://github.com/zyx3721/kvm-manager.git && cd kvm-manager/deploy
cp .env.example .env      # at minimum change DB_PASSWORD, JWT_SECRET and REDIS_PASSWORD
docker compose up -d
```

All data lands in local directories under `deploy/`:

```text
deploy/
├── KVMData/     # application logs
├── PgSqlData/   # PostgreSQL data
└── RedisData/   # Redis data (AOF persistence)
```

Key environment variables (the full list lives in `.env.example` and [the full manual](docs/manual.md)):

| Variable | Default | Description |
| --- | --- | --- |
| `JWT_SECRET` | `change-me-in-production` | Session signing secret; set a long random value in production |
| `JWT_EXPIRE_HOURS` | `12` | Session lifetime in hours; the expiry is fixed at sign-in and sessions are revoked immediately on logout |
| `SERVER_MODE` | `release` | Run mode; `release` hides debug captchas on the boot screen |
| `DB_HOST` / `DB_PORT` | `postgres` / `5432` | PostgreSQL address |
| `DB_NAME` / `DB_USER` / `DB_PASSWORD` | `kvm-manager` / `postgres` / `123456ok!` | Database and credentials |
| `REDIS_ADDR` / `REDIS_PASSWORD` | `redis:6379` / `123456` | Redis address and password |
| `RUNTIME_SYNC_INTERVAL` | `30s` | Global lightweight refresh interval |
| `RUNTIME_DEEP_SYNC_INTERVAL` | `10m` | Full deep-sync interval |
| `METRIC_RETENTION_DAYS` | `30` | Metric sample retention in days |

Service management:

```bash
docker compose ps                     # status
docker logs -f kvm-manager            # live logs
docker compose restart kvm-manager    # restart

# Upgrade: pull the new image, then recreate the app container (data stays in local directories)
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/kvm-manager:latest
docker compose up -d --force-recreate kvm-manager
```

**Last step**: deploy `kvm-agent` on every KVM host and register it under "Agent Management" — see step 4 of the binary deployment below or [the full manual, section 2.5](docs/manual.md).

### Option 2: Release binary deployment

Head to the [GitHub Releases](https://github.com/zyx3721/kvm-manager/releases) page, download the archives for your CPU architecture, then verify, extract, configure and start.

**Which files to download**

| Purpose | File |
| --- | --- |
| Backend (Linux x86_64) | `kvm-manager_<version>_linux_amd64.tar.gz` |
| Backend (Linux ARM64, Kunpeng/Phytium) | `kvm-manager_<version>_linux_arm64.tar.gz` |
| KVM host agent (x86_64 hosts) | `kvm-agent_<version>_linux_amd64.tar.gz` |
| KVM host agent (ARM64 hosts) | `kvm-agent_<version>_linux_arm64.tar.gz` |
| Frontend assets (always required) | `kvm-manager-frontend_<version>.tar.gz` |
| Checksums | matching `*_checksums.txt` |

The backend archive contains the `kvm-manager` binary, `.env.example` and a `README.txt`; the frontend archive is the Vite static build (`index.html` and `assets/`); the agent archive contains the `kvm-agent` binary. The binaries have no runtime dependencies.

**1. Verify and extract**

```bash
VERSION=1.2.0
mkdir -p /data/kvm-manager && cd /data/kvm-manager
sha256sum -c kvm-manager_${VERSION}_checksums.txt

mkdir -p backend frontend
tar -xzf kvm-manager_${VERSION}_linux_amd64.tar.gz -C backend --strip-components=1
tar -xzf kvm-manager-frontend_${VERSION}.tar.gz -C frontend
```

The resulting layout:

```text
/data/kvm-manager/
├── backend/
│   ├── kvm-manager      # backend binary
│   ├── .env.example
│   └── .env             # created in step 2
└── frontend/
    ├── index.html
    └── assets/          # browser assets
```

**2. Configure and start the backend**

```bash
cd /data/kvm-manager/backend
cp .env.example .env
vim .env               # at minimum set DB_*, REDIS_* and JWT_SECRET
./kvm-manager
```

The backend listens on `:8080` by default. On first start it runs the database migrations (including the WeCom tables) and creates the default admin `admin / 123456`. For a persistent service, use systemd:

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

**Command-line flags and version query**

The backend accepts command-line flags that temporarily override `.env` settings (explicit flags take precedence over environment variables) and supports build version queries:

```bash
./kvm-manager -v              # Show version info (version, commit, build date, Go version)
./kvm-manager -h              # Show all command-line flags

# Example: start with a temporary port and session TTL override
./kvm-manager -server-port 9090 -jwt-expire-hours 24
```

Common flags and their environment variable counterparts: `-server-host` / `-server-port` / `-server-mode` (`SERVER_*`), `-db-host` / `-db-port` / `-db-name` / `-db-user` / `-db-password` / `-db-sslmode` (`DB_*`), `-jwt-secret` / `-jwt-expire-hours` (`JWT_*`), `-redis-addr` / `-redis-password` / `-redis-db` (`REDIS_*`), plus the runtime refresh and retention flags. See `-h` output for the full list.

**3. Serve the frontend with Nginx**

The frontend is a plain Vite SPA: let Nginx serve the static files and proxy the API to the backend — no Node.js required:

```nginx
server {
    listen 80;
    server_name your-domain.com;
    client_max_body_size 50g;    # ISO uploads and other large files

    root /data/kvm-manager/frontend;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # SSE event channel: disable buffering so refresh events arrive in real time
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

**4. Deploy the KVM host agent**

On every KVM host (must be able to run `virsh`, i.e. typically the libvirt machine):

```bash
VERSION=1.2.0
mkdir -p /data/kvm-agent && cd /data/kvm-agent
sha256sum -c kvm-agent_${VERSION}_checksums.txt
tar -xzf kvm-agent_${VERSION}_linux_amd64.tar.gz --strip-components=1
```

Set the environment variables and start (the token must match what you register under "Agent Management"):

```bash
export AGENT_HOST=0.0.0.0
export AGENT_PORT=9443
export AGENT_TOKEN=replace-with-a-long-random-string
export LIBVIRT_URI=qemu:///system
./kvm-agent
```

Register the host under "Agent Management" (address + token). Once the connection test passes, hosts and VMs appear in the console.

**5. Sign in**

Same as Docker: console at `http://your-host/` (`admin / 123456`), API docs at `/swagger/index.html`, health check at `/health`.

## Permission model

Every request is checked against role permissions; unauthorized requests return 403. Permission keys look like `module.resource.action` — 34 in total — and picking any manage permission automatically grants its read counterpart.

| Identity | Default permissions |
| --- | --- |
| Built-in role `admin` | All 34 permissions |
| Built-in role `operator` | Day-to-day operations: hosts, VMs, snapshots, storage pools, network pools, agents and their views; no settings changes, deletions or force operations |
| Built-in role `viewer` | Read-only across the board: views and monitoring, no writes |
| Custom roles | Pick from the 34 permissions; system settings are split into four independent read/manage groups — base, users, auth and notifications |

By module (the full list lives in [the full manual](docs/manual.md)):

- **Resource operations** — `dashboard.read`, `hosts.*`, `host.interfaces.*`, `vms.*`, `snapshots.*`, `storage.*`, `network.*`, `agents.*`
- **Operations pages** — `operations.read`, `alerts.read` / `alerts.manage`
- **System settings** — `settings.{base|users|auth|notifications}.read` / `.manage`

## Data and security

- **Change the default password first** — update `admin` right after the first deployment.
- **Set an explicit JWT secret** — `JWT_SECRET` must be a long random value in production; rotating it invalidates every issued session.
- **Enable HTTPS** — terminate certificates in Nginx for production, see [the full manual](docs/manual.md).
- **Protect the data directories** — `PgSqlData/`, `RedisData/` and `KVMData/` under `deploy/` hold the database, cache persistence and logs; grant access only to the service account. `.env` is excluded via `.gitignore`.
- **Lock down Redis** — Redis carries the runtime cache and SSE channel; always set `requirepass`, never run it open on the network.
- **Agent tokens** — give every host agent its own strong random token; the platform stores them encrypted, and a leaked token can be reset under "Agent Management".
- **Audit and alerts** — key operations are fully audited, and agent-offline and threshold alerts push to your notification channels; wire up at least one on-call medium.

## API docs

The backend ships Swagger/OpenAPI, available as soon as it starts:

- **Swagger UI**: `http://localhost:8080/swagger/index.html`
- **Health check**: `GET /api/health`, `GET /health` (through the bundled Nginx)

Anonymous endpoints are limited to: `POST /api/auth/login`, `GET /api/auth/providers`, the WeCom endpoints (`/api/auth/wecom/*`), the password-recovery endpoints, `GET /api/public/base-config` and `GET /api/health`; everything else requires `Authorization: Bearer <token>`.

The complete per-module endpoint list (auth, dashboard, hosts, VMs, snapshots, storage pools, network pools, tasks, audit, alerts, notifications, settings, agent API) lives in [the full manual, chapter 9](docs/manual.md).

After changing an endpoint, regenerate the Swagger artifacts from `backend/`:

```bash
swag init -g cmd/server/main.go -o docs
```

## Database

PostgreSQL 17, database `kvm-manager` by default. On startup the backend applies the migrations in `backend/pkg/database/migrations/` in filename order and records them in `schema_migrations`; repeated starts skip applied versions.

| Group | Description |
| --- | --- |
| Users and sessions | `users`, `sessions`, plus role/permission and user-group tables |
| Resource registry | `agents` (KVM host agent connection info and state) |
| Tasks and audit | `tasks`, `audit_logs` and history tables |
| Alerts and notifications | `alerts`, `alert_notification_deliveries`, `notifications` and channel configuration tables |
| System settings | `system_settings` (base config, notification channels, auth providers) |
| WeCom authentication | `auth_providers`, `auth_login_states` (one-time OAuth state), `auth_wecom_bindings` (WeCom account binding) |
| Metric samples | Host and VM metric trend tables (retained per `METRIC_RETENTION_DAYS`) |

The platform creates **no** host, VM or snapshot tables — that runtime state is collected by the agents and kept in Redis. Field-level details live in [the full manual](docs/manual.md).

## FAQ

**How do I enable WeCom QR sign-in?**

Create a self-built app in the WeCom admin console and note the AgentID and Secret; add this system's domain as a trusted domain under "Web authorization & JS-SDK" and add the server's egress IP under "trusted IPs". Then fill in the corp ID, Agent ID and Secret under "Settings → Authentication → WeCom" and enable it (the redirect prefix can stay empty — it is inferred from the incoming request). Users sign in with their password once and bind their WeCom account from the top-right menu; afterwards they can pick WeCom on the sign-in page.

If your organization already runs a unified auth center (wecom-auth-center), switch "Authentication mode" to "Unified auth center" and fill in the center's address, app ID and app secret; on the center side configure `domain` (this system's external address) and `callback_path` (`/api/auth/wecom/sso/callback`). Multiple internal systems can share one WeCom app, and binding and sign-in behave identically.

**My agent keeps showing offline — what now?**

Check in order: the agent process is alive on the host (`systemctl status kvm-agent`); the address and port registered under "Agent Management" are reachable; `AGENT_TOKEN` matches the registration; the backend can reach `AGENT_PORT`. After the "offline failure count" threshold is crossed the agent is marked offline and an alert is raised, clearing automatically once syncing recovers. More diagnostics in [agent command timeouts and temp dirs](docs/agent-command-timeout-and-temp-dirs.md).

**I forgot the admin password — what now?**

Local accounts can use the "Forgot password" flow on the sign-in page (captcha + email code; enable the email channel's password-recovery purpose and set an address for admin first), or another admin can reset it under "Settings → Users".

**The noVNC console will not open — what now?**

The console is proxied by the backend to the host's VNC port: the browser must reach the platform and the backend must reach the host's VNC listener; check on the host that the VM's graphics listen address allows remote connections. More detail in [the full manual](docs/manual.md).

**What should I do after changing an endpoint or the collection logic?**

Endpoint changes need updated Swagger annotations plus `swag init -g cmd/server/main.go -o docs` in `backend/`; collection and refresh changes must update the collection and refresh documents under `docs/`. See [the full manual](docs/manual.md).

## Repository layout

```text
kvm-manager/
├── agent/                         # Agent deployed on KVM hosts
│   ├── api/router/                # Agent HTTP routes, handlers and console proxy
│   ├── cmd/agent/                 # Agent entry point
│   ├── config/                    # Agent environment configuration
│   └── internal/                  # Agent KVM operations and auth
│       ├── kvm/                   # virsh collection, interfaces, console, actions, hot-resize, media, parsing, cloning and pool checks
│       └── security/              # Agent Bearer token auth
├── backend/                       # Go control center
│   ├── api/router/                # HTTP routes, handlers, middleware, Swagger annotations and doc models
│   ├── cmd/server/                # Backend entry point
│   ├── config/                    # Backend environment configuration
│   ├── docs/                      # Generated Swagger/OpenAPI artifacts
│   ├── internal/                  # Domain models, repositories and services
│   │   ├── domain/                # Domain models
│   │   ├── repository/            # PostgreSQL repositories, queries and retention
│   │   └── service/               # Auth, notification and realtime services
│   ├── pkg/                       # Infrastructure and reusable capabilities
│   │   ├── agent/                 # KVM agent client and VM request models
│   │   ├── database/              # PostgreSQL connection and migrations
│   │   └── tokencrypto/           # Agent token encryption
│   └── .env.example               # Environment template
├── deploy/                        # Docker Compose, Nginx, Supervisor and entrypoint
├── docs/                          # Full manual, collection specs and frontend behavior docs
├── frontend/                      # React frontend (Vite SPA)
│   └── src/
│       ├── app/                   # App shell and global routes
│       ├── components/            # Cross-domain components (layout, boot screen, dropdowns, ...)
│       ├── features/              # Page modules grouped by domain
│       └── lib/                   # API client, auth, formatting, refresh events and theming
├── .github/workflows/             # GitHub Actions build and release pipelines
├── verchanglog/                   # Release notes
├── AGENTS.md                      # Project development conventions (AI collaboration)
├── LICENSE
├── README.md                      # 简体中文
└── docs/manual.md                 # Full manual (complete API list, environment variables and deployment details)
```

## Documentation

| Start here | Then read |
| --- | --- |
| [Quick start](#quick-start) | Run the backend and frontend locally; default account and ports |
| [Deployment](#deployment) | Docker and Release binary paths, environment variables, reverse proxy |
| [Permission model](#permission-model) | How the 34 permissions group and what the built-in roles get |
| [Full manual](docs/manual.md) | Complete API list, environment variables, source deployment, agent details and history |
| [VM collection spec](docs/vm-info-collection.md) | VM field sources, collection commands and fallbacks |
| [Host collection spec](docs/host-info-collection.md) | Host field sources, utilization and trend specs |
| [Refresh functions](docs/frontend-refresh-functions.md) | Automatic/manual refresh, SSE events and per-page refresh entries |
| [Operation log coverage](docs/operation-log-coverage.md) | Task, audit, alert and delivery coverage |

## Version history

| Version | Date | Notes |
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

Build artifacts and release notes for each version live on [GitHub Releases](https://github.com/zyx3721/kvm-manager/releases).

## Acknowledgements

Thanks to the following open-source projects and communities:

- [jackc/pgx](https://github.com/jackc/pgx) — high-performance PostgreSQL driver
- [redis/go-redis](https://github.com/redis/go-redis) — Redis client
- [go-ldap/ldap](https://github.com/go-ldap/ldap) — LDAP authentication
- [swaggo/swag](https://github.com/swaggo/swag) — Swagger documentation generator
- [novnc/noVNC](https://github.com/novnc/noVNC) — browser VNC console
- React, Vite and Tailwind CSS — the frontend toolchain

## License

Released under the [MIT License](LICENSE): free to use, copy, modify, merge, publish, distribute, sublicense and sell, provided the copyright and permission notices are retained in all copies or substantial parts.

## Contact

- **Email**: 416685476@qq.com
- **GitHub Issues**: [zyx3721/kvm-manager/issues](https://github.com/zyx3721/kvm-manager/issues)
- **Project home**: [github.com/zyx3721/kvm-manager](https://github.com/zyx3721/kvm-manager)

---

**⭐ If this project helps you, a Star is much appreciated!**
