package gzip

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/gzip"
)

type gzipHandler struct {
	*Options
	gzPool sync.Pool
	level  int
}

func newGzipHandler(level int, options ...Option) *gzipHandler {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		panic(fmt.Errorf("gzip: invalid compression level: %d", level))
	}

	handler := &gzipHandler{
		Options: DefaultOptions,
		level:   level,
	}
	for _, setter := range options {
		setter(handler.Options)
	}
	return handler
}

func (g *gzipHandler) Handle(c *gin.Context) {
	if fn := g.DecompressFn; fn != nil && c.Request.Header.Get("Content-Encoding") == "gzip" {
		fn(c)
	}

	if !g.shouldCompress(c.Request) {
		return
	}

	gz := g.getGzipWriter(c.Writer)
	defer g.gzPool.Put(gz)

	c.Header("Content-Encoding", "gzip")
	c.Header("Vary", "Accept-Encoding")
	c.Writer = &gzipWriter{c.Writer, gz}
	defer func() {
		gz.Close()
		c.Header("Content-Length", fmt.Sprint(c.Writer.Size()))
	}()
	c.Next()
}

func (g *gzipHandler) shouldCompress(req *http.Request) bool {
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") ||
		strings.Contains(req.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(req.Header.Get("Accept"), "text/event-stream") {
		return false
	}

	extension := filepath.Ext(req.URL.Path)
	if g.ExcludedExtensions.Contains(extension) {
		return false
	}

	if g.ExcludedPaths.Contains(req.URL.Path) {
		return false
	}
	if g.ExcludedPathesRegexs.Contains(req.URL.Path) {
		return false
	}

	return true
}

func (g *gzipHandler) getGzipWriter(w io.Writer) *gzip.Writer {
	gzp := g.gzPool.Get()
	if gzp != nil {
		gz := gzp.(*gzip.Writer)
		gz.Reset(w)
		return gz
	}

	gz, err := gzip.NewWriterLevel(w, g.level)
	if err != nil {
		panic(err)
	}

	return gz
}
