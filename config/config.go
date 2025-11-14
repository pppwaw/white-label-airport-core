package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	dns "github.com/sagernet/sing-dns"

	json "github.com/sagernet/sing/common/json"
	badoption "github.com/sagernet/sing/common/json/badoption"
)

const (
	DNSRemoteTag       = "dns-remote"
	DNSLocalTag        = "dns-local"
	DNSDirectTag       = "dns-direct"
	DNSBlockTag        = "dns-block"
	DNSFakeTag         = "dns-fake"
	DNSTricksDirectTag = "dns-trick-direct"

	OutboundDirectTag         = "direct"
	OutboundBypassTag         = "bypass"
	OutboundBlockTag          = "block"
	OutboundSelectTag         = "select"
	OutboundURLTestTag        = "urltest"
	OutboundDNSTag            = "dns-out"
	OutboundDirectFragmentTag = "direct-fragment"

	InboundTUNTag   = "tun-in"
	InboundMixedTag = "mixed-in"
	InboundDNSTag   = "dns-in"
)

var OutboundMainProxyTag = OutboundSelectTag

func BuildConfigJson(configOpt HiddifyOptions, input option.Options) (string, error) {
	options, err := BuildConfig(configOpt, input)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoderContext(OptionsContext(), &buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(options)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func BuildConfig(opt HiddifyOptions, input option.Options) (*option.Options, error) {
	fmt.Printf("config options: %++v\n", opt)

	var options option.Options
	if opt.EnableFullConfig {
		options.Inbounds = input.Inbounds
		options.DNS = input.DNS
		options.Route = input.Route
	}

	setClashAPI(&options, &opt)
	setLog(&options, &opt)
	setInbound(&options, &opt)
	setDns(&options, &opt, &input)
	setRoutingOptions(&options, &opt)
	setFakeDns(&options, &opt)
	rewriteRCodeDNSServers(options.DNS)
	err := setOutbounds(&options, &input, &opt)
	if err != nil {
		return nil, err
	}

	return &options, nil
}

func addForceDirect(options *option.Options, opt *HiddifyOptions, directDNSDomains map[string]bool) {
	remoteDNSAddress := opt.RemoteDnsAddress
	if strings.Contains(remoteDNSAddress, "://") {
		remoteDNSAddress = strings.SplitAfter(remoteDNSAddress, "://")[1]
	}
	parsedUrl, err := url.Parse(fmt.Sprintf("https://%s", remoteDNSAddress))
	if err == nil && net.ParseIP(parsedUrl.Host) == nil {
		directDNSDomains[parsedUrl.Host] = true
	}
	if len(directDNSDomains) > 0 {
		// trickDnsDomains := []string{}
		// directDNSDomains = removeDuplicateStr(directDNSDomains)
		// b, _ := batch.New(context.Background(), batch.WithConcurrencyNum[bool](10))
		// for _, d := range directDNSDomains {
		// 	b.Go(d, func() (bool, error) {
		// 		return isBlockedDomain(d), nil
		// 	})
		// }
		// b.Wait()
		// for domain, isBlock := range b.Result() {
		// 	if isBlock.Value {
		// 		trickDnsDomains = append(trickDnsDomains, domain)
		// 	}
		// }

		// trickDomains := strings.Join(trickDnsDomains, ",")
		// trickRule := Rule{Domains: trickDomains, Outbound: OutboundBypassTag}
		// trickDnsRule := trickRule.MakeDNSRule()
		// trickDnsRule.Server = DNSTricksDirectTag
		// options.DNS.Rules = append([]option.DNSRule{{Type: C.RuleTypeDefault, DefaultOptions: trickDnsRule}}, options.DNS.Rules...)

		directDNSDomainskeys := make([]string, 0, len(directDNSDomains))
		for key := range directDNSDomains {
			directDNSDomainskeys = append(directDNSDomainskeys, key)
		}

		domains := strings.Join(directDNSDomainskeys, ",")
		directRule := Rule{Domains: domains, Outbound: OutboundBypassTag}
		dnsRule := directRule.MakeDNSRule()
		dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSDirectTag)
		options.DNS.Rules = append([]option.DNSRule{{Type: C.RuleTypeDefault, DefaultOptions: dnsRule}}, options.DNS.Rules...)
	}
}

func setOutbounds(options *option.Options, input *option.Options, opt *HiddifyOptions) error {
	directDNSDomains := make(map[string]bool)
	var outbounds []option.Outbound
	var tags []string
	for _, out := range input.Outbounds {
		outbound, serverDomain, err := patchOutbound(out, *opt)
		if err != nil {
			return err
		}

		if serverDomain != "" {
			directDNSDomains[serverDomain] = true
		}
		switch outbound.Type {
		case C.TypeDirect, C.TypeBlock, C.TypeDNS:
			continue
		case C.TypeSelector, C.TypeURLTest:
			continue
		default:
			if !strings.Contains(outbound.Tag, "§hide§") {
				tags = append(tags, outbound.Tag)
			}
			outbounds = append(outbounds, *outbound)
		}
	}

	urlTest := option.Outbound{
		Type: C.TypeURLTest,
		Tag:  OutboundURLTestTag,
		Options: option.URLTestOutboundOptions{
			Outbounds: tags,
			URL:       opt.ConnectionTestUrl,
			Interval:  badoption.Duration(opt.URLTestInterval.Duration()),
			Tolerance: 1,
			IdleTimeout: badoption.Duration(
				opt.URLTestInterval.Duration() * 3,
			),
			InterruptExistConnections: true,
		},
	}
	defaultSelect := urlTest.Tag

	for _, tag := range tags {
		if strings.Contains(tag, "§default§") {
			defaultSelect = "§default§"
		}
	}
	selector := option.Outbound{
		Type: C.TypeSelector,
		Tag:  OutboundSelectTag,
		Options: option.SelectorOutboundOptions{
			Outbounds:                 append([]string{urlTest.Tag}, tags...),
			Default:                   defaultSelect,
			InterruptExistConnections: true,
		},
	}

	outbounds = append([]option.Outbound{selector, urlTest}, outbounds...)

	options.Outbounds = append(
		outbounds,
		[]option.Outbound{
			{
				Tag:     OutboundDNSTag,
				Type:    C.TypeDNS,
				Options: option.StubOptions{},
			},
			{
				Tag:  OutboundDirectTag,
				Type: C.TypeDirect,
				Options: option.DirectOutboundOptions{
					DialerOptions: option.DialerOptions{
						TCPFastOpen: false,
					},
				},
			},
			{
				Tag:  OutboundDirectFragmentTag,
				Type: C.TypeDirect,
				Options: option.DirectOutboundOptions{
					DialerOptions: option.DialerOptions{
						TCPFastOpen: false,
					},
				},
			},
			{
				Tag:     OutboundBypassTag,
				Type:    C.TypeDirect,
				Options: option.DirectOutboundOptions{},
			},
			{
				Tag:     OutboundBlockTag,
				Type:    C.TypeBlock,
				Options: option.StubOptions{},
			},
		}...,
	)

	addForceDirect(options, opt, directDNSDomains)
	return nil
}

func setClashAPI(options *option.Options, opt *HiddifyOptions) {
	if opt.EnableClashApi {
		if opt.ClashApiSecret == "" {
			opt.ClashApiSecret = generateRandomString(16)
		}
		options.Experimental = &option.ExperimentalOptions{
			ClashAPI: &option.ClashAPIOptions{
				ExternalController: fmt.Sprintf("%s:%d", "127.0.0.1", opt.ClashApiPort),
				Secret:             opt.ClashApiSecret,
			},

			CacheFile: &option.CacheFileOptions{
				Enabled: true,
				Path:    "clash.db",
			},
		}
	}
}

func setLog(options *option.Options, opt *HiddifyOptions) {
	options.Log = &option.LogOptions{
		Level:        opt.LogLevel,
		Output:       opt.LogFile,
		Disabled:     false,
		Timestamp:    true,
		DisableColor: true,
	}
}

func setInbound(options *option.Options, opt *HiddifyOptions) {
	var inboundDomainStrategy option.DomainStrategy
	if !opt.ResolveDestination {
		inboundDomainStrategy = option.DomainStrategy(dns.DomainStrategyAsIS)
	} else {
		inboundDomainStrategy = opt.IPv6Mode
	}
	if opt.EnableTunService {
		ActivateTunnelService(*opt)
	} else if opt.EnableTun {
		tunOptions := option.TunInboundOptions{
			Stack:                  opt.TUNStack,
			MTU:                    opt.MTU,
			AutoRoute:              true,
			StrictRoute:            opt.StrictRoute,
			EndpointIndependentNat: true,
			InboundOptions: option.InboundOptions{
				SniffEnabled:             true,
				SniffOverrideDestination: false,
				DomainStrategy:           inboundDomainStrategy,
			},
		}
		switch opt.IPv6Mode {
		case option.DomainStrategy(dns.DomainStrategyUseIPv4):
			tunOptions.Address = append(tunOptions.Address, netip.MustParsePrefix("172.19.0.1/28"))
		case option.DomainStrategy(dns.DomainStrategyUseIPv6):
			tunOptions.Address = append(tunOptions.Address, netip.MustParsePrefix("fdfe:dcba:9876::1/126"))
		default:
			tunOptions.Address = append(tunOptions.Address,
				netip.MustParsePrefix("172.19.0.1/28"),
				netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
			)
		}
		options.Inbounds = append(options.Inbounds, option.Inbound{
			Type:    C.TypeTun,
			Tag:     InboundTUNTag,
			Options: tunOptions,
		})

	}

	var bind string
	if opt.AllowConnectionFromLAN {
		bind = "0.0.0.0"
	} else {
		bind = "127.0.0.1"
	}

	options.Inbounds = append(
		options.Inbounds,
		option.Inbound{
			Type: C.TypeMixed,
			Tag:  InboundMixedTag,
			Options: option.HTTPMixedInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen: func() *badoption.Addr {
						addr := badoption.Addr(netip.MustParseAddr(bind))
						return &addr
					}(),
					ListenPort: opt.MixedPort,
					InboundOptions: option.InboundOptions{
						SniffEnabled:             true,
						SniffOverrideDestination: true,
						DomainStrategy:           inboundDomainStrategy,
					},
				},
				SetSystemProxy: opt.SetSystemProxy,
			},
		},
	)

	options.Inbounds = append(
		options.Inbounds,
		option.Inbound{
			Type: C.TypeDirect,
			Tag:  InboundDNSTag,
			Options: option.DirectInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen: func() *badoption.Addr {
						addr := badoption.Addr(netip.MustParseAddr(bind))
						return &addr
					}(),
					ListenPort: opt.LocalDnsPort,
				},
			},
		},
	)
}

