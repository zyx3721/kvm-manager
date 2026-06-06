# DNS 与 NAT / 桥接能力优化实施记录

本文档记录“第二点：DNS 配置方案”和“第三点：NAT / 桥接能力方案”的分析、实施过程、当前能力边界与后续补充说明。

## 一、背景

在接口创建与网络池创建能力中，原先存在几个缺口：

1. `virsh iface-define` 只负责 libvirt interface XML，不负责写入 DNS。
2. 创建 NAT / ROUTE 网络池前，没有显式确认宿主机 IPv4 转发是否已启用。
3. 创建 BRIDGE 网络池时，需要确认桥接设备真实存在。
4. 只通过 libvirt 创建 bridge interface 时，并不会自动把 CentOS/RHEL 的 `ifcfg` 配置写好，也不会自动把物理网卡加入 bridge。

因此本轮优化拆成两个方向：

- DNS 配置采用方案 B：前端按 DNS 输入内容决定是否写入宿主机系统 DNS。
- NAT / 桥接能力增强：创建前做宿主机环境校验，但不自动重启网络服务。

## 二、DNS 配置方案 B

### 2.1 选择方案 B 的原因

`virsh iface-define` 的 interface XML 不能直接写 DNS。

DNS 属于宿主机系统网络配置层，常见入口是：

- NetworkManager：`nmcli connection modify`
- CentOS/RHEL network-scripts：`/etc/sysconfig/network-scripts/ifcfg-*`

因此最终选择方案 B：在创建接口表单中提供 DNS 地址输入，填写 DNS 后再写入系统配置。

### 2.2 前端入口

接口创建弹窗中，IPv4 静态配置卡片提供：

- `DNS1`
- `添加DNS2地址` 按钮

默认只显示 DNS1。DNS1 为空时不写入系统 DNS；填写 DNS1 后会写入系统 DNS，并校验 DNS 必须是合法 IPv4 或 IPv6 地址。

如果需要配置 DNS2，点击 `添加DNS2地址` 后输入。DNS2 为空时会被忽略，等价于只配置 DNS1。

主要实现文件：

- `frontend/src/features/host-interfaces/components/AddressPanel.tsx`
- `frontend/src/features/host-interfaces/components/HostInterfaceCreateDialog.tsx`
- `frontend/src/features/host-interfaces/HostInterfacesPage.tsx`
- `frontend/src/features/host-interfaces/ipAddressValidation.ts`
- `frontend/src/lib/api.ts`

### 2.3 后端与 Agent 字段

创建宿主机接口请求中包含：

```json
{
  "applySystemConfig": true,
  "dnsServers": ["10.22.50.5", "8.8.8.8"]
}
```

后端只做参数转发与空白裁剪，真正写入由 Agent 执行。

主要实现文件：

- `backend/api/router/host_interfaces.go`
- `backend/pkg/agent/client.go`
- `agent/internal/kvm/types.go`

### 2.4 Agent 写入策略

Agent 写入 DNS 的策略如下：

1. 去重并清理空白 DNS 地址。
2. 优先检测 `nmcli` 是否存在。
3. 若 `nmcli` 可用，则按 IPv4 / IPv6 分组写入：

```bash
nmcli connection modify <interface> ipv4.ignore-auto-dns yes ipv4.dns "<dns-list>"
nmcli connection modify <interface> ipv6.ignore-auto-dns yes ipv6.dns "<dns-list>"
```

4. 若 `nmcli` 不可用或修改失败，则回退到 ifcfg：

```text
/etc/sysconfig/network-scripts/ifcfg-<interface>
```

5. 写入 ifcfg 前创建备份：

```text
ifcfg-<interface>.kvm-manager.<timestamp>.bak
```

6. 删除旧的 `DNS1`、`DNS2` 等字段后，写入新的 DNS 字段。
7. 如果 DNS 写入成功，删除本次创建的备份文件。
8. 如果 DNS 写入失败，保留备份文件用于人工恢复。

主要实现文件：

- `agent/internal/kvm/host_interface_system_config.go`

### 2.5 当前边界

DNS 写入不会自动执行：

