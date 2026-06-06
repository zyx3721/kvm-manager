package router

import "kvm-manager/backend/internal/domain"

type createSnapshotRequestDoc struct {
	VMID        string   `json:"vmId" example:"vm-1"`
	Name        string   `json:"name" example:"snap-before-upgrade"`
	Description string   `json:"description" example:"Before system upgrade"`
	Tags        []string `json:"tags" example:"production,upgrade"`
}

type updateSnapshotAnnotationRequestDoc struct {
	DisplayName string   `json:"displayName" example:"升级前快照"`
	Description string   `json:"description" example:"业务升级前创建"`
	Tags        []string `json:"tags" example:"production,upgrade"`
}

type snapshotAnnotationResponse struct {
	Snapshot domain.Snapshot `json:"snapshot"`
}

// swaggerListSnapshots godoc
// @Summary 获取快照列表
// @Description 从后端运行态缓存读取快照列表，并合并平台备注和标签。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Success 200 {object} snapshotListResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/snapshots [get]
func swaggerListSnapshots() {}

// swaggerRefreshSnapshots godoc
// @Summary 刷新快照列表
// @Description 仅刷新已登记 Agent 上的虚拟机快照列表，并更新运行态缓存；不触发宿主机、虚拟机详情和指标的全量刷新。需要 snapshots.read 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Success 200 {object} statusResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/snapshots/refresh [post]
func swaggerRefreshSnapshots() {}

// swaggerSnapshotCreate godoc
// @Summary 创建快照
// @Description 通过已登记 Agent 的加密令牌为已关机虚拟机创建 libvirt 默认内部快照，完成后刷新运行态缓存并记录任务和审计日志。需要 snapshots.create 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body createSnapshotRequestDoc true "创建快照参数"
// @Success 200 {object} snapshotActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/snapshots [post]
func swaggerSnapshotCreate() {}

// swaggerSnapshotAnnotation godoc
// @Summary 更新快照备注
// @Description 更新平台侧快照显示名、描述和标签，不修改 libvirt 快照实体。需要 snapshots.update 权限。
// @Tags realtime
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "快照 ID"
// @Param request body updateSnapshotAnnotationRequestDoc true "快照备注"
// @Success 200 {object} snapshotAnnotationResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /api/snapshots/{id}/annotation [put]
func swaggerSnapshotAnnotation() {}

// swaggerSnapshotRevert godoc
// @Summary 恢复快照
// @Description 通过已登记 Agent 的加密令牌恢复指定快照，完成后刷新运行态缓存并记录任务和审计日志。需要 snapshots.revert 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "快照 ID"
// @Success 200 {object} snapshotActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/snapshots/{id}/revert [post]
func swaggerSnapshotRevert() {}

// swaggerSnapshotDelete godoc
// @Summary 删除快照
// @Description 通过已登记 Agent 的加密令牌删除指定内部快照，完成后刷新运行态缓存并记录任务和审计日志。需要 snapshots.delete 权限。
// @Tags realtime
// @Produce json
// @Security BearerAuth
// @Param id path string true "快照 ID"
// @Success 200 {object} snapshotActionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 503 {object} errorResponse
// @Router /api/snapshots/{id}/delete [post]
func swaggerSnapshotDelete() {}
