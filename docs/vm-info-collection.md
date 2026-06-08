# 虚拟机信息采集说明

本文档说明虚拟机页面中各项信息的采集链路、Agent 执行的命令、字段计算方式和回退策略。

## 一、整体链路

虚拟机页面不直接访问宿主机命令。当前链路如下：

1. 前端调用后端 `/api/vms` 读取运行态缓存。
2. 后端按 `RUNTIME_SYNC_INTERVAL` 自动触发面向所有 Agent 的全局运行态轻量刷新；按 `RUNTIME_DEEP_SYNC_INTERVAL` 自动触发低频 full 深度刷新；手动 `/api/refresh` 也会触发 full 全量刷新。
3. 轻量刷新创建或复用 `runtime.refresh.fast` 异步任务，低频深度刷新和手动刷新创建或复用 `runtime.refresh.all` 异步任务。
4. 后台刷新 worker 按任务向每个已登记 Agent 调用 `/v1/host`、`/v1/vms?level=fast` 或 `/v1/vms`；全量任务还会调用 `/v1/vms/{name}/snapshots`。
5. 虚拟机启动、恢复、暂停、关机、停止、强制关机、重启和强制重启成功后，后端会先更新运行态缓存中的 VM 状态并广播 `runtime.updated`，再延迟 8 秒执行一次 full 同步，避免电源操作响应被完整 Agent 同步阻塞。
6. Agent 在 KVM 宿主机上执行 `virsh`、`virt-df`、`virt-filesystems`、`qemu-img`、`hostname`、`ip`、`pgrep`、`ps` 等命令。
7. 后端把 Agent 返回结果写入 Redis 运行态缓存，并通过 SSE 通知前端更新。Redis 是后端必需依赖，连接失败时后端会直接启动失败。

刷新任务进度写入 `tasks.payload`。前端各刷新入口的范围、触发接口和 fast/full 边界详见 `docs/frontend-refresh-functions.md`。

`tasks.payload` 包含：

- `totalAgents`
- `syncedAgents`
- `failedAgents`
- `currentAgent`
- 每个 Agent 的执行结果

前端可通过 `/api/tasks/{id}` 查询任务详情，也会监听以下 SSE 事件刷新页面：

- `sync.queued`
- `sync.started`
- `sync.progress`
- `sync.finished`
- `runtime.updated`

主要实现位置：

- Agent 采集：`agent/internal/kvm/virsh.go`
- Agent 介质编辑：`agent/internal/kvm/virsh_media.go`
- Agent 返回结构：`agent/internal/kvm/types.go`
- 后端 Agent 客户端：`backend/pkg/agent/client.go`
- 后端运行态缓存：`backend/internal/service/realtime/service.go`
- 前端虚拟机页面：`frontend/src/features/vms/VMsPage.tsx`
- 前端创建/迁移弹窗：`frontend/src/features/vms/components/VMCreateDialog.tsx`、`frontend/src/features/vms/components/VMMigrateDialog.tsx`

## 二、宿主机信息

### 2.1 宿主机 IP

字段：`hostName`、宿主机列表中的 `address`

Agent 优先采集默认路由源 IP：

```bash
ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}'
```

这比 `hostname -I` 更适合多网卡宿主机，通常能取到当前管理出口使用的源地址。

若失败，回退：

```bash
ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1
```

再失败才使用：

```bash
hostname -I
```

从输出中选择第一个可用 IPv4，排除 `127.*` 和 `169.254.*`。

后端同步时：

- 虚拟机列表的 `hostName` 优先使用 Agent 返回的 `hostAddress`。
- 如果 Agent 未返回实际 IP，则回退为 Agent 登记的 `endpoint`。

## 三、虚拟机基础信息

### 3.1 虚拟机列表

Agent 先枚举所有虚拟机名称：

```bash
virsh --connect <LIBVIRT_URI> list --all --name
```

对每个名称再采集详情。

Agent 执行外部命令时会统一注入以下环境变量，使 `virsh`、`qemu-img`、`df`、`ip` 等命令输出优先保持英文，避免中文系统区域设置导致状态、表头等文本解析不稳定：

- `LC_ALL=C`
- `LANG=C`
- `LANGUAGE=C`

### 3.2 名称和 UUID

字段：`name`、`uuid`、`description`

命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm>
```

解析 XML：

- `<name>` -> `name`
- `<uuid>` -> `uuid`
- `<description>` -> `description`，与虚拟机编辑页基础信息中的描述一致；缺失或空白时返回空字符串，前端列表显示为 `-`

### 3.2.1 模板标记字段

字段：`isTemplate`、`templateId`、`templateName`、`templateDescription`

这些字段不是 Agent 采集字段，也不会写入 libvirt XML。

后端处理规则：

- 模板标记保存在 PostgreSQL `vm_template_marks` 表。
- 表中只保存 `agent_id`、`vm_uuid`、模板名称、描述、创建人和时间戳等必要字段。
- CPU、内存、磁盘、IP、状态等虚拟机详情仍以 Agent 运行态采集为准，不写入模板标记表。
- 后端读取 `/api/vms` 或 `/api/vms/{id}` 时，按 `agent_id + vm_uuid` 将模板标记合并到运行态 VM。

### 3.3 状态

字段：`status`

命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
```

实际执行时同样携带统一的 C locale 环境，因此中文宿主机上 `运行中`、`关闭` 等本地化输出会优先稳定为英文 `running`、`shut off`。

归一化规则：

| 原始内容 | 页面状态 |
| :-: | :-: |
| 包含 `running` | `running` |
| 包含 `paused` | `paused` |
| 包含 `shut off` 或 `shutoff` | `stopped` |
| 包含 `crashed` | `error` |
| 其他 | `unknown` |

## 四、系统和网络信息

### 4.1 操作系统

字段：`osType`

优先使用 QEMU Guest Agent：

```bash
virsh --connect <LIBVIRT_URI> qemu-agent-command <vm> '{"execute":"guest-get-osinfo"}'
```

优先读取返回中的：

- `pretty-name`
- `name`
- `version`
- `id`

若 QEMU Guest Agent 不可用，依次回退：

1. `dumpxml` 的 metadata/osinfo。
2. `virsh --connect <LIBVIRT_URI> dominfo <vm>` 中的 `OS:` 或 `OS type:`。
3. 根据 VM 名称猜测 Windows、CentOS、Ubuntu、Debian、Rocky Linux、AlmaLinux、Fedora。
4. XML 中 `<os><type arch="...">` 或 type 文本。

该回退链路只用于 full 模式和单台虚拟机手动刷新。定时 fast 模式不会执行 QEMU Guest Agent 查询，也不会根据 VM 名称猜测操作系统，避免轻量刷新把 full 模式或操作列单台刷新采集到的准确系统信息覆盖为名称猜测值。

后端写入运行态缓存前会保护已有的高置信系统类型：如果关机、重启早期或 Guest Agent 不可用时只能采到 `CentOS`、`Ubuntu`、`x86_64 virtualization` 等兜底值，而缓存中已有 `CentOS Linux 7 (Core)` 这类更具体的系统信息，则保留已有值，避免关机后的 full 同步把准确系统信息改回兜底结果。

### 4.2 主 IP

字段：`primaryIp`

优先使用 libvirt 的 Guest Agent 地址查询：

```bash
virsh --connect <LIBVIRT_URI> domifaddr <vm> --source agent
```

若失败，回退到 DHCP lease 地址查询：

```bash
virsh --connect <LIBVIRT_URI> domifaddr <vm> --source lease
```

解析规则：

- 逐行按空白字符切分 `domifaddr` 输出。
- 从左到右查找同时包含 `.` 和 `/` 的字段，例如 `10.22.51.56/24`。
- 使用 `/` 前的内容作为 IPv4 候选值。
- 跳过 `127.*` 回环地址。
- 跳过 `169.254.*` 链路本地地址。
- 返回第一个符合条件的 IPv4。

若运行中虚拟机当前缓存 IP 为空、回环地址或链路本地地址，后端 fast 定时刷新不会额外触发完整 VM 详情采集。该字段会等待后端低频 full 深度刷新、手动 full 全量刷新、单台 VM 刷新或 VM 操作后的定向刷新补齐。

## 五、CPU 信息

### 5.1 CPU 规格

字段：`cpuCores`

命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm>
```

解析 XML 的 `<vcpu>`：

1. 优先使用 `<vcpu current="N">` 的 `current`。
2. 如果没有 `current` 或值无效，则使用 `<vcpu>` 文本值。

### 5.2 CPU 使用率

字段：`cpuUsage`、`cpuUsageAvailable`

每次 Agent 执行 `/v1/vms` 时，在本次刷新窗口内做两次批量采样，间隔 1 秒。CPU 与磁盘 I/O、网络吞吐共用同一个等待窗口，避免串行等待两次。

第一次采样：

```bash
virsh --connect <LIBVIRT_URI> domstats --cpu-total <vm1> <vm2> ...
```

等待：

```text
1 秒
```

第二次采样：

```bash
virsh --connect <LIBVIRT_URI> domstats --cpu-total <vm1> <vm2> ...
```

解析每个 VM 的 `cpu.time`，按以下公式计算：

```text
CPU 使用率 = (第二次 cpu.time - 第一次 cpu.time) / 两次采样间隔纳秒 / vCPU(current) * 100
```

说明：

- 已停止虚拟机直接返回 `0%`，并视为可用值。
- 如果两次采样缺失、`cpu.time` 无效或采样时间异常，`cpuUsageAvailable=false`。
- 前端中值为 `0` 时显示 `0%`，不会显示 `-`。

## 六、内存信息

### 6.1 内存规格

字段：`memoryBytes`

命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm>
```

Agent 默认读取当前 `dumpxml <vm>`；运行中的虚拟机会额外读取 `dumpxml --inactive <vm>`，用持久配置 XML 中的 `<currentMemory>` 作为列表内存规格。解析 XML `<currentMemory>`，缺失时使用 `<memory>`，按 KiB 转 bytes。

说明：

- `memoryBytes` 用于虚拟机列表和迁移弹窗中的内存规格展示。
- 运行中虚拟机以内存持久配置中的 `<currentMemory>` 作为规格值，不使用 live XML、`dommemstat actual` 或 `dominfo Used memory` 覆盖，避免 balloon、libvirt live XML 或 QEMU 运行态上限把列表规格显示成最大内存。
- 该 `--inactive` 读取只用于列表 `memoryBytes` 规格字段，不改变磁盘、网卡、介质等设备查看仍以当前 XML 为准的规则。

网卡列表以当前 XML 为准：

- 运行中虚拟机返回当前会话里的 `<target dev>`、`<interface type>`、`<source>` 和 `<model>`。
- 已停止虚拟机返回持久定义中的网卡信息。

### 6.2 内存使用率

字段：`memoryUsage`、`memoryUsageAvailable`

仅运行中的虚拟机计算使用率。

命令：

```bash
virsh --connect <LIBVIRT_URI> dommemstat <vm>
```

解析：

- `actual`
- `usable`
- `available`

计算：

```text
内存使用率 = (actual - usable) / actual * 100
```

说明：

- 已停止虚拟机直接返回 `0%`，并视为可用值。
- 优先使用 `usable` 计算；如果没有合法 `usable`，则使用 `available` 兜底，公式为 `(actual - available) / actual * 100`。
- 如果 `actual` 无效，或 `usable`、`available` 都无效，则 `memoryUsageAvailable=false`。

## 七、磁盘信息

### 7.1 磁盘列表

字段：`disks[]`

