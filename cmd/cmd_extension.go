package cmd

import (
	"log"

	_ "github.com/hiddify/hiddify-core/extension/repository"
	"github.com/hiddify/hiddify-core/extension/server"
	"github.com/spf13/cobra"
)

var (
	extensionBasePath string
	extensionWorkPath string
	extensionTempPath string
	extensionGRPCAddr string
	extensionWebAddr  string
	extensionHeadless bool
)

var commandExtension = &cobra.Command{
	Use:   "extension",
	Short: "extension configuration server",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		opts := server.DefaultServerOptions()
		if extensionBasePath != "" {
			opts.BasePath = extensionBasePath
		}
		if extensionWorkPath != "" {
			opts.WorkingPath = extensionWorkPath
		}
		if extensionTempPath != "" {
			opts.TempPath = extensionTempPath
		}
		if extensionGRPCAddr != "" {
			opts.GRPCAddr = extensionGRPCAddr
		}
		if extensionWebAddr != "" {
			opts.WebAddr = extensionWebAddr
		}
		opts.Headless = extensionHeadless
		if err := server.StartExtensionServer(opts); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandExtension.Flags().StringVar(&extensionBasePath, "base-path", "./tmp", "base path for libbox setup")
	commandExtension.Flags().StringVar(&extensionWorkPath, "work-path", "./", "working directory for libbox")
	commandExtension.Flags().StringVar(&extensionTempPath, "temp-path", "./tmp", "temp directory for libbox")
	commandExtension.Flags().StringVar(&extensionGRPCAddr, "grpc-addr", "127.0.0.1:12345", "gRPC listen address")
	commandExtension.Flags().StringVar(&extensionWebAddr, "web-addr", ":12346", "web UI listen address")
	commandExtension.Flags().BoolVar(&extensionHeadless, "headless", false, "run without starting the web UI (gRPC only)")
	mainCommand.AddCommand(commandExtension)
}
