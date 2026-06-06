# 任务、审计与告警日志覆盖说明

本文档说明平台“任务 / 审计 / 告警”页面三类记录的定位、写入边界、当前覆盖范围和后续开发时的同步要求。

## 一、日志类型边界

### 1.1 任务日志

任务日志写入 `tasks` 表，主要用于记录后台任务和可追踪进度的操作。

适合写入任务日志的场景：

- 后端创建后台任务后立即返回前端。
- 操作耗时较长，需要排队、运行中、完成或失败状态。
- 任务执行期间需要持续更新 payload。
- 前端需要轮询或通过 SSE 展示进度。

当前任务状态包括：

- `queued`：任务已排队。
- `running`：任务运行中。
- `completed`：任务已完成。
- `failed`：任务失败。

任务 payload 应包含用户能理解的关键字段，例如：

- 操作对象名称。
- 所属 Agent 或宿主机。
- 当前进度消息。
- 成功、失败和总数统计。
- 失败原因。

### 1.2 审计日志

审计日志写入 `audit_logs` 表，主要用于记录用户成功触发的关键操作。

适合写入审计日志的场景：

- 用户登录、登出、修改密码、找回密码。
- 用户创建、删除、禁用、角色和用户组配置。
- 系统基础配置、通知配置、认证配置变更或测试。
- Agent 创建、删除、同步、测试连接。
- 虚拟机电源、配置、设备、控制台、介质、XML、创建、克隆、迁移和迁移辅助配置等操作。
- 快照创建、恢复、删除、备注更新和刷新。
- 存储池、网络池、宿主机接口和存储卷操作。
- 告警解决、通知已读和清空。
- 异步任务在 Agent 执行阶段失败且需要保留操作痕迹。

审计 action 使用稳定的点分命名，例如：

- `agent.test`
- `vm.clone.failed`
- `notification.read_all`

审计 metadata 应避免写入敏感内容：

- 不写入明文 token。
- 不写入密码、验证码、密钥。
- 不写入完整通知配置中的敏感字段。
- 只写入对象名称、开关状态、目标资源、失败原因等排查必需信息。

### 1.3 告警日志

告警日志写入 `alerts` 表，主要用于记录平台运行态异常。

适合写入告警的场景：

- Agent 连续同步失败并被判定离线。
- 宿主机 CPU、内存或存储连续超过严重阈值。
- 虚拟机状态进入 `error` 或 `unknown`。
- 虚拟机 CPU、内存或磁盘连续超过严重阈值。

告警通常由后端运行态同步自动生成和恢复，不要求普通用户操作失败都写入告警。

如果新增的操作失败代表平台持续异常，而不是一次性请求失败，应评估是否新增告警：

- 外部系统长期不可达。
- 关键后台任务持续失败。
- 资源或状态进入需要运维处理的异常状态。

## 二、当前覆盖范围

### 2.1 认证与账号

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 登录 | 否 | 是 | 否 | 写入 `auth.login` |
| 登出 | 否 | 是 | 否 | 写入 `auth.logout` |
| 修改当前用户密码 | 否 | 是 | 否 | 写入 `auth.password.change` |
| 找回密码发送验证码 | 否 | 是 | 否 | 写入 `auth.password.reset.request` |
| 找回密码确认重置 | 否 | 是 | 否 | 写入 `auth.password.reset.confirm` |
| 图形验证码获取与校验 | 否 | 否 | 否 | 认证流程辅助动作，不写业务日志 |

### 2.2 Agent 与宿主机

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| Agent 列表读取 | 否 | 否 | 否 | 只读查询 |
| 新建 Agent | 否 | 是 | 否 | 写入 `agent.create` |
| 新建 Agent 前测试连接 | 否 | 否 | 否 | 未保存资源前的临时校验 |
| 已保存 Agent 测试连接 | 否 | 是 | 否 | 写入 `agent.test`，包含 online/offline 结果 |
| 同步单个 Agent | 否 | 是 | 是 | 写入 `agent.sync`；同步失败达到阈值会触发离线告警 |
| 删除 Agent | 否 | 是 | 自动恢复 | 写入 `agent.delete`，并解决该 Agent 活跃告警 |
| 后端定时运行态刷新 | 是 | 否 | 是 | 按调度类型创建 `runtime.refresh.fast` 或 `runtime.refresh.all` 任务 |
| 手动全量刷新接口 | 是 | 是 | 是 | 创建 `runtime.refresh.all` 任务并写入 `runtime.refresh` |

