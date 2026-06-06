package router

// swaggerHealth godoc
// @Summary 健康检查
// @Description 检查后端服务和数据库连接状态。
// @Tags health
// @Produce json
// @Success 200 {object} healthResponse
// @Failure 503 {object} errorResponse
// @Router /api/health [get]
func swaggerHealth() {}

// swaggerLogin godoc
// @Summary 登录
// @Description 使用本地账号或已启用的外部认证方式登录，返回会话 Token 和用户信息。默认空库管理员为 admin / 123456。
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "登录信息"
// @Success 200 {object} domain.Session
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/auth/login [post]
func swaggerLogin() {}

// swaggerPublicAuthProviders godoc
// @Summary 获取登录页可用认证方式
// @Description 返回已启用的外部认证方式；本地账号登录始终可用。
// @Tags auth
// @Produce json
// @Success 200 {object} publicAuthProviderListResponse
// @Failure 500 {object} errorResponse
// @Router /api/auth/providers [get]
func swaggerPublicAuthProviders() {}

// swaggerPublicSystemBaseConfig godoc
// @Summary 获取公开基础配置
// @Description 返回登录页、启动页、侧边栏品牌区和浏览器标题展示所需的网站名称、品牌名称和图标配置。
// @Tags settings
// @Produce json
// @Success 200 {object} domain.SystemBaseConfig
// @Router /api/public/base-config [get]
func swaggerPublicSystemBaseConfig() {}

// swaggerPasswordResetCaptcha godoc
// @Summary 获取找回密码图形验证码
// @Description 返回登录页找回密码流程使用的算式验证码与签名 Token。图形验证码有效期由基础配置控制。
// @Tags auth
// @Produce json
// @Success 200 {object} passwordResetCaptchaResponse
// @Failure 500 {object} errorResponse
// @Router /api/auth/password-reset/captcha [get]
func swaggerPasswordResetCaptcha() {}

// swaggerPasswordResetVerify godoc
// @Summary 校验找回密码用户名与图形验证码
// @Description 校验本地账号和图形验证码，通过后返回已启用找回密码用途的通知媒介和短期校验 Token。
// @Tags auth
// @Accept json
// @Produce json
// @Param body body passwordResetVerifyRequest true "用户名与图形验证码"
// @Success 200 {object} passwordResetChannelListResponse
// @Failure 400 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/auth/password-reset/verify [post]
func swaggerPasswordResetVerify() {}

// swaggerPasswordResetSendCode godoc
// @Summary 发送找回密码验证码
// @Description 使用已启用找回密码用途的 Webhook、邮件、飞书、企业微信或钉钉媒介发送找回密码验证码。发送前必须携带用户名与图形验证码校验得到的短期 Token，并校验验证邮箱与账号配置邮箱一致；邮件媒介会发送到账号配置邮箱，机器人媒介无需额外接收信息。验证码有效期、发送冷却和频率限制统计窗口由基础配置控制。
// @Tags auth
// @Accept json
// @Produce json
// @Param body body passwordResetSendCodeRequest true "发送配置"
// @Success 200 {object} passwordResetSendCodeResponse
// @Failure 400 {object} errorResponse
// @Failure 429 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/auth/password-reset/send-code [post]
func swaggerPasswordResetSendCode() {}

// swaggerPasswordResetConfirm godoc
// @Summary 确认找回密码并重置密码
// @Description 校验找回密码验证码后重置本地账号密码，并清理该用户已有会话。
// @Tags auth
// @Accept json
// @Produce json
// @Param body body passwordResetConfirmRequest true "验证码与新密码"
// @Success 200 {object} messageResponse
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/auth/password-reset/confirm [post]
func swaggerPasswordResetConfirm() {}

// swaggerMe godoc
// @Summary 获取当前用户
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} meResponse
// @Failure 401 {object} errorResponse
// @Router /api/auth/me [get]
func swaggerMe() {}

// swaggerLogout godoc
// @Summary 注销当前会话
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Router /api/auth/logout [post]
func swaggerLogout() {}

// swaggerChangePassword godoc
// @Summary 修改当前用户密码
// @Description 校验旧密码后更新当前登录用户密码。新密码至少 6 个字符，且不能与旧密码相同。
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePasswordRequest true "密码信息"
// @Success 200 {object} messageResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/auth/password [put]
func swaggerChangePassword() {}

// swaggerDashboardSummary godoc
// @Summary 获取仪表盘汇总
// @Description 从运行态缓存和日志告警数据聚合仪表盘信息。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.DashboardSummary
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/dashboard/summary [get]
func swaggerDashboardSummary() {}

// swaggerListHosts godoc
// @Summary 获取宿主机列表
// @Description 从后端运行态缓存读取宿主机列表。拥有宿主机、Agent、虚拟机、快照、存储池或网络池相关权限时可作为关联只读数据访问，用于资源页展示、筛选和下拉选择。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Success 200 {object} hostListResponse
// @Failure 401 {object} errorResponse
// @Router /api/hosts [get]
func swaggerListHosts() {}

// swaggerListHostInterfaces godoc
// @Summary 获取宿主机接口
// @Description 后端按 Agent ID 转发到宿主机 Agent，实时读取物理网卡、loopback 与 bridge 接口列表；需要 host.interfaces.read 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Success 200 {object} hostInterfaceListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/host-interfaces/{agentId} [get]
func swaggerListHostInterfaces() {}

// swaggerListHostInterfaceDevices godoc
// @Summary 获取宿主机接口设备候选
// @Description 后端按 Agent ID 转发到宿主机 Agent，读取 libvirt node device 中的 net 设备列表；需要 host.interfaces.read 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Success 200 {object} hostInterfaceDeviceListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/host-interfaces/{agentId}/devices/list [get]
func swaggerListHostInterfaceDevices() {}

