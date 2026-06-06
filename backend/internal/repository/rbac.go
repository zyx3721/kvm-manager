package repository

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

var BuiltinPermissions = []domain.Permission{
	{Key: "dashboard.read", Name: "查看总览", Description: "查看总览统计和运行态摘要", Category: "总览"},
	{Key: "hosts.read", Name: "查看宿主机", Description: "查看宿主机列表和指标", Category: "宿主机"},
	{Key: "host.interfaces.read", Name: "查看宿主机接口", Description: "查看宿主机物理网卡和桥接接口", Category: "宿主机接口"},
	{Key: "host.interfaces.manage", Name: "管理宿主机接口", Description: "创建和配置宿主机桥接接口", Category: "宿主机接口"},
	{Key: "agents.read", Name: "查看 Agent", Description: "查看 Agent 列表和连接状态", Category: "Agent"},
	{Key: "agents.manage", Name: "管理 Agent", Description: "新增、测试、同步和删除 Agent", Category: "Agent"},
	{Key: "vms.read", Name: "查看虚拟机 / 模板", Description: "查看虚拟机与模板列表、详情和监控", Category: "虚拟机"},
	{Key: "vms.create", Name: "创建虚拟机", Description: "创建虚拟机，或从已标记模板创建虚拟机", Category: "虚拟机"},
	{Key: "vms.update", Name: "编辑虚拟机 / 模板", Description: "修改虚拟机配置、介质、设备、名称，以及标记或取消模板", Category: "虚拟机"},
	{Key: "vms.power", Name: "电源操作", Description: "启动、关机、暂停、恢复和重启虚拟机", Category: "虚拟机"},
	{Key: "vms.delete", Name: "删除虚拟机", Description: "执行普通删除虚拟机", Category: "虚拟机"},
	{Key: "vms.force", Name: "强制操作", Description: "执行强制关机、强制重启和强制删除", Category: "虚拟机"},
	{Key: "vms.console", Name: "控制台", Description: "打开虚拟机控制台", Category: "虚拟机"},
	{Key: "vms.clone", Name: "克隆虚拟机", Description: "克隆普通虚拟机和磁盘", Category: "虚拟机"},
	{Key: "vms.migrate", Name: "迁移虚拟机", Description: "迁移虚拟机到其他宿主机", Category: "虚拟机"},
	{Key: "snapshots.read", Name: "查看快照", Description: "查看快照列表和详情", Category: "快照"},
	{Key: "snapshots.create", Name: "创建快照", Description: "为虚拟机创建快照", Category: "快照"},
	{Key: "snapshots.update", Name: "编辑快照", Description: "编辑快照显示名、描述和标签", Category: "快照"},
	{Key: "snapshots.revert", Name: "恢复快照", Description: "将虚拟机恢复到指定快照", Category: "快照"},
	{Key: "snapshots.delete", Name: "删除快照", Description: "删除已有快照", Category: "快照"},
	{Key: "storage.read", Name: "查看存储池", Description: "查看存储池、卷和 ISO 文件", Category: "存储池"},
	{Key: "storage.manage", Name: "管理存储池", Description: "创建、启停、删除存储池和卷", Category: "存储池"},
	{Key: "network.read", Name: "查看网络池", Description: "查看网络池配置", Category: "网络池"},
	{Key: "network.manage", Name: "管理网络池", Description: "创建、启停和删除网络池", Category: "网络池"},
	{Key: "operations.read", Name: "查看操作记录", Description: "查看任务和审计日志", Category: "任务 / 审计"},
	{Key: "alerts.manage", Name: "管理告警", Description: "处理告警和通知消息", Category: "任务 / 审计"},
	{Key: "settings.base.read", Name: "查看基础配置", Description: "查看系统品牌和基础参数规划", Category: "系统配置"},
	{Key: "settings.base.manage", Name: "管理基础配置", Description: "维护系统品牌和基础参数规划", Category: "系统配置"},
	{Key: "settings.users.read", Name: "查看用户配置", Description: "查看用户、用户群组和角色", Category: "系统配置"},
	{Key: "settings.users.manage", Name: "管理用户配置", Description: "维护用户、用户群组和角色", Category: "系统配置"},
	{Key: "settings.auth.read", Name: "查看认证配置", Description: "查看 AD/LDAP 等外部认证", Category: "系统配置"},
	{Key: "settings.auth.manage", Name: "管理认证配置", Description: "维护 AD/LDAP 等外部认证", Category: "系统配置"},
	{Key: "settings.notifications.read", Name: "查看通知配置", Description: "查看外部通知媒介", Category: "系统配置"},
	{Key: "settings.notifications.manage", Name: "管理通知配置", Description: "维护外部通知媒介", Category: "系统配置"},
}

