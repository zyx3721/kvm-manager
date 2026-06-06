# 前端 Select 下拉方向配置说明

本文记录当前前端项目中所有 Select / listbox 类下拉控件的展开方向配置，便于后续统一交互规范和排查弹窗、卡片、滚动容器中的遮挡问题。

## 一、统一 SelectMenu 行为

统一下拉组件位于 `frontend/src/components/kvm/SelectMenu.tsx`。

`SelectMenu` 支持以下方向配置：

| 配置 | 行为 |
| :-: | :-: |
| `placement="top"` | 固定向上展开 |
| `placement="bottom"` | 固定向下展开 |
| 未配置或 `placement="auto"` | 根据浏览器视口上下剩余空间自动选择向上或向下展开 |

自动方向判断依据是触发器相对浏览器视口的上下剩余空间，而不是当前卡片、弹窗内容区或滚动容器的高度。

## 二、固定显示上方

| 页面 / 场景 | 配置项 | 文件位置 |
| :-: | :-: | :-: |
| 存储池创建 | 格式 | `frontend/src/features/storage-pools/components/StoragePoolCreateDialog.tsx:89` |
| 宿主机接口创建 | IPv4/IPv6 模式 | `frontend/src/features/host-interfaces/components/AddressPanel.tsx:45` |
| VM 克隆 | 磁盘存储池 | `frontend/src/features/vms/components/edit/ClonePanel.tsx:256` |
| VM 克隆 | 介质策略 | `frontend/src/features/vms/components/edit/ClonePanel.tsx:288` |
| VM 编辑新增磁盘 | 存储池 | `frontend/src/features/vms/components/edit/NewDiskCard.tsx:104` |
| VM 编辑新增磁盘 | 总线 | `frontend/src/features/vms/components/edit/NewDiskCard.tsx:119` |
| VM 编辑新增磁盘 | 格式 | `frontend/src/features/vms/components/edit/NewDiskCard.tsx:169` |
| VM 创建额外数据盘 | 存储池 | `frontend/src/features/vms/components/VMCreateDialog.tsx:388` |
| VM 创建额外数据盘 | 磁盘格式 | `frontend/src/features/vms/components/VMCreateDialog.tsx:389` |
| VM 创建额外数据盘 | 磁盘总线 | `frontend/src/features/vms/components/VMCreateDialog.tsx:390` |

## 三、固定显示下方

| 页面 / 场景 | 配置项 | 文件位置 |
| :-: | :-: | :-: |
| 网络池创建 | 网络类型 | `frontend/src/features/network-pools/NetworkPoolsPage.tsx:405` |
| 存储卷克隆 | 格式 | `frontend/src/features/storage-pools/components/VolumeCloneDialog.tsx:66` |
| 登录页 | 登录方式 | `frontend/src/features/auth/LoginPage.tsx:290` |
| 宿主机监控弹窗 | 时间范围 | `frontend/src/components/kvm/HostTrendDialog.tsx:121` |
| 通用导出弹窗 | 扩展名 | `frontend/src/components/kvm/ExportDialog.tsx:105` |
| VM 监控弹窗 | 时间范围 | `frontend/src/features/vms/components/VMMonitorDialog.tsx:262` |
| 总览页 | 在线虚拟机资源选择 | `frontend/src/features/dashboard/DashboardPage.tsx:519` |
| 总览页 | 在线宿主机资源选择 | `frontend/src/features/dashboard/DashboardPage.tsx:519` |
| 快照页 | 创建快照虚拟机选择 | `frontend/src/features/snapshots/SnapshotsPage.tsx:153` |

## 四、自动显示上方或下方

