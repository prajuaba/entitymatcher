package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"entitymatcher/api"
	"entitymatcher/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	memStore := store.NewStore()
	server := api.NewServer(memStore)

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/config", server.HandleConfig)
	mux.HandleFunc("/api/upload", server.HandleUpload)
	mux.HandleFunc("/api/match/run", server.HandleRunMatch)
	mux.HandleFunc("/api/match/progress", server.HandleSSEProgress)
	mux.HandleFunc("/api/match/results", server.HandleGetResults)
	mux.HandleFunc("/api/match/action", server.HandleMatchAction)
	mux.HandleFunc("/api/match/manual-link", server.HandleManualLink)
	mux.HandleFunc("/api/destinations/search", server.HandleSearchDestinations)
	mux.HandleFunc("/api/llm/evaluate", server.HandleLLMEvaluate)
	mux.HandleFunc("/api/export/csv", server.HandleExportCSV)
	mux.HandleFunc("/api/seed", server.HandleSeedDataset)
	mux.HandleFunc("/api/seed/big", server.HandleSeedBigDataset)
	mux.HandleFunc("/api/connector/test", server.HandleTestConnector)
	mux.HandleFunc("/api/connector/introspect", server.HandleIntrospectSchema)
	mux.HandleFunc("/api/audit/logs", server.HandleGetAuditLogs)
	mux.HandleFunc("/api/audit/export", server.HandleExportAuditCSV)
	mux.HandleFunc("/api/auth/login", server.HandleLogin)
	mux.HandleFunc("/api/auth/me", server.HandleAuthMe)
	mux.HandleFunc("/api/scheduler/config", server.HandleSchedulerConfig)
	mux.HandleFunc("/api/dictionary", server.HandleDictionary)

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok","engine":"bilingual-entity-matcher"}`)
	})

	// Serve Frontend Static Files
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.Handle("/", fs)

	log.Printf("Starting High-Scale Bilingual Entity Matcher Backend API on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