// swaggerCreateHostInterface godoc
// @Summary 创建宿主机接口
// @Description 在指定宿主机 Agent 上创建 libvirt interface，支持 bridge 和 ethernet 类型，可选绑定已有设备并配置启动模式、IPv4/IPv6；绑定设备已被其他接口使用时会拒绝创建；静态地址会拒绝重复 IP、重复或重叠子网，以及不在同一子网的网关；bridge 绑定已有设备时会在执行 virsh 前备份已存在的 ifcfg-bridge 和 ifcfg-device；填写 DNS 时 applySystemConfig 为 true，并尝试通过 nmcli 或 ifcfg 写入宿主机 DNS；需要 host.interfaces.manage 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param body body hostInterfaceCreateDocRequest true "接口配置"
// @Success 200 {object} agent.HostInterface
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/host-interfaces/{agentId} [post]
func swaggerCreateHostInterface() {}

// swaggerUpdateHostInterfaceState godoc
// @Summary 修改宿主机接口状态
// @Description 在指定宿主机 Agent 上启动或停止 libvirt interface；需要 host.interfaces.manage 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param name path string true "接口名称"
// @Param body body hostInterfaceStateDocRequest true "接口状态"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/host-interfaces/{agentId}/state/{name} [put]
func swaggerUpdateHostInterfaceState() {}

// swaggerDeleteHostInterface godoc
// @Summary 删除宿主机接口
// @Description 删除指定宿主机 Agent 上已停止的 libvirt interface；需要 host.interfaces.manage 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param name path string true "接口名称"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/host-interfaces/{agentId}/delete/{name} [delete]
func swaggerDeleteHostInterface() {}

// swaggerListStoragePools godoc
// @Summary 获取宿主机存储池
// @Description 后端按 Agent ID 转发到宿主机 Agent，实时读取 libvirt 存储池列表。拥有存储池相关权限或虚拟机相关权限时可作为关联只读数据访问，用于虚拟机创建、编辑、克隆和迁移配置。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Success 200 {object} storagePoolListResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId} [get]
func swaggerListStoragePools() {}

// swaggerCreateStoragePool godoc
// @Summary 创建宿主机存储池
// @Description 在指定宿主机 Agent 上创建 libvirt 存储池，支持 dir、logical、netfs 和 iscsi。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param body body storagePoolCreateDocRequest true "存储池配置"
// @Success 200 {object} agent.StoragePool
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId} [post]
func swaggerCreateStoragePool() {}

// swaggerListISOFiles godoc
// @Summary 获取存储池 ISO 文件
// @Description 读取指定宿主机存储池中的 .iso 文件，供虚拟机编辑窗口介质页选择。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Success 200 {object} isoFileListResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/iso-files/{pool} [get]
func swaggerListISOFiles() {}

// swaggerListStorageVolumes godoc
// @Summary 获取存储池卷列表
// @Description 读取指定宿主机存储池中的卷或光盘镜像列表，供存储池详情弹窗展示。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Success 200 {object} storageVolumeListResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/volumes/{pool} [get]
func swaggerListStorageVolumes() {}

// swaggerCreateStorageVolume godoc
// @Summary 创建存储池卷
// @Description 在指定宿主机存储池中创建 qcow2、qcow、qed 或 raw 格式的存储卷。preallocMetadata 仅在 qcow2 格式下生效，会传递给 virsh vol-create-as --prealloc-metadata。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param body body storageVolumeCreateDocRequest true "存储卷配置"
// @Success 200 {object} agent.StorageVolume
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/volumes/{pool} [post]
func swaggerCreateStorageVolume() {}

// swaggerCloneStorageVolume godoc
// @Summary 克隆存储池卷
// @Description 创建后台任务克隆指定宿主机存储池中的卷；convert 为 true 时通过 qemu-img convert 转换为 raw、qcow、qcow2 或 qed。任务完成或失败后通过 SSE 通知前端。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param body body storageVolumeCloneDocRequest true "克隆配置"
// @Success 202 {object} refreshResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/volumes/{pool}/clone [post]
func swaggerCloneStorageVolume() {}

// swaggerUploadStorageVolume godoc
// @Summary 上传 ISO 到存储池
// @Description 将 multipart 表单中的 ISO 文件保存为后台任务，再上传为指定宿主机存储池卷。表单字段 file 为文件，name 为可选卷名称。任务完成或失败后通过 SSE 通知前端。
// @Tags realtime
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param file formData file true "ISO 文件"
// @Param name formData string false "卷名称"
// @Success 202 {object} refreshResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/volumes/{pool}/upload [post]
func swaggerUploadStorageVolume() {}

// swaggerDeleteStorageVolume godoc
// @Summary 删除存储池卷
// @Description 删除指定宿主机存储池中的卷或光盘镜像。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param name query string true "存储卷名称"
// @Success 200 {object} statusResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/volumes/{pool} [delete]
func swaggerDeleteStorageVolume() {}

// swaggerDeleteStoragePool godoc
// @Summary 删除宿主机存储池
// @Description 删除指定宿主机 Agent 上已停止的 libvirt 存储池定义。运行中的存储池需要先停止后才能删除。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Success 200 {object} statusResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/delete/{pool} [delete]
func swaggerDeleteStoragePool() {}

// swaggerUpdateStoragePoolState godoc
// @Summary 修改宿主机存储池状态
// @Description 在指定宿主机 Agent 上启动或停止 libvirt 存储池。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param body body poolStateUpdateDocRequest true "存储池状态"
// @Success 200 {object} poolStateUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/state/{pool} [put]
func swaggerUpdateStoragePoolState() {}

// swaggerUpdateStoragePoolAutostart godoc
// @Summary 修改宿主机存储池自启动
// @Description 在指定宿主机 Agent 上启用或关闭 libvirt 存储池自启动。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "存储池名称"
// @Param body body poolAutostartUpdateDocRequest true "存储池自启动配置"
// @Success 200 {object} poolAutostartUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/storage-pools/{agentId}/autostart/{pool} [put]
func swaggerUpdateStoragePoolAutostart() {}

