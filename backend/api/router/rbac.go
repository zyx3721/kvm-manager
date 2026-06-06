package router

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/auth"
)

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,40}$`)

type managedUserRequest struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	RoleKeys    []string `json:"roleKeys"`
	Disabled    bool     `json:"disabled"`
}

type roleRequest struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type userGroupRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Disabled    bool     `json:"disabled"`
	MemberIDs   []string `json:"memberIds"`
	RoleKeys    []string `json:"roleKeys"`
}

func (r *router) handleListManagedUsers(w http.ResponseWriter, req *http.Request) {
	items, err := r.store.ListManagedUsers(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_users_failed", "读取用户列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (r *router) handleCreateManagedUser(w http.ResponseWriter, req *http.Request) {
	input, ok := r.decodeManagedUserRequest(w, req, true)
	if !ok {
		return
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		r.logger.Error("hash managed user password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create_user_failed", "创建用户失败")
		return
	}
	user, err := r.store.CreateManagedUser(req.Context(), repository.UserInput{
		Username: input.Username, Email: input.Email, DisplayName: input.DisplayName, RoleKeys: input.RoleKeys, Disabled: input.Disabled,
	}, passwordHash)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user_exists", "用户名已存在")
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "role_not_found", "选择的角色不存在")
			return
		}
		r.logger.Error("create managed user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create_user_failed", "创建用户失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user.create", "user", user.ID, repository.ClientIP(req), map[string]any{"username": user.Username})
	writeJSON(w, http.StatusCreated, user)
}

func (r *router) handleManagedUserRoute(w http.ResponseWriter, req *http.Request) {
	id, action, ok := parseSettingsResourcePath(req.URL.Path, "/api/settings/users/")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut && action == "" {
		r.handleUpdateManagedUser(w, req, id)
		return
	}
	if req.Method == http.MethodDelete && action == "" {
		r.handleDeleteManagedUser(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "disabled" {
		r.handleSetManagedUserDisabled(w, req, id)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) handleDeleteManagedUser(w http.ResponseWriter, req *http.Request, id string) {
	if id == currentSession(req).User.ID {
		writeError(w, http.StatusBadRequest, "delete_self_forbidden", "不能删除当前登录用户")
		return
	}
	user, err := r.store.FindUserByID(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
			return
		}
		r.logger.Error("find managed user before delete failed", "error", err)
		writeError(w, http.StatusInternalServerError, "delete_user_failed", "删除用户失败")
		return
	}
	if isDefaultAdminUser(user) {
		writeError(w, http.StatusBadRequest, "default_admin_protected", "默认管理员不能删除")
		return
	}
	if !user.Disabled {
		writeError(w, http.StatusBadRequest, "user_must_be_disabled", "请先禁用用户再删除")
		return
	}
	if err := r.store.DeleteManagedUser(req.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
			return
		}
		r.logger.Error("delete managed user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "delete_user_failed", "删除用户失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user.delete", "user", id, repository.ClientIP(req), map[string]any{"username": user.Username})
	w.WriteHeader(http.StatusNoContent)
}

func (r *router) handleUpdateManagedUser(w http.ResponseWriter, req *http.Request, id string) {
	input, ok := r.decodeManagedUserRequest(w, req, false)
	if !ok {
		return
	}
	if id == currentSession(req).User.ID && input.Disabled {
		writeError(w, http.StatusBadRequest, "disable_self_forbidden", "不能禁用当前登录用户")
		return
	}
	currentUser, err := r.store.FindUserByID(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
			return
		}
		r.logger.Error("find managed user before update failed", "error", err)
		writeError(w, http.StatusInternalServerError, "update_user_failed", "保存用户失败")
		return
	}
	if isDefaultAdminUser(currentUser) && (input.Disabled || input.Username != currentUser.Username) {
		writeError(w, http.StatusBadRequest, "default_admin_protected", "默认管理员不能禁用、删除或改名")
		return
	}
	passwordHash := ""
	if input.Password != "" {
		hash, err := auth.HashPassword(input.Password)
		if err != nil {
			r.logger.Error("hash managed user password failed", "error", err)
			writeError(w, http.StatusInternalServerError, "update_user_failed", "保存用户失败")
			return
		}
		passwordHash = hash
	}
	user, err := r.store.UpdateManagedUser(req.Context(), id, repository.UserInput{
		Username: input.Username, Email: input.Email, DisplayName: input.DisplayName, RoleKeys: input.RoleKeys, Disabled: input.Disabled,
	}, passwordHash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户或角色不存在")
			return
		}
		if repository.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user_exists", "用户名已存在")
			return
		}
		r.logger.Error("update managed user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "update_user_failed", "保存用户失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user.update", "user", user.ID, repository.ClientIP(req), map[string]any{"username": user.Username})
	writeJSON(w, http.StatusOK, user)
}

func (r *router) handleSetManagedUserDisabled(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户状态格式不正确")
		return
	}
	if id == currentSession(req).User.ID && body.Disabled {
		writeError(w, http.StatusBadRequest, "disable_self_forbidden", "不能禁用当前登录用户")
		return
	}
	if body.Disabled {
		currentUser, err := r.store.FindUserByID(req.Context(), id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
				return
			}
			r.logger.Error("find managed user before disabled update failed", "error", err)
			writeError(w, http.StatusInternalServerError, "update_user_failed", "更新用户状态失败")
			return
		}
		if isDefaultAdminUser(currentUser) {
			writeError(w, http.StatusBadRequest, "default_admin_protected", "默认管理员不能禁用")
			return
		}
	}
	user, err := r.store.SetManagedUserDisabled(req.Context(), id, body.Disabled)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_user_failed", "更新用户状态失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user.disabled", "user", user.ID, repository.ClientIP(req), map[string]any{"disabled": body.Disabled})
	writeJSON(w, http.StatusOK, user)
}

func (r *router) decodeManagedUserRequest(w http.ResponseWriter, req *http.Request, create bool) (managedUserRequest, bool) {
	defer req.Body.Close()
	var input managedUserRequest
	if err := decodeJSONBody(w, req, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户参数格式不正确")
		return managedUserRequest{}, false
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.RoleKeys = normalizeRequestStrings(input.RoleKeys)
	if input.Username == "" {
		writeError(w, http.StatusBadRequest, "invalid_username", "用户名不能为空")
		return managedUserRequest{}, false
	}
	if !emailAddressPattern.MatchString(input.Email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "请输入有效的邮箱地址")
		return managedUserRequest{}, false
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	if create && len(input.Password) < 6 {
		writeError(w, http.StatusBadRequest, "invalid_password", "密码至少 6 个字符")
		return managedUserRequest{}, false
	}
	if !create && input.Password != "" && len(input.Password) < 6 {
		writeError(w, http.StatusBadRequest, "invalid_password", "密码至少 6 个字符")
		return managedUserRequest{}, false
	}
	if len(input.RoleKeys) == 0 {
		input.RoleKeys = []string{"viewer"}
	}
	return input, true
}

func (r *router) handleListRoles(w http.ResponseWriter, req *http.Request) {
	items, err := r.store.ListRoles(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_roles_failed", "读取用户角色失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (r *router) handleCreateRole(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeRoleRequest(w, req)
	if !ok {
		return
	}
	role, err := r.store.UpsertCustomRole(req.Context(), "", input)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "role_exists", "角色标识已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "save_role_failed", "保存用户角色失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.role.create", "role", role.ID, repository.ClientIP(req), map[string]any{"key": role.Key})
	writeJSON(w, http.StatusCreated, role)
}

func (r *router) handleRoleRoute(w http.ResponseWriter, req *http.Request) {
	id, _, ok := parseSettingsResourcePath(req.URL.Path, "/api/settings/roles/")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut {
		input, ok := decodeRoleRequest(w, req)
		if !ok {
			return
		}
		role, err := r.store.UpsertCustomRole(req.Context(), id, input)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "role_not_found", "内置角色不可修改或角色不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "save_role_failed", "保存用户角色失败")
			return
		}
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.role.update", "role", role.ID, repository.ClientIP(req), map[string]any{"key": role.Key})
		writeJSON(w, http.StatusOK, role)
		return
	}
	if req.Method == http.MethodDelete {
		if err := r.store.DeleteCustomRole(req.Context(), id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "role_not_found", "内置角色不可删除或角色不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "delete_role_failed", "删除用户角色失败")
			return
		}
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.role.delete", "role", id, repository.ClientIP(req), map[string]any{})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func decodeRoleRequest(w http.ResponseWriter, req *http.Request) (repository.RoleInput, bool) {
	defer req.Body.Close()
	var input roleRequest
	if err := decodeJSONBody(w, req, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "角色参数格式不正确")
		return repository.RoleInput{}, false
	}
	input.Key = strings.TrimSpace(input.Key)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Permissions = normalizeRequestStrings(input.Permissions)
	if !roleKeyPattern.MatchString(input.Key) {
		writeError(w, http.StatusBadRequest, "invalid_role_key", "角色标识需为小写字母、数字、点、下划线或连字符")
		return repository.RoleInput{}, false
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_role_name", "角色名称不能为空")
		return repository.RoleInput{}, false
	}
	return repository.RoleInput{Key: input.Key, Name: input.Name, Description: input.Description, Permissions: input.Permissions}, true
}

func (r *router) handleListUserGroups(w http.ResponseWriter, req *http.Request) {
	items, err := r.store.ListUserGroups(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_user_groups_failed", "读取用户群组失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (r *router) handleCreateUserGroup(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeUserGroupRequest(w, req)
	if !ok {
		return
	}
	group, err := r.store.UpsertUserGroup(req.Context(), "", input)
	if err != nil {
		if repository.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user_group_exists", "用户群组已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "save_user_group_failed", "保存用户群组失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user_group.create", "user_group", group.ID, repository.ClientIP(req), map[string]any{"name": group.Name})
	writeJSON(w, http.StatusCreated, group)
}

func (r *router) handleUserGroupRoute(w http.ResponseWriter, req *http.Request) {
	id, _, ok := parseSettingsResourcePath(req.URL.Path, "/api/settings/user-groups/")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut {
		input, ok := decodeUserGroupRequest(w, req)
		if !ok {
			return
		}
		group, err := r.store.UpsertUserGroup(req.Context(), id, input)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user_group_not_found", "用户群组不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "save_user_group_failed", "保存用户群组失败")
			return
		}
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user_group.update", "user_group", group.ID, repository.ClientIP(req), map[string]any{"name": group.Name})
		writeJSON(w, http.StatusOK, group)
		return
	}
	if req.Method == http.MethodDelete {
		if err := r.store.DeleteUserGroup(req.Context(), id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user_group_not_found", "用户群组不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "delete_user_group_failed", "删除用户群组失败")
			return
		}
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "settings.user_group.delete", "user_group", id, repository.ClientIP(req), map[string]any{})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func decodeUserGroupRequest(w http.ResponseWriter, req *http.Request) (repository.UserGroupInput, bool) {
	defer req.Body.Close()
	var input userGroupRequest
	if err := decodeJSONBody(w, req, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "用户群组参数格式不正确")
		return repository.UserGroupInput{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.MemberIDs = normalizeRequestStrings(input.MemberIDs)
	input.RoleKeys = normalizeRequestStrings(input.RoleKeys)
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_user_group_name", "用户群组名称不能为空")
		return repository.UserGroupInput{}, false
	}
	return repository.UserGroupInput{Name: input.Name, Description: input.Description, Disabled: input.Disabled, MemberIDs: input.MemberIDs, RoleKeys: input.RoleKeys}, true
}

func (r *router) handleListPermissions(w http.ResponseWriter, req *http.Request) {
	items := make([]domain.Permission, len(repository.BuiltinPermissions))
	copy(items, repository.BuiltinPermissions)
	for index := range items {
		items[index].ImpliedReadPermission = repository.ImpliedReadPermissions[items[index].Key]
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func parseSettingsResourcePath(path, prefix string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], "", true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func normalizeRequestStrings(values []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func isDefaultAdminUser(user domain.User) bool {
	return user.Username == "admin"
}

func hasPermission(user domain.User, permission string) bool {
	if user.Role == "admin" {
		return true
	}
	for _, item := range user.Permissions {
		if item == permission {
			return true
		}
	}
	return false
}

func hasAnyPermission(user domain.User, permissions ...string) bool {
	for _, permission := range permissions {
		if hasPermission(user, permission) {
			return true
		}
	}
	return false
}

func hasAssociatedHostReadPermission(user domain.User) bool {
	return hasAnyPermission(
		user,
		"hosts.read",
		"host.interfaces.read",
		"host.interfaces.manage",
		"agents.read",
		"agents.manage",
		"storage.read",
		"storage.manage",
		"network.read",
		"network.manage",
		"vms.read",
		"vms.create",
		"vms.update",
		"vms.power",
		"vms.delete",
		"vms.force",
		"vms.console",
		"vms.clone",
		"vms.migrate",
		"snapshots.read",
		"snapshots.create",
		"snapshots.update",
		"snapshots.revert",
		"snapshots.delete",
	)
}

func hasAssociatedVMReadPermission(user domain.User) bool {
	return hasAnyPermission(
		user,
		"vms.read",
		"vms.create",
		"vms.update",
		"vms.power",
		"vms.delete",
		"vms.force",
		"vms.console",
		"vms.clone",
		"vms.migrate",
		"host.interfaces.read",
		"host.interfaces.manage",
		"hosts.read",
		"agents.read",
		"agents.manage",
		"snapshots.read",
		"snapshots.create",
		"snapshots.update",
		"snapshots.revert",
		"snapshots.delete",
	)
}

func hasAssociatedStoragePoolReadPermission(user domain.User) bool {
	return hasAnyPermission(
		user,
		"storage.read",
		"storage.manage",
		"vms.read",
		"vms.create",
		"vms.update",
		"vms.clone",
		"vms.migrate",
	)
}

func hasAssociatedNetworkPoolReadPermission(user domain.User) bool {
	return hasAnyPermission(
		user,
		"network.read",
		"network.manage",
		"vms.read",
		"vms.create",
		"vms.update",
		"vms.clone",
		"vms.migrate",
	)
}

func (r *router) ensurePermission(w http.ResponseWriter, req *http.Request, permission string) bool {
	if hasPermission(currentSession(req).User, permission) {
		return true
	}
	writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
	return false
}

func vmRoutePermission(method, action string) string {
	if method == http.MethodGet {
		if action == "console" {
			return "vms.console"
		}
		return "vms.read"
	}
	if method == http.MethodPost && action == "" {
		return "vms.create"
	}
	if method == http.MethodPost {
		switch action {
		case "clone":
			return "vms.clone"
		case "template-mark":
			return "vms.update"
		case "template-create":
			return "vms.create"
		case "migrate", "migrate-precheck", "migrate-ssh-key", "migrate-hostname":
			return "vms.migrate"
		case "refresh":
			return "vms.read"
		case "delete":
			return "vms.delete"
		case "force-stop", "force-shutdown", "force-reboot", "force-delete":
			return "vms.force"
		default:
			return "vms.power"
		}
	}
	if method == http.MethodPut || method == http.MethodDelete {
		return "vms.update"
	}
	return ""
}