type UserInput struct {
	Username    string
	Password    string
	Email       string
	DisplayName string
	Source      string
	RoleKeys    []string
	Disabled    bool
}

type RoleInput struct {
	Key         string
	Name        string
	Description string
	Permissions []string
}

type UserGroupInput struct {
	Name        string
	Description string
	Disabled    bool
	MemberIDs   []string
	RoleKeys    []string
}

func (s *Store) ListManagedUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, username, COALESCE(email, ''), display_name, role, COALESCE(NULLIF(source, ''), 'local'), disabled, last_login_at, created_at, updated_at
		FROM users ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachAccessToUsers(ctx, users)
}

func (s *Store) CreateManagedUser(ctx context.Context, input UserInput, passwordHash string) (domain.User, error) {
	roleKeys := normalizeRoleKeys(input.RoleKeys)
	source := normalizeUserSource(input.Source)
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users(username, email, password_hash, display_name, role, source, disabled)
		VALUES($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, username, COALESCE(email, ''), display_name, role, COALESCE(NULLIF(source, ''), 'local'), disabled, last_login_at, created_at, updated_at
	`, input.Username, input.Email, passwordHash, input.DisplayName, roleKeys[0], source, input.Disabled).Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.replaceUserRoles(ctx, user.ID, roleKeys); err != nil {
		return domain.User{}, err
	}
	return s.FindUserByID(ctx, user.ID)
}

func (s *Store) UpdateManagedUser(ctx context.Context, id string, input UserInput, passwordHash string) (domain.User, error) {
	roleKeys := normalizeRoleKeys(input.RoleKeys)
	source := normalizeUserSource(input.Source)
	if passwordHash == "" {
		cmd, err := s.pool.Exec(ctx, `
			UPDATE users SET username=$2, email=$3, display_name=$4, role=$5, source=$6, disabled=$7, updated_at=now()
			WHERE id=$1
		`, id, input.Username, input.Email, input.DisplayName, roleKeys[0], source, input.Disabled)
		if err != nil {
			return domain.User{}, err
		}
		if cmd.RowsAffected() == 0 {
			return domain.User{}, ErrNotFound
		}
	} else {
		cmd, err := s.pool.Exec(ctx, `
			UPDATE users SET username=$2, email=$3, password_hash=$4, display_name=$5, role=$6, source=$7, disabled=$8, updated_at=now()
			WHERE id=$1
		`, id, input.Username, input.Email, passwordHash, input.DisplayName, roleKeys[0], source, input.Disabled)
		if err != nil {
			return domain.User{}, err
		}
		if cmd.RowsAffected() == 0 {
			return domain.User{}, ErrNotFound
		}
	}
	if err := s.replaceUserRoles(ctx, id, roleKeys); err != nil {
		return domain.User{}, err
	}
	return s.FindUserByID(ctx, id)
}

func (s *Store) SetManagedUserDisabled(ctx context.Context, id string, disabled bool) (domain.User, error) {
	cmd, err := s.pool.Exec(ctx, `UPDATE users SET disabled=$2, updated_at=now() WHERE id=$1`, id, disabled)
	if err != nil {
		return domain.User{}, err
	}
	if cmd.RowsAffected() == 0 {
		return domain.User{}, ErrNotFound
	}
	return s.FindUserByID(ctx, id)
}

func (s *Store) DeleteManagedUser(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) FindUserByID(ctx context.Context, id string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, username, COALESCE(email, ''), display_name, role, COALESCE(NULLIF(source, ''), 'local'), disabled, last_login_at, created_at, updated_at
		FROM users WHERE id=$1
	`, id).Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	users, err := s.attachAccessToUsers(ctx, []domain.User{user})
	if err != nil {
		return domain.User{}, err
	}
	return users[0], nil
}

