# Agent 命令超时与临时目录说明

本文档说明 KVM 宿主机 `/tmp` 下常见的 `go-build*`、`libguestfs*` 临时目录来源，以及 `COMMAND_TIMEOUT_SECONDS` 对 Agent 外部命令的影响范围。

## 1. 现象

在 KVM 宿主机上查看 `/tmp` 时，可能看到类似目录：

```bash
go-build1033365056
go-build1268816430
libguestfsztkkWV
libguestfs100hfE
```

这些目录通常不是项目代码主动创建的业务目录，而是 Go 工具链和 libguestfs 工具在执行过程中创建的临时工作目录。

## 2. `go-build*` 目录来源

`go-build*` 来自 `go run`。

项目 README 的开发启动示例包含：

```bash
go run cmd/server/main.go
go run cmd/agent/main.go
```

`go run` 会先把源码临时编译成可执行文件，再运行该临时可执行文件。Go 工具链默认会在系统临时目录下创建 `go-build*` 工作目录。

常见残留原因：

- 服务长期通过 `go run` 方式运行，临时编译目录仍被运行中的进程使用。
- `go run` 进程被强制终止，Go 工具链没有机会完成清理。
- 系统 `/tmp` 清理策略不是实时执行，目录会保留到下次周期清理或重启后清理。

生产环境建议：

- 不要长期使用 `go run` 运行后端或 Agent。
- 先编译二进制，再通过 systemd、Supervisor 或容器运行编译产物。

## 3. `libguestfs*` 目录来源

`libguestfs*` 来自 libguestfs 相关命令，当前项目主要通过 Agent 执行：

```bash
virt-df --csv
virt-df --csv -d <vm>
virt-filesystems --csv -d <vm> --all --long
```

这些命令用于采集虚拟机客户机文件系统的磁盘已用空间，并把文件系统使用量归属到具体虚拟磁盘。

项目触发链路：

- 后端低频 full 深度刷新会触发 Agent full VM 列表采集。
- 手动 `POST /api/refresh` 会触发 full 全量刷新。
- 单台 VM 刷新会触发该 VM 的完整运行态采集。
- VM 设备、XML、介质、快照恢复等操作完成后，可能触发定向 VM 刷新。

采集行为：

- Agent full 列表采集会优先执行一次全局 `virt-df --csv`。
- 单台 VM 刷新会执行 `virt-df --csv -d <vm>`。
- 每台 VM 仍会执行 `virt-filesystems --csv -d <vm> --all --long` 解析分区、LVM、PV、VG、LV 拓扑。
- Agent 会为 `virt-df` 和 `virt-filesystems` 默认注入 `LIBGUESTFS_BACKEND=direct`，除非 Agent 进程环境已经显式设置该变量。

## 4. `COMMAND_TIMEOUT_SECONDS` 作用范围

`COMMAND_TIMEOUT_SECONDS` 是 Agent 外部命令的默认超时配置，默认值为 `30` 秒。

该配置只作用于 Agent 内部通过 `p.output(...)` 执行的命令。它是单条外部命令的超时时间，不是一次 HTTP 请求、一次刷新任务或一次 full 采集的总超时时间。

后端等待 Agent 同步接口返回的 HTTP 超时由后端环境变量控制：

- `RUNTIME_SYNC_FAST_TIMEOUT_SECONDS`：fast 同步默认 `12` 秒。
- `RUNTIME_SYNC_FULL_TIMEOUT_SECONDS`：full 同步和单台 VM full 刷新默认 `60` 秒。

### 4.1 受影响的常见操作

宿主机信息采集：

- `hostname`
- `virsh version`
- `virsh nodeinfo`
- `ip`
- `df`
- `/proc/stat`
- `/proc/meminfo`

VM 列表、fast 刷新和 full 刷新：

- `virsh list`
- `virsh domstate`
- `virsh dumpxml`
- `virsh domstats`
- `virsh dommemstat`
- `virsh domblkinfo`
- `virsh domifaddr`
- `virsh qemu-agent-command`
- `virt-df`
- `virt-filesystems`

VM 电源和快照操作：

- `virsh start`
- `virsh shutdown`
- `virsh suspend`
- `virsh resume`
- `virsh reboot`
- `virsh reset`
- `virsh destroy`
- `virsh snapshot-list`
- `virsh snapshot-create-as`
- `virsh snapshot-revert`
- `virsh snapshot-delete`

VM 配置和设备操作：

- `virsh define`
- `virsh setvcpus`
- `virsh setmem`
- `virsh setmaxmem`
- `virsh desc`
- `virsh attach-disk`
- `virsh blockresize`
- `qemu-img info`
- `qemu-img resize`

存储池、网络池和宿主机接口的常规操作：

- `virsh pool-list`
- `virsh pool-info`
- `virsh pool-refresh`
- `virsh vol-list`
- `virsh vol-delete`
- `virsh net-list`
- `virsh net-info`
- `virsh iface-list`
- `virsh iface-dumpxml`
- `virsh iface-define`
- `ip link`
- `nmcli`

