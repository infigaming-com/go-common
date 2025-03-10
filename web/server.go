package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	engine *gin.Engine
	mode   string
	port   int64
}

type Option func(*Server)

func defaultServer() *Server {
	return &Server{
		mode: gin.ReleaseMode,
		port: 8080,
	}
}

func WithMode(mode string) Option {
	return func(s *Server) {
		s.mode = mode
	}
}

func WithPort(port int64) Option {
	return func(s *Server) {
		s.port = port
	}
}

func WithCustomHandler(handler func(c *gin.Context)) Option {
	return func(s *Server) {
		s.engine.Use(handler)
	}
}

func StartServer(lg *zap.Logger, opts ...Option) {
	s := defaultServer()

	gin.SetMode(s.mode)
	s.engine = gin.New()

	for _, opt := range opts {
		opt(s)
	}

	s.engine.Use(gin.Recovery())

	s.engine.Use(defaultHandler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.engine,
	}

	go func() {
		addr := fmt.Sprintf(":%d", s.port)
		lg.Info("starting web server ...", zap.String("address", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lg.Fatal("fail to listenAndServe", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	lg.Info("shutdown web server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		lg.Fatal("fail to shutdown web server", zap.Error(err))
	}

	select {
	case <-ctx.Done():
		lg.Info("web server shutdown timeout")
	}
	lg.Info("web server exiting")
}

func defaultHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch {
		case c.Request.URL.Path == "/":
			c.Status(http.StatusOK)
			return
		case strings.HasSuffix(c.Request.URL.Path, "/healthcheck"):
			c.Status(http.StatusOK)
			return
		}
	}
}