命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm>
```

解析 XML 中 `devices.disk`：

- `target dev` -> 磁盘名称，例如 `vda`、`sda`
- `source file` 或 `source dev` -> 磁盘路径

### 7.2 每块磁盘总大小和已用大小

字段：`disks[].bytes`、`disks[].usedBytes`

每块磁盘总大小执行：

```bash
virsh --connect <LIBVIRT_URI> domblkinfo <vm> <disk-path>
```

注意：当前实现不带 `--bytes`，因为部分 libvirt/virsh 版本不支持该参数。

解析：

- `Capacity:` -> 总大小

`domblkinfo` 输出可能带单位，当前会解析并换算为 bytes：

- `B`、`bytes`
- `KiB`、`MiB`、`GiB`、`TiB`
- `KB`、`MB`、`GB`、`TB`

每块磁盘已用大小执行：

```bash
virt-df --csv -d <vm>
```

解析 CSV 中 `Filesystem` 和 `Used`：

- `Used` 为 1K-blocks，换算为 bytes 后写入 `disks[].usedBytes`。
- `Filesystem` 可能为普通分区，如 `/dev/sda2`，也可能为 LVM 逻辑卷，如 `/dev/vgdata/data`。
- Agent 会继续执行 `virt-filesystems --csv -d <vm> --all --long` 解析分区、PV、VG、LV 拓扑。
- 能唯一追溯到 libguestfs 设备 `/dev/sdX` 时，再按 libvirt XML 中磁盘顺序映射到 `vda`、`vdb` 等磁盘。
- 无法唯一归属的文件系统不强行分摊到单块磁盘。

性能说明：

- Agent full 列表采集会优先执行一次全局 `virt-df --csv`，再按 VM 名称复用对应文件系统使用量，避免每台 VM 都启动一次 libguestfs appliance。
- 单台 VM 刷新仍执行 `virt-df --csv -d <vm>`。
- 每台 VM 仍需执行 `virt-filesystems --csv -d <vm> --all --long` 解析拓扑，用于把文件系统使用量归属到具体磁盘。
- Agent 执行 `virt-df` 和 `virt-filesystems` 时会默认注入 `LIBGUESTFS_BACKEND=direct`；如果 Agent 进程环境已显式设置该变量，则保留现有值。

说明：

- 当前磁盘使用大小不再使用 `domblkinfo Allocation`、`qemu-img actual-size`、`stat` 或 `blockdev` 作为回退。
- 如果 `virt-df` 或 `virt-filesystems` 未返回可归属结果，`disks[].usedBytes` 保持 `0`。
- 创建虚拟机后延迟 8 秒 full 同步时，若客户机文件系统尚未就绪或 `virt-df` 暂时取不到结果，也按上述规则显示 `0` 使用量；总大小仍按 `domblkinfo Capacity` 展示。
- Agent 会用英文 warning 日志记录 `virt-df`、`virt-filesystems` 失败、无结果或无法归属的情况，便于排查宿主机缺少 libguestfs 工具、权限不足或文件系统暂不可识别等问题。

### 7.3 磁盘总量和使用率

字段：`diskBytes`、`diskUsedBytes`、`diskUsage`、`diskUsageAvailable`

计算：

```text
diskBytes = 所有 disks[].bytes 之和
diskUsedBytes = 所有 disks[].usedBytes 之和
diskUsage = diskUsedBytes / diskBytes * 100
```

说明：

- full 模式和单台 VM 刷新会采集磁盘容量与客户机文件系统使用量。
- fast 模式不执行 `domblkinfo`、`virt-df` 或 `virt-filesystems`，后端会沿用上一次 full 或单台 VM 刷新得到的磁盘详情。
- `diskUsage` 表示可归属客户机文件系统已用空间占虚拟磁盘总容量的比例，不再表示 qcow2/raw 镜像层的宿主机占用比例。

## 八、运行时长

字段：`uptimeSeconds`

仅运行中的虚拟机采集。

Agent 优先用 VM UUID 匹配进程；失败后用 VM 名称匹配：

```bash
pgrep -f <VM_UUID>
pgrep -f <VM_NAME>
```

拿到 PID 后：

```bash
ps -p <pid> -o etime=
```

解析 `etime` 格式：

- `MM:SS`
- `HH:MM:SS`
- `D-HH:MM:SS`

转换为秒后返回。

前端展示规则：小于 1 天显示为 `xx时 xx分`，大于等于 1 天显示为 `xx天 xx时`。

## 九、快照信息

字段：快照页面的 `snapshots[]`

命令：

```bash
virsh --connect <LIBVIRT_URI> snapshot-list <vm>
```

Agent 解析 `snapshot-list` 表格输出：

- `Name` 转换为快照名称。
- `Creation Time` 转换为快照创建时间。
- `State` 转换为快照状态。

如果 `Creation Time` 解析失败，后端会退回本次采集时间，避免快照列表缺少时间字段。

后端会补充以下平台侧字段：

- `sizeBytes` 置为 `0`
- `created_at` 设置为本次全量同步时间

平台侧可为快照维护备注元数据：

- `displayName`
- `description`
- `tags`

这些数据保存在 `snapshot_annotations` 表中，不修改 libvirt 快照实体，也不把快照主数据落库。

快照页的创建、恢复、删除与备注编辑走控制中心接口：

| 前端动作 | 后端接口 | Agent 动作 | Agent 命令 |
| --- | :-: | :-: | :-: |
| 刷新快照 | `POST /api/snapshots/refresh` | 无 | 后端仅向各 Agent 调用 `/v1/vms/{name}/snapshots` 并替换快照运行态缓存，不触发宿主机、虚拟机详情和指标全量刷新 |
| 创建快照 | `POST /api/snapshots` | `create` | `virsh --connect <LIBVIRT_URI> snapshot-create-as <vm> <snapshot> [--description <description>] --atomic` |
| 编辑备注 | `PUT /api/snapshots/{id}/annotation` | 无 | 仅更新平台侧 `snapshot_annotations` |
| 恢复快照 | `POST /api/snapshots/{id}/revert` | `revert` | `virsh --connect <LIBVIRT_URI> snapshot-revert <vm> <snapshot>` |
| 删除快照 | `POST /api/snapshots/{id}/delete` | `delete` | `virsh --connect <LIBVIRT_URI> snapshot-delete <vm> <snapshot>` |

快照操作规则：

- 创建快照：
  - 后端根据 `vmId` 找到 VM 和所属 Agent，使用已保存的加密 Agent Token 调用 Agent `/v1/vms/{name}/snapshots`。
  - 请求参数包含 `vmId`、`name`、可选 `description` 和平台侧 `tags`。
  - 后端会预检查虚拟机必须为已关机状态，前端创建窗口也只允许选择已关机虚拟机。
  - Agent 使用 libvirt 默认内部快照创建方式，不追加 `--disk-only` 或 `--diskspec`。
  - 内部快照通常保存在虚拟机现有 qcow2 磁盘镜像内部，同时由 libvirt 维护快照元数据；创建后不会把当前活动磁盘切换到外部 overlay。
  - 内部快照依赖磁盘格式和宿主机 QEMU/libvirt 支持；不支持时需要调整磁盘格式。
  - 包含内部快照的 qcow2 镜像不能直接通过 `qemu-img resize` 扩容。若需要扩容，需先删除相关内部快照，或迁移/重建为无内部快照的磁盘后再扩容。
- 恢复与删除：
  - 后端根据快照运行态记录找到所属 VM，再根据 VM 的 `hostId` 找到 Agent。
  - 后端调用 Agent `/v1/vms/{name}/snapshots/{snapshot}/{action}` 执行恢复或删除。
  - 删除快照直接调用 libvirt 删除内部快照，不执行外部 overlay 合并或存储卷清理。
- 完成后处理：
  - 操作完成后会同步该 Agent 的运行态缓存、广播 SSE，并写入任务与审计日志。
  - VM、快照、存储卷等操作的任务、审计和告警覆盖边界详见 `docs/operation-log-coverage.md`。
- 失败与限制：
  - 若 Agent 执行创建、恢复或删除快照失败，会把 `virsh` 返回的错误摘要透传给后端和前端，便于判断是否由虚拟机状态、磁盘格式、权限或 libvirt 限制导致。
  - 若恢复快照时 libvirt/qemu-img 报告快照引用的磁盘文件不存在，Agent 和后端会提取缺失路径，并转换为简短中文提示，提醒用户找回文件或删除失效快照。
  - 普通 libvirt 快照不提供下载文件。

## 十、虚拟机创建、编辑与资源配置链路

### 10.1 页面入口与展示规则

**数据来源与配置采集**

- 虚拟机列表读取后端运行态缓存，不直接执行命令。
- 虚拟机编辑窗口打开时会调用 `/api/vms/{id}/config`，后端根据 VM 所属 Agent 转发到 `/v1/vms/{name}/config`，实时读取 libvirt 配置，不再用列表运行态字段拼接编辑内容。

**CPU、内存与自启动**

- 虚拟机编辑窗口“CPU与内存”页的“基本配置”区域展示名称、描述和自启动。
- 运行中的虚拟机会禁用名称，但仍允许修改描述。
- “CPU与内存”页仅开放当前 CPU 和当前内存热扩容，最大 CPU 和最大内存保持禁用并提示需关机后修改。
- 虚拟机编辑窗口“CPU与内存”页点击修改时，名称变更会先调用 `/api/vms/{id}/rename`，随后调用 `/api/vms/{id}/config` 修改描述、vCPU 当前/最大分配和内存当前/最大分配。
- 运行中虚拟机只允许在已预留上限内增加当前 CPU 和当前内存，描述为空时 Agent 执行 `desc --live --config --new-desc ""`。
- 已停止虚拟机允许在宿主机上限内调整当前 CPU 和最大 CPU；当前 CPU 下拉范围从 `1` 到宿主机逻辑 CPU 上限，最大 CPU 下拉范围从当前配置 CPU 到宿主机逻辑 CPU 上限。
- 已停止虚拟机允许在宿主机总内存上限内调整当前内存和最大内存；当前内存与最大内存下拉范围从 `1024 MB` 开始，按 `1024 MB` 间隔递增，并包含宿主机内存上限值。
- “随宿主机同启”按钮独立调用 `/api/vms/{id}/autostart`，不再随 CPU/内存修改按钮一起提交。
- 编辑窗口内 CPU、内存、磁盘、网络、介质和 XML 任一保存成功后，前端会立即重新读取 `/api/vms` 更新虚拟机列表，不只依赖 SSE 或定时刷新。

**磁盘扩容与内部快照**

- 已停止虚拟机扩容已有磁盘时，Agent 在执行 `qemu-img resize <path> <bytes>` 前会先执行：
  - `qemu-img info --output=json <path>`
- 若 `qemu-img info` 返回 `snapshots` 字段，说明磁盘镜像内包含内部快照，Agent 会拒绝扩容并返回中文提示。
- 运行中虚拟机扩容已有磁盘仍使用：
  - `virsh --connect <LIBVIRT_URI> blockresize <vm> <target> <bytes>B`
- 内部快照与磁盘扩容的约束来自 qcow2/qemu-img 行为，平台不会自动删除快照或合并磁盘，避免用户未确认时丢失回滚点。

**介质与 ISO 选择**

- 虚拟机编辑窗口“介质”页会按虚拟机 `hostId` 调用 `/api/storage-pools/{agentId}`，筛选该宿主机上的目录/ISO 存储池。
- 选择 ISO 存储池后，再调用 `/api/storage-pools/{agentId}/iso-files/{pool}` 读取 `.iso` 文件用于选择展示。
- 虚拟机编辑窗口“介质”页会展示当前 CDROM、目标光驱、ISO 存储池和 ISO 文件。
- 未连接时点击“连接”会调用 `PUT /api/vms/{id}/media`。
- 已连接时中间选择控件禁用并将按钮切换为“断开”，点击后调用 `DELETE /api/vms/{id}/media`。
- 连接 ISO 后会同步调整持久 XML 启动顺序：目标 CDROM 为第一启动项，第一块普通磁盘为第二启动项；断开 ISO 后恢复第一块普通磁盘为第一启动项。
- 连接和断开均仅允许已停止虚拟机操作，运行中的虚拟机会在前端和后端返回中文提示。

**存储池与网络池详情**

- 存储池和网络池页面的卡片详情弹窗可查看 libvirt 池状态，并通过独立接口修改启动状态和自启动状态。
- 存储池详情弹窗会调用 `/api/storage-pools/{agentId}/volumes/{pool}` 读取该池全部卷或光盘镜像。
- ISO 类型池展示为“光盘镜像”和“上传ISO”，其他池展示为“卷”和“添加镜像”。
- Agent 先用 `virsh vol-list <pool> --details` 获取卷名称、路径、容量和已分配容量；若卷名或路径扩展名为 `.iso`，格式直接显示 `iso`。
- 非 ISO 卷会对卷路径执行 `qemu-img info -U --output=json <path>` 读取真实 `format` 字段；若单个文件探测失败，则回退按扩展名判断，避免整个列表不可用。
- ISO 类型池通过 `/api/storage-pools/{agentId}/volumes/{pool}/upload` 上传 ISO 文件。
- 非 ISO 类型池通过 `/api/storage-pools/{agentId}/volumes/{pool}` 创建 qcow2、qcow、qed 或 raw 镜像卷，其中 qcow2 可传递 `preallocMetadata` 让 Agent 执行 `virsh vol-create-as --prealloc-metadata`。
- 前端创建镜像时，qcow2 使用 `.qcow2` 扩展名，qcow、qed 和 raw 使用 `.img` 扩展名。
- 非 ISO 卷还可通过 `/api/storage-pools/{agentId}/volumes/{pool}/clone` 克隆；勾选转换时 Agent 使用 `qemu-img convert -O raw|qcow|qcow2|qed <source> <target>` 生成目标卷。
- 上传和克隆属于大文件长耗时操作，后端会创建后台任务并立即返回，任务完成或失败后通过 SSE 通知前端展示 toast 并刷新页面。

**表格指标与滚动展示**

- CPU、内存、磁盘三列展示“规格 + 使用率”；内存规格小于 1GB 时使用 MB 展示，达到 1GB 后使用 GB 展示。
- 使用率为 `0` 时显示 `0%`。
- 采集失败且值不是 `0` 时显示 `-`。
- 磁盘列悬浮后立即显示自定义明细浮层，展示每块磁盘名称、已用大小和总大小。
- 表格超过 10 条记录时启用内部滚动；10 条以内不启用内部滚动。
- 内部滚动隐藏滚动条，表头固定在顶部。

### 10.2 创建虚拟机链路

#### 10.2.1 接口与 Agent 命令

后端接口：

```http
POST /api/vms
```

Agent 接口：

```http
POST /v1/vms
```

Agent 执行命令：

```bash
# 检查目标虚拟机名称是否已在当前 libvirt 中存在
virsh --connect <LIBVIRT_URI> dominfo <vm>

# 检查宿主机 CPU 和内存上限
virsh --connect <LIBVIRT_URI> nodeinfo

# 常规模式：为请求中的每块磁盘创建存储卷；多磁盘时会按 disks 数组重复执行
virsh --connect <LIBVIRT_URI> vol-create-as \
  --pool <pool> \
  --name <disk> \
  --capacity <bytes>B \
  --format <format> \
  [--prealloc-metadata]

# 模板模式：从已有模板磁盘克隆系统盘，源模板支持 qcow2、img、raw、qcow、qed 等可克隆格式
virsh --connect <LIBVIRT_URI> vol-clone \
  --pool <target-pool> \
  <source-volume> \
  <target-volume> \
  [--prealloc-metadata]

# 跨存储池或需要转换时，Agent 使用 qemu-img convert 后刷新目标存储池
qemu-img convert -O <format> <source-path> <target-path>
virsh --connect <LIBVIRT_URI> pool-refresh <target-pool>

