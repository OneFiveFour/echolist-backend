package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"echolist-backend/auth"
	"echolist-backend/common"
	"echolist-backend/file"
	authv1connect "echolist-backend/proto/gen/auth/v1/authv1connect"
	filev1connect "echolist-backend/proto/gen/file/v1/filev1connect"
	notesv1connect "echolist-backend/proto/gen/notes/v1/notesv1connect"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
	"echolist-backend/notes"
	"echolist-backend/tasks"
)

// envOrDefault returns the value of the environment variable named by key,
// or defaultVal if the variable is not set or empty.
func envOrDefault(key, defaultVal string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// parseDurationMinutesEnv reads an environment variable as an integer number
// of minutes and returns it as a time.Duration. If the variable is not set,
// defaultMinutes is used. The program exits fatally on parse errors.
func parseDurationMinutesEnv(logger *slog.Logger, key string, defaultMinutes int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(defaultMinutes) * time.Minute
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil {
		logger.Error("invalid env value", "key", key, "value", raw, "expected", "integer minutes")
		os.Exit(1)
	}
	return time.Duration(minutes) * time.Minute
}

// maxBytesMiddleware wraps a handler to limit request body size.
func maxBytesMiddleware(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Get data directory from environment variable, default to "./data"
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Auth configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	accessTtl := parseDurationMinutesEnv(logger, "ACCESS_TOKEN_EXPIRY_MINUTES", 15)
	refreshTtl := parseDurationMinutesEnv(logger, "REFRESH_TOKEN_EXPIRY_MINUTES", 10080) // 7 days

	// Initialize auth components — users.json lives outside the data directory
	userStore := auth.NewUserStore(filepath.Join("auth", "users.json"))
	err := userStore.LoadOrInitialize(
		envOrDefault("AUTH_DEFAULT_USER", "admin"),
		os.Getenv("AUTH_DEFAULT_PASSWORD"),
	)
	if err != nil {
		logger.Error("failed to initialize user store", "error", err)
		os.Exit(1)
	}

	tokenService := auth.NewTokenService(jwtSecret, accessTtl, refreshTtl)
	authInterceptor := auth.NewAuthInterceptor(tokenService, logger)
	loggingInterceptor := common.NewRequestLoggingInterceptor(logger)
	interceptors := connect.WithInterceptors(loggingInterceptor, authInterceptor)

	// Register handlers
	mux := http.NewServeMux()

	// Liveness probe — process is up, that's it
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness probe — verify data dir is accessible and user store loaded
	mux.HandleFunc("/healthz", healthzHandler(dataDir, userStore))

	notesPath, notesHandler := notesv1connect.NewNoteServiceHandler(
		notes.NewNotesServer(dataDir, logger),
		interceptors,
	)
	mux.Handle(notesPath, notesHandler)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		auth.NewAuthServer(userStore, tokenService, logger),
		interceptors,
	)
	mux.Handle(authPath, authHandler)

	filePath, fileHandler := filev1connect.NewFileServiceHandler(
		file.NewFileServer(dataDir, logger),
		interceptors,
	)
	mux.Handle(filePath, fileHandler)

	tasksPath, tasksHandler := tasksv1connect.NewTaskListServiceHandler(
		tasks.NewTaskServer(dataDir, logger),
		interceptors,
	)
	mux.Handle(tasksPath, tasksHandler)

	// Enable gRPC reflection for tools like grpcurl
	reflector := grpcreflect.NewStaticReflector(
		"notes.v1.NoteService",
		"auth.v1.AuthService",
		"file.v1.FileService",
		"tasks.v1.TaskListService",
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	address := ":8080"
	logger.Info("server starting",
		"address", address,
		"accessTokenTTL", accessTtl.String(),
		"refreshTokenTTL", refreshTtl.String(),
	)

	// Request size limit
	maxBodyBytes := int64(4194304) // 4MB default
	if raw := os.Getenv("MAX_REQUEST_BODY_BYTES"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			logger.Error("invalid env value", "key", "MAX_REQUEST_BODY_BYTES", "value", raw)
			os.Exit(1)
		}
		maxBodyBytes = parsed
	}

	handler := maxBytesMiddleware(maxBodyBytes, mux)

	// Graceful shutdown
	shutdownTimeout := 30 * time.Second
	if raw := os.Getenv("SHUTDOWN_TIMEOUT_SECONDS"); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil {
			logger.Error("invalid env value", "key", "SHUTDOWN_TIMEOUT_SECONDS", "value", raw)
			os.Exit(1)
		}
		shutdownTimeout = time.Duration(secs) * time.Second
	}

	srv := &http.Server{
		Addr:         address,
		Handler:      h2c.NewHandler(handler, &http2.Server{}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("shutdown error", "error", err)
		}
		logger.Info("server shutdown complete")
	}()

	// Enable HTTP/2 support for gRPC clients
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// healthzHandler returns an http.HandlerFunc that checks data directory
// accessibility and user store state, returning 503 on failure.
func healthzHandler(dataDir string, userStore *auth.UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var checks []string

		// Check 1: data directory is stat-able
		if _, err := os.Stat(dataDir); err != nil {
			checks = append(checks, "data_dir: "+err.Error())
		}

		// Check 2: data directory is writable (create + remove temp file)
		if len(checks) == 0 {
			f, err := os.CreateTemp(dataDir, ".healthz_probe_*")
			if err != nil {
				checks = append(checks, "data_dir_write: "+err.Error())
			} else {
				f.Close()
				os.Remove(f.Name())
			}
		}

		// Check 3: user store has at least one user loaded
		if !userStore.HasUsers() {
			checks = append(checks, "user_store: no users loaded")
		}

		if len(checks) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy\n"))
			for _, c := range checks {
				w.Write([]byte("- " + c + "\n"))
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
