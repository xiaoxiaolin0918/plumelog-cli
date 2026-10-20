// Package version is the plumelog CLI release identity.
package version

const (
	// Name is the binary name.
	Name = "plumelog"
	// Version is the v0 release.
	Version = "0.1.4"
	// UserAgent is sent on every HTTP request.
	UserAgent = "github.com/xiaoxiaolin0918/plumelog-cli/" + Version
)
