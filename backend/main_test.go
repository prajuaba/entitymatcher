package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/api"
	"entitymatcher/store"
)

// getTestToken generates a token for a specific role by logging in as that user.
func getTestToken(t *testing.T, username, password string) string {
	server := api.NewServer(store.NewStore())

	payload := api.LoginRequest{
		Username: username,
		Password: password,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get token for %s: %d", username, w.Code)
	}

	var resp api.LoginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Token
}

// TestRoutingPolicies tests route access control for different roles.
func TestRoutingPolicies(t *testing.T) {
	memStore := store.NewStore()
	server := api.NewServer(memStore)

	// Set up test server with routes
	mux := http.NewServeMux()
	corsHandler := newCORSMiddleware()

	// Helper to chain middlewares
	chainMiddleware := func(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
		for i := 0; i < len(middlewares); i++ {
			handler = middlewares[i](handler)
		}
		return handler
	}

	// Register representative routes for testing
	mux.HandleFunc("/api/auth/login", corsHandler(http.HandlerFunc(server.HandleLogin)).ServeHTTP)
	mux.HandleFunc("/api/auth/me", corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleAuthMe))).ServeHTTP)

	// Config: GET for any authenticated, PUT/POST for ADMIN,ENGINEER
	mux.HandleFunc("/api/config", corsHandler(chainMiddleware(
		http.HandlerFunc(server.HandleConfig),
		newMethodRoleMiddleware([]string{"PUT", "POST"}, api.RoleAdmin, api.RoleEngineer),
		api.RequireAuth,
	)).ServeHTTP)

	// Connector test: ADMIN,ENGINEER
	mux.HandleFunc("/api/connector/test",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleTestConnector),
			api.RequireRole(api.RoleAdmin, api.RoleEngineer),
			api.RequireAuth,
		)).ServeHTTP)

	// Match results: any authenticated (simpler than match/action for testing)
	mux.HandleFunc("/api/match/results",
		corsHandler(api.RequireAuth(http.HandlerFunc(server.HandleGetResults))).ServeHTTP)

	// Audit logs: ADMIN,AUDITOR
	mux.HandleFunc("/api/audit/logs",
		corsHandler(chainMiddleware(
			http.HandlerFunc(server.HandleGetAuditLogs),
			api.RequireRole(api.RoleAdmin, api.RoleAuditor),
			api.RequireAuth,
		)).ServeHTTP)

	testCases := []struct {
		name           string
		method         string
		route          string
		token          string
		expectedStatus int
	}{
		// ADMIN tests
		{name: "ADMIN GET config", method: "GET", route: "/api/config", token: getTestToken(t, "admin", "password123"), expectedStatus: http.StatusOK},
		{name: "ADMIN POST connector/test", method: "POST", route: "/api/connector/test", token: getTestToken(t, "admin", "password123"), expectedStatus: http.StatusOK},
		{name: "ADMIN GET audit/logs", method: "GET", route: "/api/audit/logs", token: getTestToken(t, "admin", "password123"), expectedStatus: http.StatusOK},

		// ENGINEER tests
		{name: "ENGINEER GET config", method: "GET", route: "/api/config", token: getTestToken(t, "engineer_alex", "password123"), expectedStatus: http.StatusOK},
		{name: "ENGINEER POST connector/test", method: "POST", route: "/api/connector/test", token: getTestToken(t, "engineer_alex", "password123"), expectedStatus: http.StatusOK},
		{name: "ENGINEER GET audit/logs (forbidden)", method: "GET", route: "/api/audit/logs", token: getTestToken(t, "engineer_alex", "password123"), expectedStatus: http.StatusForbidden},

		// REVIEWER tests
		{name: "REVIEWER GET config", method: "GET", route: "/api/config", token: getTestToken(t, "reviewer_sarah", "password123"), expectedStatus: http.StatusOK},
		{name: "REVIEWER POST connector/test (forbidden)", method: "POST", route: "/api/connector/test", token: getTestToken(t, "reviewer_sarah", "password123"), expectedStatus: http.StatusForbidden},
		{name: "REVIEWER GET match/results", method: "GET", route: "/api/match/results", token: getTestToken(t, "reviewer_sarah", "password123"), expectedStatus: http.StatusOK},

		// AUDITOR tests
		{name: "AUDITOR GET config", method: "GET", route: "/api/config", token: getTestToken(t, "auditor_mike", "password123"), expectedStatus: http.StatusOK},
		{name: "AUDITOR PUT config (forbidden)", method: "PUT", route: "/api/config", token: getTestToken(t, "auditor_mike", "password123"), expectedStatus: http.StatusForbidden},
		{name: "AUDITOR GET audit/logs", method: "GET", route: "/api/audit/logs", token: getTestToken(t, "auditor_mike", "password123"), expectedStatus: http.StatusOK},

		// No token tests
		{name: "No token GET config", method: "GET", route: "/api/config", token: "", expectedStatus: http.StatusUnauthorized},
		{name: "No token POST connector/test", method: "POST", route: "/api/connector/test", token: "", expectedStatus: http.StatusUnauthorized},
		{name: "No token GET audit/logs", method: "GET", route: "/api/audit/logs", token: "", expectedStatus: http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			var route string = tc.route

			if tc.method == "POST" || tc.method == "PUT" {
				// Provide appropriate request body for different routes
				switch tc.route {
				case "/api/config":
					// Provide valid config update
					payload := map[string]interface{}{
						"auto_match_threshold": 0.9,
					}
					body, _ = json.Marshal(payload)
				default:
					// Provide minimal request body for other POST/PUT routes
					body, _ = json.Marshal(map[string]interface{}{})
				}
			}

			// Add batch_id query param for routes that need it
			if tc.route == "/api/match/results" {
				route = tc.route + "?batch_id=test-batch"
			}

			req := httptest.NewRequest(tc.method, route, bytes.NewBuffer(body))
			if tc.token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tc.token))
			}

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