### 2.3 虚拟机

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| VM 列表和详情读取 | 否 | 否 | 否 | 只读查询 |
| 单台 VM 刷新 | 否 | 是 | 是 | 写入 `vm.refresh`，同步结果可能生成或恢复告警 |
| 启动、恢复、暂停、关机、重启 | 是 | 是 | 是 | 写入 `vm.<action>`，先更新 VM 缓存状态，后台延迟 full 同步后可能生成或恢复告警 |
| 删除 | 是 | 是 | 是 | 写入 `vm.delete`，接口返回前同步所属 Agent，删除后可能恢复相关告警 |
| 强制关机、强制重启 | 是 | 是 | 是 | 写入 `vm.force-*`，先更新 VM 缓存状态，后台延迟 full 同步后可能生成或恢复告警 |
| 强制删除 | 是 | 是 | 是 | 写入 `vm.force-delete`，接口返回前同步所属 Agent，删除后可能恢复相关告警 |
| 创建 VM | 是 | 是 | 是 | 常规和 XML 创建成功写入 `vm.create`，Agent 执行失败写入 `vm.create.failed`；后端仍兼容旧磁盘模板创建模式，其目标卷名、源模板和 Agent 信息随任务 payload 进入排查链路，不记录敏感信息；模板源盘被运行中虚拟机占用导致 `qemu-img` 写锁失败时，任务错误会映射为中文提示 |
| 标记或取消 VM 模板 | 是 | 是 | 否 | 写入 `vm.template.mark` 或 `vm.template.unmark`，只记录模板标记元数据，不采集或保存虚拟机 CPU、内存、磁盘等详情 |
| 从 VM 模板创建 | 是 | 是 | 是 | 成功写入 `vm.template.create`，Agent 执行失败写入 `vm.template.create.failed`；复用整机克隆链路复制模板磁盘卷并基于模板 XML 定义新虚拟机 |
| 克隆 VM | 是 | 是 | 是 | 成功写入 `vm.clone`，Agent 执行失败写入 `vm.clone.failed` |
| 迁移 VM | 是 | 是 | 是 | 成功写入 `vm.migrate`，Agent 执行失败写入 `vm.migrate.failed` |
| 迁移预检 | 否 | 否 | 否 | 只读诊断源 VM、目标宿主机、网络池、存储池和迁移通道，不改变状态 |
| 配置迁移 SSH 免密 | 否 | 是 | 否 | 写入 `vm.migrate.ssh_key.setup`；仅记录 VM、源 / 目标 Agent、迁移 URI 和 SSH 用户名，不记录 SSH 密码 |
| 配置迁移目标主机名 | 否 | 是 | 否 | 写入 `vm.migrate.hostname.setup`；会修复迁移目标主机名及 hosts 解析，审计 metadata 记录目标主机名，不记录敏感凭据 |
| 修改 VM 基础配置 | 是 | 是 | 是 | 写入 `vm.config.update`，任务 payload 和审计 metadata 包含 CPU、内存分配以及 `memoryStatsPeriod` |
| 重命名 | 是 | 是 | 是 | 写入 `vm.rename` |
| 修改自启动 | 是 | 是 | 是 | 写入 `vm.autostart.update` |
| 修改控制台密码配置 | 是 | 是 | 否 | 写入 `vm.console.update` |
| 打开控制台 WebSocket | 否 | 是 | 否 | 连接成功后写入 `vm.console` |
| 连接或断开 ISO 介质 | 是 | 是 | 是 | 写入 `vm.media.connect` 或 `vm.media.disconnect` |
| 修改 XML | 是 | 是 | 是 | 写入 `vm.xml.update` |
| 修改设备、扩容磁盘、新增或删除设备 | 是 | 是 | 是 | 写入 `vm.devices.update` |

