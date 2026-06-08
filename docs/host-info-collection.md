# 宿主机信息采集说明

本文档说明宿主机页面、总览页面和宿主机趋势中各项信息的采集链路、Agent 执行的命令、字段计算方式和回退策略。

## 一、整体链路

宿主机页面不直接访问宿主机命令。当前链路如下：

1. 前端调用后端 `/api/hosts` 读取宿主机运行态缓存。
2. 总览页调用 `/api/dashboard/summary`，由后端基于运行态缓存中的宿主机和虚拟机数据聚合资源统计。
3. 宿主机趋势弹窗调用 `/api/metrics/hosts/{agentId}`，读取已落库的宿主机指标样本或聚合数据。
4. 宿主机接口页面调用 `/api/host-interfaces/{agentId}`，由后端实时转发到对应 Agent 的 `/v1/host/interfaces`。
5. 宿主机接口新增弹窗调用 `POST /api/host-interfaces/{agentId}`，由 Agent 创建 Linux bridge 接口并返回最新接口信息。
6. 后端按 `RUNTIME_SYNC_INTERVAL` 自动触发面向所有 Agent 的全局运行态轻量刷新；如果当前没有已登记 Agent，自动刷新不会创建任务；手动 `/api/refresh` 触发 full 全量刷新。
7. 自动刷新创建或复用 `runtime.refresh.fast` 异步任务，手动刷新创建或复用 `runtime.refresh.all` 异步任务。
8. 后台刷新 worker 按任务向每个已登记 Agent 调用 `/v1/host`，并根据刷新类型继续调用 `/v1/vms?level=fast` 或 `/v1/vms`。
9. 虚拟机页面每行刷新按钮调用 `POST /api/vms/{id}/refresh`，只同步该虚拟机所属宿主机上的当前 VM 信息，刷新 IP 等 VM 详情但跳过快照采集，不创建全量刷新任务。
10. Agent 在 KVM 宿主机上执行 `virsh`、`hostname`、`ip`、`df` 和读取 `/proc` 文件等操作。
11. 后端把 Agent 返回结果写入 Redis 运行态缓存，并通过 SSE 通知前端更新。Redis 是后端必需依赖，连接失败时后端会直接启动失败。
12. 写入 Redis 前后，后端会重新确认对应 Agent 登记和删除标记；如果 Agent 在刷新过程中被删除，后端会清理该 Agent 的 host、VM 和快照运行态缓存并跳过写入，避免已删除宿主机被旧刷新任务重新显示。
13. 同步成功后，后端把宿主机指标写入 Redis Stream `kvm:metrics:samples`，再由指标写入任务落库到 `host_metric_samples`，用于趋势查询。

刷新任务进度写入 `tasks.payload`。前端各刷新入口的范围、触发接口和 fast/full 边界详见 `docs/frontend-refresh-functions.md`。任务、审计和告警日志覆盖范围详见 `docs/operation-log-coverage.md`。

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

- Agent 宿主机采集：`agent/internal/kvm/virsh_host.go`
- Agent 宿主机接口采集与创建：`agent/internal/kvm/virsh_host_interfaces.go`
- Agent 宿主机接口列表合并与兜底过滤：`agent/internal/kvm/host_interface_list.go`
- Agent 返回结构：`agent/internal/kvm/types.go`
- 后端 Agent 客户端：`backend/pkg/agent/client.go`
- 后端同步和运行态缓存：`backend/internal/service/realtime/service.go`
- 后端指标事件：`backend/internal/service/realtime/metrics_stream.go`
- 后端宿主机指标查询：`backend/api/router/metrics.go`
- 前端宿主机页面：`frontend/src/features/hosts/HostsPage.tsx`
- 前端宿主机接口页面：`frontend/src/features/host-interfaces/HostInterfacesPage.tsx`
- 前端宿主机趋势弹窗：`frontend/src/components/kvm/HostTrendDialog.tsx`
- 前端总览页面：`frontend/src/features/dashboard/DashboardPage.tsx`

## 二、Agent 返回字段

