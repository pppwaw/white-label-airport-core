//go:build android || ios

package v2

import "github.com/sagernet/sing-box/experimental/libbox"

func newPlatformInterface() libbox.PlatformInterface {
	// Mobile targets provide their own platform bindings via native layers.
	return nil
}
