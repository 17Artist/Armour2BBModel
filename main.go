// Copyright 2026 17Artist. Licensed under CC-BY-NC-SA-3.0.
// https://github.com/17Artist/Armour2BBModel

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed web/static/*
var static embed.FS

func main() {
	sub, _ := fs.Sub(static, "web/static")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	})

	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}
	log.Printf("Armour2BBModel on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, handler))
}