Agent `/v1/host` 返回结构如下：

| 字段 | 含义 | 主要来源 |
| :-: | :-: | :-: |
| `hostname` | 宿主机名称 | `hostname` |
| `hostAddress` | 宿主机管理地址 | 默认路由源 IP，失败后回退全局 IPv4 或 `hostname -I` |
| `status` | 宿主机状态 | Agent 成功返回时为 `online` |
| `kvmVersion` | KVM/libvirt 版本信息 | `virsh --connect <LIBVIRT_URI> version` 的第一行 |
| `kvmFullVersion` | KVM/libvirt 完整版本信息 | `virsh --connect <LIBVIRT_URI> version` 的完整输出 |
| `cpuModel` | 宿主机 CPU 型号 | `virsh --connect <LIBVIRT_URI> nodeinfo` 中的 `CPU model:` |
| `cpuCores` | 宿主机 CPU 核心数 | `virsh --connect <LIBVIRT_URI> nodeinfo` 中的 `CPU(s):` |
| `cpuUsage` | 宿主机 CPU 使用率 | 两次读取 `/proc/stat` 计算差值 |
| `memoryBytes` | 宿主机内存总量 | `virsh --connect <LIBVIRT_URI> nodeinfo` 中的 `Memory size:` |
| `memoryUsage` | 宿主机内存使用率 | `/proc/meminfo` 中的 `MemTotal` 和 `MemAvailable` |
| `storageBytes` | 宿主机存储总量 | 本机本地磁盘文件系统的 `df` 总量 |
| `storageUsage` | 宿主机存储使用率 | 本机本地磁盘文件系统的 `Used / Total` |
| `capabilities` | Agent 能力列表 | Agent 固定声明的能力集合 |

## 三、宿主机基础信息

### 3.1 主机名

字段：`hostname`

命令：

```bash
hostname
```

Agent 对输出做 `TrimSpace` 后返回。命令失败时不会中断 `/v1/host`，字段为空。

### 3.2 宿主机 IP

字段：`hostAddress`，后端宿主机列表中的 `address`

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

### 3.3 状态

字段：`status`

Agent 成功完成 `/v1/host` 时固定返回：

```text
online
```

后端会再次归一化状态。若 Agent 调用失败，本次同步不会写入新的宿主机运行态数据，并通过 Agent 同步失败、告警和任务结果体现异常。

### 3.4 KVM 版本

字段：

- `kvmVersion`
- `kvmFullVersion`

命令：

```bash
virsh --connect <LIBVIRT_URI> version
```

Agent 取输出第一行作为 `kvmVersion`，用于宿主机卡片底部展示。

Agent 同时保留完整输出作为 `kvmFullVersion`，前端在宿主机卡片底部版本文本的悬浮提示中展示完整内容。命令失败时不会中断 `/v1/host`，字段为空。

## 四、宿主机接口

接口页面字段：`name`、`type`、`mac`、`ipv4`、`ipv4Mode`、`ipv6`、`ipv6Mode`、`bridgeDevice`、`bootMode`、`status`、`stp`、`delay`

### 4.1 读取链路

读取链路：

```text
frontend HostInterfacesPage
  -> GET /api/host-interfaces/{agentId}
  -> GET /v1/host/interfaces
  -> virsh iface-list / iface-dumpxml
  -> ip -o link show / ip -o addr show
```

Agent 参考 webvirtmgr 的实现，以 libvirt interface XML 作为优先数据源：

```bash
virsh --connect <LIBVIRT_URI> iface-list --all
virsh --connect <LIBVIRT_URI> iface-list          # 仅在 --all 失败或返回为空时兜底
virsh --connect <LIBVIRT_URI> iface-list --inactive # 仅在 --all 失败或返回为空时兜底
virsh --connect <LIBVIRT_URI> iface-dumpxml <interface> --inactive
virsh --connect <LIBVIRT_URI> iface-dumpxml <interface>
```

libvirt XML 能读取已定义但未激活的接口，并能准确拿到持久配置：

- `<start mode>`
- `<protocol family>`
- `<dhcp>`
- `<ip address prefix>`