func setDns(options *option.Options, opt *HiddifyOptions, input *option.Options) {
	if input != nil && input.DNS != nil && len(input.DNS.Servers) > 0 {
		if cloned := cloneDNSOptions(input.DNS); cloned != nil {
			options.DNS = cloned
			return
		}
	}
	servers := []option.DNSServerOptions{
		buildDNSServer(DNSRemoteTag, opt.RemoteDnsAddress, DNSDirectTag, opt.RemoteDnsDomainStrategy, ""),
		buildDNSServer(DNSDirectTag, opt.DirectDnsAddress, DNSLocalTag, opt.DirectDnsDomainStrategy, ""),
		buildDNSServer(DNSLocalTag, "local", "", option.DomainStrategy(0), ""),
		buildDNSServer(DNSBlockTag, "rcode://success", "", option.DomainStrategy(0), ""),
	}

	options.DNS = &option.DNSOptions{
		RawDNSOptions: option.RawDNSOptions{
			DNSClientOptions: option.DNSClientOptions{
				IndependentCache: opt.IndependentDNSCache,
			},
			Final:   DNSRemoteTag,
			Servers: servers,
		},
	}
}

func rewriteRCodeDNSServers(dns *option.DNSOptions) {
	if dns == nil || len(dns.Servers) == 0 {
		return
	}
	rcodeServers := make(map[string]int)
	filteredServers := make([]option.DNSServerOptions, 0, len(dns.Servers))
	for _, server := range dns.Servers {
		if server.Type == C.DNSTypeLegacyRcode {
			if rcode, ok := server.Options.(int); ok {
				rcodeServers[server.Tag] = rcode
			}
			continue
		}
		filteredServers = append(filteredServers, server)
	}
	dns.Servers = filteredServers
	if len(rcodeServers) == 0 || len(dns.Rules) == 0 {
		return
	}
	for i := range dns.Rules {
		rewriteDNSRuleRCode(rcodeServers, &dns.Rules[i])
	}
}

