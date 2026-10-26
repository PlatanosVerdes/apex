package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PlatanosVerdes/apex/internal/auth"
	"github.com/PlatanosVerdes/apex/internal/gmaps"
	"github.com/PlatanosVerdes/apex/internal/limit"
	"github.com/PlatanosVerdes/apex/internal/metrics"
	"github.com/PlatanosVerdes/apex/internal/nominatim"
	"github.com/PlatanosVerdes/apex/internal/photon"
	"github.com/PlatanosVerdes/apex/internal/plan"
	"github.com/PlatanosVerdes/apex/internal/server"
	"github.com/PlatanosVerdes/apex/internal/store"
	"github.com/PlatanosVerdes/apex/internal/valhalla"
)

var buildVersion = "local"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port         = number("APEX_PORT", 8000)
		dataDir      = env("APEX_DATA_DIR", "./data")
		routerURL    = env("APEX_VALHALLA_URL", valhalla.DefaultBaseURL)
		nominatimURL = env("APEX_NOMINATIM_URL", nominatim.DefaultBaseURL)
		photonURL    = env("APEX_PHOTON_URL", photon.DefaultBaseURL)
		publicURL    = env("APEX_PUBLIC_URL", "")
		clientID     = env("APEX_GOOGLE_CLIENT_ID", "")
		logAsJSON    = env("APEX_LOG_JSON", "") == "1"
		// Never published to the tunnel: only Prometheus reads it.
		metricsAddr = env("APEX_METRICS_ADDR", "")
		// Both public servers ask for an agent that identifies the app.
		userAgent = "apex/" + buildVersion + " (+https://github.com/PlatanosVerdes/apex)"
	)

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if logAsJSON {
		log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	reg := metrics.New()
	// Every client Apex has uses the default transport, so this times every call to another server.
	http.DefaultTransport = &server.Transport{Next: http.DefaultTransport, Metrics: reg}
	users, err := store.OpenUsers(filepath.Join(dataDir, "users"))
	if err != nil {
		return err
	}
	// Cookies marked Secure only travel over HTTPS, which the public address has and localhost lacks.
	sessions, err := auth.OpenSessions(dataDir, strings.HasPrefix(publicURL, "https://"))
	if err != nil {
		return err
	}
	var google *auth.Google
	if clientID != "" {
		if publicURL == "" {
			return errors.New("APEX_GOOGLE_CLIENT_ID needs APEX_PUBLIC_URL for the sign-in callback")
		}
		google = auth.NewGoogle(clientID, env("APEX_GOOGLE_CLIENT_SECRET", ""), strings.TrimRight(publicURL, "/")+"/auth/callback", sessions)
	}
	s := &server.Server{
		Search:   photon.New(photonURL, userAgent),
		Users:    users,
		Sessions: sessions,
		Google:   google,
		DevLogin: env("APEX_DEV_LOGIN", "") == "1",
		Owner:    env("APEX_OWNER_EMAIL", ""),
		// Routes and lists saved before accounts existed; the owner takes them over on signing in.
		LegacyDir:      dataDir,
		ClientIPHeader: env("APEX_CLIENT_IP_HEADER", ""),
		Public:         limit.New(number("APEX_PUBLIC_REQUESTS", 120), duration("APEX_PUBLIC_REFILL", 500*time.Millisecond)),
		ListSource:     gmaps.New(),
		Planner:        &plan.Planner{Router: valhalla.New(routerURL, userAgent), Parallel: 2},
		// A curve search costs the public routing server five to ten requests; a fast leg costs one.
		Curves:  limit.New(number("APEX_CURVE_SEARCHES", 20), duration("APEX_CURVE_REFILL", 30*time.Second)),
		Places:  nominatim.New(nominatimURL, userAgent),
		Metrics: reg,
		Version: buildVersion,
		Log:     log,
	}
	srv := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	go s.RefreshLists(ctx, duration("APEX_LISTS_EVERY", 3*time.Hour))
	if metricsAddr != "" {
		s.Gauges()
		go serveMetrics(metricsAddr, reg, log)
	}

	log.Info("apex listening", "version", buildVersion, "port", port, "valhalla", routerURL,
		"google", google != nil, "dev_login", s.DevLogin, "public_url", publicURL)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func number(key string, def int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return def
	}
	return n
}

func duration(key string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return def
	}
	return d
}

func serveMetrics(addr string, reg *metrics.Registry, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		reg.Write(w)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Warn("metrics stopped", "err", err)
	}
}
