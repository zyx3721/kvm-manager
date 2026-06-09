# 前端刷新功能说明

本文档说明前端平台上所有刷新入口分别刷新什么、触发哪个后端链路，以及它们和后端定时刷新、手动全量刷新接口之间的关系。

## 一、刷新概念边界

### 1.1 后端定时全局运行态刷新

后端按 `RUNTIME_SYNC_INTERVAL` 周期性创建或复用刷新任务，默认间隔为 `30s`。

后端请求单个 Agent 执行 fast 同步的默认 HTTP 超时为 `12s`，可通过 `RUNTIME_SYNC_FAST_TIMEOUT_SECONDS` 调整。该超时限制的是后端等待 Agent fast 接口返回的总时间，不是 Agent 内部单条宿主机命令的超时。

这条链路是全局范围刷新，因为它面向所有已登记 Agent：

- 刷新任务类型为 `runtime.refresh.fast`。
- 后端 worker 会遍历所有 Agent。
- 如果当前没有任何已登记 Agent，定时刷新不会创建 `runtime.refresh.fast` 任务，也不会广播同步事件，避免任务列表持续出现“暂无可同步的 Agent”。
- 每个 Agent 主要采集宿主机和虚拟机运行态。
- 采集结果写入 Redis 运行态缓存。
- 同步完成后通过 SSE 广播事件，前端收到事件后重新读取后端缓存。
- 如果 Agent 在刷新任务执行期间被删除，后端写入运行态缓存前后都会确认 Agent 登记和删除标记；已删除时会清理该 Agent 的 host、VM 和快照缓存并跳过写入，避免旧刷新任务把已删除宿主机重新写回总览和宿主机页。读取 `/api/hosts`、`/api/vms` 和总览时也会过滤并清理数据库中已不存在的 Agent 运行态残留。

需要注意：这条链路是“全局运行态刷新”，但不是手动 `/api/refresh` 的 full 全量采集。它使用 fast 模式，目的是降低 30 秒周期刷新对 Agent、宿主机和网络的压力。

fast 模式通常会采集：

- 宿主机运行态信息。
- 虚拟机列表和基础状态。
- 虚拟机描述，来自 `dumpxml` 的 `<description>`。
- 虚拟机 CPU 采样。
- 虚拟机内存使用率采样，运行中 VM 使用 `dommemstat <vm>`，已停止 VM 直接返回 `0%`。
- 虚拟机磁盘 I/O 和网络吞吐采样。
- 可用于趋势图落库的 host/vm 指标样本。

fast 模式通常会跳过：

- QEMU Guest Agent 的完整 OS/IP 查询。
- 虚拟机磁盘明细深度采集，包括 `domblkinfo Capacity`、`virt-df --csv` 和 `virt-filesystems --csv --all --long`。
- 虚拟机内存明细深度采集。
- 快照采集。

fast 内存使用率来自 Agent 对运行中 VM 执行的 `dommemstat <vm>` 采样；优先按 `actual - usable` 计算，缺少 `usable` 时使用 `available` 兜底，已停止 VM 直接返回 `0%`。采样失败时后端会沿用上一次可用的内存使用率，避免 30 秒刷新把页面指标覆盖为空。后端也会合并上一次 full 模式或单台 VM 刷新采集到的高置信详情，避免 fast 刷新把 OS、IP、磁盘等详情覆盖为空。完整计算口径详见 `docs/vm-info-collection.md`。

### 1.1.1 后端低频深度刷新

后端按 `RUNTIME_DEEP_SYNC_INTERVAL` 周期性创建或复用 `runtime.refresh.all` 任务，默认间隔为 `10m`，设置为 `0` 可关闭。

后端请求单个 Agent 执行 full 同步的默认 HTTP 超时为 `60s`，可通过 `RUNTIME_SYNC_FULL_TIMEOUT_SECONDS` 调整。手动全量刷新、低频深度刷新、单 Agent full 同步和单台 VM full 刷新均使用该超时。

这条链路同样面向所有已登记 Agent，主要用于低频补齐不适合 30 秒 fast 刷新的重字段：

- QEMU Guest Agent OS 信息。
- `domifaddr` 主 IP。
- 运行中 VM 内存使用率。
- 磁盘容量和客户机文件系统使用量。
- 快照运行态缓存。

磁盘容量仍由 `domblkinfo Capacity` 获取；磁盘使用大小和使用率由 `virt-df --csv -d <vm>` 获取，并结合 `virt-filesystems --csv -d <vm> --all --long` 归属到具体磁盘。若创建 VM 后延迟 full 刷新时 `virt-df` 尚未取到文件系统结果，磁盘使用大小和使用率显示为 `0`，总容量仍按 `domblkinfo Capacity` 展示。