// swaggerListNetworkPools godoc
// @Summary 获取宿主机网络池
// @Description 后端按 Agent ID 转发到宿主机 Agent，实时读取 libvirt 网络池列表。拥有网络池相关权限或虚拟机相关权限时可作为关联只读数据访问，用于虚拟机创建、编辑、克隆和迁移配置。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Success 200 {object} networkPoolListResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/network-pools/{agentId} [get]
func swaggerListNetworkPools() {}

// swaggerCreateNetworkPool godoc
// @Summary 创建宿主机网络池
// @Description 在指定宿主机 Agent 上创建 libvirt 网络池，支持 nat、route、isolate 和 bridge；固定地址需要同时启用 DHCP；bridge 类型会检查桥接设备存在。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param body body networkPoolCreateDocRequest true "网络池配置"
// @Success 200 {object} agent.NetworkPool
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/network-pools/{agentId} [post]
func swaggerCreateNetworkPool() {}

// swaggerDeleteNetworkPool godoc
// @Summary 删除宿主机网络池
// @Description 删除指定宿主机 Agent 上已停止的 libvirt 网络池定义。运行中的网络池需要先停止后才能删除。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "网络池名称"
// @Success 200 {object} statusResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/network-pools/{agentId}/delete/{pool} [delete]
func swaggerDeleteNetworkPool() {}

// swaggerUpdateNetworkPoolState godoc
// @Summary 修改宿主机网络池状态
// @Description 在指定宿主机 Agent 上启动或停止 libvirt 网络池。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "网络池名称"
// @Param body body poolStateUpdateDocRequest true "网络池状态"
// @Success 200 {object} poolStateUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/network-pools/{agentId}/state/{pool} [put]
func swaggerUpdateNetworkPoolState() {}

// swaggerUpdateNetworkPoolAutostart godoc
// @Summary 修改宿主机网络池自启动
// @Description 在指定宿主机 Agent 上启用或关闭 libvirt 网络池自启动。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID"
// @Param pool path string true "网络池名称"
// @Param body body poolAutostartUpdateDocRequest true "网络池自启动配置"
// @Success 200 {object} poolAutostartUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/network-pools/{agentId}/autostart/{pool} [put]
func swaggerUpdateNetworkPoolAutostart() {}

// swaggerListVMs godoc
// @Summary 获取虚拟机列表
// @Description 从后端运行态缓存读取虚拟机列表，支持状态、关键词和宿主机过滤。拥有虚拟机、宿主机、Agent 或快照相关权限时可作为关联只读数据访问，用于资源页展示、筛选和快照创建选择。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param status query string false "虚拟机状态，如 running、stopped、paused、error"
// @Param q query string false "关键词，匹配名称、IP 或系统类型"
// @Param hostId query string false "宿主机 Agent ID"
// @Success 200 {object} vmListResponse
// @Failure 401 {object} errorResponse
// @Router /api/vms [get]
func swaggerListVMs() {}

// swaggerCreateVM godoc
// @Summary 创建虚拟机
// @Description 创建后台任务，在指定宿主机 Agent 上创建虚拟机。常规模式会创建系统盘卷，调用 virt-install 生成 XML 后通过 virsh define 定义虚拟机，并校验最大 CPU、最大内存不超过宿主机资源；后端仍兼容 createMode=template 的旧磁盘模板创建模式，会从已有 qcow2、img、raw、qcow 或 qed 模板磁盘克隆系统盘到目标卷名，再按导入磁盘方式定义虚拟机；XML 模式可直接提交完整 libvirt XML，后端和 Agent 会从 XML name 读取虚拟机名称，并校验宿主机、XML 非空、XML 可解析以及名称不重复。当前前端模板菜单使用 /api/vms/{id}/template-create 从已标记的虚拟机模板创建。常规和旧磁盘模板模式支持 CPU/内存、操作系统类型、ISO 镜像、ISO 总线类型、网络池、VNC 控制台、控制台密码、创建后直接启动和固件配置；未选择 ISO 镜像时仍会按 isoBus 创建空 CDROM 设备。任务完成后后端先执行 fast 同步并广播 runtime.updated，让新虚拟机尽快出现在列表中，随后后台延迟 full 同步补齐重字段。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body vmCreateDocRequest true "虚拟机创建配置"
// @Success 202 {object} vmAsyncTaskDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms [post]
func swaggerCreateVM() {}

// swaggerGetVMConsole godoc
// @Summary 查询虚拟机控制台配置
// @Description 查询虚拟机 VNC 控制台类型、端口和是否配置访问密码，不返回密码明文。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmConsoleInfoDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/console [get]
func swaggerGetVMConsole() {}

// swaggerUpdateVMConsole godoc
// @Summary 修改虚拟机控制台配置
// @Description 修改虚拟机的 VNC 控制台密码。运行中虚拟机支持启用或修改密码，并通过 update-device --live --config 同时更新当前会话与持久配置，不支持关闭已启用的密码；已停止虚拟机使用 --config 更新持久配置。开启密码时写入 graphics passwd 属性，关闭时移除密码。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmConsoleUpdateDocRequest true "控制台配置"
// @Success 200 {object} vmConsoleUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/console [put]
func swaggerUpdateVMConsole() {}

// swaggerRefreshVM godoc
// @Summary 刷新单台虚拟机运行态
// @Description 仅刷新指定虚拟机所属宿主机上的当前 VM 信息，会重新读取 Guest Agent OS、domifaddr IP、磁盘明细、CPU/内存使用率和 I/O 速率，并更新运行态缓存；不刷新快照列表。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmSingleRefreshResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/refresh [post]
func swaggerRefreshVM() {}

// swaggerGetVM godoc
// @Summary 获取虚拟机详情
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} domain.VirtualMachine
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/vms/{id} [get]
func swaggerGetVM() {}