# 通过 virt-install 生成虚拟机 XML；不会直接安装系统，输出 XML 后由 Agent 提取 <domain> 节点
virt-install \
  --connect <LIBVIRT_URI> \
  --name <vm> \
  --memory <current,maxmemory=max> \
  --vcpus <current,maxvcpus=max> \
  --disk path=<volume>,bus=<bus>,format=<format> \
  [--disk path=<volume>,bus=<bus>,format=<format> ...] \
  --disk [path=<iso_path>,]device=cdrom,readonly=on,bus=<iso_bus> \
  --network <network-arg>,model=<model> \
  --graphics vnc,listen=0.0.0.0[,password=<password>] \
  --channel unix,target_type=virtio,name=org.qemu.guest_agent.0 \
  --import \
  --noautoconsole \
   --cpu <cpu-model> \
  --os-type <linux|windows> \
  [--boot cdrom,hd | --boot hd] \
  [--boot uefi] \
  --print-xml \
  --dry-run

# 将 virt-install 生成的 domain XML 写入临时文件后定义虚拟机
virsh --connect <LIBVIRT_URI> define <domain-xml>

# 当创建请求填写描述时，写入虚拟机 description
virsh --connect <LIBVIRT_URI> desc <vm> --config --new-desc <description>

# 当勾选“创建后直接启动”时，定义完成后启动虚拟机
virsh --connect <LIBVIRT_URI> start <vm>
```

#### 10.2.2 多磁盘创建示例

多磁盘创建示例：

```bash
# 创建系统盘存储卷
virsh --connect qemu:///system vol-create-as \
  --pool default \
  --name test-vda.qcow2 \
  --capacity 42949672960B \
  --format qcow2 \
  --prealloc-metadata

# 创建第二块数据盘存储卷
virsh --connect qemu:///system vol-create-as \
  --pool default \
  --name test-vdb.qcow2 \
  --capacity 21474836480B \
  --format qcow2 \
  --prealloc-metadata

# 生成包含两块磁盘、网络池、VNC 控制台和 ISO 启动介质的虚拟机 XML
virt-install \
  --connect qemu:///system \
  --name test \
  --memory 4096 \
  --vcpus 2 \
  --disk path=/var/lib/libvirt/images/test-vda.qcow2,bus=virtio,format=qcow2 \
  --disk path=/var/lib/libvirt/images/test-vdb.qcow2,bus=virtio,format=qcow2 \
  --disk path=/iso/CentOS.iso,device=cdrom,readonly=on,bus=sata \
  --network network=br2250,model=virtio \
  --graphics vnc,listen=0.0.0.0,password=<password> \
  --import \
  --noautoconsole \
  --cpu host-passthrough \
  --os-type linux \
  --boot cdrom,hd \
  --print-xml \
  --dry-run

# 使用生成的临时 domain XML 定义虚拟机
virsh --connect qemu:///system define <temp-domain-xml>

# 如配置了创建后直接启动，则启动虚拟机
virsh --connect qemu:///system start test
```

#### 10.2.3 模板创建示例

模板创建示例：

```bash
# 从模板卷克隆新的系统盘
virsh --connect qemu:///system vol-clone \
  --pool default \
  centos-template.qcow2 \
  test-vda.qcow2

# 使用克隆出的系统盘生成导入式虚拟机 XML
virt-install \
  --connect qemu:///system \
  --name test \
  --memory 4096 \
  --vcpus 2 \
  --disk path=/var/lib/libvirt/images/test-vda.qcow2,bus=virtio,format=qcow2 \
  --disk device=cdrom,readonly=on,bus=sata \
  --network network=default,model=virtio \
  --graphics vnc,listen=0.0.0.0 \
  --import \
  --noautoconsole \
  --cpu host-passthrough \
  --os-type linux \
  --boot hd \
  --print-xml \
  --dry-run

virsh --connect qemu:///system define <temp-domain-xml>
```

模板模式失败回滚规则：

- 如果模板卷克隆成功，但 `virt-install` 生成 XML、`virsh define`、写入描述或启动失败，Agent 会删除刚克隆出的目标卷。
- 如果目标存储池已存在同名目标卷，后端会在任务排队前拒绝创建。
- 后端兼容旧磁盘模板模式时只允许 `qcow2`、`img`、`raw`、`qcow`、`qed` 等可克隆磁盘文件，不接受 ISO。
- 如果源模板磁盘正在被运行中的虚拟机使用，`qemu-img` 克隆可能返回写锁失败；后端会将该类错误映射为“模板文件正在被运行中的虚拟机使用”的中文任务失败信息。

#### 10.2.4 创建后启动同步

创建任务完成后，后端先执行 fast 同步并广播 `runtime.updated`，让新虚拟机尽快出现在列表中。随后后端延迟 8 秒触发一次 full 同步，补齐 Guest Agent、主 IP、磁盘明细和快照等重字段。勾选“创建后直接启动”时，延迟 full 同步也能降低客户机服务尚未就绪时把系统类型、主 IP 和内存使用率采成兜底值的概率。

#### 10.2.5 前端表单与校验

- 前端虚拟机页面右上角“创建虚拟机”按钮打开创建弹窗。
  - 弹窗顶部提供“常规”“模板”和“XML”菜单。
  - “常规”菜单保持原有从空磁盘创建流程。
  - “模板”菜单从已标记的虚拟机模板创建，需先在虚拟机列表中将已停止虚拟机标记为模板。
  - 从虚拟机模板创建时复用整机克隆配置，需填写新虚拟机名称、目标磁盘卷名称、目标存储池、网卡 MAC 和网络池。
  - “XML”菜单提供完整 XML 粘贴区，并复用编辑虚拟机 XML 页的搜索、高亮和上/下一个命中布局。
  - XML 创建提交时仍需要选择宿主机，虚拟机名称直接在 XML `<domain><name>` 中配置。
  - XML 创建由后端和 Agent 从 XML name 读取虚拟机名称，并校验 XML 非空、可解析以及名称不重复。
  - 创建任务提交成功后，前端使用固定 toast 卡片展示排队、创建中、完成或失败状态，避免耗时创建期间提示自动消失。
  - 按宿主机选择系统盘存储池、ISO 池、ISO 文件、ISO 总线、网络池和操作系统类型。
  - 最大 CPU 不能超过所选宿主机逻辑 CPU。
  - 最大内存不能超过所选宿主机总内存。
  - 支持添加多个数据盘。
  - 只有系统盘显示 ISO 池、ISO 镜像和 ISO 总线选择。
  - ISO 总线默认 `sata`，支持 `sata`、`ide`、`scsi` 和 `usb`。
  - 未选择 ISO 镜像时，Agent 仍会写入 `--disk device=cdrom,readonly=on,bus=<iso_bus>` 创建空 CDROM 设备。
  - 系统盘卷名可编辑，默认按虚拟机名称、磁盘总线和格式自动生成。
  - `virtio` 使用 `vda`，`sata/scsi` 使用 `sda`，`ide` 使用 `hda`。
  - `qcow2` 使用 `.qcow2`，其他格式使用 `.img`。
  - 数据盘的存储池、磁盘格式、磁盘总线、卷名称和 `preallocMetadata` 均继承系统盘配置并禁用编辑，只允许单独填写容量。
  - 后端和 Agent 会再次校验数据盘配置必须与系统盘一致。
- 创建弹窗中的磁盘格式和 VMware 的精简置备、厚置备延迟置零、厚置备置零不是一一对应关系：
  - `qcow2` 是支持快照和后备链的镜像格式，默认按需增长，更接近精简置备。
  - qcow2 metadata 预分配默认不勾选。
  - 勾选时只提前分配 qcow2 元数据，不预分配完整数据空间。
  - `raw` 是裸格式，是否表现为精简或厚置备取决于 `virsh vol-create-as`、`qemu-img`、文件系统和后端存储的分配方式。
  - 当前创建链路未实现完整厚置备或写零初始化策略。

#### 10.2.6 后端任务与 Agent 定义

- 后端在常规创建后台任务前检查目标 Agent、宿主机 CPU/内存上限、虚拟机名称和目标磁盘卷名，避免重复名称或超出宿主机资源的请求进入 Agent 执行阶段；新版请求使用 `disks` 数组描述一块或多块磁盘，未携带 `disks` 时兼容旧的单系统盘字段。
- XML 创建模式不创建磁盘卷，不检查存储池卷名，也不读取宿主机 CPU/内存上限；该模式依赖用户提交的完整 libvirt XML，并在 Agent 侧直接执行 `virsh define`。
- Agent 先创建请求中的全部磁盘卷，再调用 `virt-install --import --noautoconsole --print-xml --dry-run` 生成虚拟机 XML。
  - Agent 直接通过 `virsh define` 定义虚拟机。
  - Agent 不再二次重写生成后的内存 XML 节点。
  - 创建请求中的操作系统类型会作为 `--os-type` 传给 `virt-install`。
  - CDROM 设备始终通过 `--disk ...device=cdrom,readonly=on,bus=<iso_bus>` 写入。
  - 选择 ISO 时包含 `path=<iso_path>`。
  - 未传 `isoBus` 时默认 `sata`。
  - 当前控制台类型只支持 VNC。
  - 若请求携带控制台密码，会写入 `--graphics vnc,listen=0.0.0.0,password=<password>`。
  - 常规和模板创建会写入 QEMU Guest Agent channel：`--channel unix,target_type=virtio,name=org.qemu.guest_agent.0`。
  - XML 创建不会自动注入 channel，需由用户提交的 XML 自行包含 `<channel type="unix">` 配置。
  - 网络池为 bridge 转发模式且存在 bridge 设备名时，写入 `--network bridge=<bridge>,model=<model>`。
  - 其他网络池按 libvirt 网络名写入 `--network network=<pool>,model=<model>`。
  - Agent 执行创建前会读取 `nodeinfo` 再次校验最大 CPU 和最大内存不能超过宿主机资源。
  - 若定义失败，会清理本次已创建的所有磁盘卷。
- Agent 收到 XML 创建请求时：
  - 校验 XML 非空且可被解析。
  - 从 XML `<name>` 读取虚拟机名称。
  - 校验目标虚拟机名称尚不存在。
  - 将 XML 写入临时文件后执行 `virsh define`。
  - 若请求 `autostart=true`，定义后执行 `virsh start <vm>`。
- `virt-install` 本身也支持在创建虚拟机时创建磁盘，并不强制要求提前执行 `virsh vol-create-as`：
  - `--disk pool=<pool>,size=<gb>,format=<format>`
  - `--disk path=<path>,size=<gb>,format=<format>`
- 当前项目选择先创建卷，原因包括：
  - 在任务排队前检查卷名冲突。
  - 统一纳入存储池卷管理。
  - 在后续 `virt-install` 或 `virsh define` 失败时精确清理本次已创建的所有卷。

#### 10.2.7 完成事件与自启动边界

- 创建弹窗中的 `autostart` 表示“创建后直接启动”，开启时定义后执行 `virsh start <vm>`，不修改 libvirt 随宿主机同启配置；随宿主机同启仍通过编辑窗口的独立自启动接口修改。
- 创建任务完成后，后端广播 `vm.create.completed`，并写入任务与审计日志。
- 后端随后对目标 Agent 执行 fast 同步并广播 `runtime.updated`，让新虚拟机尽快出现在列表中。
- 后端延迟执行 full 同步并再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和快照等重字段。

### 10.3 编辑窗口真实配置采集

字段：`description`、`autostart`、`currentCpu`、`maximumCpu`、`hostCpu`、`currentMemoryBytes`、`maximumMemoryBytes`、`hostMemoryBytes`、`disks`、`interfaces`、`cdroms`、`xml`

后端接口：

```http
GET /api/vms/{id}/config
```

Agent 接口：

```http
GET /v1/vms/{name}/config
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm>
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> dominfo <vm>
virsh --connect <LIBVIRT_URI> nodeinfo
virsh --connect <LIBVIRT_URI> dumpxml <vm> --inactive
virsh --connect <LIBVIRT_URI> pool-list --all --name
virsh --connect <LIBVIRT_URI> pool-dumpxml <pool>
virsh --connect <LIBVIRT_URI> domblkinfo <vm> <disk-path>
qemu-img info -U --output=json <disk-path>
```

解析规则：

| 字段 | 来源 |
| :-: | :-: |
| `xml` | 当前 `dumpxml`，不包含 `--security-info` 隐藏的 VNC 密码等敏感字段 |
| `description` | XML `<description>` |
| `autostart` | `dominfo` 中 `Autostart` |
| `currentCpu` | XML `<vcpu current>`，缺失时使用 `<vcpu>` 文本值 |
| `maximumCpu` | XML `<vcpu>` 文本值 |
| `hostCpu` | `nodeinfo` 中 `CPU(s):` |
| `currentMemoryBytes` | XML `<currentMemory>`，缺失时使用 `<memory>`，按 KiB 转 bytes；运行中额外读取 `dumpxml --inactive`，优先使用持久配置中的 CPU/内存分配值 |
| `maximumMemoryBytes` | XML `<memory>`，按 KiB 转 bytes；运行中额外读取 `dumpxml --inactive`，优先使用持久配置中的 CPU/内存分配值 |
| `hostMemoryBytes` | `nodeinfo` 中 `Memory size:`，按 KiB 转 bytes |
| `memoryStatsPeriod` | XML `<memballoon><stats period="...">`，未配置时为 `0` |
| `disks` | XML `<devices><disk device="disk">`，`pool` 通过磁盘路径匹配存储池 `<target><path>` 的最长路径前缀，容量优先 `domblkinfo Capacity`，失败回退 `qemu-img virtual-size` |
| `interfaces` | XML `<interface>` 的 `mac/source/target/model`；运行中取当前 XML，已停止取持久定义 |
| `cdroms` | XML `<disk device="cdrom">` 的 `source/target` |

### 10.4 编辑窗口配置修改链路

当前已接入“CPU与内存”页的修改操作、“介质”页的 ISO 连接操作、“磁盘与网络”页的设备配置修改、虚拟机列表操作列的独立“克隆”弹窗和“XML”页的完整 XML 写入操作；运行中虚拟机支持在“CPU与内存”页热扩容当前 CPU 与当前内存，在“磁盘与网络”页热扩容已有磁盘并热添加新磁盘，介质连接/断开、网络设备修改、克隆和 XML 保存需关机后操作。

#### 10.4.1 CPU、内存与名称接口

后端接口：

```http
PUT /api/vms/{id}/config
```

请求体：

```json
{
  "description": "生产 Web 服务",
  "currentCpu": 4,
  "maximumCpu": 8,
  "currentMemoryMB": 8192,
  "maximumMemoryMB": 16384,
  "memoryStatsPeriod": 5
}
```

名称修改后端接口：

```http
PUT /api/vms/{id}/rename
```

名称修改请求体：

```json
{
  "name": "prod-web-01"
}
```

Agent 接口：

```http
PUT /v1/vms/{name}/config
```

名称修改 Agent 接口：

```http
PUT /v1/vms/{name}/rename
```

#### 10.4.2 Agent 资源修改命令

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> setvcpus <vm> <maximumCpu> --maximum --config
virsh --connect <LIBVIRT_URI> setvcpus <vm> <currentCpu> --config
virsh --connect <LIBVIRT_URI> setmaxmem <vm> <maximumMemoryKiB> --config
virsh --connect <LIBVIRT_URI> setmem <vm> <currentMemoryKiB> --config
virsh --connect <LIBVIRT_URI> setvcpus <vm> <currentCpu> --live --config
virsh --connect <LIBVIRT_URI> setmem <vm> <currentMemoryKiB> --live --config
virsh --connect <LIBVIRT_URI> dommemstat <vm> --period <seconds> --config
virsh --connect <LIBVIRT_URI> dommemstat <vm> --period <seconds> --live --config
virsh --connect <LIBVIRT_URI> desc <vm> --config --new-desc <description>
virsh --connect <LIBVIRT_URI> desc <vm> --live --config --new-desc <description>
virsh --connect <LIBVIRT_URI> define <temp-domain-xml>
virsh --connect <LIBVIRT_URI> domrename <vm> <new-name>
```