func (s *Store) ListRoles(ctx context.Context) ([]domain.Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, key, name, description, permissions, builtin, created_at, updated_at
		FROM roles ORDER BY builtin DESC, CASE key WHEN 'admin' THEN 1 WHEN 'operator' THEN 2 WHEN 'viewer' THEN 3 ELSE 9 END, key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]domain.Role, 0)
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) UpsertCustomRole(ctx context.Context, id string, input RoleInput) (domain.Role, error) {
	var role domain.Role
	input.Permissions = normalizePermissions(input.Permissions)
	if id == "" {
		err := s.pool.QueryRow(ctx, `
			INSERT INTO roles(key, name, description, permissions, builtin)
			VALUES($1, $2, $3, $4, false)
			RETURNING id::text, key, name, description, permissions, builtin, created_at, updated_at
		`, input.Key, input.Name, input.Description, input.Permissions).Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt)
		return role, err
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE roles SET key=$2, name=$3, description=$4, permissions=$5, updated_at=now()
		WHERE id=$1 AND builtin=false
		RETURNING id::text, key, name, description, permissions, builtin, created_at, updated_at
	`, id, input.Key, input.Name, input.Description, input.Permissions).Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Role{}, ErrNotFound
	}
	return role, err
}

func (s *Store) DeleteCustomRole(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND builtin=false`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUserGroups(ctx context.Context) ([]domain.UserGroup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, description, disabled, created_at, updated_at
		FROM user_groups ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]domain.UserGroup, 0)
	for rows.Next() {
		var group domain.UserGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.Disabled, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range groups {
		members, err := s.listGroupMembers(ctx, groups[index].ID)
		if err != nil {
			return nil, err
		}
		roles, err := s.listGroupRoles(ctx, groups[index].ID)
		if err != nil {
			return nil, err
		}
		groups[index].Members = members
		groups[index].Roles = roles
	}
	return groups, nil
}

func (s *Store) UpsertUserGroup(ctx context.Context, id string, input UserGroupInput) (domain.UserGroup, error) {
	var group domain.UserGroup
	if id == "" {
		err := s.pool.QueryRow(ctx, `
			INSERT INTO user_groups(name, description, disabled)
			VALUES($1, $2, $3)
			RETURNING id::text, name, description, disabled, created_at, updated_at
		`, input.Name, input.Description, input.Disabled).Scan(&group.ID, &group.Name, &group.Description, &group.Disabled, &group.CreatedAt, &group.UpdatedAt)
		if err != nil {
			return domain.UserGroup{}, err
		}
	} else {
		err := s.pool.QueryRow(ctx, `
			UPDATE user_groups SET name=$2, description=$3, disabled=$4, updated_at=now()
			WHERE id=$1
			RETURNING id::text, name, description, disabled, created_at, updated_at
		`, id, input.Name, input.Description, input.Disabled).Scan(&group.ID, &group.Name, &group.Description, &group.Disabled, &group.CreatedAt, &group.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserGroup{}, ErrNotFound
		}
		if err != nil {
			return domain.UserGroup{}, err
		}
	}
	if err := s.replaceGroupMembers(ctx, group.ID, input.MemberIDs); err != nil {
		return domain.UserGroup{}, err
	}
	if err := s.replaceGroupRoles(ctx, group.ID, normalizeRoleKeys(input.RoleKeys)); err != nil {
		return domain.UserGroup{}, err
	}
	groups, err := s.ListUserGroups(ctx)
	if err != nil {
		return domain.UserGroup{}, err
	}
	for _, item := range groups {
		if item.ID == group.ID {
			return item, nil
		}
	}
	return domain.UserGroup{}, ErrNotFound
}

func (s *Store) DeleteUserGroup(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM user_groups WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) replaceUserRoles(ctx context.Context, userID string, roleKeys []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, roleKey := range roleKeys {
		cmd, err := s.pool.Exec(ctx, `
			INSERT INTO user_roles(user_id, role_id)
			SELECT $1, id FROM roles WHERE key=$2
			ON CONFLICT DO NOTHING
		`, userID, roleKey)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return ErrNotFound
		}
	}
	return nil
}