// swaggerGetVMConfig godoc
// @Summary 获取虚拟机真实配置
// @Description 后端根据虚拟机所属 Agent 实时读取 libvirt 配置，Agent 使用当前 dumpxml 返回 CPU/内存上下限、介质、磁盘、网卡、自启动和描述。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmConfigDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/config [get]
func swaggerGetVMConfig() {}

// swaggerUpdateVMConfig godoc
// @Summary 修改虚拟机资源配置
// @Description 修改虚拟机描述、vCPU 当前/最大分配和内存当前/最大分配。运行中的虚拟机支持在已预留上限内热扩容当前 CPU 与内存，最大 CPU 和最大内存仍需关机后修改。后端转发到 Agent 执行 virsh setvcpus、setmaxmem、setmem 和 desc；运行中热扩容使用 --live --config。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmConfigUpdateDocRequest true "虚拟机配置"
// @Success 200 {object} vmConfigUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/config [put]
func swaggerUpdateVMConfig() {}

// swaggerRenameVM godoc
// @Summary 修改虚拟机名称
// @Description 修改已停止虚拟机名称。运行中的虚拟机会被拒绝；后端会检查同宿主机 Agent 上是否已有重名虚拟机，随后转发到 Agent 执行 virsh domrename，并同步运行态缓存。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmRenameDocRequest true "虚拟机名称"
// @Success 200 {object} vmRenameDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/rename [put]
func swaggerRenameVM() {}

// swaggerUpdateVMAutostart godoc
// @Summary 修改虚拟机自启动
// @Description 单独修改虚拟机随宿主机同启配置。后端转发到 Agent 执行 virsh autostart 或 autostart --disable，并记录任务与审计日志。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmAutostartUpdateDocRequest true "虚拟机自启动配置"
// @Success 200 {object} vmAutostartUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/autostart [put]
func swaggerUpdateVMAutostart() {}

// swaggerConnectVMMedia godoc
// @Summary 连接虚拟机介质
// @Description 为指定 CDROM 连接 ISO 镜像。运行中的虚拟机会被拒绝；已停止虚拟机使用 virsh change-media --insert --config 更新持久配置，并记录任务与审计日志。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMediaConnectDocRequest true "介质连接配置"
// @Success 200 {object} vmMediaConnectDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/media [put]
func swaggerConnectVMMedia() {}

// swaggerDisconnectVMMedia godoc
// @Summary 断开虚拟机介质
// @Description 为指定 CDROM 断开当前 ISO 镜像。运行中的虚拟机会被拒绝；已停止虚拟机使用 virsh change-media --eject --config 更新持久配置，并记录任务与审计日志。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMediaDisconnectDocRequest true "介质断开配置"
// @Success 200 {object} vmMediaDisconnectDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/media [delete]
func swaggerDisconnectVMMedia() {}

// swaggerUpdateVMXML godoc
// @Summary 修改虚拟机 XML
// @Description 为已停止虚拟机写入完整 libvirt XML。运行中的虚拟机会被拒绝；Agent 校验 XML 非空、格式可解析且 domain name 与当前虚拟机一致后执行 virsh define。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmXMLUpdateDocRequest true "虚拟机 XML 配置"
// @Success 200 {object} vmXMLUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/xml [put]
func swaggerUpdateVMXML() {}

// swaggerUpdateVMDevices godoc
// @Summary 修改虚拟机磁盘与网络设备
// @Description 修改虚拟机网卡网络池、新增/删除网卡、扩容已有磁盘、新增磁盘或删除磁盘。运行中的虚拟机仅支持通过 virsh blockresize 热扩容已有磁盘，并通过 virsh attach-disk --live --config 热添加新磁盘；网络设备修改和删除磁盘需关机后操作。已停止虚拟机的网卡、磁盘和网络池变更会写入 libvirt XML 后执行 virsh define；网卡优先按 MAC 匹配，Agent 会根据目标网络池切换 interface type 与 source 属性；已有磁盘只允许扩容不允许缩容；删除磁盘会从 XML 移除普通 disk 设备并删除对应存储卷，且至少保留一块磁盘。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmDeviceUpdateDocRequest true "虚拟机设备配置"
// @Success 200 {object} vmDeviceUpdateDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/devices [put]
func swaggerUpdateVMDevices() {}

// swaggerMarkVMTemplate godoc
// @Summary 标记虚拟机模板
// @Description 将已停止虚拟机标记为模板。模板标记只写入数据库中的 agent_id、vm_uuid、模板名称、描述和创建人等必要字段，不保存 CPU、内存、磁盘等虚拟机详情；运行态 VM 返回时会按 agent_id + vm_uuid 合并 isTemplate、templateId、templateName 和 templateDescription 字段。需要 vms.update 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmTemplateMarkDocRequest true "模板标记信息"
// @Success 200 {object} vmTemplateMarkDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/vms/{id}/template-mark [post]
func swaggerMarkVMTemplate() {}

// swaggerUnmarkVMTemplate godoc
// @Summary 取消虚拟机模板标记
// @Description 删除虚拟机模板标记，不删除虚拟机本体和磁盘卷。需要 vms.update 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmTemplateUnmarkDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/vms/{id}/template-mark [delete]
func swaggerUnmarkVMTemplate() {}

// swaggerCreateVMFromTemplate godoc
// @Summary 从虚拟机模板创建
// @Description 基于已标记的虚拟机模板创建新虚拟机。接口复用整机克隆参数，要求模板虚拟机已停止，排队前检查新虚拟机名称、宿主机资源、目标存储池卷名和目标卷扩展名；Agent 复制模板磁盘卷并基于模板 XML 重写名称、UUID、磁盘路径、MAC 和网络池后定义新虚拟机。需要 vms.create 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "模板虚拟机 ID"
// @Param body body vmCloneDocRequest true "从模板创建配置"
// @Success 202 {object} vmAsyncTaskDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/template-create [post]
func swaggerCreateVMFromTemplate() {}

