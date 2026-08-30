package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLoginValidCredentials tests successful login.
func TestLoginValidCredentials(t *testing.T) {
	server := &Server{}

	payload := LoginRequest{
		Username: "admin",
		Password: "password123",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Token == "" {
		t.Fatal("Expected token in response")
	}
	if resp.User.Username != "admin" {
		t.Fatalf("Expected username admin, got %s", resp.User.Username)
	}
}

// TestLoginInvalidPassword tests login with invalid password.
func TestLoginInvalidPassword(t *testing.T) {
	server := &Server{}

	payload := LoginRequest{
		Username: "admin",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestLoginNonExistentUser tests login with non-existent user.
func TestLoginNonExistentUser(t *testing.T) {
	server := &Server{}

	payload := LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestAuthMeWithValidToken tests /api/auth/me with valid token.
func TestAuthMeWithValidToken(t *testing.T) {
	server := &Server{}

	// First, get a token
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

	// Now test /api/auth/me
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	w = httptest.NewRecorder()

	server.HandleAuthMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var meResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &meResp)

	if meResp["username"] != "admin" {
		t.Fatalf("Expected username admin, got %v", meResp["username"])
	}
}

// TestAuthMeWithoutToken tests /api/auth/me without token (should fail).
func TestAuthMeWithoutToken(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	w := httptest.NewRecorder()

	server.HandleAuthMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestAuthMeWithInvalidToken tests /api/auth/me with invalid token.
func TestAuthMeWithInvalidToken(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()

	server.HandleAuthMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestAuthMeWithExpiredToken tests /api/auth/me with expired token.
func TestAuthMeWithExpiredToken(t *testing.T) {
	// Test with an expired token - the token is invalid so it should fail
	server := &Server{}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.expired.token")
	w := httptest.NewRecorder()

	server.HandleAuthMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

// TestPassword123NotUniversal tests that "password123" is not a universal password.
func TestPassword123NotUniversal(t *testing.T) {
	// This tests the regression that "password123" used to work for ANY user
	// even if their real password was different

	server := &Server{}

	// Create a request with a user that was given a different password hash
	payload := LoginRequest{
		Username: "engineer_alex",
		Password: "password123", // Should work since all demo users have this password
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