低频深度刷新不会在后端启动时立即排队，而是等待第一个 `RUNTIME_DEEP_SYNC_INTERVAL` 到达后再尝试创建任务；创建前会避让已有 queued 或 running 的 fast/full 刷新任务，避免服务启动或手动刷新时堆积多个全局同步任务。

如果当前没有任何已登记 Agent，低频深度刷新同样不会创建 `runtime.refresh.all` 任务。

### 1.2 手动全量刷新接口

`POST /api/refresh` 会创建或复用 `runtime.refresh.all` 任务。

这条链路同样面向所有已登记 Agent，但采集级别更重：

- 调用完整 VM 列表采集。
- 读取更完整的 Guest Agent OS、`domifaddr` IP、磁盘和内存详情。
- 同步快照运行态缓存。
- 完成后广播 SSE 事件，前端再重新读取缓存。

主布局右上角的刷新图标会触发该接口，适合在需要立刻拉取所有 Agent 的完整宿主机、VM 详情和快照缓存时使用。按钮仅对具备 `agents.manage` 权限的用户显示；如果已有 queued 或 running 的全量刷新任务，后端会复用当前任务，避免任务堆积。

按钮位置：

- `frontend/src/components/layout/HeaderFullRefreshButton.tsx`
- `frontend/src/components/layout/KvmLayout.tsx`

### 1.3 前端全局运行态事件刷新

前端在主布局中连接 `/api/events` SSE 事件流。

收到以下事件后，前端会触发 `kvm:refresh` 浏览器事件：

- `runtime.updated`
- `sync.finished`
- `sync.failed`

主布局还会把部分资源专项 SSE 事件转发为前端内部资源事件：

- `storage.pool.updated`
- `network.pool.updated`
- `host.interface.updated`

`kvm:refresh` 本身不采集宿主机，也不直接调用 `/api/refresh`。它只是告诉当前页面：“后端缓存可能已经变化，请重新读取你关心的数据”。

因此可以这样理解：

- 后端 30 秒定时任务负责真正采集和更新缓存。
- SSE 负责通知前端缓存已变化。
- 前端全局运行态事件刷新负责让页面重新读取缓存。

### 1.4 页面局部刷新

页面局部刷新通常只刷新当前页面或当前对象需要的数据。

典型例子：

- 宿主机页新增 Agent 保存时会先测试连接并登记 Agent，随后立即释放保存按钮，再后台触发该 Agent 的 full 同步。
- 虚拟机列表每行“刷新”只刷新单台 VM。
- 快照页“刷新快照”只刷新快照缓存。
- 存储池页“刷新”只重新读取当前宿主机的存储池列表。
- 存储池、网络池和宿主机接口的增删改操作完成后，会广播对应资源事件，相关页面按 `agentId` 自动重新读取当前宿主机数据。
- 监控弹窗“刷新监控”只重新读取指标曲线。
- 忘记密码页验证码刷新只重新生成图形验证码。

## 二、全局刷新入口

### 2.1 SSE 事件流

实现位置：

- `frontend/src/components/layout/KvmLayout.tsx`
- `frontend/src/lib/refresh.ts`

主要作用：

- 建立 `/api/events` SSE 长连接。
- 接收后端同步进度事件。
- 顶部栏展示“同步排队中”“同步已开始”“同步中 x/y”等状态。
- 在运行态更新后触发 `emitKvmRefresh()`。
- 运行态更新后重新读取右上角通知消息和未读数量。

它主要刷新的是页面展示层的数据读取动作，而不是直接采集数据。

### 2.2 全局事件影响的页面

以下页面订阅了 `onKvmRefresh`：

| 页面 | 重新读取的数据 | 主要用途 |
| :-: | :-: | :-: |
| 总览页 | 仪表盘汇总、宿主机列表、虚拟机列表 | 更新资源统计、在线宿主机、运行中非模板 VM、非模板 VM 状态分布 |
| 虚拟机页 | VM 列表和状态计数 | 更新虚拟机运行状态、IP、资源使用率、I/O、筛选结果 |
| 宿主机页 | 宿主机、Agent、VM 列表 | 更新宿主机资源、Agent 状态、宿主机下 VM 数量 |
| 存储池页 | 当前宿主机存储池列表 | 更新池状态、容量、已分配、自启状态，并同步打开的详情 |
| 网络池页 | 当前宿主机网络池列表 | 更新网络池状态、转发模式、DHCP、固定地址和自启状态 |
| 宿主机接口页 | 当前宿主机接口列表 | 更新物理网卡、bridge 接口和在线状态 |
| 快照页 | 快照、VM、宿主机列表 | 更新快照表格和关联 VM/宿主机展示 |
| 任务 / 审计 / 告警页 | 当前标签页记录 | 更新后台任务、审计日志、告警列表 |

网络池页和接口页当前没有订阅 `onKvmRefresh`，它们主要依靠页面按钮、宿主机切换和操作完成后的局部重新读取。

## 三、各页面刷新功能

### 3.1 总览页