// swaggerCloneVM godoc
// @Summary 克隆虚拟机
// @Description 为已停止虚拟机克隆磁盘卷并基于源虚拟机 XML 定义新虚拟机。运行中的虚拟机会被拒绝；后端创建后台任务前会检查宿主机 CPU/内存上限、克隆虚拟机名称是否已存在，检查目标存储池中目标卷名是否已存在，并校验目标卷扩展名必须与源磁盘一致；Agent 执行前也会校验最大 CPU、最大内存不超过宿主机资源，随后执行存储卷克隆、重写名称/UUID/磁盘路径/MAC/网卡网络池后调用 virsh define。选择不同目标存储池时，Agent 使用 qemu-img convert 将源卷复制到目标存储池；autostart 为 true 时表示克隆后直接启动，并会强制断开克隆定义中的 CDROM 介质。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmCloneDocRequest true "虚拟机克隆配置"
// @Success 202 {object} vmCloneDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/clone [post]
func swaggerCloneVM() {}

// swaggerMigrateVM godoc
// @Summary 迁移虚拟机
// @Description 创建后台任务迁移虚拟机。前端默认勾选复制本地磁盘，并要求结构化预检通过后才启用迁移按钮。正式迁移提交只做请求格式、虚拟机、源目标 Agent、迁移方式和 URI 格式等基础校验，不重复执行完整远程预检，排队后由源 Agent 执行迁移并反馈最终结果。热迁移复制本地磁盘时，源 Agent 先通过 SSH 按源磁盘原路径复制磁盘，再执行 virsh migrate --live --unsafe，并按请求追加 --persistent、--auto-converge 和 --postcopy；热迁移未复制磁盘时按共享存储迁移并可追加 --undefinesource。冷迁移未复制磁盘时仍走共享存储迁移；冷迁移复制本地磁盘时由源 Agent 通过 SSH 复制磁盘、重写 XML 并远程 virsh define。复制本地磁盘并勾选迁移后清理源虚拟机时，成功后会删除源定义和源普通磁盘；共享存储迁移只取消源定义，不删除磁盘。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMigrateDocRequest true "虚拟机迁移配置"
// @Success 202 {object} vmAsyncTaskDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/migrate [post]
func swaggerMigrateVM() {}

// swaggerPrecheckVMMigration godoc
// @Summary 预检虚拟机迁移
// @Description 返回结构化迁移预检清单，不创建后台任务；这是完整远程迁移预检入口。预检项包括源 VM 状态、目标同名虚拟机、目标 CPU/内存容量、基础 CPU 架构、网络池/桥接设备、共享存储或目标存储路径和迁移通道；勾选复制本地磁盘时会检查目标宿主机是否存在每块源磁盘路径所在的存储池，并检查目标池中是否已存在同路径或同名磁盘卷，同时要求 qemu+ssh:// 迁移 URI；未勾选复制本地磁盘时检查源目标存储池是否显示共享存储特征；仅当迁移 URI 以 qemu+ssh:// 开头时检测 SSH 非交互通道，其他 URI 返回跳过项。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMigrateDocRequest true "虚拟机迁移配置"
// @Success 200 {object} vmMigrationPrecheckDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/migrate-precheck [post]
func swaggerPrecheckVMMigration() {}

// swaggerSetupVMMigrationSSHKey godoc
// @Summary 配置迁移 SSH 免密
// @Description 当以 qemu+ssh:// 开头的迁移通道预检提示需要密码时，前端可提交目标 SSH 用户和密码，由源 Agent 将源宿主机公钥写入目标宿主机 authorized_keys；密码仅用于本次配置，不在平台侧持久化。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMigrationSSHKeyDocRequest true "迁移 SSH 免密配置"
// @Success 200 {object} vmMigrationSSHKeyDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/migrate-ssh-key [post]
func swaggerSetupVMMigrationSSHKey() {}

// swaggerSetupVMMigrationHostname godoc
// @Summary 修复迁移目标主机名
// @Description 当热迁移通道预检提示目标宿主机主机名解析为 localhost 时，前端可提交目标主机名。后端转发源 Agent，通过 SSH 设置目标宿主机 hostname，并在源宿主机和目标宿主机 /etc/hosts 写入目标 IP 与主机名解析；配置成功后前端重新执行迁移预检。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Param body body vmMigrationHostnameDocRequest true "迁移目标主机名配置"
// @Success 200 {object} vmMigrationHostnameDocResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/migrate-hostname [post]
func swaggerSetupVMMigrationHostname() {}

// swaggerVMConsole godoc
// @Summary 打开虚拟机 Web 控制台
// @Description WebSocket 接口。前端 noVNC 连接该接口，后端使用已登记 Agent 的加密令牌转发到 Agent VNC 代理。
// @Tags realtime
// @Produce plain
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Router /api/vms/{id}/console/ws [get]
func swaggerVMConsole() {}

// swaggerHostMetrics godoc
// @Summary 查询宿主机指标趋势
// @Description 查询宿主机 CPU、内存、存储和虚拟机数量趋势。agentId=all 时聚合全部宿主机。range 支持 1h、24h、7d、30d 和 custom。自定义时间范围需要传入分钟精度或 RFC3339 格式的 start 和 end。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param agentId path string true "Agent ID 或 all"
// @Param range query string false "趋势时间范围" Enums(1h,24h,7d,30d,custom) default(1h)
// @Param start query string false "自定义开始时间，例如 2026-05-23T10:00"
// @Param end query string false "自定义结束时间，例如 2026-05-23T12:00"
// @Success 200 {object} metricSeriesDocResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/metrics/hosts/{agentId} [get]
func swaggerHostMetrics() {}

// swaggerVMMetrics godoc
// @Summary 查询虚拟机指标趋势
// @Description 查询虚拟机 CPU、内存、磁盘使用率、磁盘 I/O 和网络吞吐趋势。range 支持 1h、24h、7d、30d 和 custom。自定义时间范围需要传入分钟精度或 RFC3339 格式的 start 和 end。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param vmId path string true "虚拟机 ID"
// @Param range query string false "趋势时间范围" Enums(1h,24h,7d,30d,custom) default(1h)
// @Param start query string false "自定义开始时间，例如 2026-05-23T10:00"
// @Param end query string false "自定义结束时间，例如 2026-05-23T12:00"
// @Success 200 {object} metricSeriesDocResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/metrics/vms/{vmId} [get]
func swaggerVMMetrics() {}

