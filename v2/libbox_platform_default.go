//go:build !android && !ios

package v2

import (
	"errors"
	"log"

	"github.com/sagernet/sing-box/experimental/libbox"
)

var errUnsupportedPlatformFeature = errors.New("libbox platform feature not supported on CLI runtime")

func newPlatformInterface() libbox.PlatformInterface {
	return &cliPlatform{}
}

type cliPlatform struct{}

func (p *cliPlatform) LocalDNSTransport() libbox.LocalDNSTransport {
	return nil
}

func (p *cliPlatform) UsePlatformAutoDetectInterfaceControl() bool {
	return false
}

func (p *cliPlatform) AutoDetectInterfaceControl(fd int32) error {
	return errUnsupportedPlatformFeature
}

func (p *cliPlatform) OpenTun(options libbox.TunOptions) (int32, error) {
	return 0, errUnsupportedPlatformFeature
}

func (p *cliPlatform) WriteLog(message string) {
	log.Printf("[libbox] %s", message)
}

func (p *cliPlatform) UseProcFS() bool {
	return false
}

func (p *cliPlatform) FindConnectionOwner(ipProtocol int32, sourceAddress string, sourcePort int32, destinationAddress string, destinationPort int32) (int32, error) {
	return 0, errUnsupportedPlatformFeature
}

func (p *cliPlatform) PackageNameByUid(uid int32) (string, error) {
	return "", errUnsupportedPlatformFeature
}

func (p *cliPlatform) UIDByPackageName(packageName string) (int32, error) {
	return 0, errUnsupportedPlatformFeature
}

func (p *cliPlatform) StartDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	return nil
}

func (p *cliPlatform) CloseDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	return nil
}

func (p *cliPlatform) GetInterfaces() (libbox.NetworkInterfaceIterator, error) {
	return &networkIterator{}, nil
}

func (p *cliPlatform) UnderNetworkExtension() bool {
	return false
}

func (p *cliPlatform) IncludeAllNetworks() bool {
	return false
}

func (p *cliPlatform) ReadWIFIState() *libbox.WIFIState {
	return nil
}

func (p *cliPlatform) SystemCertificates() libbox.StringIterator {
	return newStringIterator(nil)
}

func (p *cliPlatform) ClearDNSCache() {}

func (p *cliPlatform) SendNotification(notification *libbox.Notification) error {
	return nil
}

type stringIterator struct {
	values []string
	index  int
}

func newStringIterator(values []string) libbox.StringIterator {
	return &stringIterator{values: values}
}

func (i *stringIterator) Len() int32 {
	return int32(len(i.values))
}

func (i *stringIterator) HasNext() bool {
	return i.index < len(i.values)
}

func (i *stringIterator) Next() string {
	if !i.HasNext() {
		return ""
	}
	val := i.values[i.index]
	i.index++
	return val
}

type networkIterator struct {
	values []*libbox.NetworkInterface
	index  int
}

func (n *networkIterator) HasNext() bool {
	return n.index < len(n.values)
}

func (n *networkIterator) Next() *libbox.NetworkInterface {
	if !n.HasNext() {
		return nil
	}
	value := n.values[n.index]
	n.index++
	return value
}