#### 10.4.3 描述、重命名与运行中边界

说明：

- 已停止虚拟机描述非空时执行 `desc --config --new-desc <description>`。
- 运行中虚拟机描述非空时执行 `desc --live --config --new-desc <description>`，同时更新运行态和持久配置。
- 已停止虚拟机描述为空时执行 `desc --config --new-desc ""`；运行中虚拟机描述为空时执行 `desc --live --config --new-desc ""`；前端为空时以 `None` 作为占位展示。
- 名称修改仅允许已停止虚拟机，后端会先通过 Agent 快速 VM 列表检查同宿主机是否已有重名虚拟机，Agent 执行 `domrename` 后返回新名称的轻量配置。
- 运行中的虚拟机若只修改描述，Agent 允许执行；若请求增加当前 CPU 或当前内存，Agent 使用 `setvcpus --live --config` 和 `setmem --live --config` 同时更新运行态与持久配置。
- 运行中虚拟机配置读取仍以普通 `dumpxml` 作为编辑窗口当前视图；CPU/内存页的 `currentMemoryBytes`、`maximumMemoryBytes` 属于配置编辑值，会额外读取 `dumpxml --inactive` 获取持久 XML 中的 `<currentMemory>` 和 `<memory>`，避免 libvirt 启动后 live XML 或 `dommemstat actual` 把当前内存呈现为最大内存。
- 运行中的虚拟机不允许修改最大 CPU、最大内存，不允许缩小当前 CPU 或当前内存，也不允许超过 XML 中已预留的最大 CPU 和最大内存；内存热扩容还依赖 libvirt、虚拟机 XML 预留和 Guest OS 热添加能力。
- CPU/内存修改成功后 Agent 返回轻量配置，跳过 `domblkinfo` 和 `qemu-img info` 容量探测，避免资源修改按钮等待磁盘容量读取。
- 后端不再在该接口内同步整个 Agent 运行态缓存，仅广播运行态更新事件；运行态列表会由定时刷新或手动刷新更新。
- Agent 会重复校验运行中 CPU 与内存热扩容边界，避免绕过前端后执行缩容或超过预留上限的配置。

#### 10.4.4 磁盘与网络接口

磁盘与网络页后端接口：

```http
PUT /api/vms/{id}/devices
```

Agent 接口：

```http
PUT /v1/vms/{name}/devices
```

请求体：

```json
{
  "interfaces": [
    { "name": "vnet0", "mac": "52:54:00:12:34:56", "source": "default" }
  ],
  "newInterfaces": [
    { "source": "default", "model": "virtio" }
  ],
  "deletedInterfaces": [
    { "name": "vnet1", "mac": "52:54:00:65:43:21" }
  ],
  "diskResizes": [
    { "name": "vda", "capacityBytes": 214748364800 }
  ],
  "newDisks": [
    {
      "name": "ct7-template-vdb.qcow2",
      "pool": "default",
      "target": "vdb",
      "bus": "virtio",
      "format": "qcow2",
      "capacityBytes": 107374182400,
      "preallocMetadata": false
    }
  ],
  "deletedDisks": [
    { "name": "vdb" }
  ]
}
```

#### 10.4.5 磁盘与网络前端规则

处理规则：

- 前端按虚拟机 `hostId` 读取 `/api/network-pools/{agentId}` 和 `/api/storage-pools/{agentId}`。顶部摘要展示网络吞吐、网络设备和存储设备三等份，其中网络设备显示当前保留网卡加新增草稿数量。
- 网络设备支持修改已有网卡网络池、新增网卡和删除网卡。已有网卡下拉默认匹配当前来源，展示网络池名称，悬浮提示仅展示设备名和转发模式，并使用 `·` 分隔；下拉超过 4 行时内部滚动。
- 已有网卡详情会根据当前选择的网络池实时推导目标 source 和 interface type：bridge 转发网络池显示真实 bridge 设备名和 `bridge`，NAT、route、isolated 等 libvirt 网络池显示网络池名称和 `network`。
- 存储设备中，已有磁盘不再允许选择存储池，只允许填写大于当前容量的目标容量进行扩容；磁盘详情展示卷名、容量、存储池和总线，不展示完整路径；第一块存储设备不可删除，其他普通磁盘可标记删除，保存后会从 XML 移除对应 `<disk device="disk">` 并删除对应存储卷，且至少保留一块磁盘。
- 编辑弹窗内非“磁盘与网络”页保存后，Agent 返回的配置可能不执行磁盘容量探测；前端合并新配置时会用弹窗内旧配置或虚拟机列表中同名/同路径磁盘的容量补齐 `bytes=0` 的磁盘，避免切换到“磁盘与网络”页时容量显示为 0。
- 新增磁盘通过“添加新磁盘”配置卷名、存储池、目标设备、总线、格式、容量和 qcow2 metadata 预分配，存储池默认沿用当前已有磁盘所在存储池，存储池下拉向上展开且超过 4 行内部滚动；新增磁盘卷名扩展名必须与格式一致，`qcow2` 使用 `.qcow2`，`raw`、`qcow`、`qed` 使用 `.img`。
- 运行中虚拟机支持扩容已有磁盘和添加新磁盘；网络设备修改和删除磁盘需关机后操作。

#### 10.4.6 网卡修改规则

- 运行中虚拟机不支持修改、新增或删除网络设备，需先关闭虚拟机后再操作。
- Agent 根据当前 XML 校验请求内容：
  - 校验网卡 MAC 或 target。
  - 校验已有磁盘 target 存在。
  - 网卡修改和删除优先按 MAC 匹配目标网卡。
  - 如果选择的网络池为 bridge 转发模式且存在 bridge 设备名，则将 `<interface type>` 切换为 `bridge` 并写入 `<source bridge="...">`。
  - 其他 libvirt 网络池写为 `type="network"` 和 `<source network="网络池名">`。
  - 未匹配到网络池时按当前接口类型兜底写入对应 source 属性。
  - 新增网卡会在 `<devices>` 内追加 `<interface>` 节点，默认模型为 `virtio`，支持 `virtio`、`e1000`、`e1000e`、`rtl8139` 和 `vmxnet3`。
  - 已停止虚拟机需要重写整份 XML 并执行 `virsh define` 时，会以 `dumpxml --security-info` 读取的 XML 作为改写基底，避免普通 `dumpxml` 隐藏 VNC `passwd` 属性导致控制台密码被清空。

#### 10.4.7 磁盘扩容与新增删除规则

- 磁盘扩容规则：
  - 已停止虚拟机已有磁盘扩容前执行 `qemu-img info --output=json <path>`。
  - 如果镜像信息包含 `snapshots` 字段，说明 qcow2 镜像存在内部快照，Agent 会拒绝扩容并返回中文提示。
  - 已停止虚拟机已有磁盘扩容执行 `qemu-img resize <path> <capacityBytes>`，容量参数使用纯数字字节值，兼容不接受 `B` 后缀的 qemu-img 版本。
  - 运行中虚拟机已有磁盘扩容执行 `virsh --connect <LIBVIRT_URI> blockresize <vm> <target> <capacityBytes>B`。
  - `blockresize` 会通知 QEMU block backend 和 Guest 重新识别容量。
  - 所有磁盘扩容只允许增大容量，不允许缩容。
- 新增磁盘规则：
  - 新增磁盘会检查目标设备不重复。
  - 新增磁盘会检查目标存储池内卷名不重复。
  - 校验通过后先创建存储卷。
  - 运行中虚拟机执行 `virsh --connect <LIBVIRT_URI> attach-disk <vm> <volume-path> <target> --targetbus <bus> --driver qemu --subdriver <format> --live --config` 热添加并写入持久配置。
  - 已停止虚拟机在 `<devices>` 内新增 `<disk type="file" device="disk">` 节点并执行 `virsh define`，定义前会保留 VNC 控制台密码等安全字段。
- 删除磁盘规则：
  - 仅支持已停止虚拟机。
  - 第一块普通磁盘不可删除。
  - 仅允许删除普通磁盘，不删除 CDROM。
  - Agent 会先从 inactive XML 移除目标 `<disk>` 并执行 `virsh define`，随后删除对应存储卷。
  - 删除请求不能删除全部普通磁盘，至少保留一块磁盘。

#### 10.4.8 修改完成同步

- 修改成功后，Agent 返回包含磁盘容量探测的配置，避免扩容后编辑窗口容量回显为 0；后端记录 `vm.devices.update` 任务和审计日志，并异步同步对应 Agent 的运行态缓存。

### 10.5 编辑窗口克隆链路

#### 10.5.1 接口与 Agent 命令

后端接口：

```http
POST /api/vms/{id}/clone
```

Agent 接口：

```http
POST /v1/vms/{name}/clone
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> dominfo <clone-vm>
virsh --connect <LIBVIRT_URI> dumpxml <vm>
virsh --connect <LIBVIRT_URI> vol-clone --pool <pool> <source-volume> <target-volume> [--prealloc-metadata]
virsh --connect <LIBVIRT_URI> define <temp-domain-xml>
virsh --connect <LIBVIRT_URI> desc <clone-vm> --config --new-desc <description>
virsh --connect <LIBVIRT_URI> start <clone-vm>
```

#### 10.5.2 前端克隆表单

处理规则：

- 前端点击虚拟机列表操作列的“克隆”按钮打开独立克隆弹窗，左侧展示源虚拟机摘要，右侧配置克隆名称、描述、克隆后直接启动、CPU/内存、网卡、磁盘和 CDROM 策略。克隆名称输入框默认留空，占位提示为当前虚拟机名称。克隆资源配置会展示源宿主机 CPU/内存上限，最大 CPU 和最大内存不能超过该宿主机资源。

#### 10.5.3 网卡克隆规则

- 前端进入克隆弹窗时会为每张网卡重新生成 `52:54:00` 前缀的 MAC，并允许选择目标网络池。
  - 网络池下拉展示网络池名称。
  - 选项悬浮提示为设备名 `bridge`。
  - 下拉超过 4 行时内部滚动。
  - 若源网卡类型为 `bridge`，前端会把所选网络池转换为真实 bridge 设备名提交。
  - Agent 也会在收到网络池名称时读取网络池 XML 并解析为真实 `<bridge name>`。
  - 若源 XML 未提供网卡 `<target dev>`，克隆请求仍会按网卡顺序和源 MAC 匹配新的 MAC 与网络池。

#### 10.5.4 磁盘克隆规则

- 磁盘目标卷名输入框默认留空。
  - 占位提示为 `当前虚拟机名称-磁盘名-扩展名`，如 `ct7-template-vda.qcow2`。
  - 输入克隆名称后，磁盘目标卷名会自动更新为 `克隆名称-磁盘名-扩展名`。
  - 提交前前端会校验目标卷扩展名必须与源磁盘文件扩展名一致。
  - 后端排队前也会再次校验目标卷扩展名。
  - 每块磁盘允许选择目标存储池。
  - 存储池下拉展示存储池名称，选项悬浮提示为存储路径 `path`。
  - 下拉超过 4 行时内部滚动。
  - 只有扩展名为 `.qcow2` 的磁盘会显示 metadata 预分配选项。

#### 10.5.5 克隆后台任务