Agent 会把这些字段映射为：

- `bootMode`
- `ipv4Mode`
- `ipv6Mode`
- `ipv4`
- `ipv6`
- `bridgeDevice`
- `stp`
- `delay`

如果 `virsh iface-dumpxml <interface> --inactive` 返回的 bridge 中没有下挂设备，Agent 会再读取 `virsh iface-dumpxml <interface>`，用当前激活 XML 中的 `<bridge><interface name="...">` 补充 `bridgeDevice`。

### 4.2 运行态字段补充

Agent 会再使用文本命令补充同名接口的运行态字段：

```bash
ip -o link show
ip -o addr show
```

该路径解析以下运行态字段：

- 接口名称
- MAC
- 运行状态
- master 设备
- 地址列表

接口类型映射规则：

- `lo` 映射为 `loopback`。
- 存在 `/sys/class/net/<interface>/bridge` 的接口映射为 `bridge`。
- 其他接口默认为 `ethernet`。

地址选择规则：

- IPv4 选择第一个可用非回环、非链路本地地址。
- IPv6 优先选择非 `fe80:` 地址。
- IPv6 只有链路本地地址时标记为 `link-local`。

项目不再执行 `ip -j address show`。

### 4.3 libvirt 列表优先规则

当 libvirt interface 列表可用且非空时：

- 页面只展示 `virsh iface-list --all` 返回的 libvirt 管理接口。
- `ip -o` 文本命令仅用于补充这些同名接口的 MAC、状态、地址等运行态字段。
- 不会追加 `vnet*`、VLAN 子接口、容器网桥等系统运行态设备。

IPv6 展示补充规则：

- 若 libvirt 未配置 IPv6 协议导致模式为 `none`，但运行态存在 `fe80:` 链路本地地址，则展示模式提升为 `link-local`。
- 已有 `dhcp` 或 `static` 配置不会被运行态地址覆盖。

为降低接口列表加载耗时，Agent 使用以下策略：

- 优先从一次 `virsh iface-list --all` 输出中解析接口名称、状态和 MAC。
- 只有该命令失败或返回为空时，才降级执行 `virsh iface-list` 与 `virsh iface-list --inactive` 合并列表。
- `iface-dumpxml` 仍按接口执行，因为 `iface-list --all` 只能提供列表、状态和 MAC。
- 启动模式、协议、DHCP/static 地址、网关、bridge 下挂设备、STP 和 delay 等配置字段仍需要从 XML 读取。
- 系统文本命令补充运行态字段时，也只处理 libvirt 列表中的同名接口。

只有在 libvirt interface 列表不可用或为空时，Agent 才使用 `ip -o` 文本结果作为兜底列表。兜底列表会过滤明显的运行态噪声接口，包括 `vnet*`、`tap*`、`veth*`、`docker*`、`br-*`、`cni*`、`flannel*`、`vxlan*`、`tun*`、名称包含 `idrac`、`ipmi`、`bmc` 的带外管理接口，以及带点号的 VLAN 子接口。

bridge 接口会额外读取：

```bash
cat /sys/class/net/<interface>/bridge/stp_state
cat /sys/class/net/<interface>/bridge/forward_delay
```

`stp_state=1` 映射为 `on`，其他值映射为 `off`。读取失败时字段为空。

### 4.4 创建链路

创建链路：

```text
frontend HostInterfaceCreateDialog
  -> POST /api/host-interfaces/{agentId}
  -> POST /v1/host/interfaces
  -> virsh iface-define <xml>
  -> virsh iface-start <interface>
```

当前创建能力支持 `bridge` 与 `ethernet` 类型。

Agent 会校验接口名称规则：

- 长度不超过 15。
- 只允许字母、数字、点、短横线和下划线。

`startMode` 仅允许：

- `none`
- `onboot`
- `hotplug`

新增弹窗默认选择 `onboot`。

新增弹窗的设备下拉参考 webvirtmgr，优先读取 libvirt node device 中的 net 设备：