func rewriteDNSRuleRCode(rcodeServers map[string]int, rule *option.DNSRule) {
	switch rule.Type {
	case C.RuleTypeLogical:
		rule.LogicalOptions.DNSRuleAction = rewriteDNSRuleActionRCode(rcodeServers, rule.LogicalOptions.DNSRuleAction)
		for i := range rule.LogicalOptions.Rules {
			rewriteDNSRuleRCode(rcodeServers, &rule.LogicalOptions.Rules[i])
		}
	default:
		rule.DefaultOptions.DNSRuleAction = rewriteDNSRuleActionRCode(rcodeServers, rule.DefaultOptions.DNSRuleAction)
	}
}

func rewriteDNSRuleActionRCode(rcodeServers map[string]int, action option.DNSRuleAction) option.DNSRuleAction {
	if action.Action != C.RuleActionTypeRoute {
		return action
	}
	rcode, ok := rcodeServers[action.RouteOptions.Server]
	if !ok {
		return action
	}
	action.Action = C.RuleActionTypePredefined
	value := option.DNSRCode(rcode)
	action.PredefinedOptions.Rcode = &value
	action.RouteOptions = option.DNSRouteActionOptions{}
	return action
}

func cloneDNSOptions(src *option.DNSOptions) *option.DNSOptions {
	content, err := json.MarshalContext(OptionsContext(), src)
	if err != nil {
		fmt.Printf("failed to marshal dns options: %v\n", err)
		return nil
	}
	var dst option.DNSOptions
	if err := json.UnmarshalContext(OptionsContext(), content, &dst); err != nil {
		fmt.Printf("failed to unmarshal dns options: %v\n", err)
		return nil
	}
	return &dst
}

