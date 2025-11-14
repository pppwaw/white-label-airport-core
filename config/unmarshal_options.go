package config

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

var (
	optionsCtx     context.Context
	optionsCtxOnce sync.Once
)

// OptionsContext returns the shared include-aware context used for
// marshaling/unmarshaling sing-box option structures.
func OptionsContext() context.Context {
	optionsCtxOnce.Do(func() {
		optionsCtx = include.Context(context.Background())
	})
	return optionsCtx
}

// UnmarshalOptions decodes a JSON payload into option.Options with the include
// registries loaded so that contextual types (outbounds, inbounds, etc.) are
// resolved correctly.
func UnmarshalOptions(content []byte) (*option.Options, error) {
	var options option.Options
	err := options.UnmarshalJSONContext(OptionsContext(), content)
	if err != nil {
		return nil, err
	}
	return &options, nil
}

// MarshalOptions serializes option.Options with the include context so that
// any ContextMarshaler hooks are honored.
func MarshalOptions(options *option.Options) ([]byte, error) {
	return json.MarshalContext(OptionsContext(), options)
}

func cloneOutbound(out option.Outbound) (option.Outbound, error) {
	content, err := json.MarshalContext(OptionsContext(), &out)
	if err != nil {
		return option.Outbound{}, err
	}
	var cloned option.Outbound
	err = cloned.UnmarshalJSONContext(OptionsContext(), content)
	if err != nil {
		return option.Outbound{}, err
	}
	return cloned, nil
}