- 克隆窗口使用单一常规配置流程，不提供 XML 克隆入口。
- 克隆请求携带 CPU/内存配置，Agent 基于源虚拟机 XML 生成克隆定义，并重写 `<vcpu>`、`<memory>` 和 `<currentMemory>`；`cdromPolicy=disconnect` 时会移除克隆定义中 CDROM 的 `source file/dev`，`inherit` 则保留源介质连接状态。若启用“克隆后直接启动”，前端、后端和 Agent 都会强制使用 `cdromPolicy=disconnect`。
- 描述为空时克隆 VM 不保留源描述；描述非空时定义后执行 `virsh desc --config --new-desc` 写入。克隆后直接启动开启时定义后执行 `virsh start <clone-vm>`，不再设置 libvirt 随宿主机同启。
- 后端创建 `vm.clone` 后台任务前，会先调用 Agent `/v1/host` 检查宿主机 CPU/内存上限，再读取 Agent VM 列表检查克隆虚拟机名称是否已存在，并按目标存储池读取卷列表检查目标卷名是否已存在或在同一次请求中重复。预检通过后才创建后台任务并立即返回；前端收到排队成功后关闭克隆弹窗，并使用任务 toast 以 3 秒间隔轮询任务状态，从“克隆虚拟机排队中”持续更新到完成或失败。任务执行时 Agent 仍会再次拒绝克隆已存在名称、已运行的源虚拟机或超过宿主机上限的最大 CPU/最大内存。
- Agent 克隆每块磁盘后，会基于源虚拟机 XML 生成新定义：
  - 移除原 UUID 和顶层描述。
  - 替换 `<name>`。
  - 替换 CPU/内存。
  - 替换磁盘 `<source>` 路径。
  - 替换网卡 `<mac>` 和网卡 `<source>`。
  - 移除接口运行时 `<target>`。
  - 最后执行 `virsh define`。
- 网卡 source 属性会按接口类型写入：
  - `network` 类型使用 `network`。
  - `bridge` 类型使用 `bridge`。
  - `direct` 类型使用 `dev`。
- `bridge` 类型会优先把网络池名称解析为网络池 XML 中的真实 bridge 设备名，避免把 libvirt 网络名称误写成宿主机 bridge 设备。
- 磁盘复制规则：
  - 目标存储池与源存储池相同时使用 `virsh vol-clone`。
  - 目标存储池不同时使用 `qemu-img convert` 将源卷复制到目标存储池路径。

#### 10.5.6 完成事件与失败清理

- 后台克隆任务完成或失败后，后端通过 SSE 推送 `vm.clone.completed` 或 `vm.clone.failed`，前端收到后展示 toast。
- 克隆成功后，后端先对所属 Agent 执行 fast 同步并广播 `runtime.updated`，让新虚拟机尽快出现在列表中。
- 随后后端延迟执行 full 同步，再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和快照等重字段。
- 如果定义新虚拟机失败，Agent 会删除本次已克隆出的存储卷，避免留下孤立镜像。

### 10.6 编辑窗口 XML 修改链路

后端接口：

```http
PUT /api/vms/{id}/xml
```

请求体：

```json
{
  "xml": "<domain type='kvm'>...</domain>"
}
```

Agent 接口：

```http
PUT /v1/vms/{name}/xml
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> dumpxml <vm> --security-info
virsh --connect <LIBVIRT_URI> define <temp-domain-xml>
```

处理规则：

- 前端“XML”页默认只读，点击“编辑”后进入可编辑状态，再通过“保存”提交或“取消”还原。
- 仅已停止虚拟机允许保存 XML；运行中虚拟机前端禁用编辑入口，后端和 Agent 也会拒绝。
- Agent 会校验 XML 非空、可被解析，且 XML 中 `<name>` 必须与当前虚拟机名称一致，避免通过 XML 页误改名或覆盖其他虚拟机。
- XML 页展示的 `config.xml` 来源于普通 `dumpxml`，不会回显 VNC `passwd` 等敏感字段。
- XML 保存前，Agent 会读取 `dumpxml --security-info`，当用户提交的 XML 仍包含 VNC graphics 且未显式携带 `passwd` 时，自动保留原有 VNC 密码，避免保存 XML 时把控制台密码误清空。
- 保存成功后 Agent 返回最新轻量配置，后端记录 `vm.xml.update` 任务和审计日志，并异步同步运行态缓存。

### 10.7 编辑窗口自启动修改链路

“随宿主机同启”按钮单独保存，不受 CPU/内存配置修改按钮影响。

后端接口：

```http
PUT /api/vms/{id}/autostart
```

请求体：

```json
{
  "autostart": true
}
```

Agent 接口：

```http
PUT /v1/vms/{name}/autostart
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> autostart <vm>
virsh --connect <LIBVIRT_URI> autostart <vm> --disable
```

### 10.8 编辑窗口介质连接与断开链路

#### 10.8.1 连接介质

后端接口：

```http
PUT /api/vms/{id}/media
```

请求体：

```json
{
  "target": "sda",
  "isoPath": "/var/lib/libvirt/images/CentOS-7.iso"
}
```

Agent 接口：

```http
PUT /v1/vms/{name}/media
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> dumpxml <vm>
virsh --connect <LIBVIRT_URI> change-media <vm> <target> <isoPath> --insert --config
virsh --connect <LIBVIRT_URI> dumpxml <vm> --security-info
virsh --connect <LIBVIRT_URI> define <updated-boot-order-xml>
```

#### 10.8.2 断开介质

断开介质时使用同一路径的 DELETE 方法：

```http
DELETE /api/vms/{id}/media
```

请求体：

```json
{
  "target": "sda"
}
```

Agent 接口：

```http
DELETE /v1/vms/{name}/media
```

Agent 执行命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> dumpxml <vm>
virsh --connect <LIBVIRT_URI> change-media <vm> <target> --eject --config
virsh --connect <LIBVIRT_URI> dumpxml <vm> --security-info
virsh --connect <LIBVIRT_URI> define <updated-boot-order-xml>
```

#### 10.8.3 约束与同步规则

约束规则：

- `target` 必须是当前 VM XML 中已存在的 CDROM 目标设备，如 `sda`。
- `isoPath` 必须非空，并由介质页从宿主机 ISO 存储池读取后传入。
- 已连接介质时，前端禁用目标光驱、存储池、ISO 文件和刷新按钮，只允许点击“断开”；断开成功后恢复选择与连接能力。
- 运行中虚拟机会被拒绝，不允许连接或断开介质。
- 已停止虚拟机使用 `--config` 更新持久配置。
- 连接介质后 Agent 会同步调整启动顺序：如果 XML 已使用 `<os><boot dev="..."/>`，则改为 `cdrom`、`hd`；如果 XML 使用设备级 `<boot order>`，则把目标 CDROM 设置为 `1`、第一块普通磁盘设置为 `2`，并移除其他磁盘/CDROM 的重复启动顺序。断开介质时对应恢复硬盘优先，且不会混用 OS 级 boot 和设备级 boot。
- 介质操作后的启动顺序 XML 使用 `dumpxml --security-info` 读取后再 `define`，避免普通 `dumpxml` 隐藏 VNC `passwd` 属性导致控制台密码被清空。
- 操作成功后，后端立即返回 Agent 读取到的最新 VM 配置，并分别记录 `vm.media.connect` 或 `vm.media.disconnect` 任务和审计日志；Agent 运行态缓存同步和 `runtime.updated` SSE 广播在后台异步执行，避免介质操作响应被全量同步阻塞。

### 10.9 存储池、网络池与 ISO 镜像采集

控制台新增宿主机维度的“存储池”和“网络池”页面。前端先读取 `/api/hosts` 获取宿主机 Agent ID，再按宿主机调用后端代理接口。后端解密 Agent Token 后转发到对应 Agent，不把存储池和网络池落库。

#### 10.9.1 存储池接口

存储池接口：

```http
GET /api/storage-pools/{agentId}
POST /api/storage-pools/{agentId}
GET /api/storage-pools/{agentId}/iso-files/{pool}
GET /api/storage-pools/{agentId}/volumes/{pool}
POST /api/storage-pools/{agentId}/volumes/{pool}
POST /api/storage-pools/{agentId}/volumes/{pool}/clone
POST /api/storage-pools/{agentId}/volumes/{pool}/upload
DELETE /api/storage-pools/{agentId}/volumes/{pool}?name={volume}
DELETE /api/storage-pools/{agentId}/delete/{pool}
PUT /api/storage-pools/{agentId}/state/{pool}
PUT /api/storage-pools/{agentId}/autostart/{pool}
```

Agent 接口：

```http
GET /v1/storage-pools
POST /v1/storage-pools
GET /v1/storage-pools/{pool}/iso-files
GET /v1/storage-pools/{pool}/volumes
POST /v1/storage-pools/{pool}/volumes
POST /v1/storage-pools/{pool}/volumes/clone
POST /v1/storage-pools/{pool}/volumes/upload
DELETE /v1/storage-pools/{pool}/volumes?name={volume}
DELETE /v1/storage-pools/{pool}/delete
PUT /v1/storage-pools/{pool}/state
PUT /v1/storage-pools/{pool}/autostart
```

Agent 执行命令：

执行创建命令前，Agent 会先校验存储池路径：

- 目录类型卷要求目标路径为绝对路径；若路径已存在则必须是目录。
- LVM 类型卷要求设备路径为宿主机上已存在的绝对块设备路径。
- NETFS 要求主机名、远端绝对路径和本地绝对路径。
- iSCSI 要求主机名、目标 IQN 和本地绝对路径。

读取卷列表前会先执行 `pool-refresh`，确保宿主机后台直接新增的卷文件能被重新扫描。

存储池列表容量字段：

- Agent 通过 `virsh pool-info <pool> --bytes` 读取每个 libvirt 存储池的 `capacity`、`allocation` 和 `available`。
- 目录池会根据目标路径所在文件系统设备号返回 `capacitySource`。
- 前端存储池页面的单个卡片继续展示对应 libvirt 池自身容量。
- 顶部“总容量”和“已分配”按 `capacitySource` 去重汇总，多个目录池落在同一底层文件系统时只计入一次。
- 如果路径不存在或 Agent 无法识别底层文件系统，`capacitySource` 为空，前端会退回按单个存储池独立计入。

#### 10.9.2 ISO 上传规则

ISO 上传规则：

- ISO 上传名称必须以 `.iso` 结尾。
- 上传会先保存到后端临时文件。
- 后端创建后台任务后再转发到 Agent。
- Agent 按实际接收的临时文件大小创建目标卷。
- 任务完成后删除临时文件。

#### 10.9.3 非 ISO 卷处理规则

非 ISO 卷处理规则：

- 非 ISO 创建卷使用 `virsh vol-create-as`。
- qcow2 勾选 metadata 时追加 `--prealloc-metadata`。
- 克隆不转换时使用 `virsh vol-clone`。
- 转换时使用 `qemu-img convert` 后刷新存储池。
- 上传和克隆任务完成后会广播 `storage.volume.completed` 或 `storage.volume.failed` SSE 事件。
- 上传和克隆在 Agent 执行阶段失败时，会更新任务为 `failed` 并写入对应失败审计；任务、审计和告警边界详见 `docs/operation-log-coverage.md`。

#### 10.9.4 删除与失败回退规则

删除与失败回退规则：

- 删除非 ISO 存储卷前，Agent 会读取虚拟机配置并检查磁盘路径是否仍被引用。
- 若正在被虚拟机使用，会拒绝删除并提示先移除磁盘或关闭相关虚拟机。
- 若源镜像正在被运行中的虚拟机使用，`qemu-img convert` 可能因无法获取 qcow2 写锁而失败。
- 出现 qcow2 写锁失败时，前端会提示先关闭相关虚拟机或改用快照创建一致性副本。
- 若 `pool-build` 或 `pool-start` 失败，Agent 会自动 `pool-destroy` 并 `pool-undefine` 刚定义的存储池。
- 自动清理刚定义的存储池可以避免创建失败后刷新列表仍显示 inactive 的残留池。
- 如果创建存储池时 libvirt 返回 `Storage source conflict with pool`，表示目标路径或存储源已被其他存储池使用，平台会提示更换路径或先处理冲突的存储池。
- 停止 NFS/netfs 存储池时，libvirt 会执行底层 `umount`。若返回 `device is busy`，表示挂载目录仍被虚拟机、终端或其他进程占用，平台会提示先释放占用后再停止存储池。

```bash
virsh --connect <LIBVIRT_URI> pool-list --all --name
virsh --connect <LIBVIRT_URI> pool-info <pool> --bytes
virsh --connect <LIBVIRT_URI> pool-dumpxml <pool>
virsh --connect <LIBVIRT_URI> vol-list <pool> --details
virsh --connect <LIBVIRT_URI> vol-delete --pool <pool> <volume>
virsh --connect <LIBVIRT_URI> pool-define-as <name> dir --target <path>
virsh --connect <LIBVIRT_URI> pool-define-as <name> logical --source-dev <device> --target <target>
virsh --connect <LIBVIRT_URI> pool-define-as <name> netfs --source-host <host> --source-path <remote> --target <local> [--source-format <format>]
virsh --connect <LIBVIRT_URI> pool-define-as <name> iscsi --source-host <host> --source-dev <target-iqn> --target <path>
virsh --connect <LIBVIRT_URI> pool-refresh <pool>
virsh --connect <LIBVIRT_URI> pool-build <pool>
virsh --connect <LIBVIRT_URI> pool-start <pool>
virsh --connect <LIBVIRT_URI> pool-autostart <pool>
virsh --connect <LIBVIRT_URI> pool-autostart <pool> --disable
virsh --connect <LIBVIRT_URI> pool-destroy <pool>
```

#### 10.9.5 网络池接口

网络池接口：

```http
GET /api/network-pools/{agentId}
POST /api/network-pools/{agentId}
DELETE /api/network-pools/{agentId}/delete/{pool}
PUT /api/network-pools/{agentId}/state/{pool}
PUT /api/network-pools/{agentId}/autostart/{pool}
```

Agent 接口：

```http
GET /v1/network-pools
POST /v1/network-pools
DELETE /v1/network-pools/{pool}/delete
PUT /v1/network-pools/{pool}/state
PUT /v1/network-pools/{pool}/autostart
```

Agent 执行命令：

创建网络池时，Agent 会根据网络类型生成对应的 libvirt 网络 XML，并通过 `virsh net-define` 定义后启动网络池。

#### 10.9.6 Fixed Address 配置

Fixed Address 配置说明：

- Fixed Address 仅用于需要预生成 DHCP 固定地址条目的网络池。
- 启用 Fixed Address 时必须同时启用 DHCP。
- Agent 会根据填写的 IPv4 CIDR 子网计算网关、DHCP 范围和固定地址列表。
- 网关使用网段第一个可用地址，DHCP 与固定地址从下一个可用地址开始到广播地址前一个地址结束。
- 例如 `192.168.100.0/24` 会生成 `192.168.100.2` 到 `192.168.100.254` 的固定地址 `<host mac="..." ip="..."/>` 条目。
- 为避免误填超大网段导致 libvirt XML 过大，单个网络池最多生成 4096 条固定地址。

#### 10.9.7 Open vSwitch 配置

Open vSwitch 配置说明：

- Open vSwitch 仅用于 BRIDGE 类型网络池。
- 启用 Open vSwitch 时，Agent 会在 libvirt 网络 XML 中写入 `<virtualport type="openvswitch"/>`。
- 该配置表示桥接网络按 Open vSwitch 虚拟端口方式接入。
- 只有宿主机上的桥接设备本身由 Open vSwitch 管理时才需要启用。
- 普通 Linux bridge 不需要启用 Open vSwitch。

#### 10.9.8 网络池创建前校验

NAT 与 ROUTE 网络池创建前，Agent 会检查 `net.ipv4.ip_forward` 是否可读取且值为 `1`，未启用时拒绝创建，避免创建后虚拟机无法转发出网。BRIDGE 网络池创建前会检查填写的桥接设备必须已存在且确认为 Linux bridge。该流程只处理 libvirt 网络池定义，不处理 qemu bridge helper 配置，也不会自动把物理网卡加入桥、迁移物理网卡 IP/DNS 或重启系统网络服务。

```bash
virsh --connect <LIBVIRT_URI> net-list --all --name
virsh --connect <LIBVIRT_URI> net-info <network>
virsh --connect <LIBVIRT_URI> net-dumpxml <network>
virsh --connect <LIBVIRT_URI> net-define <network-xml>
virsh --connect <LIBVIRT_URI> net-start <network>
virsh --connect <LIBVIRT_URI> net-autostart <network>
virsh --connect <LIBVIRT_URI> net-autostart <network> --disable
virsh --connect <LIBVIRT_URI> net-destroy <network>
```

#### 10.9.9 资源配置边界汇总

该小节只汇总虚拟机资源配置和存储/网络池之间的边界，具体接口和命令分别见 `10.4`、`10.8` 和 `10.9.1` 至 `10.9.8`。

运行中虚拟机允许的资源修改：

```bash
virsh --connect <LIBVIRT_URI> setvcpus <vm> <currentCpu> --live --config
virsh --connect <LIBVIRT_URI> setmem <vm> <currentMemoryKiB> --live --config
virsh --connect <LIBVIRT_URI> blockresize <vm> <target> <capacityBytes>B
virsh --connect <LIBVIRT_URI> attach-disk <vm> <volume-path> <target> --targetbus <bus> --driver qemu --subdriver <format> --live --config
```

约束规则：

- `currentCpu`、`maximumCpu`、`currentMemoryMB`、`maximumMemoryMB` 必须为正整数。
- 当前分配不能大于最大分配。
- 前端会限制最大分配不超过当前读取到的宿主机 CPU/内存。
- 后端和 Agent 会重复校验正整数、当前值与最大值关系、描述长度。
- 运行中虚拟机仅允许增加当前 CPU、当前内存、已有磁盘容量和新磁盘。
- 运行中虚拟机不能缩容，不能修改最大 CPU/最大内存，不能修改网卡，也不能连接或断开介质。
- vCPU 最大值和最大内存主要写入持久配置，是否可对运行中 VM 即时生效取决于 libvirt、虚拟机 XML 预留和 Guest OS 配置。
- 介质页根据宿主机 ISO 文件完成选择展示；已停止虚拟机连接或断开介质时使用 `--config` 写入持久配置。
- 存储池和网络池接口只代理对应 Agent 的 libvirt 资源，不把池详情落库。
- 操作完成后的同步、任务和审计由具体业务接口负责，不由池列表采集接口统一处理。

## 十一、虚拟机迁移链路

### 11.1 接口与请求字段

后端正式迁移接口：

```http
POST /api/vms/{id}/migrate
```

后端结构化预检接口：

```http
POST /api/vms/{id}/migrate-precheck
```

Agent 执行接口：

```http
POST /v1/vms/{name}/migrate
```

后端请求体：

```json
{
  "targetAgentId": "agent-target",
  "destinationUri": "qemu+ssh://192.168.10.22/system",
  "live": true,
  "copyDisks": true,
  "persistent": true,
  "undefineSource": true,
  "autoConverge": true,
  "postCopy": false
}
```

字段语义：

| 前端选项 | 请求字段 | 当前含义 |
| :-: | :-: | :-: |
| 目标宿主机 | `targetAgentId` | 后端用于定位目标 Agent，并在迁移完成后同步目标宿主机运行态缓存 |
| 迁移 URI | `destinationUri` | 为空时后端按目标 Agent 的 `endpoint` 生成 `qemu+ssh://<target-host>/system` |
| 热迁移 | `live=true` | 运行中虚拟机默认选择；迁移期间尽量保持业务运行 |
| 冷迁移 | `live=false` | 非运行状态虚拟机默认选择；不会自动关机 |
| 复制本地磁盘 | `copyDisks=true` | 表示源磁盘是本地磁盘，需要平台通过 SSH 复制到目标宿主机同路径；不再映射 `--copy-storage-all` |
| 不复制本地磁盘 | `copyDisks=false` | 表示 NFS、iSCSI、RBD 等共享或远端存储，目标宿主机可直接访问相同磁盘路径 |
| 持久化目标定义 | `persistent=true` | 迁移后在目标宿主机保留持久 VM 定义 |
| 迁移后清理源虚拟机 | `undefineSource=true` 且 `copyDisks=true` | 复制本地磁盘迁移成功后删除源定义和源普通磁盘 |
| 迁移后取消源定义 | `undefineSource=true` 且 `copyDisks=false` | 共享存储迁移追加 `--undefinesource`，只取消源定义，不删除磁盘 |
| 自动收敛 | `autoConverge=true` | 热迁移优化项，降低迁移期间虚拟机写入速度以帮助内存脏页收敛 |
| Post-copy | `postCopy=true` | 热迁移优化项，可降低收敛等待，但迁移中断后的恢复风险更高 |

