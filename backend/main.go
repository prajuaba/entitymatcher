package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"entitymatcher/api"
	"entitymatcher/matcher"
	"entitymatcher/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	repo, closeStore, err := selectStore(startupCtx)
	startupCancel()
	if err != nil {
		log.Fatalf("Startup failed: %v", err)
	}
	server := api.NewServer(repo)

	// Load a previously-fitted, active calibration model (if any) so it's ready the moment
	// CalibrationEnabled is turned on -- fitting/persisting a model and enabling calibration for
	// matching runs are separate, independent operator decisions.
	if activeModel, ok, err := repo.GetActiveCalibrationModel(); err != nil {
		log.Printf("Failed to check for an active calibration model: %v", err)
	} else if ok {
		cal, err := store.UnmarshalCalibrator([]byte(activeModel.ModelJSON))
		if err != nil {
			log.Printf("Failed to load active calibration model %s: %v", activeModel.ID, err)
		} else {
			server.SetCalibrator(cal)
			log.Printf("Loaded active calibration model %s (fitted %s, %d observations)",
				activeModel.ID, activeModel.CreatedAt.Format(time.RFC3339), activeModel.ObservationCount)
		}
	}

	// Load persisted custom alias dictionary entries so operator-added aliases survive a
	// restart. The built-in defaults seeded by matcher.NewCustomDictionary() stay; persisted
	// entries are applied on top, so an operator alias overrides a default of the same name.
	if entries, err := repo.ListDictionaryEntries(); err != nil {
		log.Printf("Failed to load custom alias dictionary: %v", err)
	} else if len(entries) > 0 {
		dict := matcher.GetGlobalDictionary()
		for _, e := range entries {
			dict.Set(e.Alias, e.Canonical)
		}
		log.Printf("Loaded %d custom alias(es) from the dictionary", len(entries))
	}

	mux := http.NewServeMux()

	// Middleware helper to apply CORS and route protection
	corsHandler := newCORSMiddleware()

	// Helper to chain middlewares: iterate forward so last middleware becomes outermost
	// This ensures execution order: last middleware runs first (outermost)
	chainMiddleware := func(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
		for i := 0; i < len(middlewares); i++ {
			handler = middlewares[i](handler)
		}
		return handler
	}

	// PUBLIC routes (no auth required)
	mux.HandleFunc("/api/health", corsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok","engine":"bilingual-entity-matcher"}`)
	})).ServeHTTP)

	mux.HandleFunc("/api/auth/login", corsHandler(http.HandlerFunc(server.HandleLogin)).ServeHTTP)

	// Authenticated routes - wrap with auth middleware
	mux.HandleFunc("/api/auth/me",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleAuthMe))).ServeHTTP)

	// Config: GET for any authenticated, PUT/POST for ADMIN,ENGINEER
	mux.HandleFunc("/api/config", corsHandler(chainMiddleware(
		http.HandlerFunc(server.HandleConfig),
		newMethodRoleMiddleware([]string{"PUT", "POST"}, api.RoleAdmin, api.RoleEngineer),
		api.RequireAuth,
	)).ServeHTTP)

	// Upload (ADMIN, ENGINEER)
	mux.HandleFunc("/api/upload",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleUpload),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Upload from real CSV/Excel files (ADMIN, ENGINEER)
	mux.HandleFunc("/api/upload/file",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleUploadFile),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Match run (ADMIN, ENGINEER)
	mux.HandleFunc("/api/match/run",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleRunMatch),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Match progress with query token support
	mux.HandleFunc("/api/match/progress",
		corsHandler(api.RequireAuthAllowQueryToken(http.HandlerFunc(server.HandleSSEProgress))).ServeHTTP)

	// Match results (authenticated)
	mux.HandleFunc("/api/match/results",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleGetResults))).ServeHTTP)

	// Job history (authenticated)
	mux.HandleFunc("/api/jobs",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleListJobs))).ServeHTTP)

	// Match action (ADMIN, REVIEWER)
	mux.HandleFunc("/api/match/action",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleMatchAction),
			api.RequireRole(api.RoleAdmin, api.RoleReviewer),
			api.RequireAuth,
		)).ServeHTTP)

	// Manual link (ADMIN, REVIEWER)
	mux.HandleFunc("/api/match/manual-link",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleManualLink),
			api.RequireRole(api.RoleAdmin, api.RoleReviewer),
			api.RequireAuth,
		)).ServeHTTP)

	// Seed (ADMIN, ENGINEER)
	mux.HandleFunc("/api/seed",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleSeedDataset),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Seed big (ADMIN, ENGINEER)
	mux.HandleFunc("/api/seed/big",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleSeedBigDataset),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Connector test (ADMIN, ENGINEER)
	mux.HandleFunc("/api/connector/test",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleTestConnector),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Connector introspect (ADMIN, ENGINEER)
	mux.HandleFunc("/api/connector/introspect",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleIntrospectSchema),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Connector introspect from an uploaded file (ADMIN, ENGINEER)
	mux.HandleFunc("/api/connector/introspect/upload",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleIntrospectUploadedFile),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Connector ingest (ADMIN, ENGINEER)
	mux.HandleFunc("/api/connector/ingest",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleConnectorIngest),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Connector settings: GET for any authenticated, PUT/POST for ADMIN,ENGINEER
	mux.HandleFunc("/api/connector/settings",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleConnectorSettings),
			newMethodRoleMiddleware([]string{"PUT", "POST"}, api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Dictionary: GET for any authenticated, POST for ADMIN,ENGINEER
	mux.HandleFunc("/api/dictionary",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleDictionary),
			newMethodRoleMiddleware([]string{"POST"}, api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Scheduler config: GET for any authenticated, POST/PUT for ADMIN only
	mux.HandleFunc("/api/scheduler/config",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleSchedulerConfig),
			newMethodRoleMiddleware([]string{"POST", "PUT"}, api.RoleAdmin),
			api.RequireAuth,
		)).ServeHTTP)

	// Destinations search (authenticated)
	mux.HandleFunc("/api/destinations/search",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleSearchDestinations))).ServeHTTP)

	// Export CSV (authenticated)
	mux.HandleFunc("/api/export/csv",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleExportCSV))).ServeHTTP)

	// LLM evaluate (authenticated)
	mux.HandleFunc("/api/llm/evaluate",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleLLMEvaluate))).ServeHTTP)

	// Audit logs (ADMIN, AUDITOR)
	mux.HandleFunc("/api/audit/logs",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleGetAuditLogs),
			api.RequireRole(api.RoleAdmin, api.RoleAuditor),
			api.RequireAuth,
		)).ServeHTTP)

	// Audit export (ADMIN, AUDITOR)
	mux.HandleFunc("/api/audit/export",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleExportAuditCSV),
			api.RequireRole(api.RoleAdmin, api.RoleAuditor),
			api.RequireAuth,
		)).ServeHTTP)

	// Calibration fit (ADMIN only)
	mux.HandleFunc("/api/calibration/fit",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleCalibrationFit),
			api.RequireRole(api.RoleAdmin),
			api.RequireAuth,
		)).ServeHTTP)

	// Calibration status (ADMIN only)
	mux.HandleFunc("/api/calibration/status",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleCalibrationStatus),
			api.RequireRole(api.RoleAdmin),
			api.RequireAuth,
		)).ServeHTTP)

	// Serve Frontend Static Files
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.Handle("/", fs)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting High-Scale Bilingual Entity Matcher Backend API on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutdown signal received, stopping gracefully...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop scheduler
	server.StopScheduler()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	closeStore()
	log.Println("Server shutdown complete")
}