// swaggerRefreshAsync godoc
// @Summary 创建异步刷新任务
// @Description 创建或复用正在运行的运行态刷新任务。后端工作线程会同步所有已登记 Agent，并通过 SSE 推送进度和完成事件。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Success 202 {object} refreshResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/refresh [post]
func swaggerRefreshAsync() {}

// swaggerEvents godoc
// @Summary SSE 事件流
// @Description 前端订阅此接口接收运行态资源更新事件。该接口返回 text/event-stream。
// @Tags realtime
// @Produce plain
// @Security BearerAuth
// @Success 200 {string} string "SSE stream"
// @Failure 401 {object} errorResponse
// @Router /api/events [get]
func swaggerEvents() {}

// swaggerListAgents godoc
// @Summary 获取 Agent 列表
// @Tags agents
// @Produce json
// @Security BearerAuth
// @Success 200 {object} agentListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/agents [get]
func swaggerListAgents() {}

// swaggerProbeAgent godoc
// @Summary 测试未登记 Agent 连接
// @Tags agents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createAgentRequest true "Agent 连接信息"
// @Success 200 {object} agentProbeResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/agents/test-connection [post]
func swaggerProbeAgent() {}

// swaggerCreateAgent godoc
// @Summary 登记 Agent
// @Tags agents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createAgentRequest true "Agent 登记信息"
// @Success 201 {object} domain.Agent
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/agents [post]
func swaggerCreateAgent() {}

// swaggerTestAgent godoc
// @Summary 测试已登记 Agent 连接
// @Tags agents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Agent ID"
// @Param body body agentTokenRequest true "Agent 令牌"
// @Success 200 {object} agentProbeResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/agents/{id}/test-connection [post]
func swaggerTestAgent() {}

// swaggerSyncAgent godoc
// @Summary 立即同步指定 Agent
// @Tags agents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Agent ID"
// @Param body body agentTokenRequest true "Agent 令牌"
// @Success 200 {object} agentSyncResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/agents/{id}/sync [post]
func swaggerSyncAgent() {}

// swaggerDeleteAgent godoc
// @Summary 删除 Agent
// @Tags agents
// @Produce json
// @Security BearerAuth
// @Param id path string true "Agent ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/agents/{id} [delete]
func swaggerDeleteAgent() {}

// swaggerVMStart godoc
// @Summary 启动虚拟机
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/start [post]
func swaggerVMStart() {}

// swaggerVMResume godoc
// @Summary 恢复已暂停虚拟机
// @Description 恢复 paused 状态虚拟机，对应 Agent resume 操作。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/resume [post]
func swaggerVMResume() {}

// swaggerVMStop godoc
// @Summary 关闭虚拟机
// @Description 正常关闭虚拟机，对应 Agent shutdown 操作。命令成功后后端先更新运行态缓存为已停止并返回，再后台延迟 full 同步所属 Agent。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/stop [post]
func swaggerVMStop() {}

// swaggerVMForceStop godoc
// @Summary 强制关闭虚拟机
// @Description 强制关闭虚拟机，对应 Agent destroy 操作。命令成功后后端先更新运行态缓存为已停止并返回，再后台延迟 full 同步所属 Agent。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/force-stop [post]
func swaggerVMForceStop() {}

// swaggerVMShutdown godoc
// @Summary 正常关机
// @Description 正常关机别名，等同于 stop，对应 Agent shutdown 操作。命令成功后后端先更新运行态缓存为已停止并返回，再后台延迟 full 同步所属 Agent。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/shutdown [post]
func swaggerVMShutdown() {}

// swaggerVMForceShutdown godoc
// @Summary 强制关机
// @Description 强制关机别名，等同于 force-stop，对应 Agent destroy 操作。命令成功后后端先更新运行态缓存为已停止并返回，再后台延迟 full 同步所属 Agent。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/force-shutdown [post]
func swaggerVMForceShutdown() {}

// swaggerVMReboot godoc
// @Summary 重启虚拟机
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/reboot [post]
func swaggerVMReboot() {}

// swaggerVMForceReboot godoc
// @Summary 强制重启虚拟机
// @Description 强制重置虚拟机，对应 Agent reset 操作。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/force-reboot [post]
func swaggerVMForceReboot() {}

// swaggerVMPause godoc
// @Summary 暂停虚拟机
// @Description 暂停虚拟机，对应 Agent suspend 操作。命令成功后后端先更新运行态缓存为已暂停并返回，再后台延迟 full 同步所属 Agent。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/pause [post]
func swaggerVMPause() {}

// swaggerVMDelete godoc
// @Summary 删除虚拟机
// @Description 删除虚拟机定义并移除普通磁盘存储卷，不删除连接到 CDROM 的 ISO 介质。删除成功后后端先从运行态缓存移除该虚拟机并广播 runtime.updated，让列表尽快消失，随后后台延迟 full 同步所属 Agent 兜底校准。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/delete [post]
func swaggerVMDelete() {}

// swaggerVMForceDelete godoc
// @Summary 强制删除虚拟机
// @Description 强制关闭虚拟机后删除虚拟机定义并移除普通磁盘存储卷，不删除连接到 CDROM 的 ISO 介质。删除成功后后端先从运行态缓存移除该虚拟机并广播 runtime.updated，让列表尽快消失，随后后台延迟 full 同步所属 Agent 兜底校准。
// @Tags vms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "虚拟机 ID"
// @Success 200 {object} vmActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/vms/{id}/force-delete [post]
func swaggerVMForceDelete() {}