```bash
virsh --connect <LIBVIRT_URI> nodedev-list --cap net
virsh --connect <LIBVIRT_URI> nodedev-dumpxml <node-device>
```

若 node device 读取失败，Agent 兜底使用 `ip -o link show` 解析设备名称。

选择绑定设备时：

- 前端会根据当前接口列表预校验该设备是否已被其他接口使用。
- Agent 创建前也会重新读取宿主机接口。
- Agent 确认该设备存在且未作为其他接口的下挂设备后，才将其写入 libvirt interface XML。

`bridge` 类型写入：

```xml
<bridge stp="on" delay="0">
  <interface name="em1" type="ethernet"/>
</bridge>
```

`ethernet` 类型绑定设备时写入：

```xml
<link dev="em1"/>
```

### 4.5 IPv4、IPv6 与 DNS 规则

IPv4/IPv6 支持三种模式：

- `none`
- `dhcp`
- `static`

XML 写入规则：

- `dhcp` 写入 `<dhcp/>`。
- `static` 写入 `<ip address prefix>`。
- 网关不为空时写入 `<route gateway>`。

前端表单默认值与选项：

- IPv4 表单默认 `dhcp`，选项展示为 `DHCP`、`Static`、`No configuation`。
- IPv6 表单默认 `none`，选项展示为 `No configuation`、`DHCP`、`Static`。

创建静态 IPv4/IPv6 前，前端会基于当前接口列表预校验：

- 重复 IP
- 重复或重叠子网
- 网关必须与地址处于同一 CIDR 子网
- 网关不能等于接口地址

Agent 会再次读取宿主机接口并进行同样的硬校验。

拒绝创建的场景包括：

- 任一已有接口已使用相同 IP。
- 新旧 CIDR 子网存在重叠。

这样可以避免 ARP/NDP 冲突和同宿主机路由歧义。

若本次创建 bridge 选择了一个已有物理设备作为绑定设备，则该物理设备原有 IP 和子网允许迁移到新 bridge，不参与重复 IP/子网冲突判断。

静态 IPv4 DNS 表单规则：

- 静态 IPv4 卡片默认显示 `DNS1 地址` 输入框。
- DNS1 为空时不写入系统 DNS。
- 填写 DNS1 后，前端会校验 DNS 地址格式并写入系统 DNS。
- 需要第二个 DNS 时，可点击 `添加DNS2地址` 显示 DNS2 输入框。
- DNS2 为空时会被忽略。

Agent 写入 DNS 的策略：

- 先尝试通过 `nmcli connection modify <interface>` 分别写入 IPv4/IPv6 DNS。
- 若宿主机没有可用 `nmcli` 或连接修改失败，则回退到 CentOS/RHEL `network-scripts`。
- 回退时在 `/etc/sysconfig/network-scripts/ifcfg-<interface>` 写入 `DNS1`、`DNS2` 等字段。

### 4.6 ifcfg 备份与系统配置边界

ifcfg 备份规则：

- 写入 ifcfg 前会生成 `.kvm-manager.<timestamp>.bak` 备份。
- 写入成功后会删除该备份。
- 写入失败时保留备份用于人工恢复。

bridge 类型可选择绑定已有设备。绑定设备后，Agent 会在执行 `virsh iface-define` 前检查以下文件是否已存在：

- `/etc/sysconfig/network-scripts/ifcfg-<bridge>`
- `/etc/sysconfig/network-scripts/ifcfg-<device>`

如果文件已存在，Agent 会先生成 `ifcfg-xxx.YYYYMMDDHHMMSS.bak` 备份。

当前项目的系统配置边界：

- 不再提供“写入桥接 ifcfg 配置”开关。
- 不主动重写 bridge 或物理设备 ifcfg。
- bridge 下挂设备和对应系统配置交给 `virsh iface-define` / `virsh iface-start` 处理。
- 不会自动重启 `network`/`NetworkManager`。
- 不会执行 `nmcli con up/down`。

这些限制用于避免远程管理宿主机时因网络重启导致 Agent 失联。

### 4.7 创建启动规则