告警列为“是”表示该操作后的同步结果可能触发或恢复运行态告警，不代表每次操作都新增告警。

### 2.4 快照

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 快照列表读取 | 否 | 否 | 否 | 只读查询 |
| 刷新快照缓存 | 否 | 是 | 否 | 写入 `snapshot.refresh` |
| 创建快照 | 是 | 是 | 否 | 写入 `snapshot.create` |
| 恢复快照 | 是 | 是 | 是 | 写入 `snapshot.revert`，恢复后刷新 VM 运行态 |
| 删除快照 | 是 | 是 | 否 | 写入 `snapshot.delete` |
| 更新备注、显示名和标签 | 否 | 是 | 否 | 写入 `snapshot.annotation.update` |

### 2.5 存储池与存储卷

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 存储池、卷和 ISO 列表读取 | 否 | 否 | 否 | 只读查询 |
| 创建存储池 | 否 | 是 | 否 | 写入 `storage_pool.create` |
| 删除存储池 | 否 | 是 | 否 | 写入 `storage_pool.delete` |
| 启停存储池 | 否 | 是 | 否 | 写入 `storage_pool.state.update` |
| 修改存储池自启动 | 否 | 是 | 否 | 写入 `storage_pool.autostart.update` |
| 创建存储卷 | 否 | 是 | 否 | 写入 `storage_volume.create` |
| 删除存储卷 | 否 | 是 | 否 | 写入 `storage_volume.delete` |
| 克隆存储卷 | 是 | 是 | 否 | 成功写入 `storage_volume.clone`，Agent 执行失败写入 `storage_volume.clone.failed` |
| 上传 ISO | 是 | 是 | 否 | 成功写入 `storage_volume.upload`，Agent 执行失败写入 `storage_volume.upload.failed` |

### 2.6 网络池与宿主机接口

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 网络池列表读取 | 否 | 否 | 否 | 只读查询 |
| 创建网络池 | 否 | 是 | 否 | 写入 `network_pool.create` |
| 删除网络池 | 否 | 是 | 否 | 写入 `network_pool.delete` |
| 启停网络池 | 否 | 是 | 否 | 写入 `network_pool.state.update` |
| 修改网络池自启动 | 否 | 是 | 否 | 写入 `network_pool.autostart.update` |
| 宿主机接口列表读取 | 否 | 否 | 否 | 只读查询 |
| 宿主机接口设备列表读取 | 否 | 否 | 否 | 创建辅助查询 |
| 创建宿主机接口 | 否 | 是 | 否 | 写入 `host_interface.create` |
| 启停宿主机接口 | 否 | 是 | 否 | 写入 `host_interface.state.update` |
| 删除宿主机接口 | 否 | 是 | 否 | 写入 `host_interface.delete` |

### 2.7 告警与通知中心

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 告警列表读取 | 否 | 否 | 否 | 只读查询 |
| 手动解决告警 | 否 | 是 | 是 | 写入 `alert.resolve` 并更新告警状态 |
| 通知列表读取 | 否 | 否 | 否 | 只读查询 |
| 单条通知标记已读 | 否 | 是 | 是 | 写入 `notification.read` 并更新告警通知状态 |
| 全部通知标记已读 | 否 | 是 | 是 | 写入 `notification.read_all` |
| 清空通知中心 | 否 | 是 | 是 | 写入 `notification.clear`，不解决告警 |
| 外部告警通知发送成功 | 否 | 否 | 是 | 更新 `notificationSentAt`，不写用户审计 |

### 2.8 系统配置与权限

| 操作 | 任务 | 审计 | 告警 | 说明 |
| :-: | :-: | :-: | :-: | :-: |
| 基础配置更新 | 否 | 是 | 否 | 写入 `settings.base.update` |
| 通知媒介更新 | 否 | 是 | 否 | 写入 `settings.notification.update` |
| 通知媒介测试 | 否 | 是 | 否 | 写入 `settings.notification.test` |
| 认证配置更新 | 否 | 是 | 否 | 写入 `settings.auth_provider.update` |
| 认证配置测试 | 否 | 是 | 否 | 写入 `settings.auth_provider.test` |
| 用户创建、更新、删除、禁用 | 否 | 是 | 否 | 写入 `settings.user.*` |
| 角色创建、更新、删除 | 否 | 是 | 否 | 写入 `settings.role.*` |
| 用户组创建、更新、删除 | 否 | 是 | 否 | 写入 `settings.user_group.*` |