```bash
systemctl restart network
systemctl restart NetworkManager
nmcli con up <connection>
nmcli con down <connection>
```

这样做是为了避免远程管理宿主机时，因为网络服务重启导致 Agent 失联。

## 三、NAT / ROUTE 网络池优化

### 3.1 创建前检查 IPv4 转发

NAT / ROUTE 网络池需要宿主机具备 IPv4 转发能力。创建前 Agent 会执行：

```bash
sysctl -n net.ipv4.ip_forward
```

只有返回值为 `1` 时才允许继续创建。

如果未启用，会返回错误，前端提示用户先配置：

```text
net.ipv4.ip_forward=1
```

### 3.2 不自动修改 sysctl 的原因

本次没有自动写入：

```text
/etc/sysctl.conf
/etc/sysctl.d/*.conf
```

也没有自动执行：

```bash
sysctl -w net.ipv4.ip_forward=1
```

原因是 IP 转发属于宿主机全局网络策略，可能影响安全边界、路由行为和现有业务流量。当前项目只在创建前做硬校验，把是否启用交给管理员显式配置。

### 3.3 主要实现文件

- `agent/internal/kvm/virsh_network_environment.go`
- `agent/internal/kvm/virsh_pools.go`
- `frontend/src/features/network-pools/NetworkPoolsPage.tsx`
- `backend/api/router/pools.go`
- `backend/pkg/agent/pool_types.go`

## 四、BRIDGE 网络池优化

### 4.1 创建前检查桥接设备

BRIDGE 网络池创建前，Agent 会检查表单填写的 bridge 设备：

```bash
test -d /sys/class/net/<bridge>/bridge
```

只有设备存在且确认为 Linux bridge 时，才继续定义 libvirt 网络池。

这样可以避免把普通物理网卡、错误名称或不存在的设备写入 libvirt 网络池。

### 4.2 当前边界

该流程只负责：

- 校验 bridge 设备存在
- 创建 libvirt BRIDGE 网络池

它不会自动执行：

- 创建 Linux bridge 设备
- 把物理网卡加入 bridge
- 迁移物理网卡 IP / 网关 / DNS
- 重启系统网络服务

## 五、桥接设备与 ifcfg 备份

接口创建不再提供“写入桥接 ifcfg 配置”开关，也不再通过 Agent 主动重写 bridge 或物理设备的 ifcfg 内容。

当前策略是：

- 用户在创建 `bridge` 类型接口时，可以选择一个已有设备作为桥接设备。
- Agent 会把该设备写入 libvirt interface XML。
- 具体桥接创建、物理设备下挂和 network-scripts 配置调整交给 `virsh iface-define` / `virsh iface-start` 处理。

典型 XML 片段：

```xml
<bridge stp="on" delay="0">
  <interface name="em1" type="ethernet"/>
</bridge>
```

### 5.1 执行前备份

创建 `bridge` 类型接口且选择了桥接设备时，Agent 会在执行 `virsh iface-define` 前检查：

```text
/etc/sysconfig/network-scripts/ifcfg-<bridge>
/etc/sysconfig/network-scripts/ifcfg-<device>
```

如果文件已存在，会先生成备份：

```text
ifcfg-<name>.YYYYMMDDHHMMSS.bak
```

如果文件不存在，则不会生成备份，也不会额外创建空配置文件。

### 5.2 与 DNS 写入的关系

DNS 写入由前端根据 DNS 输入内容设置 `applySystemConfig`。

填写 DNS 时：

- Agent 优先尝试 `nmcli connection modify`。
- 如果 `nmcli` 不可用或写入失败，则回退到 ifcfg DNS 写入。
- 回退写入 DNS 前，会备份目标 `ifcfg-<interface>`。

## 六、静态 IP / 网关 / DNS 校验

本轮优化过程中也补充了静态地址相关校验。

### 6.1 IP 与子网重复校验

创建静态 IPv4 / IPv6 时，前端和 Agent 都会检查：

- 是否已有接口使用相同 IP。
- 是否已有接口使用重复或重叠子网。

重复 IP 和重复子网都会被拒绝，避免 ARP / NDP 冲突和同宿主机路由歧义。

