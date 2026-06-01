package static

import (
	"embed"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/fmotalleb/hermes/internal/web"
)

//go:embed *
var embeddedStatic embed.FS

func Register(router interface{ GET(string, web.HandlerFunc) }) {
	router.GET("/", staticHandler)
	router.GET("/{path...}", staticHandler)
}

func staticHandler(ctx *web.Context) (any, error) {
	requestPath := ctx.PathParam("path")
	requestPath = strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if requestPath == "." {
		requestPath = ""
	}

	if requestPath == "" {
		return readEmbeddedFile("index.html")
	}

	if requestPath == "api" || strings.HasPrefix(requestPath, "api/") {
		return web.File{}, notFound("file", requestPath)
	}

	if filepath.Ext(requestPath) == "" {
		return readEmbeddedFile("index.html")
	}

	return readEmbeddedFile(requestPath)
}

func readEmbeddedFile(filePath string) (any, error) {
	data, err := embeddedStatic.ReadFile(filePath)
	if err != nil {
		return web.File{}, notFound("file", strings.TrimPrefix(filePath, ""))
	}

	return web.File{
		Content:     data,
		ContentType: contentTypeForFile(filePath, data),
	}, nil
}

func notFound(name, value string) error {
	return &notFoundError{name: name, value: value}
}

type notFoundError struct {
	name  string
	value string
}

func (e *notFoundError) Error() string { return e.name + " not found: " + e.value }
func (e *notFoundError) StatusCode() int { return http.StatusNotFound }
func (e *notFoundError) Body() any { return map[string]string{"name": e.name, "value": e.value} }

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

