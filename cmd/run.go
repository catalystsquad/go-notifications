package cmd

import (
	"context"
	"fmt"
	"github.com/catalystsquad/app-utils-go/logging"
	pkg2 "github.com/catalystsquad/go-scheduler/pkg"
	"github.com/catalystsquad/go-scheduler/pkg/cockroachdb_store"
	"github.com/catalystsquad/grpc-base-go/pkg"
	notificationsv1alpha1 "github.com/catalystsquad/protos-go-notifications/gen/proto/go/notifications/v1alpha1"
	"github.com/catalystsquad/template-go-cobra-app/internal"
	"github.com/catalystsquad/template-go-cobra-app/internal/config"
	"github.com/catalystsquad/template-go-cobra-app/notification_store"
	"github.com/catalystsquad/template-go-cobra-app/notification_store/notifo_store"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/nozzle/e"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net/http"
	"os"
	"time"
)

var ServerConfig pkg.GrpcServerConfig
var runCmd = NewRunCommand()

func NewRunCommand() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the notifications service",
		Long:  `Run the notifications service`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
			return initializeConfig(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			runServer()
		},
	}
	runCmd.Flags().IntVar(&ServerConfig.Port, "port", 6000, "port to serve grpc on")
	runCmd.Flags().BoolVar(&ServerConfig.SentryEnabled, "sentry-enabled", false, "set the flag to enable sentry integration")
	runCmd.Flags().BoolVar(&ServerConfig.PrometheusEnabled, "prometheus-enabled", false, "set the flag to enable grpc prometheus metrics")
	runCmd.Flags().IntVar(&ServerConfig.PrometheusPort, "prometheus-port", 0, "what port to serve prometheus metrics on")
	runCmd.Flags().StringVar(&ServerConfig.PrometheusPath, "prometheus-path", "", "what path to serve prometheus metrics on")
	runCmd.Flags().StringVar(&ServerConfig.CaptureErrormessage, "capture-error-message", "", "sets the error message used when capturing errors in sentry")
	runCmd.Flags().StringVar(&ServerConfig.TlsCertPath, "tls-cert-path", "", "path to tls certificates")
	runCmd.Flags().StringVar(&ServerConfig.TlsKeyPath, "tls-key-path", "", "path to tls key")
	runCmd.Flags().StringVar(&ServerConfig.TlsCaPath, "tls-ca-path", "", "path to tls ca certificate")
	runCmd.Flags().Uint16Var(&ServerConfig.MinTlsVersion, "min-tls-version", 0, "set the flag to enable sentry integration")
	runCmd.Flags().IntVar(&config.AppConfig.HttpPort, "http-port", 1323, "port to serve http on")
	runCmd.Flags().BoolVar(&config.AppConfig.ServeHttp, "serve-http", false, "use this flag to enable http server via grpc gateway")
	runCmd.Flags().DurationVar(&config.AppConfig.ScheduleWindow, "schedule-window", 1*time.Second, "the time window to schedule notifications for. If this is set to 30 seconds for example, it will schedule notifications set to be delivered in the next 30 seconds, every 30 seconds.")
	runCmd.Flags().DurationVar(&config.AppConfig.RunnerWindow, "runner-window", 1*time.Second, "the time window to run notifications for. If this is set to 30 seconds for example, it will deliver notifications set to be delivered in the next 30 seconds, every 30 seconds.")
	runCmd.Flags().DurationVar(&config.AppConfig.CleanupWindow, "cleanup-window", 1*time.Second, "the time window to cleanup for. If this is set to 30 seconds for example, it will clean up delivered notifications every 30 seconds.")
	runCmd.Flags().StringVar(&config.AppConfig.CockroachdbUri, "cockroachdb-uri", "", "the cockroachdb connection string")
	runCmd.Flags().StringVar(&config.AppConfig.NotifoApiKey, "notifo-api-key", "", "the notifo api key")
	runCmd.Flags().StringVar(&config.AppConfig.NotifoBaseUrl, "notifo-base-url", "http://localhost:5000", "the notifo base url")
	runCmd.Flags().StringVar(&config.AppConfig.NotifoAppId, "notifo-app-id", "", "the notifo app id")
	rootCmd.AddCommand(runCmd)

	return runCmd
}

func runServer() {
	// instantiate store
	notification_store.NotificationStore = notifo_store.NotifoNotificationStore{}
	go startScheduler()
	server, err := pkg.NewGrpcServer(ServerConfig)
	if err != nil {
		logging.Log.WithError(e.Wrap(err)).Error("error instantiating grpc server")
		os.Exit(1)
	}
	registerServices(server.Server)
	err = maybeServeHttp()
	if err != nil {
		logging.Log.WithError(e.Wrap(err)).Error("error serving grpc gateway")
		os.Exit(1)
	}
	err = server.Run()
	if err != nil {
		logging.Log.WithError(e.Wrap(err)).Error("error running grpc server")
		os.Exit(1)
	}
}

func startScheduler() error {
	cockroachdbStore := cockroachdb_store.NewCockroachdbStore(config.AppConfig.CockroachdbUri, nil)
	var err error
	internal.Scheduler, err = pkg2.NewScheduler(config.AppConfig.ScheduleWindow, config.AppConfig.RunnerWindow, config.AppConfig.CleanupWindow, internal.HandleScheduledNotification, cockroachdbStore)
	if err != nil {
		return err
	}
	internal.Scheduler.Run()
	return nil
}

func registerServices(server *grpc.Server) {
	notificationsApiServer := internal.NotificationsServiceServer{}
	notificationsv1alpha1.RegisterNotificationsServiceServer(server, notificationsApiServer)
}

func maybeServeHttp() error {
	if config.AppConfig.ServeHttp {
		// Register gRPC server endpoint
		// Note: Make sure the gRPC server is running properly and accessible
		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		grpcAddress := fmt.Sprintf("localhost:%d", ServerConfig.Port)
		err := notificationsv1alpha1.RegisterNotificationsServiceHandlerFromEndpoint(context.Background(), mux, grpcAddress, opts)
		if err != nil {
			return err
		}
		// Start HTTP server (and proxy calls to gRPC server endpoint)
		// TODO detect liveness?
		go http.ListenAndServe(fmt.Sprintf(":%d", config.AppConfig.HttpPort), mux)
		return nil
	}
	return nil
}