### 11.2 预检规则

通用预检：

- 前端虚拟机列表操作列提供迁移入口；目标宿主机不能与源宿主机相同。
- 前端只在宿主机数量不少于 2 台时允许打开迁移弹窗。
- 运行中虚拟机默认选择热迁移；非运行状态虚拟机默认选择冷迁移。
- 前端默认勾选“复制本地磁盘”，适用于源磁盘位于宿主机本地存储的场景。
- 前端会阻止运行中虚拟机选择冷迁移，也会阻止非运行状态虚拟机选择热迁移。
- 前端迁移按钮默认禁用，并通过自定义悬浮提示显示“预检通过后再执行迁移”。
- 只有当前迁移参数对应的结构化预检结果为通过时，前端才启用迁移按钮。
- 重复执行预检或修复预检失败项后自动重新预检期间，前端会禁用迁移按钮，但按钮文本保持“迁移”；迁移窗口内其他配置、关闭和修复入口同步禁用。
- 前端执行迁移时直接复用预检通过时保存的请求参数，不再重复执行弹窗内的目标宿主机、运行态和资源预判断。
- 目标宿主机、迁移类型、迁移 URI、复制本地磁盘、源定义处理或热迁移优化参数发生变化后，前端会清空旧预检结果并要求重新预检。
- 后端结构化预检接口是完整远程迁移预检入口，负责读取源 VM 配置并校验运行态：
  - 运行中的虚拟机只能选择热迁移；
  - 非运行状态虚拟机只能选择冷迁移；
  - 后端不会自动关闭虚拟机。
- 结构化预检会读取目标宿主机轻量 VM 列表，若目标宿主机已存在同名虚拟机，则拒绝迁移。
- 结构化预检会读取目标宿主机 `/v1/host`，检查目标 CPU 数和内存总量是否能容纳 VM 最大 CPU/最大内存，并使用 VM XML 架构与目标 `cpuModel` 做基础架构预检。
- 结构化预检会读取目标网络池，要求 VM 当前网卡来源能在目标宿主机匹配到同名网络池或相同 bridge 设备，且对应网络池为 active。
- 后端正式迁移接口不重复执行完整远程预检，只做请求格式、虚拟机存在性、源目标 Agent、迁移方式和迁移 URI 格式等基础校验，随后创建 `vm.migrate` 后台任务并返回排队结果。
- 如果预检通过后目标环境发生变化，最终由 Agent 执行阶段返回失败原因并记录任务失败和 `vm.migrate.failed` 审计。

存储预检：

- `copyDisks=false` 时，结构化预检会读取源宿主机和目标宿主机的存储池，要求每块磁盘能识别到同名存储池，并且源目标存储池显示共享特征。
- 共享特征包括网络/逻辑类共享池，或同名目录池路径一致。
- 目录池路径一致不能证明底层一定是共享存储，最终可用性以 libvirt 迁移结果为准。
- `copyDisks=true` 时，结构化预检跳过共享存储预检，但会要求目标宿主机存在每块源磁盘路径所在的存储池。
- `copyDisks=true` 时，结构化预检会读取目标存储池卷列表，若目标池已存在同路径或同名磁盘卷，则提示“目标宿主机已存在磁盘文件 ...”，避免进入 Agent 复制阶段后才失败。
- 若目标宿主机没有源磁盘路径所在的存储池，结构化预检会提示，例如“目标宿主机没有源磁盘路径 /kvm/images/vm1.qcow2 所在的存储池，无法执行迁移复制磁盘”。

迁移通道预检：

- 仅当迁移 URI 以 `qemu+ssh://` 开头时，结构化预检会调用源 Agent 执行非交互迁移通道检测，等价于 `virsh --connect <destinationUri?no_tty=1> list --all`。
- 如果检测到需要 SSH 密码，或源宿主机尚未信任目标宿主机 SSH 指纹，接口返回 `vm_migrate_ssh_password_required`，前端保持迁移窗口打开，并在“迁移通道”预检卡片右侧显示配置免密按钮。
- SSH 免密配置由前端调用 `/api/vms/{id}/migrate-ssh-key`，后端转发源 Agent。
- 源 Agent 生成或复用本机 `~/.ssh/id_ed25519.pub`，通过本次输入的目标 SSH 用户和密码写入目标宿主机 `authorized_keys`，配置成功后前端会自动重新执行迁移预检。
- 密码仅用于本次配置，不写数据库、任务 payload、审计 metadata 或服务日志。
- 热迁移会额外检查目标 libvirt `hostname`、目标宿主机 `hostname` 和 `hostname -f` 的解析结果是否为 `localhost`、`127.*` 或 `::1`。
- 如果目标主机名解析为 localhost，接口返回 `vm_migrate_target_hostname_localhost`，前端会在“迁移通道”预检卡片右侧显示修复主机名按钮。
- 修复主机名由前端调用 `/api/vms/{id}/migrate-hostname`，后端转发源 Agent。
- 源 Agent 会通过 SSH 执行 `hostnamectl set-hostname <hostname>` 修改目标宿主机 hostname，并在源宿主机和目标宿主机 `/etc/hosts` 写入目标 IP 与 hostname 解析。
- 修复主机名也会覆盖源宿主机无法解析目标 hostname 导致迁移执行阶段出现 `Unable to resolve address '<hostname>'` 的问题。
- `copyDisks=true` 时必须使用 `qemu+ssh://` 迁移 URI，因为平台需要通过 SSH 复制本地磁盘。
- `copyDisks=false` 且迁移 URI 不是 `qemu+ssh://` 时，预检清单会显示跳过 SSH 免密检测。

### 11.3 热迁移

#### 11.3.1 共享存储热迁移

适用条件：

- VM 处于运行中；
- `live=true`；
- `copyDisks=false`；
- 目标宿主机可访问同一套 NFS、iSCSI、RBD 或其他共享存储路径。

执行命令：

```bash
virsh --connect <LIBVIRT_URI> migrate \
  --live \
  [--persistent] \
  [--undefinesource] \
  [--auto-converge] \
  [--postcopy] \
  <vm> <destinationUri>
```

#### 11.3.2 本地磁盘热迁移

适用条件：

- VM 处于运行中；
- `live=true`；
- `copyDisks=true`；
- 源磁盘位于宿主机本地存储；
- 迁移 URI 必须以 `qemu+ssh://` 开头。

执行流程：