实现位置：

- `frontend/src/features/dashboard/DashboardPage.tsx`

触发方式：

- 页面首次进入。
- 收到全局 `kvm:refresh` 事件。
- 初始加载失败后点击“重试”。

调用接口：

- `GET /api/dashboard/summary`
- `GET /api/hosts`
- `GET /api/vms`
- 选中对象变化时调用 `GET /api/metrics/vms/{vmId}`
- 选中对象变化时调用 `GET /api/metrics/hosts/{agentId}`

主要刷新：

- 平台资源总览。
- 宿主机在线数量。
- 非模板虚拟机运行数量。
- CPU、内存、存储等汇总指标。
- 最近任务、告警等摘要信息。
- 选中 VM 或宿主机的最近 1 小时趋势。

说明：

- 总览页不会自己触发后端采集。
- 总览页读取的是后端运行态缓存和指标样本。
- 全局 SSE 事件到达后，总览页会重新读取这些缓存数据。
- 仪表盘汇总中的 VM 运行/总数、状态分布、vCPU 已分配和最近 VM 列表均排除已标记模板。
- VM 模板标记或取消标记成功后，后端会广播 `runtime.updated`；总览页收到事件后重新读取汇总，立即按最新模板标记调整非模板 VM 统计。

### 3.2 虚拟机列表页

实现位置：

- `frontend/src/features/vms/VMsPage.tsx`
- `frontend/src/features/vms/components/VMTable.tsx`

触发方式：

- 页面首次进入。
- 搜索、状态筛选、宿主机筛选变化。
- 收到全局 `kvm:refresh` 事件。
- 虚拟机操作完成后主动重新读取列表。

调用接口：

- `GET /api/vms`
- `GET /api/hosts`

主要刷新：

- VM 列表。
- VM 状态。
- 主 IP。
- CPU、内存、磁盘使用率。
- 磁盘 I/O 和网络吞吐。
- 所属宿主机筛选项。
- 全部、运行中、已停止、已暂停、异常等状态计数。

说明：

- 列表级刷新只是重新读取后端缓存。
- 它不会主动触发 `/api/refresh`。
- 后端缓存是否更新，取决于定时 fast 刷新、手动 full 刷新、单台 VM 刷新或 VM 操作后的异步同步。
- 启动、恢复、暂停、关机、停止、强制关机、重启和强制重启成功后，后端会先更新当前 VM 的缓存状态并广播 `runtime.updated`，再延迟 8 秒后台 full 同步该 Agent，避免操作响应被完整 Agent 同步阻塞。
- 删除和强制删除成功后，后端会先从运行态缓存移除该 VM 并广播 `runtime.updated`，让 VM 列表尽快消失；随后后台延迟 8 秒 full 同步所属 Agent，兜底校准宿主机 VM 数量、快照和其他运行态字段。
- VM 基础配置、设备、XML、介质和自启动修改成功后，后端会定向刷新当前 VM 运行态并广播页面更新。
- VM 创建弹窗支持常规、模板和 XML 三种创建菜单。提交成功后前端会用任务 toast 卡片轮询创建任务，执行中固定展示并允许选中文本复制，任务完成或失败后更新卡片状态并在约 5 秒后自动隐藏；模板菜单现在从已标记的虚拟机模板创建，调用 `POST /api/vms/{id}/template-create` 后复用整机克隆链路复制模板磁盘和 XML 基本配置。
- VM 创建任务和从模板创建任务完成后，后端先对所属 Agent 执行 fast 同步并广播 `runtime.updated`，让新虚拟机尽快出现在列表中；随后后台延迟 8 秒 full 同步并再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和快照等重字段。
- VM 克隆任务完成后，后端先对所属 Agent 执行 fast 同步并广播 `runtime.updated`，让新虚拟机尽快出现在列表中；随后后台延迟 8 秒 full 同步并再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和快照等重字段。
- VM 迁移任务完成后，后端会先对源宿主机和目标宿主机执行 fast 同步并广播 `runtime.updated`，让虚拟机列表尽快显示迁移后的宿主机归属；随后后台再执行 full 同步并再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和指标等重字段。
- VM 页面通过“虚拟机 / 模板”切换按钮拆分普通虚拟机和模板视图。模板标记与取消标记只更新数据库中的模板标记表，后端广播 `runtime.updated`，前端重新读取 `/api/vms` 后由运行态 VM 与模板标记合并得到最新视图；该操作不触发 Agent 采集。

#### 3.2.1 虚拟机操作刷新链路表