| 页面 / 场景 | 配置项 | 文件位置 |
| :-: | :-: | :-: |
| 忘记密码 | 通知媒介 | `frontend/src/features/auth/ForgotPasswordPage.tsx:318` |
| 网络池页 | 宿主机选择 | `frontend/src/features/network-pools/NetworkPoolsPage.tsx:159` |
| 存储池页 | 宿主机选择 | `frontend/src/features/storage-pools/StoragePoolsPage.tsx:168` |
| 快照页 | Agent 宿主机筛选 | `frontend/src/features/snapshots/SnapshotsPage.tsx:155` |
| 快照页 | 虚拟机筛选 | `frontend/src/features/snapshots/SnapshotsPage.tsx:160` |
| 通知媒介配置 | 邮件内容类型、飞书消息类型、企业微信消息类型、钉钉消息类型；选项悬浮提示固定显示在右侧 | `frontend/src/features/settings/components/NotificationSettingsPanel.tsx` |
| 存储池详情 | 添加镜像格式 | `frontend/src/features/storage-pools/components/StoragePoolDetailDialog.tsx:501` |
| 存储池详情 | 容量单位 | `frontend/src/features/storage-pools/components/StoragePoolDetailDialog.tsx:514` |
| VM 工具栏 | 宿主机筛选 | `frontend/src/features/vms/components/VMToolbar.tsx:84` |
| VM 批量操作 | 选择操作 | `frontend/src/features/vms/components/VMBulkActionBar.tsx:46` |
| VM 创建 XML | 宿主机 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 宿主机 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 操作系统类型 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | CPU 模式 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 存储池 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 磁盘格式 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 磁盘总线 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | ISO 池 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | ISO 镜像 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | ISO 总线 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 模板创建 | 虚拟机模板选择 | `frontend/src/features/vms/components/create/TemplateVMCreatePanel.tsx` |
| VM 磁盘模板创建 | 模板存储池 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:260` |
| VM 磁盘模板创建 | 模板文件 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:271` |
| VM 磁盘模板创建 | 目标存储池 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:280` |
| VM 磁盘模板创建 | 磁盘总线 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:300` |
| VM 磁盘模板创建 | ISO 池 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:327` |
| VM 磁盘模板创建 | ISO 镜像 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:335` |
| VM 磁盘模板创建 | 光驱总线 | `frontend/src/features/vms/components/create/DiskTemplateCreatePanel.tsx:344` |
| VM 创建 | 网络池 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 网卡模型 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 固件 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 创建 | 控制台 | `frontend/src/features/vms/components/create/CreatePanels.tsx` |
| VM 迁移 | 目标宿主机 | `frontend/src/features/vms/components/VMMigrateDialog.tsx:150` |
| VM 迁移 | 迁移类型 | `frontend/src/features/vms/components/VMMigrateDialog.tsx:153` |
| VM 编辑设备 | 网卡网络池 | `frontend/src/features/vms/components/edit/DevicesPanel.tsx:260` |
| VM 编辑设备 | 新增网卡网络池 | `frontend/src/features/vms/components/edit/DevicesPanel.tsx:295` |
| VM 编辑设备 | 新增网卡模型 | `frontend/src/features/vms/components/edit/DevicesPanel.tsx:309` |
| VM 编辑介质 | 目标光驱 | `frontend/src/features/vms/components/edit/MediaPanel.tsx:206` |
| VM 编辑介质 | 存储池 | `frontend/src/features/vms/components/edit/MediaPanel.tsx:221` |
| VM 编辑介质 | ISO 文件 | `frontend/src/features/vms/components/edit/MediaPanel.tsx:243` |
| VM 克隆 | 网卡网络池 | `frontend/src/features/vms/components/edit/ClonePanel.tsx:228` |
| VM 资源编辑 | 当前 CPU 分配 | `frontend/src/features/vms/components/edit/ResourcesPanel.tsx:175` |
| VM 资源编辑 | 最大 CPU 分配 | `frontend/src/features/vms/components/edit/ResourcesPanel.tsx:184` |
| VM 资源编辑 | 当前内存分配 | `frontend/src/features/vms/components/edit/ResourcesPanel.tsx:200` |
| VM 资源编辑 | 最大内存分配 | `frontend/src/features/vms/components/edit/ResourcesPanel.tsx:209` |

## 五、手写 listbox 控件说明

以下控件没有使用统一 `SelectMenu`，但视觉上仍属于 Select / listbox 类控件：

| 组件 | 方向行为 | 文件位置 |
| :-: | :-: | :-: |
| `LoginProviderSelect` | 固定向下 | `frontend/src/features/auth/LoginPage.tsx:290` |
| `HostTrendDialog` 的 `RangePicker` | 固定向下 | `frontend/src/components/kvm/HostTrendDialog.tsx:121` |
| `VMMonitorDialog` 的 `RangePicker` | 固定向下 | `frontend/src/features/vms/components/VMMonitorDialog.tsx:262` |
| `DashboardPage` 的 `ResourceSelect` | 固定向下 | `frontend/src/features/dashboard/DashboardPage.tsx:519` |
| `SnapshotsPage` 的 `SnapshotVMSelect` | 固定向下 | `frontend/src/features/snapshots/SnapshotsPage.tsx:153` |
| `VMToolbar` 的 `VMColumnPicker` | 菜单通过 portal 挂载到 `document.body`，并根据浏览器视口上下剩余空间自动选择方向 | `frontend/src/features/vms/components/VMToolbar.tsx` |
| `VMEditControls` 的 `NumberSelect` | 菜单保留在组件内部，打开时提升当前卡片层级，并根据浏览器视口上下剩余空间自动选择方向 | `frontend/src/features/vms/components/VMEditControls.tsx:4` |

## 六、后续统一建议

- 新增或修改下拉控件时，优先复用 `SelectMenu`，避免继续增加手写 listbox。
- 如果需求是“随着当前卡片布局高度自动显示上方或下方”，现有 `SelectMenu` 还不满足该语义；当前自动逻辑基于浏览器视口，而不是卡片或滚动容器。
- 若要支持卡片 / 弹窗内容区内的智能方向，应为 `SelectMenu` 增加容器边界感知能力，例如传入 boundary element，或基于最近滚动容器计算可用空间。
