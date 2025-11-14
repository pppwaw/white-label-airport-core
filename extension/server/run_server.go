package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	v2 "github.com/hiddify/hiddify-core/v2"

	"github.com/hiddify/hiddify-core/utils"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

type ServerOptions struct {
	BasePath    string
	WorkingPath string
	TempPath    string
	GRPCAddr    string
	WebAddr     string
	StaticDir   string
	CertPath    string
	KeyPath     string
	AutoSetup   bool
	Headless    bool
}

func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		BasePath:    "./tmp",
		WorkingPath: "./",
		TempPath:    "./tmp",
		GRPCAddr:    "127.0.0.1:12345",
		WebAddr:     ":12346",
		StaticDir:   "./extension/html",
		CertPath:    "cert/server-cert.pem",
		KeyPath:     "cert/server-key.pem",
		AutoSetup:   true,
	}
}

func StartTestExtensionServer() {
	if err := StartExtensionServer(DefaultServerOptions()); err != nil {
		log.Fatal(err)
	}
}

func StartExtensionServer(opts ServerOptions) error {
	if opts.GRPCAddr == "" {
		opts.GRPCAddr = "127.0.0.1:12345"
	}
	if opts.AutoSetup {
		if err := v2.Setup(opts.BasePath, opts.WorkingPath, opts.TempPath, 0, false); err != nil {
			return err
		}
	}
	grpcServer, err := v2.StartCoreGrpcServer(opts.GRPCAddr)
	if err != nil {
		return err
	}
	fmt.Printf("Extension gRPC listening on %s\n", opts.GRPCAddr)
	if opts.Headless {
		waitForShutdown(grpcServer)
		return nil
	}
	if opts.WebAddr == "" {
		opts.WebAddr = ":12346"
	}
	if opts.StaticDir == "" {
		opts.StaticDir = "./extension/html"
	}
	if opts.CertPath == "" {
		opts.CertPath = "cert/server-cert.pem"
	}
	if opts.KeyPath == "" {
		opts.KeyPath = "cert/server-key.pem"
	}
	fmt.Printf("Waiting for CTRL+C to stop\n")
	return runWebserver(grpcServer, opts)
}

func allowCors(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Access-Control-Allow-Origin", "*")
	resp.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	resp.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if req.Method == "OPTIONS" {
		resp.WriteHeader(http.StatusOK)
		return
	}
}

func runWebserver(grpcServer *grpc.Server, opts ServerOptions) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcWeb := grpcweb.WrapServer(grpcServer)
	fileServer := http.FileServer(http.Dir(opts.StaticDir))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		allowCors(resp, req)
		if grpcWeb.IsGrpcWebRequest(req) || grpcWeb.IsAcceptableGrpcCorsRequest(req) || grpcWeb.IsGrpcWebSocketRequest(req) {
			grpcWeb.ServeHTTP(resp, req)
			return
		}
		fileServer.ServeHTTP(resp, req)
	})

	rpcWebServer := &http.Server{
		Handler: mux,
		Addr:    opts.WebAddr,
	}
	log.Printf("Serving grpc-web from https://%s/", opts.WebAddr)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		utils.GenerateCertificate(opts.CertPath, opts.KeyPath, true, true)
		if err := rpcWebServer.ListenAndServeTLS(opts.CertPath, opts.KeyPath); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Web server (gRPC-web) shutdown with error: %s", err)
		}
		grpcServer.Stop()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		log.Println("Context canceled, shutting down servers...")
	case sig := <-sigChan:
		log.Printf("Received signal: %s, shutting down servers...", sig)
	}

	if err := rpcWebServer.Shutdown(ctx); err != nil {
		log.Printf("gRPC-web server shutdown with error: %s", err)
	}

	wg.Wait()
	log.Println("Server shutdown complete")
	return nil
}

func waitForShutdown(grpcServer *grpc.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received signal: %s, shutting down gRPC server...", sig)
	grpcServer.GracefulStop()
}
