package core

type FirewallConfig []Area

type RoleHierarchy map[string][]string

type RbacConfig struct {
	RoleHierarchy RoleHierarchy
}

type Config struct {
	Firewall FirewallConfig
	Rbac     RbacConfig
}
