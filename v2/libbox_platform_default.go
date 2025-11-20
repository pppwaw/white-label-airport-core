//go:build !android && !ios && (linux || darwin)

package v2

import (
	"errors"
	"log"
	"net"
	"strings"

	"github.com/sagernet/sing-box/experimental/libbox"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
)

var errUnsupportedPlatformFeature = errors.New("libbox platform feature not supported on CLI runtime")

func newPlatformInterface() libbox.PlatformInterface {
	interfaceFinder := control.NewDefaultInterfaceFinder()
	if err := interfaceFinder.Update(); err != nil {
		log.Printf("[libbox] failed to initialize interface finder: %v", err)
	}
	return &cliPlatform{interfaceFinder: interfaceFinder}
}

type cliPlatform struct {
	interfaceFinder *control.DefaultInterfaceFinder
	networkMonitor  tun.NetworkUpdateMonitor
	ifMonitor       tun.DefaultInterfaceMonitor
	ifMonitorElem   *list.Element[tun.DefaultInterfaceUpdateCallback]
}

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
	return openTunDevice(options, p.interfaceFinder)
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
	if p.ifMonitor != nil || p.networkMonitor != nil {
		return nil
	}
	networkMonitor, err := tun.NewNetworkUpdateMonitor(logger.NOP())
	if err != nil {
		return err
	}
	ifMonitor, err := tun.NewDefaultInterfaceMonitor(networkMonitor, logger.NOP(), tun.DefaultInterfaceMonitorOptions{
		InterfaceFinder: p.interfaceFinder,
	})
	if err != nil {
		networkMonitor.Close()
		return err
	}
	element := ifMonitor.RegisterCallback(func(defaultInterface *control.Interface, _ int) {
		if defaultInterface == nil {
			listener.UpdateDefaultInterface("", -1, false, false)
			return
		}
		listener.UpdateDefaultInterface(defaultInterface.Name, int32(defaultInterface.Index), false, false)
	})
	if err = networkMonitor.Start(); err != nil {
		ifMonitor.UnregisterCallback(element)
		networkMonitor.Close()
		return err
	}
	if err = ifMonitor.Start(); err != nil {
		ifMonitor.UnregisterCallback(element)
		networkMonitor.Close()
		return err
	}
	p.networkMonitor = networkMonitor
	p.ifMonitor = ifMonitor
	p.ifMonitorElem = element
	return nil
}

func (p *cliPlatform) CloseDefaultInterfaceMonitor(listener libbox.InterfaceUpdateListener) error {
	if p.ifMonitor != nil {
		if p.ifMonitorElem != nil {
			p.ifMonitor.UnregisterCallback(p.ifMonitorElem)
			p.ifMonitorElem = nil
		}
		p.ifMonitor.Close()
		p.ifMonitor = nil
	}
	if p.networkMonitor != nil {
		p.networkMonitor.Close()
		p.networkMonitor = nil
	}
	return nil
}

func (p *cliPlatform) GetInterfaces() (libbox.NetworkInterfaceIterator, error) {
	if err := p.interfaceFinder.Update(); err != nil {
		return nil, err
	}
	var interfaces []*libbox.NetworkInterface
	for _, iface := range p.interfaceFinder.Interfaces() {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses := make([]string, 0, len(iface.Addresses))
		for _, prefix := range iface.Addresses {
			if prefix.Addr().IsUnspecified() {
				continue
			}
			addresses = append(addresses, prefix.String())
		}
		ifaceType := classifyInterfaceType(iface.Name)
		interfaces = append(interfaces, &libbox.NetworkInterface{
			Index:     int32(iface.Index),
			MTU:       int32(iface.MTU),
			Name:      iface.Name,
			Addresses: newStringIterator(addresses),
			Flags:     int32(iface.Flags),
			Type:      ifaceType,
			DNSServer: newStringIterator(nil),
			Metered:   ifaceType == libbox.InterfaceTypeCellular,
		})
	}
	return &networkIterator{values: interfaces}, nil
}

func (p *cliPlatform) UsePlatformInterfaceGetter() bool {
	return true
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

func classifyInterfaceType(name string) int32 {
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lowerName, "rmnet"),
		strings.HasPrefix(lowerName, "ww"),
		strings.HasPrefix(lowerName, "pdp"),
		strings.HasPrefix(lowerName, "wwan"),
		strings.HasPrefix(lowerName, "cell"),
		strings.HasPrefix(lowerName, "usb"):
		return libbox.InterfaceTypeCellular
	case strings.HasPrefix(lowerName, "wl"),
		strings.HasPrefix(lowerName, "wlan"),
		strings.HasPrefix(lowerName, "wifi"),
		strings.HasPrefix(lowerName, "ath"):
		return libbox.InterfaceTypeWIFI
	case strings.HasPrefix(lowerName, "en"),
		strings.HasPrefix(lowerName, "eno"),
		strings.HasPrefix(lowerName, "ens"),
		strings.HasPrefix(lowerName, "eth"),
		strings.HasPrefix(lowerName, "em"):
		return libbox.InterfaceTypeEthernet
	default:
		return libbox.InterfaceTypeOther
	}
}
