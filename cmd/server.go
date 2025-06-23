package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/api/router"
	"github.com/zenstats/zenstats/internal/event"
	"github.com/zenstats/zenstats/pkg/globals"
)

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server at the specified address",
	Long: `Start the server at the specified address
the address is defined in config file`,
	Run: func(cmd *cobra.Command, args []string) {
		Init()

		queue := globals.GetQueue()

		event, err := event.NewEventWork(queue, 1024)
		if err != nil {
			slog.Error("Failed to start event worker", "error", err)
			return
		}
		event.Run()

		slog.Debug("Starting server")
		if !config.Conf.AppDebug {
			gin.SetMode(gin.ReleaseMode)
		}

		r := gin.New()
		r.Use(
			gin.Recovery(),
		)

		api := r.Group("/api")
		router.RegisterRouter(api)

		httpBase := fmt.Sprintf("%s:%d", config.Conf.Scheme.Address, config.Conf.Scheme.HttpPort)
		slog.Info(fmt.Sprintf("start HTTP server %s", httpBase))
		httpSrv := &http.Server{Addr: httpBase, Handler: r}
		go func() {
			err := httpSrv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error(fmt.Sprintf("failed to start http: %s", httpBase))
				os.Exit(1)
			}
		}()

		var unixSrv *http.Server
		if config.Conf.Scheme.UnixFile != "" {
			slog.Info("start unix server", "file", config.Conf.Scheme.UnixFile)
			unixSrv = &http.Server{Handler: r}
			go func() {
				listener, err := net.Listen("unix", config.Conf.Scheme.UnixFile)
				if err != nil {
					slog.Error("failed to listen unix", "error", err)
					os.Exit(1)
				}

				err = unixSrv.Serve(listener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("failed to start unix", "error", err)
					os.Exit(1)
				}
			}()
		}

		// Wait for interrupt signal to gracefully shutdown the server with
		// a timeout of 1 second.
		quit := make(chan os.Signal, 1)
		// kill (no param) default send syscanll.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("Shutdown server...")
		Release()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		var wg sync.WaitGroup
		// Shutdown Http Server
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := httpSrv.Shutdown(ctx); err != nil {
				slog.Error("HTTP server shutdown", "error", err)
			}
		}()

		// Shutdown work
		wg.Add(1)
		go func() {
			defer wg.Done()
			event.Shutdown()
		}()

		// Shutdown Unix Server
		if config.Conf.Scheme.UnixFile != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := unixSrv.Shutdown(ctx); err != nil {
					slog.Error("Unix server shutdown ", "error", err)
				}
			}()
		}
		wg.Wait()
		slog.Info("Server exit")
	},
}

func init() {
	RootCmd.AddCommand(ServerCmd)
}
