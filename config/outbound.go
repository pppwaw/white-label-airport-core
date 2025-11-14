package config

import (
	"fmt"
	"net"

	"github.com/sagernet/sing-box/option"
)

func patchOutbound(base option.Outbound, configOpt HiddifyOptions) (*option.Outbound, string, error) {
	outbound, err := cloneOutbound(base)
	if err != nil {
		return nil, "", fmt.Errorf("error patching outbound[%s][%s]: %w", base.Tag, base.Type, err)
	}

	serverDomain := outboundServerDomain(outbound)
	patchOutboundTLSTricks(&outbound, configOpt)
	patchOutboundMux(&outbound, configOpt)

	return &outbound, serverDomain, nil
}

func patchOutboundMux(outbound *option.Outbound, configOpt HiddifyOptions) {
	if !configOpt.Mux.Enable {
		return
	}
	muxOptions := option.OutboundMultiplexOptions{
		Enabled:    true,
		Padding:    configOpt.Mux.Padding,
		Protocol:   configOpt.Mux.Protocol,
		MaxStreams: configOpt.Mux.MaxStreams,
	}

	switch opts := outbound.Options.(type) {
	case *option.VLESSOutboundOptions:
		opts.Multiplex = cloneMuxOptions(muxOptions)
	case *option.VMessOutboundOptions:
		opts.Multiplex = cloneMuxOptions(muxOptions)
	case *option.TrojanOutboundOptions:
		opts.Multiplex = cloneMuxOptions(muxOptions)
	}
}

func cloneMuxOptions(opts option.OutboundMultiplexOptions) *option.OutboundMultiplexOptions {
	clone := opts
	return &clone
}

func patchOutboundTLSTricks(outbound *option.Outbound, configOpt HiddifyOptions) {
	switch opts := outbound.Options.(type) {
	case *option.VLESSOutboundOptions:
		patchTLSOptions(opts.OutboundTLSOptionsContainer.TLS, configOpt)
	case *option.VMessOutboundOptions:
		patchTLSOptions(opts.OutboundTLSOptionsContainer.TLS, configOpt)
	case *option.TrojanOutboundOptions:
		patchTLSOptions(opts.OutboundTLSOptionsContainer.TLS, configOpt)
	}
}

func patchTLSOptions(tls *option.OutboundTLSOptions, configOpt HiddifyOptions) {
	if tls == nil || !tls.Enabled {
		return
	}
	if tls.Reality != nil && tls.Reality.Enabled {
		return
	}
	if configOpt.TLSTricks.EnableFragment {
		tls.Fragment = true
	}
}

func outboundServerDomain(outbound option.Outbound) string {
	var server string
	var detour string

	switch opts := outbound.Options.(type) {
	case *option.VLESSOutboundOptions:
		server = opts.Server
		detour = opts.DialerOptions.Detour
	case *option.VMessOutboundOptions:
		server = opts.Server
		detour = opts.DialerOptions.Detour
	case *option.TrojanOutboundOptions:
		server = opts.Server
		detour = opts.DialerOptions.Detour
	case *option.ShadowsocksOutboundOptions:
		server = opts.Server
		detour = opts.DialerOptions.Detour
	}

	if detour != "" || server == "" {
		return ""
	}
	if net.ParseIP(server) != nil {
		return ""
	}
	return fmt.Sprintf("full:%s", server)
}