1. 源 Agent 读取源 VM 持久 XML 与普通磁盘列表。
2. 使用 `qemu+ssh://` 迁移 URI 解析目标 SSH 主机。
3. 通过远程 `virsh pool-list` / `pool-dumpxml` 获取目标宿主机所有存储池路径。
4. 要求目标宿主机存在每块普通磁盘源路径所在的存储池。
5. 使用 `scp` 将每块普通磁盘复制到目标宿主机上的源磁盘原路径。
6. 复制完成后执行 `virsh migrate --live --unsafe`。
7. 按请求追加 `--persistent`、`--auto-converge` 和 `--postcopy`。
8. 如果勾选“迁移后清理源虚拟机”，迁移成功后源端先 `destroy`，再 `undefine` 并删除源普通磁盘。

执行命令：

```bash
scp /var/lib/libvirt/images/web-01.qcow2 192.168.10.22:/var/lib/libvirt/images/web-01.qcow2

virsh --connect <LIBVIRT_URI> migrate \
  --live \
  --unsafe \
  [--persistent] \
  [--auto-converge] \
  [--postcopy] \
  <vm> <destinationUri>
```

热迁移复制本地磁盘不再使用 `--copy-storage-all`，但会在平台完成磁盘复制后追加 `--unsafe`，避免 libvirt 因“无共享存储迁移”拒绝执行。

### 11.4 冷迁移

#### 11.4.1 共享存储冷迁移

适用条件：

- VM 已停止；
- `live=false`；
- `copyDisks=false`；
- 目标宿主机可访问同一套共享存储路径。

执行命令：

```bash
virsh --connect <LIBVIRT_URI> migrate \
  [--persistent] \
  [--undefinesource] \
  <vm> <destinationUri>
```

#### 11.4.2 本地磁盘冷迁移

适用条件：

- VM 已停止；
- `live=false`；
- `copyDisks=true`；
- 源磁盘位于宿主机本地存储；
- 迁移 URI 必须以 `qemu+ssh://` 开头。

执行流程：

1. 源 Agent 读取源 VM 持久 XML 与普通磁盘列表。
2. 使用 `qemu+ssh://` 迁移 URI 解析目标 SSH 主机。
3. 通过远程 `virsh pool-list` / `pool-dumpxml` 获取目标宿主机所有存储池路径。
4. 要求目标宿主机存在每块普通磁盘源路径所在的存储池。
5. 使用 `scp` 将每块普通磁盘复制到目标宿主机上的源磁盘原路径。
6. 重写 XML 中普通磁盘 source 路径，保留原 VM 名称、UUID、MAC、描述和其他配置。
7. 将 XML 复制到目标宿主机并远程执行 `virsh --connect <LIBVIRT_URI> define <xml>`。
8. 如果勾选“迁移后清理源虚拟机”，再在源宿主机执行 `virsh undefine <vm>` 并删除源普通磁盘。

执行命令：

```bash
ssh 192.168.10.22 virsh --connect qemu:///system pool-list --all --name
scp /var/lib/libvirt/images/web-01.qcow2 192.168.10.22:/var/lib/libvirt/images/web-01.qcow2
scp /tmp/kvm-manager-migrate-web-01.xml 192.168.10.22:/tmp/kvm-manager-migrate-web-01.xml
ssh 192.168.10.22 virsh --connect qemu:///system define /tmp/kvm-manager-migrate-web-01.xml
virsh --connect qemu:///system undefine web-01
```

### 11.5 任务、审计与同步

- 后端创建 `vm.migrate` 后台任务，由源宿主机 Agent 执行迁移；目标 Agent 不主动拉取虚拟机。
- 任务完成后，后端先广播 `vm.migrate.completed`，再对源宿主机和目标宿主机执行 fast 同步并广播 `runtime.updated`，让虚拟机列表尽快显示迁移后的宿主机归属。
- 随后后端在后台执行 full 同步并再次广播 `runtime.updated`，补齐 Guest Agent、IP、磁盘明细和指标等重字段。
- 任务失败时，后端记录 `vm.migrate.failed` 审计，广播 `vm.migrate.failed`。
- 本地磁盘复制会显著增加网络与 I/O 压力。
- 不做增量同步时，热迁移复制本地磁盘要求业务能接受磁盘复制期间源盘仍可能有写入的风险；`--unsafe` 只放行 libvirt 的无共享存储安全检查，不提供增量同步能力；若业务强一致要求高，应优先使用共享存储迁移或在维护窗口内冷迁移。
- 热迁移执行阶段若 libvirt 返回 `non-migratable device`，表示虚拟机包含 QEMU 无法迁移运行状态的设备；Agent 会提示具体设备名称，需移除或更换该设备后重试，或关机后执行冷迁移。
- 迁移前必须确认目标宿主机 CPU 兼容、网络桥接名/网络池一致、共享存储可访问或已选择复制本地磁盘，并确保 libvirt/SSH/TLS 与防火墙配置满足迁移要求。

### 11.6 典型命令示例

```bash
# 共享存储热迁移：目标宿主机可访问相同磁盘路径，只迁移运行态和内存
virsh --connect qemu:///system migrate \
  --live \
  --persistent \
  --undefinesource \
  web-01 qemu+ssh://192.168.10.22/system

# 共享存储热迁移 + 自动收敛
virsh --connect qemu:///system migrate \
  --live \
  --persistent \
  --undefinesource \
  --auto-converge \
  web-01 qemu+ssh://192.168.10.22/system

# Post-copy 热迁移：降低长时间无法收敛的等待，但迁移中断风险更高
virsh --connect qemu:///system migrate \
  --live \
  --persistent \
  --undefinesource \
  --postcopy \
  web-01 qemu+ssh://192.168.10.22/system

# 共享存储冷迁移：虚拟机应已停止，不追加 --live
virsh --connect qemu:///system migrate \
  --persistent \
  --undefinesource \
  web-01 qemu+ssh://192.168.10.22/system

# 本地磁盘热迁移：先复制磁盘，再执行不带 --copy-storage-all 的热迁移
scp /var/lib/libvirt/images/web-01.qcow2 192.168.10.22:/var/lib/libvirt/images/web-01.qcow2
virsh --connect qemu:///system migrate \
  --live \
  --unsafe \
  --persistent \
  --auto-converge \
  web-01 qemu+ssh://192.168.10.22/system

# 本地磁盘冷迁移：虚拟机已停止，执行 SSH 磁盘复制和远程 define
ssh 192.168.10.22 virsh --connect qemu:///system pool-list --all --name
scp /var/lib/libvirt/images/web-01.qcow2 192.168.10.22:/var/lib/libvirt/images/web-01.qcow2
scp /tmp/kvm-manager-migrate-web-01.xml 192.168.10.22:/tmp/kvm-manager-migrate-web-01.xml
ssh 192.168.10.22 virsh --connect qemu:///system define /tmp/kvm-manager-migrate-web-01.xml
virsh --connect qemu:///system undefine web-01
```

## 十二、虚拟机操作链路

### 12.1 接口与权限

虚拟机操作按钮不会要求前端再次输入 Agent Token。Agent Token 在添加 Agent 时已经保存，后端保存摘要用于校验，同时保存加密密文用于自动刷新和 VM 操作。

后端接口：

```http
POST /api/vms/{id}/{action}
```

Agent 接口：

```http
POST /v1/vms/{name}/{action}
```

调用规则：

- 前端操作列根据 `vms.power`、`vms.delete`、`vms.force` 等权限控制按钮显隐。
- 删除和强制删除需要二次确认并输入虚拟机名称。
- 后端通过运行态缓存读取 VM 的 `hostId`，再解密对应 Agent 的 `TokenCiphertext`。
- Agent 只执行白名单内的动作映射，不接受任意 virsh 子命令。

### 12.2 操作映射

当前操作映射：

| 前端动作 | 后端接口 | Agent 动作 | Agent 命令 |
| :-: | :-: | :-: | :-: |
| 启动 | `POST /api/vms/{id}/start` | `start` | `virsh --connect <LIBVIRT_URI> start <vm>` |
| 暂停 | `POST /api/vms/{id}/pause` | `suspend` | `virsh --connect <LIBVIRT_URI> suspend <vm>` |
| 恢复 | `POST /api/vms/{id}/resume` | `resume` | `virsh --connect <LIBVIRT_URI> resume <vm>` |
| 重启 | `POST /api/vms/{id}/reboot` | `reboot` | `virsh --connect <LIBVIRT_URI> reboot <vm>` |
| 强制重启 | `POST /api/vms/{id}/force-reboot` | `reset` | `virsh --connect <LIBVIRT_URI> reset <vm>` |
| 停止 | `POST /api/vms/{id}/stop` | `shutdown` | `virsh --connect <LIBVIRT_URI> shutdown <vm>` |
| 强制停止 | `POST /api/vms/{id}/force-stop` | `destroy` | `virsh --connect <LIBVIRT_URI> destroy <vm>` |
| 关机 | `POST /api/vms/{id}/shutdown` | `shutdown` | `virsh --connect <LIBVIRT_URI> shutdown <vm>` |
| 强制关机 | `POST /api/vms/{id}/force-shutdown` | `destroy` | `virsh --connect <LIBVIRT_URI> destroy <vm>` |
| 删除 | `POST /api/vms/{id}/delete` | `delete` | 已停止时执行 `undefine`，再删除普通磁盘卷 |
| 强制删除 | `POST /api/vms/{id}/force-delete` | `force-delete` | 非停止状态先 `destroy`，再执行 `undefine` 并删除普通磁盘卷 |

### 12.3 后端同步、任务与审计

电源类操作：

- 启动、恢复、暂停、关机、停止、强制关机、重启和强制重启的目标状态可预测。
- Agent 调用成功后，后端先更新 Redis 运行态缓存中的当前 VM 状态。
- 后端立即广播 `runtime.updated`，让前端尽快刷新按钮状态和列表状态。
- 后端延迟 8 秒 full 同步该 Agent，给 Guest Agent、网络和系统服务预留准备时间。

删除类操作：

- 删除和强制删除会移除 VM 定义和普通磁盘卷。
- Agent 删除成功后，后端先从 Redis 运行态缓存移除该 VM 及其快照缓存。
- 后端立即广播 `runtime.updated`，让前端尽快从列表移除该 VM。
- 后端延迟 8 秒 full 同步该 Agent，兜底校准宿主机 VM 数量、快照和其他运行态字段。

记录规则：

- 每次操作成功后，后端创建已完成的 `vm.{action}` 任务记录。
- 每次操作成功后，后端写入同名审计 action。
- Agent 执行失败时，接口返回中文错误，不写成功审计。

### 12.4 删除与强制删除

正常删除适用条件：

- VM 必须处于 `stopped`。
- 删除前 Agent 会读取 VM 配置，收集普通磁盘对应的存储池和卷名。
- CDROM 介质不会被删除。

执行命令：

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> undefine <vm>
virsh --connect <LIBVIRT_URI> vol-delete --pool <pool> <volume>
```

强制删除执行流程：

1. Agent 先读取 `domstate`。
2. 如果 VM 不是停止状态，先执行 `destroy`。
3. 再执行 `undefine`。
4. 最后删除普通磁盘卷。

```bash
virsh --connect <LIBVIRT_URI> domstate <vm>
virsh --connect <LIBVIRT_URI> destroy <vm>
virsh --connect <LIBVIRT_URI> undefine <vm>
virsh --connect <LIBVIRT_URI> vol-delete --pool <pool> <volume>
```

边界规则：

- 正常删除不会隐式关闭正在运行的 VM。
- 如果 libvirt 返回 `cannot delete inactive domain with N snapshots`，后端和 Agent 会转换为中文提示，要求先删除快照。
- 删除磁盘依赖配置中能识别到 `pool` 和磁盘路径；无法识别的磁盘不会被平台猜测删除。

## 十三、控制台通道方案

### 13.1 接口与访问条件

当前已实现 noVNC + 后端 WebSocket 反向代理 + Agent VNC 白名单代理链路。控制台不要求再次输入 Agent Token，后端会使用已登记 Agent 的加密令牌访问 Agent。

控制台配置查询接口：

```http
GET /api/vms/{id}/console
GET /v1/vms/{name}/console
```

控制台 WebSocket 接口：

```http
GET /api/vms/{id}/console/ws?token=<SESSION_TOKEN>
GET /v1/vms/{name}/console/ws
```

访问条件：

- 前端虚拟机列表操作列提供“控制台”入口。
- 前端打开控制台前先查询 `passwordEnabled`。
- 如果 VM 正在运行且启用了 VNC 密码，前端先弹出密码输入窗口。
- 后端 WebSocket 仅允许 `running` 状态 VM 连接；已停止 VM 会被拒绝。
- 当前实现只支持 VNC/noVNC，不支持 SPICE 客户端接入。

### 13.2 访问链路

执行流程：

1. 前端调用 `GET /api/vms/{id}/console` 查询控制台配置。
2. 后端使用 VM `hostId` 找到 Agent 并解密 Agent Token。
3. 后端转发 Agent `GET /v1/vms/{name}/console`。
4. Agent 读取包含敏感字段的 VM XML，只返回控制台类型、监听地址、端口和 `passwordEnabled`。
5. 前端创建 noVNC 连接到 `GET /api/vms/{id}/console/ws?token=<SESSION_TOKEN>`。
6. 后端校验登录会话、VM 状态和 Agent Token。
7. 后端连接 Agent WebSocket，并携带 `Authorization: Bearer <AGENT_TOKEN>`。
8. Agent 再次读取 VM 控制台信息，只连接该 VM XML 中对应的 VNC TCP 端口。
9. 后端在浏览器 WebSocket 与 Agent WebSocket 之间透明转发 VNC 二进制数据。

Agent 获取控制台信息命令：

```bash
virsh --connect <LIBVIRT_URI> dumpxml <vm> --security-info
```

解析规则：

- 只接受 `<graphics type="vnc">`。
- 读取 `listen` 和 `port` 作为 Agent 到本机 VNC 的连接目标。
- `listen` 为空、`0.0.0.0`、`::` 或 `[::]` 时改用 `127.0.0.1`。
- `passwd` 属性存在且非空时，`passwordEnabled=true`。
- 查询接口不返回密码明文。

### 13.3 控制台密码配置

创建虚拟机时：

- 前端可选择是否启用 VNC 控制台密码。
- 后端和 Agent 只校验启用密码时密码不能为空。
- 密码不会写入平台数据库。
- Agent 创建 XML 时通过 `virt-install --graphics vnc,listen=0.0.0.0,password=<password>` 写入 VNC 配置。

编辑虚拟机时：

- 基础信息页可修改 VNC 密码状态。
- 已停止 VM 支持启用、修改和关闭控制台密码。
- 运行中 VM 支持启用或修改密码，并通过 live/config 同时更新当前会话与持久配置。
- 运行中 VM 不支持关闭已启用的控制台密码，避免 QEMU VNC 运行态清空密码后仍保持 password 认证造成状态不一致。

配置接口：

```http
PUT /api/vms/{id}/console
PUT /v1/vms/{name}/console
```

Agent 执行命令：

```bash
# 读取包含 passwd 的 graphics 配置
virsh --connect <LIBVIRT_URI> dumpxml --security-info <vm>