创建过程中的启动规则：

- 若 `startMode` 为 `onboot` 或 `hotplug`，Agent 会在 `iface-define` 后执行 `virsh iface-start <interface>`。
- 若启动命令返回失败但重新读取接口状态已经是 `up`，则按成功处理，避免出现“提示失败但刷新后已运行”的误报。
- 若最终状态仍未运行，Agent 会执行 `virsh iface-undefine <interface>`，避免残留半成品定义。
- `startMode=none` 时只定义接口，不立即启动。

前端提交创建后，创建按钮会显示“创建中”并禁用，避免重复提交。后端转发创建接口请求使用 30 秒接口操作超时；若前端仍收到 Agent 超时，会自动刷新接口列表，如果发现目标接口已存在，则按创建成功处理并关闭弹窗。

### 4.8 状态与删除链路

状态与删除链路：

```text
frontend HostInterfaceDetailDialog
  -> PUT /api/host-interfaces/{agentId}/state/{interface}
  -> PUT /v1/host/interfaces/{interface}/state
  -> virsh iface-start <interface> 或 virsh iface-destroy <interface>

frontend HostInterfaceDetailDialog
  -> DELETE /api/host-interfaces/{agentId}/delete/{interface}
  -> DELETE /v1/host/interfaces/{interface}/delete
  -> virsh iface-undefine <interface>
  -> 若删除对象为 bridge 且宿主机仍残留同名运行态 bridge：
     ip link set <interface> down
     ip link delete <interface> type bridge
```

删除前置规则：

- 删除前 Agent 会确认接口已停止。
- 运行中的接口必须先执行停止后才能删除。
- 停止接口时若 `virsh iface-destroy` 返回失败但重新读取状态已经是 `down`，也会按成功处理。

删除执行规则：

- Agent 先执行 `virsh iface-undefine` 移除 libvirt 接口定义。
- 若该接口类型为 `bridge`，且 `ip link show <interface>` 仍能看到同名运行态设备，Agent 会先确认 `/sys/class/net/<interface>/bridge` 存在，再删除该残留 Linux bridge。
- 未启动直接删除时通常不会产生运行态 bridge，清理步骤会自动跳过。
- 非 bridge 类型不会执行运行态删除，避免误删物理网卡。

## 五、CPU 信息

### 5.1 CPU 型号和核心数

字段：

- `cpuModel`
- `cpuCores`

命令：

```bash
virsh --connect <LIBVIRT_URI> nodeinfo
```

解析输出中的：

```text
CPU model:
CPU(s):
```

`CPU model:` 后的文本作为 `cpuModel`，用于迁移前基础 CPU 架构预检。`CPU(s):` 行第一个数字作为宿主机 CPU 核心数。

### 5.2 CPU 使用率

字段：`cpuUsage`

Agent 每次执行 `/v1/host` 时读取两次 `/proc/stat`，间隔 1 秒。

第一次采样：

```bash
awk '/^cpu / {print $2+$3+$4+$5+$6+$7+$8, $5}' /proc/stat
```

等待：

```text
1 秒
```

第二次采样：

```bash
awk '/^cpu / {print $2+$3+$4+$5+$6+$7+$8, $5}' /proc/stat
```

计算公式：

```text
CPU 使用率 = ((第二次 total - 第一次 total) - (第二次 idle - 第一次 idle)) / (第二次 total - 第一次 total) * 100
```

说明：

- 结果会被限制在 `0` 到 `100` 之间。
- 任一采样失败或采样差值无效时返回 `0`。
- 采样窗口为 1 秒，与虚拟机 CPU 使用率采样窗口保持一致。

## 六、内存信息

### 6.1 内存总量

字段：`memoryBytes`

命令：

```bash
virsh --connect <LIBVIRT_URI> nodeinfo
```

解析输出中的：

```text
Memory size:
```

`virsh nodeinfo` 中该字段单位为 KiB，Agent 转换为 bytes：

```text
memoryBytes = Memory size * 1024
```

### 6.2 内存使用率

字段：`memoryUsage`

