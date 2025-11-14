package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hiddify/hiddify-core/config"

	"github.com/hiddify/hiddify-core/v2"

	_ "github.com/sagernet/gomobile"
)

func Setup(baseDir string, workingDir string, tempDir string, debug bool) error {
	return v2.Setup(baseDir, workingDir, tempDir, 0, debug)
	// return v2.Start(17078)
}

func Parse(path string, tempPath string, debug bool) error {
	config, err := config.ParseConfig(tempPath, debug)
	if err != nil {
		return err
	}
	return os.WriteFile(path, config, 0o644)
}

func BuildConfig(path string, HiddifyOptionsJson string) (string, error) {
	os.Chdir(filepath.Dir(path))
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	options, err := config.UnmarshalOptions(fileContent)
	if err != nil {
		return "", err
	}
	HiddifyOptions := &config.HiddifyOptions{}
	err = json.Unmarshal([]byte(HiddifyOptionsJson), HiddifyOptions)
	if err != nil {
		return "", nil
	}
	return config.BuildConfigJson(*HiddifyOptions, *options)
}
