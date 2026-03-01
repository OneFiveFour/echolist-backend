package main

import (
	"context"
	"log"
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
	"echolist-backend/file"
	authv1connect "echolist-backend/proto/gen/auth/v1/authv1connect"
	filev1connect "echolist-backend/proto/gen/file/v1/filev1connect"
	notesv1connect "echolist-backend/proto/gen/notes/v1/notesv1connect"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
	"echolist-backend/server"
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
func parseDurationMinutesEnv(key string, defaultMinutes int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(defaultMinutes) * time.Minute
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("Invalid value for %s: %q (expected integer minutes)", key, raw)
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
	// Get data directory from environment variable, default to "./data"
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Auth configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	accessTtl := parseDurationMinutesEnv("ACCESS_TOKEN_EXPIRY_MINUTES", 15)
	refreshTtl := parseDurationMinutesEnv("REFRESH_TOKEN_EXPIRY_MINUTES", 10080) // 7 days

	// Initialize auth components — users.json lives outside the data directory
	userStore := auth.NewUserStore(filepath.Join("auth", "users.json"))
	err := userStore.LoadOrInitialize(
		envOrDefault("AUTH_DEFAULT_USER", "admin"),
		os.Getenv("AUTH_DEFAULT_PASSWORD"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}

	tokenService := auth.NewTokenService(jwtSecret, accessTtl, refreshTtl)
	authInterceptor := auth.NewAuthInterceptor(tokenService)
	interceptors := connect.WithInterceptors(authInterceptor)

	// Register handlers
	mux := http.NewServeMux()

	notesPath, notesHandler := notesv1connect.NewNoteServiceHandler(
		server.NewNotesServer(dataDir),
		interceptors,
	)
	mux.Handle(notesPath, notesHandler)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		auth.NewAuthServer(userStore, tokenService),
		interceptors,
	)
	mux.Handle(authPath, authHandler)

	filePath, fileHandler := filev1connect.NewFileServiceHandler(
		file.NewFileServer(dataDir),
		interceptors,
	)
	mux.Handle(filePath, fileHandler)

	tasksPath, tasksHandler := tasksv1connect.NewTaskListServiceHandler(
		tasks.NewTaskServer(dataDir),
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
	log.Printf("Authentication enabled. Access token TTL: %v, Refresh token TTL: %v", accessTtl, refreshTtl)
	log.Println("ConnectRPC Server listening on", address)

	// Request size limit
	maxBodyBytes := int64(4194304) // 4MB default
	if raw := os.Getenv("MAX_REQUEST_BODY_BYTES"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.Fatalf("Invalid value for MAX_REQUEST_BODY_BYTES: %q", raw)
		}
		maxBodyBytes = parsed
	}

	handler := maxBytesMiddleware(maxBodyBytes, mux)

	// Graceful shutdown
	shutdownTimeout := 30 * time.Second
	if raw := os.Getenv("SHUTDOWN_TIMEOUT_SECONDS"); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil {
			log.Fatalf("Invalid value for SHUTDOWN_TIMEOUT_SECONDS: %q", raw)
		}
		shutdownTimeout = time.Duration(secs) * time.Second
	}

	srv := &http.Server{
		Addr:    address,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		log.Println("Server shutdown complete")
	}()

	// Enable HTTP/2 support for gRPC clients
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
