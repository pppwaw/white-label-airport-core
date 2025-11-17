package v2

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hiddify/hiddify-core/config"
	pb "github.com/hiddify/hiddify-core/hiddifyrpc"
	"github.com/hiddify/hiddify-core/v2/service_manager"
	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	sWorkingPath          string
	statusPropagationPort int64
)

func InitHiddifyService() error {
	return service_manager.StartServices()
}

func Setup(basePath string, workingPath string, tempPath string, statusPort int64, debug bool) error {
	statusPropagationPort = int64(statusPort)
	setupOptions := libbox.SetupOptions{
		BasePath:    basePath,
		WorkingPath: workingPath,
		TempPath:    tempPath,
		Username:    "",
		IsTVOS:      false,
	}
	if err := libbox.Setup(&setupOptions); err != nil {
		return E.Cause(err, "setup libbox")
	}
	sWorkingPath = workingPath
	os.Chdir(sWorkingPath)
	hiddifySettingsFile = filepath.Join(sWorkingPath, "hiddify-settings.json")
	if err := loadHiddifySettingsFromDisk(); err != nil {
		log.Warn("failed to load persisted Hiddify options: ", err)
	}

	var defaultWriter io.Writer
	if !debug {
		defaultWriter = io.Discard
	}
	factory, err := log.New(
		log.Options{
			DefaultWriter: defaultWriter,
			BaseTime:      time.Now(),
			Observable:    true,
		})
	coreLogFactory = factory

	if err != nil {
		return E.Cause(err, "create logger")
	}
	return InitHiddifyService()
}

func NewService(options option.Options) (*libbox.BoxService, error) {
	content, err := config.MarshalOptions(&options)
	if err != nil {
		return nil, E.Cause(err, "encode config")
	}
	Log(pb.LogLevel_DEBUG, pb.LogType_SERVICE, "Config content: "+string(content))
	service, err := libbox.NewService(string(content), newPlatformInterface())
	if err != nil {
		return nil, E.Cause(err, "create service")
	}
	return service, nil
}

func readOptions(configContent string) (option.Options, error) {
	options, err := config.UnmarshalOptions([]byte(configContent))
	if err != nil {
		return option.Options{}, E.Cause(err, "decode config")
	}
	return *options, nil
}