| 操作 | 触发接口或入口 | 后端缓存处理 | SSE / 前端刷新 | 后台补齐 |
| :-: | :-: | :-: | :-: | :-: |
| 创建虚拟机 | `POST /api/vms` | 任务完成后 fast 同步所属 Agent | 广播 `vm.create.completed` 和 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 从模板创建 | `POST /api/vms/{id}/template-create` | 任务完成后 fast 同步所属 Agent | 广播 `vm.template.create.completed` 和 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 克隆虚拟机 | `POST /api/vms/{id}/clone` | 任务完成后 fast 同步所属 Agent | 广播 `vm.clone.completed` 和 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 迁移虚拟机 | `POST /api/vms/{id}/migrate` | 任务完成后 fast 同步源 Agent 和目标 Agent | 广播 `vm.migrate.completed` 和 `runtime.updated`，VM 列表重新读取缓存 | 后台 full 同步源 Agent 和目标 Agent，再广播 `runtime.updated` |
| 启动 / 恢复 / 暂停 | `POST /api/vms/{id}/{action}` | 直接更新当前 VM 缓存状态 | 广播 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 关机 / 停止 / 强制关机 / 强制停止 | `POST /api/vms/{id}/{action}` | 直接更新当前 VM 缓存状态 | 广播 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 重启 / 强制重启 | `POST /api/vms/{id}/{action}` | 直接更新当前 VM 缓存状态为运行中 | 广播 `runtime.updated`，VM 列表重新读取缓存 | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 删除 / 强制删除 | `POST /api/vms/{id}/{action}` | 从运行态缓存移除该 VM 及其快照缓存 | 广播 `runtime.updated`，VM 列表重新读取缓存并移除该 VM | 延迟 8 秒 full 同步所属 Agent，再广播 `runtime.updated` |
| 基础配置 / 资源配置 / 控制台 / 自启动 | `PUT /api/vms/{id}/...` | 定向刷新当前 VM 运行态 | 广播 `runtime.updated`，VM 列表和编辑弹窗重新读取缓存 | 无全局 full 任务；后续定时 full 兜底 |
| 设备配置 / XML / 介质连接 / 介质断开 | `PUT` 或 `DELETE /api/vms/{id}/...` | 定向刷新当前 VM 运行态 | 广播 `runtime.updated`，VM 列表和编辑弹窗重新读取缓存 | 无全局 full 任务；后续定时 full 兜底 |
| 行内刷新 | `POST /api/vms/{id}/refresh` | 定向刷新当前 VM full 运行态 | 广播 `runtime.updated`，VM 列表重新读取缓存 | 无额外后台补齐 |
| 模板标记 / 取消模板标记 | `POST` 或 `DELETE /api/vms/{id}/template-mark` | 仅更新模板标记表，不采集 Agent | 广播 `runtime.updated`，前端重新读取 VM 列表并合并模板标记 | 无 |
| 快照恢复 | `POST /api/snapshots/{id}/revert` | 定向刷新目标 VM full 运行态，并刷新快照缓存 | 广播 `runtime.updated`，VM 列表和快照页重新读取缓存 | 无额外后台补齐 |

#### 3.2.2 VM 详情字段刷新来源

当前 VM 详情字段不是全部都由 30 秒 fast 刷新重新采集。fast 主要负责轻量运行态和趋势指标，后端低频 full 深度刷新、手动 full 全量刷新、单台 VM 刷新和 VM 操作后的定向刷新负责补齐更重的 Guest Agent、磁盘明细和配置详情。

| 字段 | 主要刷新来源 | 说明 |
| :-: | :-: | :-: |
| 描述 | 30 秒 fast 刷新、单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步、VM 基础配置保存后的定向 VM 刷新 | Agent 读取 `dumpxml` 的 `<description>`，与 VM 编辑页基础信息中的描述一致；fast 本轮描述为空时会保留上一次缓存值，避免短周期刷新把列表描述覆盖为空 |
| IP 地址 / 主 IP | 单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步、电源操作后延迟 full 同步、VM 配置类操作后的定向 VM 刷新 | full/单台刷新优先执行 `virsh domifaddr <vm> --source agent`，失败后回退 `--source lease`，并过滤 `127.*` 与 `169.254.*`；fast 通常不主动查询 IP，会保留已有可用 IP。若运行中 VM 缓存 IP 为空、回环或链路本地地址，会等待低频 full、手动 full、单台刷新或定向 VM 刷新补齐 |
| 操作系统 | 单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步、电源操作后延迟 full 同步、VM 配置类操作后的定向 VM 刷新 | full/单台刷新优先通过 QEMU Guest Agent 获取系统信息，失败后按 XML 和本地规则回退；fast 不执行 Guest Agent 查询，也不会用低置信兜底值覆盖已有高置信 OS |
| 内存使用率 | 30 秒 fast 刷新、单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步、电源操作后延迟 full 同步、VM 配置类操作后的定向 VM 刷新 | fast/full/单台刷新均对运行中 VM 使用 `dommemstat <vm>`，优先按 `actual - usable` 计算，没有 `usable` 时使用 `available` 兜底；已停止 VM 直接返回 `0%`。fast 本轮字段不完整时会保留上一次可用值 |
| CPU 使用率 | 30 秒 fast 刷新、单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步 | CPU 属于轻量采样，Agent 在 fast 和 full VM 列表采集时都会通过 `domstats --cpu-total` 做两次采样并计算使用率 |
| 磁盘明细 / 磁盘容量 | 单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步、VM 设备修改后的定向 VM 刷新、快照恢复后的定向 VM 刷新、电源操作后延迟 full 同步 | 磁盘明细属于较重采集，fast 通常跳过并保留上一次 full/单台刷新得到的磁盘列表、容量和使用率；手动 full、低频 full 和 Agent full 同步优先用全局 `virt-df --csv` 复用 VM 文件系统 Used，单台和定向 VM 刷新使用 `virt-df --csv -d <vm>`；磁盘总容量始终来自 `domblkinfo Capacity`；磁盘扩容、新增磁盘、删除磁盘、XML 修改或快照恢复后会定向刷新当前 VM |
| 磁盘 I/O / 网络吞吐 | 30 秒 fast 刷新、单台 VM 刷新、手动 full 全量刷新、后端低频 full 深度刷新、Agent full 同步 | 为了持续写入趋势图样本，fast 和 full 都会通过 `domstats --block --interface` 做两次采样并计算速率 |
| 内存统计周期配置 | VM 编辑 CPU 与内存页保存、创建 VM 后默认配置 | 创建 VM 完成后默认执行 `dommemstat <vm> --period 5 --config`；编辑页启用后运行中 VM 使用 `--live --config`，停止 VM 使用 `--config`。该配置影响后续 `dommemstat` 是否能持续拿到可用内存统计字段 |