func (s *Store) replaceGroupMembers(ctx context.Context, groupID string, memberIDs []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM user_group_members WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	for _, memberID := range normalizeIDs(memberIDs) {
		if _, err := s.pool.Exec(ctx, `INSERT INTO user_group_members(group_id, user_id) VALUES($1, $2) ON CONFLICT DO NOTHING`, groupID, memberID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) replaceGroupRoles(ctx context.Context, groupID string, roleKeys []string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM user_group_roles WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	for _, roleKey := range roleKeys {
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO user_group_roles(group_id, role_id)
			SELECT $1, id FROM roles WHERE key=$2
			ON CONFLICT DO NOTHING
		`, groupID, roleKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachAccessToUsers(ctx context.Context, users []domain.User) ([]domain.User, error) {
	for index := range users {
		roles, err := s.listUserEffectiveRoles(ctx, users[index].ID)
		if err != nil {
			return nil, err
		}
		users[index].Roles = roles
		users[index].Permissions = uniquePermissions(roles)
	}
	return users, nil
}

func (s *Store) listUserEffectiveRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT r.id::text, r.key, r.name, r.description, r.permissions, r.builtin, r.created_at, r.updated_at,
		       CASE r.key WHEN 'admin' THEN 1 WHEN 'operator' THEN 2 WHEN 'viewer' THEN 3 ELSE 9 END AS sort_order
		FROM roles r
		WHERE r.id IN (
			SELECT role_id FROM user_roles WHERE user_id=$1
			UNION
			SELECT ugr.role_id
			FROM user_group_roles ugr
			JOIN user_group_members ugm ON ugm.group_id = ugr.group_id
			JOIN user_groups g ON g.id = ugm.group_id AND g.disabled = false
			WHERE ugm.user_id=$1
		)
		ORDER BY sort_order, r.key
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]domain.Role, 0)
	for rows.Next() {
		var role domain.Role
		var sortOrder int
		if err := rows.Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt, &sortOrder); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return s.fallbackRole(ctx, userID)
	}
	return roles, nil
}

func (s *Store) fallbackRole(ctx context.Context, userID string) ([]domain.Role, error) {
	var roleKey string
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(NULLIF(role, ''), 'viewer') FROM users WHERE id=$1`, userID).Scan(&roleKey); err != nil {
		return nil, err
	}
	var role domain.Role
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, key, name, description, permissions, builtin, created_at, updated_at
		FROM roles WHERE key=$1
	`, roleKey).Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return []domain.Role{}, nil
	}
	if err != nil {
		return nil, err
	}
	_ = s.replaceUserRoles(ctx, userID, []string{role.Key})
	return []domain.Role{role}, nil
}

func (s *Store) listGroupMembers(ctx context.Context, groupID string) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id::text, u.username, COALESCE(u.email, ''), u.display_name, u.role, COALESCE(NULLIF(u.source, ''), 'local'), u.disabled, u.last_login_at, u.created_at, u.updated_at
		FROM users u JOIN user_group_members m ON m.user_id = u.id
		WHERE m.group_id=$1
		ORDER BY u.username
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachAccessToUsers(ctx, users)
}

func (s *Store) listGroupRoles(ctx context.Context, groupID string) ([]domain.Role, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text, r.key, r.name, r.description, r.permissions, r.builtin, r.created_at, r.updated_at
		FROM roles r JOIN user_group_roles gr ON gr.role_id = r.id
		WHERE gr.group_id=$1
		ORDER BY CASE r.key WHEN 'admin' THEN 1 WHEN 'operator' THEN 2 WHEN 'viewer' THEN 3 ELSE 9 END, r.key
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]domain.Role, 0)
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Key, &role.Name, &role.Description, &role.Permissions, &role.Builtin, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func normalizeRoleKeys(keys []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	if len(normalized) == 0 {
		return []string{"viewer"}
	}
	return normalized
}

func normalizeUserSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "ldap":
		return "ldap"
	default:
		return "local"
	}
}

func normalizeIDs(ids []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func normalizePermissions(permissions []string) []string {
	seen := map[string]struct{}{}
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		seen[permission] = struct{}{}
		if readPermission, ok := impliedReadPermissions[permission]; ok {
			seen[readPermission] = struct{}{}
		}
	}
	items := make([]string, 0, len(seen))
	for permission := range seen {
		items = append(items, permission)
	}
	sort.Strings(items)
	return items
}

var ImpliedReadPermissions = map[string]string{
	"agents.manage":                 "agents.read",
	"host.interfaces.manage":        "host.interfaces.read",
	"vms.create":                    "vms.read",
	"vms.update":                    "vms.read",
	"vms.power":                     "vms.read",
	"vms.delete":                    "vms.read",
	"vms.force":                     "vms.read",
	"vms.console":                   "vms.read",
	"vms.clone":                     "vms.read",
	"vms.migrate":                   "vms.read",
	"snapshots.create":              "snapshots.read",
	"snapshots.update":              "snapshots.read",
	"snapshots.revert":              "snapshots.read",
	"snapshots.delete":              "snapshots.read",
	"storage.manage":                "storage.read",
	"network.manage":                "network.read",
	"alerts.manage":                 "operations.read",
	"settings.base.manage":          "settings.base.read",
	"settings.users.manage":         "settings.users.read",
	"settings.auth.manage":          "settings.auth.read",
	"settings.notifications.manage": "settings.notifications.read",
}

var impliedReadPermissions = ImpliedReadPermissions

func uniquePermissions(roles []domain.Role) []string {
	seen := map[string]struct{}{}
	for _, role := range roles {
		for _, permission := range role.Permissions {
			permission = strings.TrimSpace(permission)
			if permission != "" {
				seen[permission] = struct{}{}
			}
		}
	}
	items := make([]string, 0, len(seen))
	for permission := range seen {
		items = append(items, permission)
	}
	sort.Strings(items)
	return items
}