## 三、失败记录规则

### 3.1 同步接口失败

同步接口在请求内立即执行，失败时直接返回错误。

当前规则：

- 成功后写审计日志。
- Agent 离线或运行态异常通过告警链路体现。
- 请求参数校验失败、权限失败和资源不存在不写审计。

典型接口：

- `POST /api/agents/{id}/sync`
- `POST /api/vms/{id}/refresh`
- `POST /api/snapshots/refresh`

### 3.2 异步任务失败

异步任务已经完成排队，后续在后台执行。

当前规则：

- 任务创建后写入 `tasks`。
- 任务开始后更新为 `running`。
- Agent 执行失败时更新为 `failed`。
- Agent 执行失败同时写入对应 `.failed` 审计 action。
- 后端抢占任务失败、读取临时文件失败等内部启动问题只更新任务失败，不额外写审计。

当前已覆盖失败审计的异步操作：

- `vm.create.failed`
- `vm.clone.failed`
- `vm.migrate.failed`
- `storage_volume.clone.failed`
- `storage_volume.upload.failed`

### 3.3 普通请求失败

普通请求失败通常不写审计。

不写审计的原因：

- 参数格式错误可能来自误操作或扫描请求。
- 权限失败可能产生大量噪音。
- 资源不存在或预检查失败未真正改变系统状态。

如果未来需要审计高风险失败尝试，应单独设计失败审计策略，避免影响现有操作页可读性。

## 四、后续开发同步要求

### 4.1 新增或修改用户操作

新增或修改用户可触发操作时，必须评估以下问题：

- 是否改变平台状态、KVM 资源、系统配置或通知状态。
- 是否需要写审计日志。
- 是否是后台长耗时操作。
- 是否需要写任务日志。
- 是否可能引发持续异常。
- 是否需要写入或恢复告警。
- 是否需要同步 README 和本文档。

### 4.2 审计字段要求

新增审计日志时，metadata 至少应包含：

- 用户可识别的资源名称。
- Agent、宿主机或 VM 名称。
- 操作目标和关键开关值。
- 失败审计中的用户可读错误信息。

metadata 禁止包含：

- Agent token。
- 登录密码、控制台密码、找回密码验证码。
- 通知 Webhook URL、SMTP 密码、LDAP bind 密码等敏感配置。
- 未脱敏的完整请求体。

### 4.3 任务字段要求

新增任务日志时，payload 至少应包含：

- `message`：当前状态中文描述。
- 操作对象名称，例如 `vm`、`pool`、`volume`。
- 所属 Agent 或宿主机名称。

如果任务遍历多个 Agent，应包含：

- `totalAgents`
- `syncedAgents`
- `failedAgents`
- `currentAgent`
- `agentResults`

### 4.4 告警字段要求

新增告警时，应保证：

- `source_type` 稳定，能反映告警来源。
- `source_id` 能定位唯一资源。
- `title` 稳定，便于同一活跃告警 upsert。
- `message` 面向运维人员可读。
- `metadata` 包含 metric、value、limit、agent、vm 等定位字段。

新增自动恢复逻辑时，应同步调用：

- `ResolveActiveAlert`
- `ResolveActiveAlertsBySource`

## 五、文档同步清单

修改任务、审计、告警相关功能时，需要同步检查：

- `docs/operation-log-coverage.md`
- `docs/frontend-refresh-functions.md`
- `docs/vm-info-collection.md`
- `docs/host-info-collection.md`
- `README.md`
- `AGENTS.md`

如果只是新增审计 action 或调整任务 payload，通常更新本文档和 README 简述即可。

如果日志变更伴随刷新链路变化，需要同步更新 `docs/frontend-refresh-functions.md`。

如果日志变更伴随 Agent 采集、VM 操作命令、快照、控制台或存储命令变化，需要同步更新对应采集文档。
