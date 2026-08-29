package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequireAuthNoToken tests RequireAuth without token returns 401.
func TestRequireAuthNoToken(t *testing.T) {
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "unauthorized" {
		t.Fatalf("Expected error 'unauthorized', got %s", resp["error"])
	}
}

// TestRequireAuthInvalidToken tests RequireAuth with invalid token.
func TestRequireAuthInvalidToken(t *testing.T) {
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestRequireAuthValidToken tests RequireAuth with valid token.
func TestRequireAuthValidToken(t *testing.T) {
	server := &Server{}

	// Get a valid token
	payload := LoginRequest{
		Username: "admin",
		Password: "password123",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	server.HandleLogin(w, req)

	var loginResp LoginResponse
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp.Token

	// Test RequireAuth middleware
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFrom(r.Context())
		if claims == nil {
			t.Fatal("Expected claims in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

// TestRequireAuthOPTION tests OPTIONS preflight bypasses auth.
func TestRequireAuthOPTION(t *testing.T) {
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/protected", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", w.Code)
	}
}

// TestRequireRole tests role-based access control.
func TestRequireRole(t *testing.T) {
	// Create a context with AUDITOR claims
	auditorClaims := &JWTClaims{
		UserID:   "usr-04",
		Username: "auditor_mike",
		Name:     "Mike (Compliance Auditor)",
		Role:     RoleAuditor,
	}

	ctx := context.WithValue(context.Background(), claimsKey{}, auditorClaims)

	// Test: AUDITOR accessing ADMIN-only endpoint should fail
	handler := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "forbidden" {
		t.Fatalf("Expected error 'forbidden', got %s", resp["error"])
	}
}

// TestRequireRoleAllowed tests role-based access when role is allowed.
func TestRequireRoleAllowed(t *testing.T) {
	// Create a context with AUDITOR claims
	auditorClaims := &JWTClaims{
		UserID:   "usr-04",
		Username: "auditor_mike",
		Name:     "Mike (Compliance Auditor)",
		Role:     RoleAuditor,
	}

	ctx := context.WithValue(context.Background(), claimsKey{}, auditorClaims)

	// Test: AUDITOR accessing endpoint that allows AUDITOR role should succeed
	handlerCalled := false
	handler := RequireRole(RoleAdmin, RoleAuditor)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/audit", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	if !handlerCalled {
		t.Fatal("Expected handler to be called")
	}
}

// TestRequireRoleNoClaims tests that missing claims returns 401.
func TestRequireRoleNoClaims(t *testing.T) {
	handler := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestRequireAuthAllowQueryToken tests token from query parameter.
func TestRequireAuthAllowQueryToken(t *testing.T) {
	server := &Server{}

	// Get a valid token
	payload := LoginRequest{
		Username: "admin",
		Password: "password123",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	server.HandleLogin(w, req)

	var loginResp LoginResponse
	json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp.Token

	// Test RequireAuthAllowQueryToken with token in query parameter
	handler := RequireAuthAllowQueryToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFrom(r.Context())
		if claims == nil {
			t.Fatal("Expected claims in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req = httptest.NewRequest("GET", fmt.Sprintf("/stream?access_token=%s", token), nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

// TestClaimsFrom tests retrieving claims from context.
func TestClaimsFrom(t *testing.T) {
	claims := &JWTClaims{
		UserID:   "usr-01",
		Username: "admin",
		Role:     RoleAdmin,
	}

	ctx := context.WithValue(context.Background(), claimsKey{}, claims)
	retrievedClaims := ClaimsFrom(ctx)

	if retrievedClaims == nil {
		t.Fatal("Expected claims to be retrieved")
	}

	if retrievedClaims.UserID != "usr-01" {
		t.Fatalf("Expected user_id usr-01, got %s", retrievedClaims.UserID)
	}

	if retrievedClaims.Role != RoleAdmin {
		t.Fatalf("Expected role ADMIN, got %s", retrievedClaims.Role)
	}
}

// TestClaimsFromMissing tests that ClaimsFrom returns nil when claims not in context.
func TestClaimsFromMissing(t *testing.T) {
	ctx := context.Background()
	claims := ClaimsFrom(ctx)

	if claims != nil {
		t.Fatal("Expected nil when claims not in context")
	}
}