命令：

```bash
awk '/MemTotal:/ {t=$2} /MemAvailable:/ {a=$2} END {if (t>0) print t, a}' /proc/meminfo
```

计算公式：

```text
内存使用率 = (MemTotal - MemAvailable) / MemTotal * 100
```

说明：

- `/proc/meminfo` 中单位为 KiB，但使用率只需要比例，不需要转换单位。
- 结果会被限制在 `0` 到 `100` 之间。
- 命令失败、`MemTotal` 无效或 `MemAvailable` 无效时返回 `0`。

## 七、存储信息

### 7.1 存储总量和使用率

字段：`storageBytes`、`storageUsage`

Agent 只读取宿主机本机当前挂载的本地磁盘文件系统：

```bash
df -PB1 --local
```

解析 `df` 输出：

- 第 1 列 `Filesystem` -> 文件系统来源
- 第 2 列 `1B-blocks` -> 文件系统总容量
- 第 3 列 `Used` -> 文件系统已用容量
- 最后一列 `Mounted on` -> 挂载点

计算公式：

```text
storageBytes = 去重后的本地磁盘文件系统 1B-blocks 合计
storageUsage = 去重后的本地磁盘文件系统 Used 合计 / storageBytes * 100
```

筛选和去重规则：

- 只统计 `Filesystem` 为 `/dev/*`、`UUID=*` 或 `LABEL=*` 的本地磁盘文件系统。
- 跳过 `tmpfs`、`devtmpfs`、`overlay`、`proc`、`sysfs`、`cgroup` 等内存、容器和内核伪文件系统。
- 同一个 `Filesystem` 出现在多个挂载点时只计入一次，避免 bind mount 等场景重复累加。

说明：宿主机存储展示的是本机当前挂载的本地磁盘容量，不依赖是否创建 libvirt storage pool，也不统计 NFS/iSCSI 等远端文件系统。

## 八、后端同步和缓存

宿主机运行态刷新分为后端定时 `runtime.refresh.fast`、手动 `runtime.refresh.all`、单 Agent 同步和页面局部重新读取。宿主机页面、接口页面、存储池页面、网络池页面和趋势弹窗的具体刷新入口与刷新范围详见 `docs/frontend-refresh-functions.md`。

存储池、网络池和宿主机接口属于按宿主机实时读取的资源，不依赖 30 秒运行态缓存刷新。对应增删改操作完成后，后端会广播 `storage.pool.updated`、`network.pool.updated` 或 `host.interface.updated`，前端对应资源页会按 `agentId` 自动重新读取当前宿主机数据。

后端同步时会先调用 Agent `/v1/host`：

```text
GET /v1/host
```

然后映射为后端 `Host` 运行态模型：

| 后端字段 | 来源 |
| :-: | :-: |
| `id` | Agent ID |
| `name` | Agent 登记名称 |
| `address` | `hostAddress`，为空时回退 `endpoint` |
| `hostname` | Agent `hostname` |
| `cluster` | 当前固定为 `default` |
| `status` | Agent `status` 归一化结果 |
| `cpuCores` | Agent `cpuCores` |
| `cpuUsage` | Agent `cpuUsage`，并限制在 `0` 到 `100` |
| `memoryBytes` | Agent `memoryBytes` |
| `memoryUsage` | Agent `memoryUsage`，并限制在 `0` 到 `100` |
| `storageBytes` | Agent `storageBytes` |
| `storageUsage` | Agent `storageUsage`，并限制在 `0` 到 `100` |
| `vmCount` | 本次同步拿到的虚拟机数量 |
| `kvmVersion` | Agent `kvmVersion` |
| `kvmFullVersion` | Agent `kvmFullVersion` |

运行态数据保存规则：

1. 写入 Redis 运行态缓存。
2. 前端通过 `/api/hosts` 读取运行态缓存，不直接查询数据库中的宿主机资源。
3. 写入前后复查 Agent 登记和删除标记，若已删除则清理对应运行态缓存并跳过写入。
4. 读取宿主机和总览时会过滤数据库中已不存在的 Agent，并顺带清理孤儿运行态缓存。
5. 同步完成后广播 `runtime.updated` 等 SSE 事件，前端据此刷新页面。