// selectStore chooses the repository backing this process.
//
// When DATABASE_URL is empty the process runs on the in-memory store and says so,
// because that data does not survive a restart.
//
// When DATABASE_URL is set, Postgres is REQUIRED: an unreachable database is a fatal
// startup error, never a silent fall back to memory. Degrading quietly would leave an
// operator who asked for persistence running an audit trail that evaporates on restart,
// which is worse than failing to boot.
//
// The returned closer releases the pool; it is a no-op for the in-memory store.
func selectStore(ctx context.Context) (store.Repository, func(), error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		s := store.NewStore()
		log.Println("Using in-memory store; data will be lost on restart.")
		return s, func() {}, nil
	}

	pg, err := store.NewPostgresStore(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("DATABASE_URL is set, PostgreSQL is required, but connecting failed: %w", err)
	}

	log.Println("Using PostgreSQL persistence.")
	return pg, pg.Close, nil
}

// newCORSMiddleware creates a CORS middleware that reads allowed origins from env.
func newCORSMiddleware() func(http.Handler) http.Handler {
	corsOrigins := os.Getenv("CORS_ORIGINS")
	var allowedOrigins map[string]bool
	if corsOrigins != "" {
		allowedOrigins = make(map[string]bool)
		for _, origin := range strings.Split(corsOrigins, ",") {
			allowedOrigins[strings.TrimSpace(origin)] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle preflight
			if r.Method == "OPTIONS" {
				if allowedOrigins != nil {
					origin := r.Header.Get("Origin")
					if allowedOrigins[origin] {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// For actual requests
			if allowedOrigins != nil {
				origin := r.Header.Get("Origin")
				if allowedOrigins[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			next.ServeHTTP(w, r)
		})
	}
}

// newMethodRoleMiddleware creates middleware that checks role only for specific HTTP methods.
// If the current request method is in the methods slice, role check is enforced.
// Otherwise, the request passes through without role restriction.
func newMethodRoleMiddleware(methods []string, roles ...api.Role) func(http.Handler) http.Handler {
	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[m] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if methodSet[r.Method] {
				// Apply role check via RequireRole
				api.RequireRole(roles...)(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}