### 4.2 不受该配置影响的操作

以下操作使用 `storageOutput(...)` 或单独的迁移超时，不直接受 `COMMAND_TIMEOUT_SECONDS` 控制：

- 创建 VM 时的 `virt-install --print-xml --dry-run`。
- 创建 VM、XML 创建 VM 中部分 `virsh define` 调用。
- 存储卷克隆中的 `virsh vol-clone`。
- 跨存储池克隆中的 `qemu-img convert`。
- ISO 上传中的 `virsh vol-upload`。
- 迁移相关的 `ssh`、`scp`、`virsh migrate` 等命令，多数使用迁移专用超时。

## 5. 为什么 `libguestfs*` 不会自动清理

正常情况下，libguestfs 命令退出时会尝试清理自己的临时目录。

但当前 Agent 对 `virt-df` 和 `virt-filesystems` 也使用 `COMMAND_TIMEOUT_SECONDS`。如果命令超过超时时间，Go 的 `exec.CommandContext` 会终止该外部命令。命令被终止后，libguestfs 可能没有机会执行退出清理逻辑，因此 `/tmp/libguestfs*` 会残留。

残留更容易出现于以下场景：

- `COMMAND_TIMEOUT_SECONDS` 设置过短。当前默认值已调整为 `30` 秒；如果仍不足以完成 libguestfs appliance 启动和磁盘探测，可继续调大。
- 虚拟机磁盘较多、镜像较大、宿主机 I/O 忙或 libguestfs 首次启动较慢。
- 客户机文件系统异常、分区表不可识别、LVM 拓扑复杂。
- full 刷新同时涉及多台 VM，每台 VM 都需要执行 `virt-filesystems`。
- 命令被超时、系统信号或人工 kill 中断。

另外，Linux 的 `/tmp` 通常不是创建后立即自动清理。是否清理由发行版、`systemd-tmpfiles`、重启策略或自定义清理任务决定。

## 6. 如何确认是否仍有进程占用

清理前先确认没有相关进程正在运行：

```bash
ps -ef | egrep 'virt-df|virt-filesystems|libguestfs|supermin' | grep -v grep
```

查看 Agent 日志中是否存在超时或 libguestfs 失败：

```bash
grep -i "virt-df\|virt-filesystems\|timed out\|guest filesystem" /data/agent/agent.log
```

查看 `/tmp` 下残留目录的时间分布：

```bash
ll -t /tmp | grep libguestfs
```

如果目录时间与后端 `RUNTIME_DEEP_SYNC_INTERVAL` 的 full 刷新周期接近，通常说明它们来自低频深度刷新。

## 7. 安全清理建议

确认没有相关进程占用后，可以清理较旧的残留目录：

```bash
find /tmp -maxdepth 1 -type d -name 'libguestfs*' -mmin +60 -exec rm -rf -- {} +
find /tmp -maxdepth 1 -type d -name 'go-build*' -mmin +60 -exec rm -rf -- {} +
```

如果 Agent 正在执行 full 刷新或单台 VM 刷新，不建议立即删除最新的 `libguestfs*` 目录。

## 8. 长期治理建议

### 8.1 调整 Agent 命令超时

如果宿主机上频繁出现 `libguestfs*` 残留，优先把 Agent 的超时调大：

```bash
COMMAND_TIMEOUT_SECONDS=30
```

磁盘较多、I/O 慢或 VM 数量较多时可考虑：

```bash
COMMAND_TIMEOUT_SECONDS=60
```

调整后需要重启 Agent 生效。

### 8.2 调整 full 深度刷新频率

后端 full 深度刷新由 `RUNTIME_DEEP_SYNC_INTERVAL` 控制，默认 `10m`。

如果不需要频繁刷新真实磁盘使用率，可以调大间隔：

```bash
RUNTIME_DEEP_SYNC_INTERVAL=30m
```

如需关闭低频 full 深度刷新：

```bash
RUNTIME_DEEP_SYNC_INTERVAL=0
```

关闭后，手动 `POST /api/refresh` 和单台 VM 刷新仍可触发完整采集。

### 8.3 避免生产环境长期使用 `go run`

生产环境建议使用编译后的二进制运行 Agent：

```bash
cd /data/agent
go build -o kvm-agent ./cmd/agent
./kvm-agent
```

这样可以避免长期运行 `go run` 产生和持有 `/tmp/go-build*` 临时目录。

### 8.4 增加系统清理任务

可以结合服务器运维策略增加周期清理任务。清理前必须确认没有相关进程占用。

示例：

```bash
ps -ef | egrep 'virt-df|virt-filesystems|libguestfs|supermin|go run' | grep -v grep
find /tmp -maxdepth 1 -type d -name 'libguestfs*' -mtime +1 -exec rm -rf -- {} +
find /tmp -maxdepth 1 -type d -name 'go-build*' -mtime +1 -exec rm -rf -- {} +
```

## 9. 关联文档

- `docs/vm-info-collection.md`
- `docs/frontend-refresh-functions.md`
- `docs/host-info-collection.md`
- `README.md`