## 九、趋势数据

同步成功后，后端会构造宿主机指标事件并写入 Redis Stream `kvm:metrics:samples`。

宿主机指标样本字段包括：

| 字段 | 含义 |
| :-: | :-: |
| `agentId` | Agent ID |
| `hostName` | 宿主机展示名称 |
| `cpuUsage` | CPU 使用率 |
| `memoryUsage` | 内存使用率 |
| `memoryBytes` | 内存总量 |
| `storageUsage` | 存储使用率 |
| `storageBytes` | 存储总量 |
| `diskReadBytesPerSecond` | 宿主机磁盘读取速率 |
| `diskWriteBytesPerSecond` | 宿主机磁盘写入速率 |
| `networkRxBytesPerSecond` | 宿主机网络流入速率 |
| `networkTxBytesPerSecond` | 宿主机网络流出速率 |
| `vmCount` | 虚拟机数量 |
| `collectedAt` | 采集时间 |

`StartMetricWriter` 是后端启动时创建的指标写入后台协程，不是独立命令或额外服务。它随 Redis 运行态缓存一起启动，负责消费 `kvm:metrics:samples` Stream 中的指标事件，并把宿主机样本落库到 `host_metric_samples`。Redis 连接失败时后端会直接启动失败。

趋势接口：

```text
GET /api/metrics/hosts/{agentId}?range=1h
```

其中 `{agentId}` 可传具体 Agent ID；传 `all` 时后端会聚合全部宿主机。

支持的范围：

| range | 时间范围 | 默认聚合粒度 |
| :-: | :-: | :-: |
| `1h` | 最近 1 小时 | 1 分钟 |
| `24h` | 最近 24 小时 | 30 分钟 |
| `7d` | 最近 7 天 | 1 小时 |
| `30d` | 最近 30 天 | 1 天 |
| `custom` | 自定义开始和结束时间 | 按窗口长度自动选择 |

前端横轴刻度与数据聚合粒度分开控制：

- `1h` 按 1 分钟数据点悬浮查看。
  - 上方四个半宽图每 4 分钟显示刻度。
  - 底部网络吞吐宽图每 2 分钟显示刻度。
- `24h` 按 30 分钟数据点悬浮查看。
  - 上方四个半宽图每 1.5 小时显示刻度。
  - 网络吞吐宽图每 1 小时显示刻度。
- `7d` 按每小时数据点悬浮查看。
  - 上方四个半宽图显示每天 07:00、15:00 和 23:00。
  - 网络吞吐宽图显示每天 03:00、07:00、11:00、15:00、19:00 和 23:00。
- `30d` 按每日数据点悬浮查看。
  - 上方四个半宽图每 4 天显示刻度。
  - 网络吞吐宽图每 2 天显示刻度。

`custom` 范围需要传入：

```text
start=2026-05-24T10:00&end=2026-05-24T11:00
```

也支持 RFC3339 格式时间。后端要求 `start < end`。

## 十、总览页面统计

总览页面调用：

```text
GET /api/dashboard/summary
```

后端基于当前运行态缓存中的宿主机列表聚合：

| 汇总字段 | 计算方式 |
| :-: | :-: |
| `totalHosts` | 宿主机数量 |
| `onlineHosts` | `status == online` 的宿主机数量 |
| `totalMemoryBytes` | 累加所有宿主机 `memoryBytes` |
| `usedMemoryBytes` | 累加 `memoryBytes * memoryUsage / 100` |
| `totalDiskBytes` | 累加所有宿主机 `storageBytes` |
| `usedDiskBytes` | 累加 `storageBytes * storageUsage / 100` |
| `averageCpu` | 所有宿主机 `cpuUsage` 的平均值 |
| `averageMemory` | 所有宿主机 `memoryUsage` 的平均值 |

后端基于当前运行态缓存中的虚拟机列表聚合时，会先合并数据库中的模板标记，并排除 `isTemplate == true` 的虚拟机模板：

