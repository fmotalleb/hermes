package main

import (
	"embed"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
)

//go:embed static
var embeddedStatic embed.FS

func serveStatic(ctx *gofr.Context) (any, error) {
	requestPath := ctx.PathParam("path")
	requestPath = strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if requestPath == "." {
		requestPath = ""
	}

	if requestPath == "" {
		return readEmbeddedFile("static/index.html")
	}

	if requestPath == "api" || strings.HasPrefix(requestPath, "api/") {
		return response.File{}, gofrHTTP.ErrorEntityNotFound{Name: "file", Value: requestPath}
	}

	if filepath.Ext(requestPath) == "" {
		return readEmbeddedFile("static/index.html")
	}

	filePath := path.Join("static", requestPath)
	return readEmbeddedFile(filePath)
}

func readEmbeddedFile(filePath string) (any, error) {
	data, err := embeddedStatic.ReadFile(filePath)
	if err != nil {
		return response.File{}, gofrHTTP.ErrorEntityNotFound{Name: "file", Value: strings.TrimPrefix(filePath, "static/")}
	}

	return response.File{
		Content:     data,
		ContentType: contentTypeForFile(filePath, data),
	}, nil
}

func contentTypeForFile(filePath string, data []byte) string {
	switch ext := strings.ToLower(filepath.Ext(filePath)); ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".txt":
		return "text/plain; charset=utf-8"
	}

	if ct := mime.TypeByExtension(filepath.Ext(filePath)); ct != "" {
		return ct
	}

	return httpDetectContentType(data)
}

func httpDetectContentType(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	if len(data) > 512 {
		data = data[:512]
	}

	return fsContentType(data)
}

func fsContentType(data []byte) string {
	return http.DetectContentType(data)
}
