package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hiddify/hiddify-core/config"
	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

var (
	templatePath = flag.String("template", "config/config.json.template", "path to embedded sing-box template output")
	defaultPath  = flag.String("default", "config/default.json", "path to default config snapshot output")
	samplePath   = flag.String("sample", "tmp/default-config.json", "path to CLI sample config output")
)

func main() {
	flag.Parse()

	hiddify, err := config.NormalizeHiddifyOptions(config.DefaultHiddifyOptions())
	if err != nil {
		log.Fatalf("normalize hiddify options: %v", err)
	}

	configJSON, err := config.BuildConfigJson(*hiddify, demoOptions())
	if err != nil {
		log.Fatalf("build config json: %v", err)
	}

	configJSON, err = injectDemoOutbound(configJSON)
	if err != nil {
		log.Fatalf("inject demo outbound: %v", err)
	}

	if err := writeFile(*templatePath, []byte(configJSON)); err != nil {
		log.Fatalf("write template: %v", err)
	}
	if err := writeFile(*defaultPath, []byte(configJSON)); err != nil {
		log.Fatalf("write default: %v", err)
	}
	if err := writeFile(*samplePath, []byte(configJSON)); err != nil {
		log.Fatalf("write sample: %v", err)
	}
	fmt.Printf("default config regenerated at %s, %s, %s\n", *templatePath, *defaultPath, *samplePath)
}

func demoOptions() option.Options {
	return option.Options{
		Outbounds: []option.Outbound{
			{
				Tag:  "demo-shadowsocks",
				Type: constant.TypeShadowsocks,
				Options: option.ShadowsocksOutboundOptions{
					Method:   "2022-blake3-aes-128-gcm",
					Password: "change-me",
					ServerOptions: option.ServerOptions{
						Server:     "203.0.113.8",
						ServerPort: 443,
					},
				},
			},
		},
	}
}

func writeFile(path string, data []byte) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func injectDemoOutbound(raw string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", err
	}
	outbounds, ok := payload["outbounds"].([]any)
	if !ok {
		return raw, nil
	}
	for _, item := range outbounds {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["tag"] == "demo-shadowsocks" {
			entry["server"] = "demo.sing-box.example"
			entry["server_port"] = 443
			entry["method"] = "2022-blake3-aes-128-gcm"
			entry["password"] = "AAAAAAAAAAAAAAAAAAAAAA=="
		}
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	return buf.String(), nil
}
