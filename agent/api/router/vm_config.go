package router

import (
	"net/http"
	"strings"

	"kvm-manager/agent/internal/kvm"
)

func (r *Router) handleUpdateVMConfig(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMConfigUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机配置参数格式不正确")
		return
	}
	body.Description = strings.TrimSpace(body.Description)
	config, err := r.provider.UpdateVMConfig(vmName, body)
	if err != nil {
		r.logger.Error("vm config update failed", "vm", vmName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_config_update_failed", operationErrorMessage("修改虚拟机配置失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleRenameVM(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMRenameRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机名称参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	config, err := r.provider.RenameVM(vmName, body)
	if err != nil {
		r.logger.Error("vm rename failed", "vm", vmName, "target", body.Name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_rename_failed", operationErrorMessage("修改虚拟机名称失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleUpdateVMAutostart(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMAutostartUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机自启动参数格式不正确")
		return
	}
	if err := r.provider.UpdateVMAutostart(vmName, body); err != nil {
		r.logger.Error("vm autostart update failed", "vm", vmName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_autostart_update_failed", operationErrorMessage("修改虚拟机自启动失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"autostart": body.Autostart})
}

func (r *Router) handleUpdateVMConsole(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMConsoleUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "控制台配置参数格式不正确")
		return
	}
	body.Password = strings.TrimSpace(body.Password)
	config, err := r.provider.UpdateVMConsole(vmName, body)
	if err != nil {
		r.logger.Error("vm console update failed", "vm", vmName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_console_update_failed", operationErrorMessage("修改控制台配置失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleConnectVMMedia(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMMediaConnectRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "介质连接参数格式不正确")
		return
	}
	body.Target = strings.TrimSpace(body.Target)
	body.ISOPath = strings.TrimSpace(body.ISOPath)
	config, err := r.provider.ConnectVMMedia(vmName, body)
	if err != nil {
		r.logger.Error("vm media connect failed", "vm", vmName, "target", body.Target, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_media_connect_failed", operationErrorMessage("连接介质失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleDisconnectVMMedia(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMMediaDisconnectRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "介质断开参数格式不正确")
		return
	}
	body.Target = strings.TrimSpace(body.Target)
	config, err := r.provider.DisconnectVMMedia(vmName, body)
	if err != nil {
		r.logger.Error("vm media disconnect failed", "vm", vmName, "target", body.Target, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_media_disconnect_failed", operationErrorMessage("断开介质失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleUpdateVMXML(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMXMLUpdateRequest
	if err := decodeJSONBodyLimit(w, req, 4<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机 XML 参数格式不正确")
		return
	}
	config, err := r.provider.UpdateVMXML(vmName, body)
	if err != nil {
		r.logger.Error("vm xml update failed", "vm", vmName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_xml_update_failed", operationErrorMessage("修改虚拟机 XML 失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleUpdateVMDevices(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMDeviceUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机设备参数格式不正确")
		return
	}
	config, err := r.provider.UpdateVMDevices(vmName, body)
	if err != nil {
		r.logger.Error("vm devices update failed", "vm", vmName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_devices_update_failed", operationErrorMessage("修改虚拟机设备失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleCloneVM(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.VMCloneRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机克隆参数格式不正确")
		return
	}
	config, err := r.provider.CloneVM(vmName, body)
	if err != nil {
		r.logger.Error("vm clone failed", "vm", vmName, "clone", body.Name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_clone_failed", operationErrorMessage("克隆虚拟机失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}
