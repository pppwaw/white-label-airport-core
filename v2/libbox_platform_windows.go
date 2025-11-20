//go:build windows && !android && !ios

package v2

import "github.com/sagernet/sing-box/experimental/libbox"

func newPlatformInterface() libbox.PlatformInterface {
	return nil
}