func buildDNSServer(tag, address, resolver string, strategy option.DomainStrategy, detour string) option.DNSServerOptions {
	server := option.DNSServerOptions{
		Tag:  tag,
		Type: C.DNSTypeLegacy,
		Options: &option.LegacyDNSServerOptions{
			Address:         address,
			AddressResolver: resolver,
			Strategy:        strategy,
			Detour:          detour,
		},
	}
	if err := server.Upgrade(OptionsContext()); err != nil {
		fmt.Printf("failed to upgrade DNS server %s (%s): %v\n", tag, address, err)
	}
	return server
}

func setFakeDns(options *option.Options, opt *HiddifyOptions) {
	if opt.EnableFakeDNS {
		inet4Range := netip.MustParsePrefix("198.18.0.0/15")
		inet6Range := netip.MustParsePrefix("fc00::/18")
		inet4Prefix := badoption.Prefix(inet4Range)
		inet6Prefix := badoption.Prefix(inet6Range)
		options.DNS.RawDNSOptions.Servers = append(
			options.DNS.RawDNSOptions.Servers,
			option.DNSServerOptions{
				Tag:  DNSFakeTag,
				Type: C.DNSTypeFakeIP,
				Options: option.FakeIPDNSServerOptions{
					Inet4Range: &inet4Prefix,
					Inet6Range: &inet6Prefix,
				},
			},
		)
		dnsRule := option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Inbound: []string{InboundTUNTag},
			},
		}
		dnsRule.DNSRuleAction = option.DNSRuleAction{
			Action: C.RuleActionTypeRoute,
			RouteOptions: option.DNSRouteActionOptions{
				Server:       DNSFakeTag,
				DisableCache: true,
			},
		}
		options.DNS.Rules = append(
			options.DNS.Rules,
			option.DNSRule{Type: C.RuleTypeDefault, DefaultOptions: dnsRule},
		)

	}
}

func routeActionForOutbound(tag string) option.RuleAction {
	if tag == "" {
		tag = OutboundMainProxyTag
	}
	return option.RuleAction{
		Action: C.RuleActionTypeRoute,
		RouteOptions: option.RouteActionOptions{
			Outbound: tag,
		},
	}
}