如果创建 bridge 接口时选择了已有物理设备作为绑定设备，则该物理设备原有 IP 和子网允许迁移到新 bridge。

这种情况下，前端和 Agent 会跳过该绑定设备自身的重复 IP / 子网冲突判断，但仍会校验其他接口，避免多个非源接口重复占用同一地址或子网。

### 6.2 绑定设备重复校验

创建接口并选择绑定设备时，前端和 Agent 都会检查该设备是否已作为其他接口的下挂设备。

如果已有接口的 `bridgeDevice` 等于本次选择的设备，则拒绝创建，避免同一个物理设备被重复加入多个接口。

### 6.3 网关同子网校验

静态 IP 配置时，网关必须满足：

- 网关是合法 IP 地址。
- 网关与接口地址属于同一 CIDR 子网。
- 网关不能等于接口地址本身。

示例：

```text
IP:      10.22.51.48/24
Gateway: 10.22.52.254
```

上述配置会被拒绝，因为网关不在 `10.22.51.0/24` 子网内。

### 6.4 DNS 校验

填写 DNS 时：

- DNS 必须是合法 IPv4 或 IPv6 地址。
- 空值会被裁剪。
- 重复值会被去重。
- DNS1 和 DNS2 都为空时不写入系统 DNS。

## 七、操作建议

### 7.1 创建 NAT / ROUTE 网络池前

建议先在宿主机确认：

```bash
sysctl -n net.ipv4.ip_forward
```

如果不是 `1`，由管理员按环境策略配置持久化 sysctl。

### 7.2 创建 BRIDGE 网络池前

建议先确认 bridge 设备已经存在：

```bash
ip link show br0
test -d /sys/class/net/br0/bridge
```

如果还没有 bridge，应先在“接口”页面创建 bridge，并按需选择要桥接的已有设备。

### 7.3 创建 bridge 绑定设备前

建议确认：

- Agent 运行用户有权限读取 `/etc/sysconfig/network-scripts` 并写入备份文件。
- 物理网卡当前 IP 迁移到 bridge 后，远程连接策略已评估。
- 已准备好通过控制台或带外管理恢复网络。

### 7.4 写入后生效方式

平台不会自动重启网络。管理员可根据宿主机实际网络栈手动选择生效方式，例如：

```bash
systemctl restart network
```

或使用 NetworkManager 对应命令。

在远程宿主机上执行网络重启前，应确保有控制台或带外管理通道。

## 八、权限与迁移说明

本轮优化没有新增数据库字段，不需要新增迁移。

权限方面复用已有权限点：

- 宿主机接口创建与系统配置写入：`host.interfaces.manage`
- 网络池创建：网络池管理权限

没有新增独立 RBAC 权限点，原因是这些能力仍属于现有网络池和宿主机接口创建动作，不新增独立页面入口或后台任务入口。

## 九、验证记录

实施过程中执行过以下验证：

```bash
cd frontend
npm run type-check
```

```bash
cd agent
go test ./internal/kvm ./api/router
```

```bash
cd backend
go test ./api/router ./pkg/agent
swag init -g cmd/server/main.go -o docs
```

```bash
codegraph sync
```

其中 `swag init` 可能出现 backend 根目录无 Go 文件的 warning，但只要最终生成 `backend/docs/docs.go`、`backend/docs/swagger.json` 和 `backend/docs/swagger.yaml`，即表示 Swagger 文档生成完成。

## 十、后续可补充方向

后续如果要继续增强，可以考虑：

1. 增加 NetworkManager keyfile 写入能力，覆盖不使用 network-scripts 的系统。
2. 增加“网络配置预览”步骤，在真正写入 ifcfg 前展示将被修改的字段。
3. 增加网络配置回滚入口，基于 `.kvm-manager.<timestamp>.bak` 选择性恢复。
4. 增加桥接配置生效检查，只读检测 bridge 是否已持有目标 IP、物理设备是否已 enslave 到 bridge。
5. 增加发行版识别，针对 CentOS/RHEL、Ubuntu Netplan、Debian ifupdown 等做分支提示。
6. 增加 sysctl 持久化配置的可选开关，但默认仍建议保持手动确认。