# 运行中同时修改当前会话与持久配置
virsh --connect <LIBVIRT_URI> update-device <vm> <graphics-xml> --live --config

# 已停止时只修改持久配置
virsh --connect <LIBVIRT_URI> update-device <vm> <graphics-xml> --config
```

### 13.4 安全边界

- Agent 不提供通用 TCP 代理，只根据当前 VM XML 中的 VNC graphics 连接对应端口。
- Agent 控制台 WebSocket 仍走 `/v1/*` Bearer Token 鉴权。
- 后端控制台 WebSocket 使用当前登录会话 token 鉴权。
- 后端在 WebSocket 连接成功后写入 `vm.console` 审计日志。
- noVNC 密码校验由 noVNC 与 QEMU VNC 服务完成，平台不保存也不回显密码。
- VNC 密码只存在于 libvirt/QEMU 的 graphics 配置中；后端和 Agent 配置查询只返回 `passwordEnabled`。

## 十四、运行态刷新与指标链路

### 14.1 触发入口与刷新类型

刷新入口：

| 入口 | 后端行为 | 刷新范围 |
| :-: | :-: | :-: |
| 定时轻量刷新 | 创建或复用 `runtime.refresh.fast` | 全部 Agent 的 fast VM 列表 |
| 定时深度刷新 | 创建或复用 `runtime.refresh.all` | 全部 Agent 的 full VM 列表与快照 |
| 手动全量刷新 | `POST /api/refresh` 创建或复用 `runtime.refresh.all` | 全部 Agent 的 full VM 列表与快照 |
| 单台 VM 刷新 | `POST /api/vms/{id}/refresh` | 当前 VM 所属 Agent 的单台 VM full 详情 |
| VM 操作后同步 | 根据操作类型选择状态快改、延迟 full 或即时 full | 当前 VM 所属 Agent |

默认配置：

- `RUNTIME_SYNC_INTERVAL` 控制定时 fast 刷新，默认 `30s`，设置为 `0` 可关闭。
- `RUNTIME_DEEP_SYNC_INTERVAL` 控制定时 full 深度刷新，默认 `10m`，设置为 `0` 可关闭。
- `RUNTIME_SYNC_CONCURRENCY` 控制 Agent 同步并发数。
- 前端主布局右上角全量刷新图标触发 `/api/refresh`。
- 前端订阅 `/api/events`，收到 `runtime.updated` 后重新读取运行态缓存。

### 14.2 后端任务与缓存写入

全局刷新任务：

- 后端 refresh worker 从任务队列消费 `runtime.refresh.fast` 和 `runtime.refresh.all`。
- 任务开始时广播 `sync.started`。
- 每个 Agent 同步进度会广播 `sync.progress`。
- 任务结束后广播 `runtime.updated` 和 `sync.finished`。
- 如果全部 Agent 同步失败，任务状态为 `failed`；否则记录已同步和失败数量。

运行态缓存：

- 运行态缓存统一写入 Redis 的 `kvm:runtime:*` key。
- Redis 是后端必需依赖，后端启动时会校验连接。
- full 刷新会保存 host、VM 和快照缓存。
- fast 刷新只保存 host 和 VM 缓存，不覆盖快照缓存。
- 写入运行态缓存前后会复查 Agent 登记和删除标记；如果 Agent 已删除，后端会清理该 Agent 的 host、VM 和快照缓存并跳过写入。读取 VM 列表时也会过滤数据库中已不存在的 Agent 残留，防止并发刷新任务把已删除资源重新写回或继续展示。
- Agent 同步失败会累计失败次数，达到阈值后创建 Agent 离线告警。

单台 VM 刷新：

- `POST /api/vms/{id}/refresh` 直接调用 Agent `/v1/vms/{name}/refresh`。
- 该路径不创建 `runtime.refresh.all` 全局任务。
- 后端同时读取 Agent `/v1/host`，用于更新当前宿主机摘要。
- 成功后只更新该 VM 的运行态缓存并追加一条 VM 指标样本。
- 成功后写入 `vm.refresh` 审计并广播 `runtime.updated`。

### 14.3 full、fast 与定向刷新差异

full 模式：

- Agent 调用 `/v1/vms`。
- 采集 VM 状态、XML 基础规格、Guest Agent OS/IP、磁盘明细、内存使用率、CPU 使用率、磁盘 I/O 和网络吞吐。
- 后端会继续逐 VM 调用 Agent 快照接口，更新快照缓存。
- 手动 `/api/refresh` 和低频深度刷新都使用 full 模式。

fast 模式：

- Agent 调用 `/v1/vms?level=fast`。
- 保留 VM 状态、XML 基础规格、CPU 采样、运行中 VM 内存使用率、磁盘 I/O 和网络吞吐。
- 跳过 QEMU Guest Agent OS/IP 查询。
- 跳过磁盘 `domblkinfo Capacity`、`virt-df --csv` 和 `virt-filesystems --csv --all --long` 明细。
- 跳过快照采集。
- 后端会合并上一次 full 或单台刷新得到的 OS、主 IP、描述、磁盘明细和可用内存使用率。
- fast 模式不会根据 VM 名称猜测 OS 类型；Agent 返回 OS 为空时，后端优先保留已有可信系统信息。

单台 VM full 刷新：

- Agent 调用 `/v1/vms/{name}/refresh`。
- 重新读取 Guest Agent OS、`domifaddr` IP、磁盘明细、CPU/内存使用率和 I/O 速率。
- 不采集快照。
- 适合修改 VM IP、磁盘或配置后立即定向刷新。

内存使用率命令：

```bash
virsh --connect <LIBVIRT_URI> dommemstat <vm>
```

内存使用率规则：

- 已停止 VM 直接返回 `0%`，不执行 `dommemstat`。
- 优先读取 `actual` 作为当前分配内存。
- 读取 `usable` 作为可回收内存。
- 当总量大于 0 且可回收内存不超过总量时，按 `(总量 - 可回收内存) / 总量 * 100` 计算使用率。
- 如果没有合法 `usable`，则使用 `available` 兜底并按同一公式计算。
- 采样失败或字段不完整时，本轮 fast 不标记内存使用率可用，后端合并逻辑会保留上一次可用值。

### 14.4 VM 操作后的刷新策略

状态可预测的电源操作：

- 启动、恢复、暂停、关机、停止、强制关机、重启和强制重启执行成功后，后端先更新当前 VM 状态并广播 `runtime.updated`。
- 后端随后延迟 8 秒 full 同步该 Agent。
- 延迟 full 用于避免 Guest Agent、IP、OS 和内存使用率被过早采成兜底值。

删除操作：

- 删除和强制删除执行成功后，后端在接口返回前 full 同步该 Agent。
- 同步完成后广播 `runtime.updated`，保证列表不再显示已删除 VM。

配置类操作：

- VM 基础配置、设备、XML、介质、自启动、克隆、创建和迁移完成后，后端会按对应业务路径同步目标 Agent 或源/目标 Agent。
- 快照恢复成功后，后端先对目标 VM 执行单台 VM full 刷新，再刷新快照缓存并广播 `runtime.updated`。
- 各页面刷新入口、刷新范围和接口对照详见 `docs/frontend-refresh-functions.md`。

### 14.5 指标写入与查询

写入路径：

1. Agent 同步成功后，后端构造 host/vm 指标样本。
2. 指标样本写入 Redis Stream `kvm:metrics:samples`。
3. 后端 metric writer 通过消费组 `kvm-manager-metric-writers` 消费 Stream。
4. 消费成功后写入 PostgreSQL 的 `host_metric_samples` 和 `vm_metric_samples`。
5. 消费成功后 ack Stream 消息。

保留与裁剪：

- Redis Stream 按 `METRIC_STREAM_MAXLEN` 近似裁剪。
- 原始指标样本按 `METRIC_RETENTION_DAYS` 定期清理。
- 指标 rollup 会物化写入 `host_metric_rollups` 和 `vm_metric_rollups`。
- 查询 24h、7d、30d 时优先读取 rollup，不足时回退原始样本动态聚合。

查询接口：

```http
GET /api/metrics/hosts/{agentId}?range=1h|24h|7d|30d
GET /api/metrics/hosts/all?range=1h|24h|7d|30d
GET /api/metrics/vms/{vmId}?range=1h|24h|7d|30d
GET /api/metrics/vms/{vmId}?range=custom&start=<time>&end=<time>
```

聚合粒度：

| 时间范围 | 查询粒度 |
| :-: | :-: |
| `1h` | 1 分钟 |
| `24h` | 30 分钟 |
| `7d` | 1 小时 |
| `30d` | 1 天 |
| `custom` | 按窗口长度动态选择 1 分钟、30 分钟、1 小时或 1 天 |

## 十五、VM 磁盘 I/O 与网络吞吐采集

### 15.1 Agent 采集命令

VM 监控弹窗中的“磁盘 I/O”和“网络吞吐量”来自 Agent 对 libvirt 计数器的短周期采样。Agent 在 full 与 fast VM 列表采集时都会执行：

```bash
virsh --connect <LIBVIRT_URI> domstats --cpu-total <vm1> <vm2> ...
virsh --connect <LIBVIRT_URI> domstats --block --interface <vm1> <vm2> ...
```

采样规则：

- Agent 先读取一次 CPU 和 I/O 计数器。
- 等待约 1 秒。
- 再读取第二次 CPU 和 I/O 计数器。
- CPU、磁盘 I/O 和网络吞吐共用同一个等待窗口，避免串行等待两次。
- 如果 VM 列表为空，Agent 不执行采样命令。

### 15.2 速率计算规则

磁盘与网络计数器：

- 对同一 VM 聚合所有 `block.*.rd.bytes`。
- 对同一 VM 聚合所有 `block.*.wr.bytes`。
- 对同一 VM 聚合所有 `net.*.rx.bytes`。
- 对同一 VM 聚合所有 `net.*.tx.bytes`。
- 速率按 `(第二次计数 - 第一次计数) / 间隔秒数` 计算。
- 单位为 bytes/s。

异常处理：

- 如果计数器回退，速率按 `0` 处理。
- 如果采样失败，当前 VM 本轮速率为 `0`。
- 如果某台 VM 不存在或无法读取，不阻断其他 VM 采集。
- fast 模式仍保留 I/O 采样，因为趋势图依赖后台定时刷新持续写入样本。

### 15.3 字段流转

字段流转：

| Agent JSON 字段 | 后端领域字段 | 数据库字段 | 前端趋势用途 |
| :-: | :-: | :-: | :-: |
| `diskReadBytesPerSecond` | `DiskReadBytesPerSec` | `disk_read_bytes_per_second` | 磁盘读取 KB/s |
| `diskWriteBytesPerSecond` | `DiskWriteBytesPerSec` | `disk_write_bytes_per_second` | 磁盘写入 KB/s |
| `networkRxBytesPerSecond` | `NetworkRxBytesPerSec` | `network_rx_bytes_per_second` | 网络流入 KB/s |
| `networkTxBytesPerSecond` | `NetworkTxBytesPerSec` | `network_tx_bytes_per_second` | 网络流出 KB/s |

落库路径：

```text
Agent /v1/vms 响应
-> 后端运行态 VM
-> Redis Stream kvm:metrics:samples
-> metric writer
-> PostgreSQL vm_metric_samples
-> vm_metric_rollups
```

### 15.4 前端展示

VM 列表：

- CPU、内存、磁盘三列展示“规格 + 使用率”。
- 磁盘列悬浮后显示每块磁盘名称、已用大小和总大小。
- I/O 速率字段不会直接作为列表主列展示，但会进入指标样本。

VM 监控弹窗：

- 每台 VM 的“监控”按钮打开居中弹窗。
- 支持 `1h`、`24h`、`7d`、`30d` 和自定义时间范围。
- 展示 CPU、内存、磁盘使用率、磁盘 I/O 和网络吞吐图形卡片。
- 磁盘 I/O 展示读取和写入。
- 网络吞吐展示流入、流出和按同一时间点 `流入 + 流出` 计算的平均带宽。

总览页：

- “在线虚拟机资源利用率”只面向在线虚拟机。
- 前端调用 `/api/metrics/vms/{vmId}?range=1h` 读取最近 1 小时趋势。
- 该趋势来自数据库指标样本，不直接读取运行态缓存。
