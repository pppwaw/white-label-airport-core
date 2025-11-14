package config

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	badoption "github.com/sagernet/sing/common/json/badoption"
)

func TestBuildConfigAddsSelectorAndURLTest(t *testing.T) {
	opt := DefaultHiddifyOptions()
	options, err := BuildConfig(*opt, option.Options{
		Outbounds: []option.Outbound{minimalShadowsocksOutbound("proxy-a")},
	})
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}

	selector := findOutbound(t, options, OutboundSelectTag)
	if selector.Type != C.TypeSelector {
		t.Fatalf("selector type = %s, want %s", selector.Type, C.TypeSelector)
	}
	selectorOptions, ok := selector.Options.(option.SelectorOutboundOptions)
	if !ok {
		t.Fatalf("selector options type = %T, want option.SelectorOutboundOptions", selector.Options)
	}
	if len(selectorOptions.Outbounds) == 0 || selectorOptions.Outbounds[0] != OutboundURLTestTag {
		t.Fatalf("selector outbounds = %v, expect url-test tagged first", selectorOptions.Outbounds)
	}
	if selectorOptions.Default != OutboundURLTestTag {
		t.Fatalf("selector default = %s, want %s", selectorOptions.Default, OutboundURLTestTag)
	}

	urlTest := findOutbound(t, options, OutboundURLTestTag)
	if urlTest.Type != C.TypeURLTest {
		t.Fatalf("url-test type = %s, want %s", urlTest.Type, C.TypeURLTest)
	}
	urlTestOptions, ok := urlTest.Options.(option.URLTestOutboundOptions)
	if !ok {
		t.Fatalf("url-test options type = %T, want option.URLTestOutboundOptions", urlTest.Options)
	}
	if !containsString(urlTestOptions.Outbounds, "proxy-a") {
		t.Fatalf("url-test does not contain proxy tag: %v", urlTestOptions.Outbounds)
	}
	expectedInterval := badoption.Duration(opt.URLTestInterval.Duration())
	if urlTestOptions.Interval != expectedInterval {
		t.Fatalf("url-test interval = %v, want %v", urlTestOptions.Interval, expectedInterval)
	}
}

func TestBuildConfigAddsDirectFragmentDialerOptions(t *testing.T) {
	opt := DefaultHiddifyOptions()
	options, err := BuildConfig(*opt, option.Options{
		Outbounds: []option.Outbound{minimalShadowsocksOutbound("proxy-a")},
	})
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}

	directFragment := findOutbound(t, options, OutboundDirectFragmentTag)
	if directFragment.Type != C.TypeDirect {
		t.Fatalf("direct fragment type = %s, want %s", directFragment.Type, C.TypeDirect)
	}
	directOptions, ok := directFragment.Options.(option.DirectOutboundOptions)
	if !ok {
		t.Fatalf("direct fragment options type = %T, want option.DirectOutboundOptions", directFragment.Options)
	}
	if directOptions.DialerOptions.TCPFastOpen {
		t.Fatal("direct fragment dialer should disable TCP Fast Open")
	}
}

func minimalShadowsocksOutbound(tag string) option.Outbound {
	return option.Outbound{
		Tag:  tag,
		Type: C.TypeShadowsocks,
		Options: option.ShadowsocksOutboundOptions{
			Method:   "2022-blake3-aes-128-gcm",
			Password: "secret",
			ServerOptions: option.ServerOptions{
				Server:     "203.0.113.1",
				ServerPort: 443,
			},
		},
	}
}

func findOutbound(t *testing.T, options *option.Options, tag string) option.Outbound {
	t.Helper()
	for _, outbound := range options.Outbounds {
		if outbound.Tag == tag {
			return outbound
		}
	}
	t.Fatalf("outbound %s not found", tag)
	return option.Outbound{}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