### 3.3 单台虚拟机刷新

实现位置：

- `frontend/src/features/vms/VMsPage.tsx`
- `frontend/src/features/vms/components/VMTable.tsx`

触发方式：

- 点击虚拟机表格操作列中的“刷新”按钮。

调用接口：

- `POST /api/vms/{id}/refresh`

主要刷新：

- 当前这一台 VM 的运行态信息。
- XML `<description>` 描述。
- Guest Agent OS 和 `domifaddr` IP。
- 磁盘明细。
- CPU 和内存使用率。
- 磁盘 I/O。
- 网络吞吐。

说明：

- 该刷新只针对单台 VM。
- 后端会定位该 VM 所属宿主机和 Agent。
- 不创建 `runtime.refresh.all` 全量任务。
- 不采集快照。
- 成功后前端会把该 VM 的最新数据替换到当前列表中。
- 快照恢复成功后，后端也会复用这类定向 VM full 刷新，确保恢复后的磁盘明细、容量和配置能自动回写运行态缓存。

适用场景：

- 修改虚拟机 IP 后，想立即重新读取该 VM 的最新 IP。
- Guest Agent 刚恢复，想针对单台 VM 补齐系统和地址信息。
- 不想等待下一轮 30 秒定时刷新。

### 3.4 宿主机页 Agent 同步

实现位置：

- `frontend/src/features/hosts/HostsPage.tsx`

触发方式：

- 新增 Agent 保存成功后自动同步一次。
- 点击已保存 Agent 的“同步 Agent”按钮。
- 收到全局 `kvm:refresh` 事件后重新读取页面数据。

调用接口：

- `POST /api/agents/{id}/sync`
- `GET /api/hosts`
- `GET /api/agents`
- `GET /api/vms`

主要刷新：

- 指定 Agent 对应宿主机的运行态信息。
- 该宿主机下虚拟机列表和数量。
- Agent 连接状态。
- 宿主机资源使用率。
- 宿主机卡片上的 KVM 版本、CPU、内存、存储和 VM 数量。

说明：

- “同步 Agent”是指定 Agent 范围内的采集动作。
- 它不是所有 Agent 的全局刷新。
- 宿主机页本身没有独立的页面级“刷新”按钮。
- “测试连接”主要验证 Agent 可达性，成功后也会重新读取宿主机页数据。

### 3.5 存储池页刷新

实现位置：

- `frontend/src/features/storage-pools/StoragePoolsPage.tsx`

触发方式：

- 页面首次进入。
- 切换宿主机。
- 点击页面右上角“刷新”按钮。
- 收到全局 `kvm:refresh` 事件。
- 创建、启停、删除存储池后重新读取。

调用接口：

- `GET /api/hosts`
- `GET /api/storage-pools/{agentId}`

主要刷新：

- 宿主机选择列表。
- 当前宿主机下的存储池列表。
- 存储池状态。
- 存储池容量、已分配、可用空间。
- 存储池类型、路径、自启动状态。
- 存储池卷数量，列表页只做轻量计数。
- 存储池汇总卡片中的总容量和已分配。

说明：