// swaggerListTasks godoc
// @Summary 获取任务列表
// @Tags logs
// @Produce json
// @Security BearerAuth
// @Param status query string false "任务状态，如 queued、running、completed、failed"
// @Param q query string false "搜索关键词，可匹配任务类型、状态、目标、进度和错误信息"
// @Param payloadKey query string false "任务载荷顶层字段名，配合 payloadValue 精确限定 JSON 字段；仅填写字段名时匹配存在该字段的任务"
// @Param payloadValue query string false "任务载荷字段值关键词；payloadKey 为空时在整段载荷 JSON 中模糊搜索"
// @Param limit query string false "返回数量，可选 30、50、100、200 或 all"
// @Param page query int false "页码，从 1 开始"
// @Success 200 {object} taskListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/tasks [get]
func swaggerListTasks() {}

// swaggerGetTask godoc
// @Summary 获取任务详情
// @Tags logs
// @Produce json
// @Security BearerAuth
// @Param id path string true "任务 ID"
// @Success 200 {object} taskDetailResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/tasks/{id} [get]
func swaggerGetTask() {}

// swaggerListAuditLogs godoc
// @Summary 获取审计日志列表
// @Tags logs
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词，可匹配动作、用户、资源、IP 和元数据"
// @Param metadataKey query string false "审计元数据顶层字段名，配合 metadataValue 精确限定 JSON 字段；仅填写字段名时匹配存在该字段的日志"
// @Param metadataValue query string false "审计元数据字段值关键词；metadataKey 为空时在整段元数据 JSON 中模糊搜索"
// @Param limit query string false "返回数量，可选 30、50、100、200 或 all"
// @Param page query int false "页码，从 1 开始"
// @Success 200 {object} auditLogListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/audit-logs [get]
func swaggerListAuditLogs() {}

// swaggerListAlerts godoc
// @Summary 获取告警列表
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Param status query string false "告警状态，如 active、resolved"
// @Param q query string false "搜索关键词，可匹配级别、状态、标题、消息、来源和外部通知状态"
// @Param metadataKey query string false "告警元数据顶层字段名，配合 metadataValue 精确限定 JSON 字段；仅填写字段名时匹配存在该字段的告警"
// @Param metadataValue query string false "告警元数据字段值关键词；metadataKey 为空时在整段元数据 JSON 中模糊搜索"
// @Param limit query string false "返回数量，可选 30、50、100、200 或 all"
// @Param page query int false "页码，从 1 开始"
// @Success 200 {object} alertListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/alerts [get]
func swaggerListAlerts() {}

// swaggerResolveAlert godoc
// @Summary 手动解决告警
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Param id path string true "告警 ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/alerts/{id}/resolve [post]
func swaggerResolveAlert() {}

// swaggerListNotifications godoc
// @Summary 获取通知消息
// @Description 返回右上角通知中心展示的告警消息，默认仅返回未清空的活跃告警。
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Param status query string false "通知来源告警状态，active 或 all"
// @Param limit query string false "返回数量，默认 20"
// @Success 200 {object} alertListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/notifications [get]
func swaggerListNotifications() {}

// swaggerUnreadNotificationCount godoc
// @Summary 获取未读通知数量
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} unreadNotificationCountResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/notifications/unread-count [get]
func swaggerUnreadNotificationCount() {}

// swaggerReadNotification godoc
// @Summary 标记单条通知已读
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Param id path string true "通知对应告警 ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/notifications/{id}/read [post]
func swaggerReadNotification() {}

// swaggerReadAllNotifications godoc
// @Summary 标记全部通知已读
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/notifications/read-all [post]
func swaggerReadAllNotifications() {}

// swaggerClearNotifications godoc
// @Summary 清空通知中心消息
// @Description 清空仅影响通知中心展示，不会解决告警。
// @Tags alerts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/notifications/clear [post]
func swaggerClearNotifications() {}

// swaggerListNotificationChannels godoc
// @Summary 获取通知媒介配置
// @Description 返回 Webhook、邮件、飞书、企业微信和钉钉通知媒介配置。平台内告警通过右上角通知中心展示。邮件 password、飞书/钉钉 secret 不返回明文，已配置时返回 hasPassword 或 hasSecret 标记。需要通知配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} notificationChannelListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/notifications [get]
func swaggerListNotificationChannels() {}

// swaggerUpdateNotificationChannel godoc
// @Summary 更新通知媒介配置
// @Description 通知媒介包含告警通知和找回密码两个独立用途开关；任一用途启用时 Webhook 需要 url，可选 method、headers；邮件需要 smtpHost、smtpPort、username、password、from、to，可选 fromName、useTLS 或 startTLS 且 TLS 与 STARTTLS 不能同时启用；飞书、企业微信、钉钉需要 webhookUrl，飞书和钉钉可选 secret。邮件 password、飞书/钉钉 secret 留空时保留已保存值，填写新值时替换。两个用途都关闭时允许保存空配置，用于清空已保存配置。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "通知媒介 ID：webhook、email、lark、wechat、dingtalk"
// @Param request body notificationChannelRequestDoc true "通知媒介配置"
// @Success 200 {object} domain.NotificationChannel
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/notifications/{id} [put]
func swaggerUpdateNotificationChannel() {}

// swaggerTestNotificationChannel godoc
// @Summary 测试通知媒介
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "通知媒介 ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/settings/notifications/{id}/test [post]
func swaggerTestNotificationChannel() {}

// swaggerListAuthProviders godoc
// @Summary 获取认证配置
// @Description 返回 AD/LDAP 等外部认证配置。LDAP bindPassword 不返回明文，已配置时返回 hasBindPassword 标记。需要认证配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} authProviderListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/auth-providers [get]
func swaggerListAuthProviders() {}

