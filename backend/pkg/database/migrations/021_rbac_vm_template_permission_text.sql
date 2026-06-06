UPDATE roles
SET description = '系统配置、Agent 管理、删除/强制操作、虚拟机与模板等所有资源操作',
    updated_at = now()
WHERE key = 'admin' AND builtin = TRUE;

UPDATE roles
SET description = '虚拟机启停、编辑、模板创建/标记、快照、存储池/网络池日常操作，不能修改系统配置和 Agent',
    updated_at = now()
WHERE key = 'operator' AND builtin = TRUE;

UPDATE roles
SET description = '只读查看虚拟机、模板和其他资源，不能执行写操作',
    updated_at = now()
WHERE key = 'viewer' AND builtin = TRUE;