- 页面“刷新”不会触发全局运行态刷新任务。
- 它只重新读取当前页面需要的宿主机和存储池数据。
- 存储池列表读取卷数量时不解析完整卷详情，也不探测卷格式。
- 如果详情弹窗已打开，全局刷新或操作完成后会尝试同步详情中的当前存储池数据。
- 存储池创建、删除、状态、自启动、卷创建、卷删除、卷克隆完成和 ISO 上传完成后，后端会广播 `storage.pool.updated`，存储池页会按当前宿主机自动重新读取。

### 3.6 存储池详情刷新

实现位置：

- `frontend/src/features/storage-pools/components/StoragePoolDetailDialog.tsx`

触发方式：

- 打开详情弹窗。
- 存储池状态或自启动修改成功。
- 删除存储卷成功。
- 创建镜像卷成功。
- 上传 ISO 成功。
- 克隆存储卷任务完成。

调用接口：

- `GET /api/storage-pools/{agentId}/volumes/{pool}`
- 父页面的 `GET /api/storage-pools/{agentId}`

主要刷新：

- 当前存储池的卷或 ISO 列表。
- 卷数量。
- 卷容量、格式、路径。
- 存储池容量和用量。
- 存储池状态和自启动展示。

说明：

- 读取卷列表前，Agent 侧会执行存储池刷新，确保宿主机后台新增的卷文件能被扫描出来。
- 详情页会读取完整卷详情并识别卷格式，保留创建、克隆、删除和 ISO 上传所需的完整判断逻辑。
- 上传和克隆属于后台任务，任务完成后会通过 SSE 通知前端，再刷新对应页面数据。

### 3.7 网络池页刷新

实现位置：

- `frontend/src/features/network-pools/NetworkPoolsPage.tsx`

触发方式：

- 页面首次进入。
- 切换宿主机。
- 点击页面右上角“刷新”按钮。
- 创建、启停、删除网络池后重新读取。

调用接口：

- `GET /api/hosts`
- `GET /api/network-pools/{agentId}`

主要刷新：

- 宿主机选择列表。
- 当前宿主机下的网络池列表。
- 网络池状态。
- 桥接设备。
- 转发模式。
- 子网池。
- DHCP 配置和固定地址。
- 自启动状态。

说明：

- 网络池页刷新是页面局部刷新。
- 网络池创建、删除、状态和自启动修改后，后端会广播 `network.pool.updated`，网络池页会按当前宿主机自动重新读取。
- 网络池详情内修改状态或自启动后，会重新读取页面列表并更新详情。

### 3.8 宿主机接口页刷新

实现位置：

- `frontend/src/features/host-interfaces/HostInterfacesPage.tsx`

触发方式：

- 页面首次进入。
- 切换宿主机。
- 点击页面右上角“刷新”按钮。
- 新增接口、启停接口、删除接口后重新读取。

调用接口：

- `GET /api/hosts`
- `GET /api/host-interfaces/{agentId}`

主要刷新：

- 宿主机选择列表。
- 当前宿主机下的物理网卡、loopback 和 bridge 接口。
- 接口在线状态。
- 接口类型。
- MAC、地址、桥接绑定等接口信息。
- 页面统计中的接口总数、在线接口数和桥接接口数。

说明：

- 接口页读取的是 Agent 当前接口信息，不是从运行态缓存表中读取。
- 创建接口遇到 Agent 超时时，前端会自动刷新接口列表；如果目标接口已经存在，会按创建成功处理。
- 接口创建、启动、停止和删除后，后端会广播 `host.interface.updated`，接口页会按当前宿主机自动重新读取。

### 3.9 快照页刷新

实现位置：

- `frontend/src/features/snapshots/SnapshotsPage.tsx`

触发方式：

- 页面首次进入。
- 点击“刷新快照”按钮。
- 创建、恢复、删除快照后重新读取。
- 收到全局 `kvm:refresh` 事件。

调用接口：

- `POST /api/snapshots/refresh`
- `GET /api/snapshots`
- `GET /api/vms`
- `GET /api/hosts`

主要刷新：

- 快照运行态缓存。
- 快照列表。
- 快照大小。
- 快照类型。
- 所属 VM 信息。
- 所属宿主机信息。
- 平台侧备注和标签。

说明：

- “刷新快照”只刷新快照运行态缓存。
- 它不会触发宿主机、虚拟机详情和指标的全量刷新。
- 手动 `/api/refresh` full 全量刷新会采集快照，但快照页按钮是更小范围的专项刷新。
- 快照恢复会改变 VM 配置和磁盘状态，因此恢复成功后后端会额外定向刷新目标 VM 的完整运行态，再刷新快照缓存并广播页面更新。

### 3.10 任务 / 审计 / 告警页刷新

实现位置：

- `frontend/src/features/operations/OperationsPage.tsx`

触发方式：

- 页面首次进入。
- 切换任务、审计、告警标签。
- 搜索、分页、状态筛选、高级 JSON 字段筛选变化。
- 收到全局 `kvm:refresh` 事件。
- 告警处理完成后重新读取。

