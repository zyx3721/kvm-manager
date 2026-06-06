package router

import (
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
)

type updateVMDevicesRequest struct {
	Interfaces        []updateVMDeviceInterfaceRequest  `json:"interfaces"`
	NewInterfaces     []updateVMDeviceNewInterface      `json:"newInterfaces"`
	DeletedInterfaces []updateVMDeviceDeleteInterface   `json:"deletedInterfaces"`
	DiskResizes       []updateVMDeviceDiskResizeRequest `json:"diskResizes"`
	NewDisks          []updateVMDeviceNewDiskRequest    `json:"newDisks"`
	DeletedDisks      []updateVMDeviceDeleteDiskRequest `json:"deletedDisks"`
}

type updateVMDeviceInterfaceRequest struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Source string `json:"source"`
}

type updateVMDeviceNewInterface struct {
	Source string `json:"source"`
	Model  string `json:"model"`
}

type updateVMDeviceDeleteInterface struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
}

type updateVMDeviceDiskResizeRequest struct {
	Name          string `json:"name"`
	CapacityBytes int64  `json:"capacityBytes"`
}

type updateVMDeviceDeleteDiskRequest struct {
	Name string `json:"name"`
}

type updateVMDeviceNewDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	Target           string `json:"target"`
	Bus              string `json:"bus"`
	Format           string `json:"format"`
	CapacityBytes    int64  `json:"capacityBytes"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

func (r *router) handleUpdateVMDevices(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body updateVMDevicesRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机设备参数格式不正确")
		return
	}
	if err := validateVMDevicesUpdate(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_devices", err.Error())
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		if err := validateRunningVMDevicesUpdate(body); err != nil {
			writeError(w, http.StatusBadRequest, "vm_running", err.Error())
			return
		}
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	request := agent.VMDeviceUpdateRequest{}
	for _, iface := range body.Interfaces {
		request.Interfaces = append(request.Interfaces, agent.VMDeviceInterfaceRequest{
			Name:   strings.TrimSpace(iface.Name),
			MAC:    strings.TrimSpace(iface.MAC),
			Source: strings.TrimSpace(iface.Source),
		})
	}
	for _, iface := range body.NewInterfaces {
		request.NewInterfaces = append(request.NewInterfaces, agent.VMDeviceNewInterface{
			Source: strings.TrimSpace(iface.Source),
			Model:  strings.TrimSpace(iface.Model),
		})
	}
	for _, iface := range body.DeletedInterfaces {
		request.DeletedInterfaces = append(request.DeletedInterfaces, agent.VMDeviceDeleteInterface{
			Name: strings.TrimSpace(iface.Name),
			MAC:  strings.TrimSpace(iface.MAC),
		})
	}
	for _, disk := range body.DiskResizes {
		request.DiskResizes = append(request.DiskResizes, agent.VMDeviceDiskResizeRequest{
			Name:          strings.TrimSpace(disk.Name),
			CapacityBytes: disk.CapacityBytes,
		})
	}
	for _, disk := range body.DeletedDisks {
		request.DeletedDisks = append(request.DeletedDisks, agent.VMDeviceDeleteDiskRequest{
			Name: strings.TrimSpace(disk.Name),
		})
	}
	for _, disk := range body.NewDisks {
		request.NewDisks = append(request.NewDisks, agent.VMDeviceNewDiskRequest{
			Name:             strings.TrimSpace(disk.Name),
			Pool:             strings.TrimSpace(disk.Pool),
			Target:           strings.TrimSpace(disk.Target),
			Bus:              strings.TrimSpace(disk.Bus),
			Format:           strings.TrimSpace(disk.Format),
			CapacityBytes:    disk.CapacityBytes,
			PreallocMetadata: disk.PreallocMetadata,
		})
	}
	config, err := client.UpdateVMDevices(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, request)
	if err != nil {
		r.logger.Error("agent vm devices update failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_devices_update_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	metadata := map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "interfaces": len(request.Interfaces), "newInterfaces": len(request.NewInterfaces), "deletedInterfaces": len(request.DeletedInterfaces), "diskResizes": len(request.DiskResizes), "newDisks": len(request.NewDisks), "deletedDisks": len(request.DeletedDisks)}
	task, err := r.store.CreateTask(req.Context(), "vm.devices.update", "completed", "virtual_machine", id, metadata, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm devices update task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.devices.update", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "interfaces": len(request.Interfaces), "newInterfaces": len(request.NewInterfaces), "deletedInterfaces": len(request.DeletedInterfaces), "diskResizes": len(request.DiskResizes), "newDisks": len(request.NewDisks), "deletedDisks": len(request.DeletedDisks)})
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func validateVMDevicesUpdate(body updateVMDevicesRequest) error {
	for _, iface := range body.Interfaces {
		if strings.TrimSpace(iface.Source) == "" {
			return errors.New("网络池不能为空")
		}
		if strings.TrimSpace(iface.MAC) == "" && strings.TrimSpace(iface.Name) == "" {
			return errors.New("网络设备信息缺失，请刷新配置后重试")
		}
	}
	for _, iface := range body.NewInterfaces {
		if strings.TrimSpace(iface.Source) == "" {
			return errors.New("新增网卡网络池不能为空")
		}
		if model := strings.TrimSpace(iface.Model); model != "" && !supportedVMInterfaceModel(model) {
			return errors.New("新增网卡模型不支持")
		}
	}
	for _, iface := range body.DeletedInterfaces {
		if strings.TrimSpace(iface.MAC) == "" && strings.TrimSpace(iface.Name) == "" {
			return errors.New("删除网卡信息缺失，请刷新配置后重试")
		}
	}
	for _, disk := range body.DiskResizes {
		if strings.TrimSpace(disk.Name) == "" || disk.CapacityBytes <= 0 {
			return errors.New("扩容磁盘名称和目标容量不能为空")
		}
	}
	for _, disk := range body.DeletedDisks {
		if strings.TrimSpace(disk.Name) == "" {
			return errors.New("删除磁盘名称不能为空")
		}
	}
	for _, disk := range body.NewDisks {
		if strings.TrimSpace(disk.Name) == "" || strings.TrimSpace(disk.Pool) == "" || strings.TrimSpace(disk.Target) == "" || strings.TrimSpace(disk.Bus) == "" || strings.TrimSpace(disk.Format) == "" || disk.CapacityBytes <= 0 {
			return errors.New("新增磁盘名称、存储池、目标设备、总线、格式和容量不能为空")
		}
		if !supportedVMDeviceDiskFormat(disk.Format) {
			return errors.New("新增磁盘格式不支持")
		}
		if strings.ContainsAny(strings.TrimSpace(disk.Name), `/\`) {
			return errors.New("新增磁盘名称不能包含路径分隔符")
		}
		if !vmDeviceDiskNameMatchesFormat(disk.Name, disk.Format) {
			return errors.New("新增磁盘名称扩展名必须与格式一致")
		}
	}
	return nil
}

func validateRunningVMDevicesUpdate(body updateVMDevicesRequest) error {
	if len(body.Interfaces) > 0 || len(body.NewInterfaces) > 0 || len(body.DeletedInterfaces) > 0 {
		return errors.New("虚拟机正在运行，不支持修改网络设备，请先关闭虚拟机后再操作")
	}
	if len(body.DeletedDisks) > 0 {
		return errors.New("虚拟机正在运行，不支持删除磁盘，请先关闭虚拟机后再操作")
	}
	return nil
}

func supportedVMInterfaceModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "virtio", "e1000", "e1000e", "rtl8139", "vmxnet3":
		return true
	default:
		return false
	}
}

func supportedVMDeviceDiskFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "qcow2", "qcow", "qed", "raw":
		return true
	default:
		return false
	}
}

func vmDeviceDiskNameMatchesFormat(name string, format string) bool {
	extension := ".img"
	if strings.EqualFold(strings.TrimSpace(format), "qcow2") {
		extension = ".qcow2"
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), extension)
}
