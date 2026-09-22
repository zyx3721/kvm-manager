// Package buildinfo 承载构建信息，由发布构建通过 -ldflags -X 注入，源码直接编译时使用默认值。
package buildinfo

// Version、Commit、BuildDate 为版本信息变量，发布构建时通过 -ldflags -X 注入
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
