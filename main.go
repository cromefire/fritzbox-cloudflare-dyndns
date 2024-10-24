package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/cloudflare"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/dyndns"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/polling"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/util"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Load any env variables defined in .env.dev files
	_ = godotenv.Load(".env", ".env.dev")

	rootLogger := slog.Default()

	updater, updateStatus := newUpdater(rootLogger)
	updater.StartWorker()

	ctx, cancel := context.WithCancelCause(context.Background())

	ipv6LocalAddress := os.Getenv("DEVICE_LOCAL_ADDRESS_IPV6")

	var localIp net.IP
	if ipv6LocalAddress != "" {
		localIp = net.ParseIP(ipv6LocalAddress)
		if localIp == nil {
			rootLogger.Error("Failed to parse IP from DEVICE_LOCAL_ADDRESS_IPV6, exiting")
			return
		}
		rootLogger.Info("Using the IPv6 Prefix to construct the IPv6 Address")
	}

	bind := os.Getenv("METRICS_BIND")
	pollStatus := polling.StartPollServer(updater.In, &localIp, rootLogger)
	pushStatus := startPushServer(updater.In, &localIp, rootLogger, cancel)
	status := util.Status{
		Push:    pushStatus,
		Poll:    pollStatus,
		Updates: updateStatus,
	}
	if bind != "" {
		// TODO: Read from file
		token := os.Getenv("METRICS_TOKEN")
		startMetricsServer(bind, rootLogger, status, token, cancel)
	}

	// Create a OS signal shutdown channel
	shutdown := make(chan os.Signal)

	signal.Notify(shutdown, syscall.SIGTERM)
	signal.Notify(shutdown, syscall.SIGINT)

	// Wait for either the context to finish or the shutdown signal
	select {
	case <-ctx.Done():
		rootLogger.Error("Context closed", util.ErrorAttr(context.Cause(ctx)))
		os.Exit(1)
	case <-shutdown:
		break
	}

	rootLogger.Info("Shutdown detected")
}

func newUpdater(logger *slog.Logger) (*cloudflare.Updater, []*util.UpdateStatus) {
	const subsystem = "cf_updater"
	logger = logger.With(util.SubsystemAttr(subsystem))
	u := cloudflare.NewUpdater(slog.Default().With(util.SubsystemAttr(subsystem)), subsystem)

	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	email := os.Getenv("CLOUDFLARE_API_EMAIL")
	key := os.Getenv("CLOUDFLARE_API_KEY")

	if token == "" {
		if email == "" || key == "" {
			logger.Info("Env CLOUDFLARE_API_TOKEN not found, disabling Cloudflare updates", util.SubsystemAttr(subsystem))
			return u, nil
		} else {
			logger.Warn("Using deprecated credentials via the API key")
		}
	}

	ipv4Zone := os.Getenv("CLOUDFLARE_ZONES_IPV4")
	ipv6Zone := os.Getenv("CLOUDFLARE_ZONES_IPV6")

	if ipv4Zone == "" && ipv6Zone == "" {
		logger.Warn("Env CLOUDFLARE_ZONES_IPV4 and CLOUDFLARE_ZONES_IPV6 not found, disabling Cloudflare updates", util.SubsystemAttr(subsystem))
		return u, nil
	}

	if ipv4Zone != "" {
		u.SetIPv4Zones(ipv4Zone)
	}

	if ipv6Zone != "" {
		u.SetIPv6Zones(ipv6Zone)
	}

	var err error
	var status []*util.UpdateStatus

	if token != "" {
		err, status = u.InitWithToken(token)
	} else {
		err, status = u.InitWithKey(email, key)
	}

	if err != nil {
		logger.Error("Failed to init Cloudflare updater, disabling Cloudflare updates")
		os.Exit(1)
	}

	return u, status
}

func startPushServer(out chan<- *net.IP, localIp *net.IP, logger *slog.Logger, cancel context.CancelCauseFunc) *util.PushStatus {
	const subsystem = "push_server"
	logger = logger.With(util.SubsystemAttr(subsystem))
	bind := os.Getenv("DYNDNS_SERVER_BIND")

	if bind == "" {
		logger.Info("Env DYNDNS_SERVER_BIND not found, disabling DynDns server")
		return nil
	}

	status := util.PushStatus{
		Succeeded: true,
	}

	server := dyndns.NewServer(out, localIp, logger, subsystem, &status)
	server.Username = os.Getenv("DYNDNS_SERVER_USERNAME")
	server.Password = os.Getenv("DYNDNS_SERVER_PASSWORD")

	pushMux := http.NewServeMux()

	pushMux.HandleFunc("/ip", server.Handler)

	s := &http.Server{
		Addr:     bind,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Handler:  pushMux,
	}

	go func() {
		err := s.ListenAndServe()
		cancel(errors.Join(errors.New("http server error"), err))
	}()

	logger.Info("DynDns server started", slog.String("addr", bind))

	return &status
}

func startMetricsServer(bind string, logger *slog.Logger, status util.Status, token string, cancel context.CancelCauseFunc) {
	const subsystem = "metrics"
	logger = logger.With(util.SubsystemAttr(subsystem))
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if status.Poll != nil && !status.Poll.Succeeded {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if status.Push != nil && !status.Push.Succeeded {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if status.Updates != nil {
			anyUnsuccessful := false
			for _, u := range status.Updates {
				if !u.Succeeded {
					anyUnsuccessful = true
					break
				}
			}

			if anyUnsuccessful {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
		encoder := json.NewEncoder(w)
		err := encoder.Encode(status)
		if err != nil {
			logger.Error("Failed to encode health check response", util.ErrorAttr(err))
			return
		}
	})
	metricsMux.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	metricServer := &http.Server{
		Addr:     bind,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	if token == "" {
		metricServer.Handler = metricsMux
	} else {
		tokenHandler := util.NewTokenHandler(metricsMux, token)
		metricServer.Handler = tokenHandler
	}

	go func() {
		err := metricServer.ListenAndServe()
		cancel(errors.Join(errors.New("metrics http server error"), err))
	}()

	logger.Info("metrics server started", slog.String("addr", bind))
}
