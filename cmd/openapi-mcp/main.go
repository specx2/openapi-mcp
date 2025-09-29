package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpsrv "github.com/mark3labs/mcp-go/server"
	"github.com/specx2/openapi-mcp/cmd/openapi-mcp/server"
)

func main() {
	defaultSpec := filepath.Join("test", "_gigasdk", "spec", "projectManage.swagger.yaml")

	specPaths := []string{defaultSpec}
	var specFlagSet bool
	flag.Func("spec", "Path to an OpenAPI specification file (repeatable)", func(value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		if !specFlagSet {
			specPaths = specPaths[:0]
			specFlagSet = true
		}
		specPaths = append(specPaths, value)
		return nil
	})

	baseURLFlag := flag.String("base-url", "", "Upstream API base URL (defaults to env or http://127.0.0.1:8000)")
	timeoutFlag := flag.Duration("timeout", 15*time.Second, "HTTP client timeout")
	serverName := flag.String("server-name", "openapi-mcp", "MCP server name")
	serverVersion := flag.String("server-version", "0.1.0", "MCP server version")
	logOutput := flag.String("log-output", "", "Write logs to this destination (stdout, stderr, or file path)")
	teeConsole := flag.Bool("log-tee-console", false, "If true and log-output is a file, also write logs to stderr")
	flag.Parse()

	cleanup, err := configureLogging(*logOutput, *teeConsole)
	if err != nil {
		log.Fatalf("failed to configure logging: %v", err)
	}
	defer cleanup()

	if len(specPaths) == 0 {
		log.Fatalf("no OpenAPI specifications provided")
	}

	specs := make([]server.Spec, 0, len(specPaths))
	for _, path := range specPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("failed to read spec %s: %v", path, err)
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}
		specs = append(specs, server.Spec{
			Path: filepath.ToSlash(absPath),
			Data: data,
		})
	}

	baseURL := firstNonEmpty(*baseURLFlag, os.Getenv("GIGASDK_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}

	srv, err := server.New(server.Options{
		Specs:         specs,
		BaseURL:       baseURL,
		Timeout:       *timeoutFlag,
		ServerName:    *serverName,
		ServerVersion: *serverVersion,
	})
	if err != nil {
		log.Fatalf("failed to construct OpenAPI MCP server: %v", err)
	}

	stdio := mcpsrv.NewStdioServer(srv.MCPServer())
	log.Printf("OpenAPI MCP server ready. Target base URL: %s", baseURL)

	if err := stdio.Listen(context.Background(), os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		log.Fatalf("stdio server stopped: %v", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func configureLogging(target string, tee bool) (func(), error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "stderr":
		log.SetOutput(os.Stderr)
		return func() {}, nil
	case "stdout":
		log.SetOutput(os.Stdout)
		return func() {}, nil
	default:
		abs, err := filepath.Abs(target)
		if err != nil {
			return func() {}, err
		}
		dir := filepath.Dir(abs)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return func() {}, err
		}
		file, err := os.OpenFile(abs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return func() {}, err
		}
		if tee {
			log.SetOutput(io.MultiWriter(file, os.Stderr))
		} else {
			log.SetOutput(file)
		}
		return func() {
			_ = file.Close()
		}, nil
	}
}