调用接口：

- `GET /api/tasks`
- `GET /api/audit-logs`
- `GET /api/alerts`

主要刷新：

- 后台刷新任务。
- 虚拟机操作任务。
- 审计日志。
- 平台告警。
- 当前筛选条件下的分页数据。

说明：

- 该页面不会触发资源采集。
- 它主要用于观察刷新任务、操作任务和告警状态。
- 全局刷新任务进度也会在顶部栏展示。
- 任务、审计和告警的写入边界、操作覆盖矩阵和失败记录规则详见 `docs/operation-log-coverage.md`。

### 3.11 VM 监控弹窗刷新

实现位置：

- `frontend/src/features/vms/components/VMMonitorDialog.tsx`

触发方式：

- 打开 VM 监控弹窗。
- 切换时间范围。
- 修改自定义时间范围。
- 点击“刷新监控”图标按钮。

调用接口：

- `GET /api/metrics/vms/{vmId}?range=...`

主要刷新：

- VM CPU 占用率曲线。
- VM 内存占用率曲线。
- VM 磁盘占用率曲线。
- VM 磁盘读取和写入速率曲线。
- VM 网络流入、流出和平均带宽曲线。

说明：

- 监控弹窗读取的是已落库的指标样本或聚合数据。
- 它不会直接触发 Agent 采集。
- 指标样本主要由后端定时运行态刷新链路持续写入。

### 3.12 宿主机监控弹窗刷新

实现位置：

- `frontend/src/components/kvm/HostTrendDialog.tsx`

触发方式：

- 打开宿主机监控弹窗。
- 切换时间范围。
- 修改自定义时间范围。
- 点击“刷新监控”图标按钮。

调用接口：

- `GET /api/metrics/hosts/{agentId}?range=...`

主要刷新：

- 宿主机 CPU 占用率曲线。
- 宿主机内存占用率曲线。
- 宿主机逻辑磁盘占用率曲线。
- 宿主机磁盘读取和写入速率曲线。
- 宿主机网络流入、流出和平均带宽曲线。

说明：

- 宿主机趋势来自数据库指标样本。
- 指标样本由后台刷新同步成功后写入 Redis Stream，再由 metric writer 落库。

### 3.13 VM 编辑介质页刷新

实现位置：

- `frontend/src/features/vms/components/edit/MediaPanel.tsx`

触发方式：

- 打开 VM 编辑弹窗的介质页。
- 点击存储池选择旁边的“刷新”按钮。
- 切换 ISO 存储池。
- 断开介质后，如果尚未加载过存储池，会重新读取。

调用接口：

- `GET /api/storage-pools/{agentId}`
- `GET /api/storage-pools/{agentId}/iso-files/{pool}`

主要刷新：

- 当前 VM 所在宿主机的 ISO/目录类存储池。
- 当前存储池中的 ISO 文件列表。
- 可连接到光驱的 ISO 路径和大小。

说明：

- 虚拟机运行中或已有介质连接时，介质选择和刷新按钮会被禁用。
- 这条刷新只服务于编辑弹窗的 ISO 选择，不刷新 VM 列表。

### 3.14 忘记密码验证码刷新

实现位置：

- `frontend/src/features/auth/ForgotPasswordPage.tsx`

触发方式：

- 进入忘记密码页面。
- 点击验证码按钮。
- 图形验证码过期后自动刷新。
- 身份校验失败后重新生成验证码。

调用接口：

- `GET /api/auth/password-reset/captcha`

主要刷新：

- 图形验证码题目。
- 验证码 token。
- 用户输入的验证码答案。

说明：

- 这是认证流程内的安全验证码刷新。
- 它和 KVM 资源运行态刷新没有关系。

### 3.15 通知面板刷新

实现位置：

- `frontend/src/components/layout/KvmLayout.tsx`
- `frontend/src/components/layout/NotificationPanel.tsx`

触发方式：

- 页面主布局首次加载。
- 点击右上角全量刷新图标。
- 打开右上角通知面板。
- 收到 `runtime.updated` 事件。
- 全部已读、清空通知、点击通知后。

调用接口：

- `GET /api/notifications`
- `GET /api/notifications/unread-count`

主要刷新：

- 通知列表。
- 未读数量。
- 通知已读状态。

说明：

- 通知刷新不采集 KVM 资源。
- 它主要用于同步告警和后台任务相关通知。

## 四、刷新链路对照表