func dnsRouteActionForServer(server string) option.DNSRuleAction {
	return option.DNSRuleAction{
		Action: C.RuleActionTypeRoute,
		RouteOptions: option.DNSRouteActionOptions{
			Server: server,
		},
	}
}

func resolveRuleOutbound(selection string) string {
	switch selection {
	case "bypass":
		return OutboundBypassTag
	case "block":
		return OutboundBlockTag
	case "proxy", "":
		return OutboundMainProxyTag
	default:
		return selection
	}
}

func setRoutingOptions(options *option.Options, opt *HiddifyOptions) {
	dnsRules := []option.DefaultDNSRule{}
	routeRules := []option.Rule{}
	rulesets := []option.RuleSet{}

	if opt.EnableTun && runtime.GOOS == "android" {
		routeRules = append(
			routeRules,
			option.Rule{
				Type: C.RuleTypeDefault,

				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{
						Inbound:     []string{InboundTUNTag},
						PackageName: []string{"app.hiddify.com"},
					},
					RuleAction: routeActionForOutbound(OutboundBypassTag),
				},
			},
		)
		// routeRules = append(
		// 	routeRules,
		// 	option.Rule{
		// 		Type: C.RuleTypeDefault,
		// 		DefaultOptions: option.DefaultRule{
		// 			ProcessName: []string{"Hiddify", "Hiddify.exe", "HiddifyCli", "HiddifyCli.exe"},
		// 			Outbound:    OutboundBypassTag,
		// 		},
		// 	},
		// )
	}
	routeRules = append(routeRules, option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Inbound: []string{InboundDNSTag},
			},
			RuleAction: routeActionForOutbound(OutboundDNSTag),
		},
	})
	routeRules = append(routeRules, option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Port: []uint16{53},
			},
			RuleAction: routeActionForOutbound(OutboundDNSTag),
		},
	})

	// {
	// 	Type: C.RuleTypeDefault,
	// 	DefaultOptions: option.DefaultRule{
	// 		ClashMode: "Direct",
	// 		Outbound:  OutboundDirectTag,
	// 	},
	// },
	// {
	// 	Type: C.RuleTypeDefault,
	// 	DefaultOptions: option.DefaultRule{
	// 		ClashMode: "Global",
	// 		Outbound:  OutboundMainProxyTag,
	// 	},
	// },	}

	if opt.BypassLAN {
		routeRules = append(
			routeRules,
			option.Rule{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{
						IPIsPrivate: true,
					},
					RuleAction: routeActionForOutbound(OutboundBypassTag),
				},
			},
		)
	}

	for _, rule := range opt.Rules {
		routeRule := rule.MakeRule()
		targetOutbound := resolveRuleOutbound(rule.Outbound)
		routeRule.RuleAction = routeActionForOutbound(targetOutbound)

		if routeRule.IsValid() {
			routeRules = append(
				routeRules,
				option.Rule{
					Type:           C.RuleTypeDefault,
					DefaultOptions: routeRule,
				},
			)
		}

		dnsRule := rule.MakeDNSRule()
		switch targetOutbound {
		case OutboundBypassTag:
			dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSDirectTag)
		case OutboundBlockTag:
			dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSBlockTag)
			dnsRule.DNSRuleAction.RouteOptions.DisableCache = true
		default:
			if opt.EnableFakeDNS {
				fakeDnsRule := dnsRule
				fakeDnsRule.DNSRuleAction = dnsRouteActionForServer(DNSFakeTag)
				fakeDnsRule.RawDefaultDNSRule.Inbound = []string{InboundTUNTag, InboundMixedTag}
				dnsRules = append(dnsRules, fakeDnsRule)
			}
			dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSRemoteTag)
		}
		dnsRules = append(dnsRules, dnsRule)
	}

	parsedURL, err := url.Parse(opt.ConnectionTestUrl)
	if err == nil {
		var dnsCPttl uint32 = 3000
		dnsRule := option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				Domain: []string{parsedURL.Host},
			},
		}
		dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSRemoteTag)
		dnsRule.DNSRuleAction.RouteOptions.RewriteTTL = &dnsCPttl
		dnsRules = append(dnsRules, dnsRule)
	}

	if opt.BlockAds {
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geosite-ads",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geosite-category-ads-all.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geosite-malware",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geosite-malware.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geosite-phishing",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geosite-phishing.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geosite-cryptominers",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geosite-cryptominers.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geoip-phishing",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geoip-phishing.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geoip-malware",
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/block/geoip-malware.srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})

		routeRules = append(routeRules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					RuleSet: []string{
						"geosite-ads",
						"geosite-malware",
						"geosite-phishing",
						"geosite-cryptominers",
						"geoip-malware",
						"geoip-phishing",
					},
				},
				RuleAction: routeActionForOutbound(OutboundBlockTag),
			},
		})
		dnsRule := option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: []string{
					"geosite-ads",
					"geosite-malware",
					"geosite-phishing",
					"geosite-cryptominers",
					"geoip-malware",
					"geoip-phishing",
				},
			},
		}
		dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSBlockTag)
		dnsRules = append(dnsRules, dnsRule)

	}
	if opt.Region != "other" {
		dnsRule := option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				DomainSuffix: []string{"." + opt.Region},
			},
		}
		dnsRule.DNSRuleAction = dnsRouteActionForServer(DNSDirectTag)
		dnsRules = append(dnsRules, dnsRule)
		routeRules = append(routeRules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					DomainSuffix: []string{"." + opt.Region},
				},
				RuleAction: routeActionForOutbound(OutboundDirectTag),
			},
		})
		directRule := option.DefaultDNSRule{
			RawDefaultDNSRule: option.RawDefaultDNSRule{
				RuleSet: []string{
					"geoip-" + opt.Region,
					"geosite-" + opt.Region,
				},
			},
		}
		directRule.DNSRuleAction = dnsRouteActionForServer(DNSDirectTag)
		dnsRules = append(dnsRules, directRule)

		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geoip-" + opt.Region,
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/country/geoip-" + opt.Region + ".srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})
		rulesets = append(rulesets, option.RuleSet{
			Type:   C.RuleSetTypeRemote,
			Tag:    "geosite-" + opt.Region,
			Format: C.RuleSetFormatBinary,
			RemoteOptions: option.RemoteRuleSet{
				URL:            "https://raw.githubusercontent.com/hiddify/hiddify-geo/rule-set/country/geosite-" + opt.Region + ".srs",
				UpdateInterval: badoption.Duration(5 * time.Hour * 24),
			},
		})

		routeRules = append(routeRules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					RuleSet: []string{
						"geoip-" + opt.Region,
						"geosite-" + opt.Region,
					},
				},
				RuleAction: routeActionForOutbound(OutboundDirectTag),
			},
		})

	}
	options.Route = &option.RouteOptions{
		Rules:               routeRules,
		Final:               OutboundMainProxyTag,
		AutoDetectInterface: true,
		OverrideAndroidVPN:  runtime.GOOS == "android",
		RuleSet:             rulesets,
		// GeoIP: &option.GeoIPOptions{
		// 	Path: opt.GeoIPPath,
		// },
		// Geosite: &option.GeositeOptions{
		// 	Path: opt.GeoSitePath,
		// },
	}
	if opt.EnableDNSRouting {
		for _, dnsRule := range dnsRules {
			if dnsRule.IsValid() {
				options.DNS.Rules = append(
					options.DNS.Rules,
					option.DNSRule{
						Type:           C.RuleTypeDefault,
						DefaultOptions: dnsRule,
					},
				)
			}
		}
	}
}

func isBlockedDomain(domain string) bool {
	if strings.HasPrefix("full:", domain) {
		return false
	}
	ips, err := net.LookupHost(domain)
	if err != nil {
		// fmt.Println(err)
		return true
	}

	// Print the IP addresses associated with the domain
	fmt.Printf("IP addresses for %s:\n", domain)
	for _, ip := range ips {
		if strings.HasPrefix(ip, "10.") {
			return true
		}
	}
	return false
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func generateRandomString(length int) string {
	// Determine the number of bytes needed
	bytesNeeded := (length*6 + 7) / 8

	// Generate random bytes
	randomBytes := make([]byte, bytesNeeded)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "hiddify"
	}

	// Encode random bytes to base64
	randomString := base64.URLEncoding.EncodeToString(randomBytes)

	// Trim padding characters and return the string
	return randomString[:length]
}