// swaggerUpdateAuthProvider godoc
// @Summary 更新认证配置
// @Description 首期支持 AD/LDAP。显示名称不能为空；启用后登录页会显示对应认证方式，且必须提供服务器地址、端口、Base DN、用户过滤器、绑定 DN 和绑定密码。LDAP bindPassword 留空时保留已保存值，填写新值时替换。LDAPS 通常使用 636 端口，StartTLS 通常使用 389 端口，二者不能同时启用。关闭认证时允许保存空配置，用于清空已保存配置。外部认证用户必须先在用户配置中创建并启用后才可登录。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "认证配置 ID：ldap"
// @Param request body authProviderRequestDoc true "认证配置"
// @Success 200 {object} domain.AuthProvider
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/auth-providers/{id} [put]
func swaggerUpdateAuthProvider() {}

// swaggerTestAuthProvider godoc
// @Summary 测试认证配置
// @Description 使用已保存认证配置测试 LDAP 连接、绑定账号和用户搜索，成功时返回匹配用户数量。若配置了用户组过滤器，则按该配置统计匹配用户数，登录时也会要求用户匹配该组条件；用户组过滤器可直接填写用户组 DN，后端会按 memberOf 自动转换。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "认证配置 ID"
// @Success 200 {object} authProviderTestResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/settings/auth-providers/{id}/test [post]
func swaggerTestAuthProvider() {}

// swaggerGetSystemBaseConfig godoc
// @Summary 获取基础配置
// @Description 返回网站名称、登录页名称、控制台品牌名称、控制台品牌副标题、图标、安全时效、资源阈值和 Agent 判定配置。需要基础配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} domain.SystemBaseConfig
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/base-config [get]
func swaggerGetSystemBaseConfig() {}

// swaggerUpdateSystemBaseConfig godoc
// @Summary 更新基础配置
// @Description 更新网站名称、登录/注册界面名称、控制台品牌名称、控制台品牌副标题、图标、安全时效、资源阈值和 Agent 判定配置。图标支持站内路径或图片 Data URL，最大 512KB。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body systemBaseConfigRequestDoc true "基础配置"
// @Success 200 {object} domain.SystemBaseConfig
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/base-config [put]
func swaggerUpdateSystemBaseConfig() {}

// swaggerListManagedUsers godoc
// @Summary 获取平台用户
// @Description 返回平台用户及其有效角色和权限。AD/LDAP 登录用户也必须先在此列表中创建并启用。需要用户配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} managedUserListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/users [get]
func swaggerListManagedUsers() {}

// swaggerCreateManagedUser godoc
// @Summary 创建平台用户
// @Description 创建平台用户必须填写本地密码。启用 AD/LDAP 后，用户仍必须先在此创建并启用；本地登录校验数据库密码，AD/LDAP 登录校验 LDAP 密码且不写入数据库。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body managedUserRequestDoc true "用户配置"
// @Success 201 {object} domain.User
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/users [post]
func swaggerCreateManagedUser() {}

// swaggerUpdateManagedUser godoc
// @Summary 更新平台用户
// @Description 更新用户名、显示名称、角色、禁用状态和可选新密码。密码留空时不修改数据库中的本地密码。默认 admin 管理员不能改名或禁用。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户 ID"
// @Param request body managedUserRequestDoc true "用户配置"
// @Success 200 {object} domain.User
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/users/{id} [put]
func swaggerUpdateManagedUser() {}

// swaggerDeleteManagedUser godoc
// @Summary 删除平台用户
// @Description 删除平台用户前必须先禁用该用户。删除会清理该用户会话、直接角色和群组成员关系；审计日志和历史任务保留记录。不能删除当前登录用户和默认 admin 管理员。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户 ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/users/{id} [delete]
func swaggerDeleteManagedUser() {}

// swaggerSetManagedUserDisabled godoc
// @Summary 启用或禁用平台用户
// @Description 默认 admin 管理员不能禁用。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户 ID"
// @Param request body managedUserDisabledRequestDoc true "用户状态"
// @Success 200 {object} domain.User
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/users/{id}/disabled [post]
func swaggerSetManagedUserDisabled() {}

// swaggerListUserGroups godoc
// @Summary 获取用户群组
// @Description 返回用户群组及成员、角色配置。需要用户配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} userGroupListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/user-groups [get]
func swaggerListUserGroups() {}

// swaggerCreateUserGroup godoc
// @Summary 创建用户群组
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body userGroupRequestDoc true "用户群组"
// @Success 201 {object} domain.UserGroup
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/user-groups [post]
func swaggerCreateUserGroup() {}

// swaggerUpdateUserGroup godoc
// @Summary 更新用户群组
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户群组 ID"
// @Param request body userGroupRequestDoc true "用户群组"
// @Success 200 {object} domain.UserGroup
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/user-groups/{id} [put]
func swaggerUpdateUserGroup() {}

// swaggerDeleteUserGroup godoc
// @Summary 删除用户群组
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户群组 ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/user-groups/{id} [delete]
func swaggerDeleteUserGroup() {}

// swaggerListRoles godoc
// @Summary 获取用户角色
// @Description 返回内置角色与自定义角色。需要用户配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} userRoleListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/roles [get]
func swaggerListRoles() {}

// swaggerCreateRole godoc
// @Summary 创建自定义角色
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body userRoleRequestDoc true "角色配置"
// @Success 201 {object} domain.Role
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/roles [post]
func swaggerCreateRole() {}

// swaggerUpdateRole godoc
// @Summary 更新自定义角色
// @Description 内置角色 admin、operator、viewer 不允许修改。
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "角色 ID"
// @Param request body userRoleRequestDoc true "角色配置"
// @Success 200 {object} domain.Role
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/roles/{id} [put]
func swaggerUpdateRole() {}

// swaggerDeleteRole godoc
// @Summary 删除自定义角色
// @Description 内置角色 admin、operator、viewer 不允许删除。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Param id path string true "角色 ID"
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/settings/roles/{id} [delete]
func swaggerDeleteRole() {}

// swaggerListPermissions godoc
// @Summary 获取可选权限点
// @Description 返回当前系统可配置到自定义角色中的权限点。需要用户配置查看或管理权限。
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} permissionListResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Router /api/settings/permissions [get]
func swaggerListPermissions() {}