| 汇总字段 | 计算方式 |
| :-: | :-: |
| `totalVMs` | 非模板虚拟机数量 |
| `runningVMs` | 非模板虚拟机中 `status == running` 的数量 |
| `stoppedVMs` | 非模板虚拟机中 `status == stopped` 的数量 |
| `pausedVMs` | 非模板虚拟机中 `status == paused` 的数量 |
| `errorVMs` | 非模板虚拟机中 `status == error` 的数量 |
| `usedVCPUs` | 累加非模板虚拟机 `cpuCores` |
| `recentVMs` | 非模板虚拟机列表前 6 条 |

说明：

- 总览资源统计读取的是运行态缓存，不直接触发 Agent 采集。
- 虚拟机模板标记与取消标记只更新后端数据库标记表并广播 `runtime.updated`，总览重新读取后会立即按最新模板标记调整虚拟机统计口径。
- 总览页不再展示全局“资源利用率”和聚合“资源趋势”。当前布局将虚拟机与宿主机拆成两组：虚拟机状态分布右侧展示在线虚拟机资源利用率，并按选中的在线 VM 调用 `/api/metrics/vms/{vmId}?range=1h` 显示最近 1 小时趋势；宿主机组展示在线宿主机资源利用率、宿主机状态分布，并按选中的在线宿主机调用 `/api/metrics/hosts/{agentId}?range=1h` 显示最近 1 小时趋势。
- 宿主机 `/v1/host` 已采集宿主机级磁盘 I/O 和网络吞吐，趋势查询返回对应速率字段。

## 十一、前端展示规则

宿主机页面调用 `/api/hosts`、`/api/agents` 和 `/api/vms`。宿主机卡片展示：

- CPU：`cpuCores` 和 `cpuUsage`
- 内存：`memoryBytes` 和 `memoryUsage`
- 存储：`storageBytes` 和 `storageUsage`
- 虚拟机数量：`vmCount`
- KVM 版本：`kvmVersion`，悬浮提示展示 `kvmFullVersion`

宿主机页面当前有兜底百分比逻辑：

```text
如果实时 usage > 0，使用实时 usage；
如果实时 usage == 0 且 total > 0，则使用虚拟机资源合计 / 宿主机总量估算；
如果 total <= 0，则使用实时 usage。
```

该逻辑用于避免 Agent 未采集到使用率时页面完全空白，但也意味着宿主机页面在 `usage == 0` 时可能显示 VM 合计估算值，而总览页直接使用后端返回的宿主机实时使用率。

宿主机监控弹窗调用 `/api/metrics/hosts/{agentId}`，展示 CPU、内存、逻辑磁盘占用率、磁盘 I/O 和网络吞吐量。网络吞吐量图同时展示流入、流出和按同一时间点 `流入 + 流出` 计算的平均带宽。趋势数据来自数据库指标样本，不直接读取运行态缓存。

## 十二、已知限制和注意事项

后端同步时：

- 宿主机列表的 `address` 优先使用 Agent 返回的 `hostAddress`。
- 如果 Agent 未返回实际 IP，则回退为 Agent 登记的 `endpoint`。


1. 宿主机状态由 `/v1/host` 成功返回决定，Agent 失败时主要通过任务失败、Agent 同步状态和告警体现。
2. CPU 使用率采样窗口为 1 秒，与虚拟机 CPU 使用率采样窗口保持一致。
3. 存储容量只统计宿主机本机当前挂载的本地磁盘文件系统，不依赖 libvirt storage pool。
4. NFS、iSCSI、Ceph 等远端或分布式文件系统不计入宿主机本机存储；如需查看这些资源，应通过对应存储池或独立存储监控查看。
5. 宿主机页面的百分比兜底逻辑可能导致它与总览页面在 `usage == 0` 时显示不一致。
6. 宿主机级磁盘 I/O 通过 `/proc/diskstats` 按整盘设备聚合，网络吞吐通过 `/proc/net/dev` 排除 `lo` 后聚合。