| 刷新入口 | 是否采集 Agent | 范围 | 是否采集快照 | 主要刷新内容 |
| :-: | :-: | :-: | :-: | :-: |
| 后端定时 `runtime.refresh.fast` | 是 | 所有 Agent | 否 | 宿主机、VM 基础运行态、CPU/内存/I/O 指标样本 |
| 右上角全量刷新图标 / 手动 `POST /api/refresh` | 是 | 所有 Agent | 是 | 宿主机、VM 完整运行态、快照 |
| SSE `kvm:refresh` | 否 | 当前前端页面 | 否 | 通知页面重新读取后端缓存 |
| 新增 Agent 保存后的后台同步 | 后台采集 | 单个 Agent | 是 | 新 Agent 的宿主机、VM 完整运行态和快照 |
| 删除 Agent | 否 | 单个 Agent | 否 | 删除登记并清理该 Agent 的 host、VM 和快照运行态缓存；并发中的旧刷新任务写缓存前后会复查登记和删除标记，已删除则跳过写入 |
| Agent “同步 Agent” | 是 | 单个 Agent | 是 | 指定宿主机、其 VM 完整运行态和快照数据 |
| VM 行内“刷新” | 是 | 单台 VM | 否 | 单台 VM 的描述、OS/IP、磁盘、CPU/内存、I/O |
| VM 电源操作后的延迟 8 秒 full 同步 | 是 | 单个 Agent | 是 | 电源操作后补齐宿主机、VM 完整运行态和快照，接口先返回 |
| VM 删除后的缓存移除与延迟 8 秒 full 同步 | 后台延迟采集 | 单个 Agent | 是 | 先从缓存移除 VM 并刷新列表，随后 full 同步宿主机、快照和指标 |
| 快照恢复后的定向刷新 | 是 | 单台 VM | 否 | 恢复后的 VM 磁盘明细、容量、配置和运行态 |
| 快照页“刷新快照” | 是 | 各 Agent 的快照 | 是 | 快照运行态缓存 |
| 存储池页“刷新” | 读取 Agent/后端接口 | 当前宿主机 | 否 | 存储池列表和容量 |
| 网络池页“刷新” | 读取 Agent/后端接口 | 当前宿主机 | 否 | 网络池列表和配置 |
| 接口页“刷新” | 读取 Agent/后端接口 | 当前宿主机 | 否 | 物理网卡和 bridge 接口 |
| 监控弹窗“刷新监控” | 否 | 单个 VM 或宿主机 | 否 | 已落库指标曲线 |
| 介质页“刷新” | 读取 Agent/后端接口 | 当前 VM 所在宿主机 | 否 | ISO 存储池和 ISO 文件 |
| 验证码刷新 | 否 | 当前找回密码流程 | 否 | 图形验证码 |
| 通知刷新 | 否 | 当前用户 | 否 | 通知列表和未读数 |

## 五、已优化的自动刷新点

以下操作完成后会自动刷新到相关页面，不需要再手动点击 VM 行内刷新或页面刷新：

- 快照恢复成功后，后端定向刷新目标 VM 的完整运行态。
- VM 基础配置和内存统计周期修改成功后，后端定向刷新目标 VM 的完整运行态。
- VM 设备修改成功后，后端定向刷新目标 VM 的完整运行态。
- VM XML 保存成功后，后端定向刷新目标 VM 的完整运行态。
- VM 介质连接或断开成功后，后端定向刷新目标 VM 的完整运行态。
- VM 自启动配置修改成功后，后端定向刷新目标 VM 的完整运行态。
- VM 启动、恢复、暂停、关机、停止、强制关机、重启和强制重启成功后，后端先更新当前 VM 缓存状态，再后台延迟 8 秒 full 同步所属 Agent。
- VM 删除和强制删除成功后，后端先从运行态缓存移除该 VM 并广播 `runtime.updated`，再后台延迟 8 秒 full 同步所属 Agent。
- Agent 删除成功后，后端会移除该 Agent 的 host、VM 和快照运行态缓存；如果已有 fast/full 刷新任务仍在执行，刷新任务写缓存前后会复查 Agent 登记和删除标记并跳过已删除 Agent，避免总览或宿主机页出现已删除宿主机残留。
- 存储池和存储卷相关操作完成后，前端存储池页按 `agentId` 自动重读当前宿主机数据。
- 网络池相关操作完成后，前端网络池页按 `agentId` 自动重读当前宿主机数据。
- 宿主机接口相关操作完成后，前端接口页按 `agentId` 自动重读当前宿主机数据。

## 六、后续修改注意事项

修改刷新相关功能时，需要同步评估：

- 是否新增或改变了 Agent 采集范围。
- 是否改变 fast/full 的采集边界。
- 是否改变 Redis 运行态缓存写入内容。
- 是否改变 SSE 事件名称、发送时机或前端订阅页面。
- 是否新增页面刷新按钮或局部刷新入口。
- 是否需要调整权限点，例如 `agents.manage`、`snapshots.read`、`vms.read`、`vms.update` 等。
- 是否需要更新 `README.md`、`docs/vm-info-collection.md`、`docs/host-info-collection.md` 和本文档。

如果刷新链路改变了用户能看到的数据新鲜度、采集成本、权限边界或任务类型，必须在同一变更中同步文档。
