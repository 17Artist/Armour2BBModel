// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed web/static/*
var static embed.FS

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	defaultPort := 8080
	explicitPort := false
	if value := os.Getenv("PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 65535 {
			return fmt.Errorf("PORT must be an integer between 0 and 65535")
		}
		defaultPort, explicitPort = parsed, true
	}
	port := flag.Int("port", defaultPort, "local port (0 selects an available port)")
	noBrowser := flag.Bool("no-browser", false, "do not automatically open the default browser")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			explicitPort = true
		}
	})
	if *port < 0 || *port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}

	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil && !explicitPort {
		log.Printf("Port %d is unavailable; selecting another local port.", *port)
		listener, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		return fmt.Errorf("cannot start the local workbench: %w", err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           staticHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	serverResult := make(chan error, 1)
	go func() { serverResult <- server.Serve(listener) }()

	url := "http://" + listener.Addr().String() + "/"
	log.Printf("Armour2BBModel\n%s\nKeep this window open while using the workbench. Press Ctrl+C to exit.", url)
	if runtime.GOOS == "windows" && !*noBrowser {
		if err := openBrowser(url); err != nil {
			log.Printf("Cannot open the browser automatically. Open %s manually. (%v)", url, err)
		}
	}

	select {
	case err := <-serverResult:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

func openBrowser(url string) error {
	// Use Windows' built-in URL handler; no bundled browser or extra runtime is required.
	launcher := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	if err := launcher.Start(); err != nil {
		return err
	}
	go func() { _ = launcher.Wait() }()
	return nil
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(static, "web/static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		// The bootstrap JS and Go WASM runtime must come from the same build.
		w.Header().Set("Cache-Control", "no-cache")
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		path = strings.TrimPrefix(path, "/")

		// 尝试提供 .gz 预压缩版本
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			if data, err := fs.ReadFile(sub, path+".gz"); err == nil {
				ct := "application/octet-stream"
				if strings.HasSuffix(path, ".wasm") {
					ct = "application/wasm"
				} else if strings.HasSuffix(path, ".js") {
					ct = "application/javascript"
				} else if strings.HasSuffix(path, ".html") {
					ct = "text/html; charset=utf-8"
				}
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Content-Encoding", "gzip")
				w.Write(data)
				return
			}
		}

		// 回退到未压缩版本
		files.ServeHTTP(w, r)
	})
}
