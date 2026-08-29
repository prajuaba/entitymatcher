package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleEngineer Role = "ENGINEER"
	RoleReviewer Role = "REVIEWER"
	RoleAuditor  Role = "AUDITOR"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
}

var sampleUsers = map[string]User{
	"admin":          {ID: "usr-01", Username: "admin", Password: "password123", Name: "Enterprise Admin", Role: RoleAdmin},
	"engineer_alex": {ID: "usr-02", Username: "engineer_alex", Password: "password123", Name: "Alex (Data Engineer)", Role: RoleEngineer},
	"reviewer_sarah": {ID: "usr-03", Username: "reviewer_sarah", Password: "password123", Name: "Sarah (Review Operator)", Role: RoleReviewer},
	"auditor_mike":   {ID: "usr-04", Username: "auditor_mike", Password: "password123", Name: "Mike (Compliance Auditor)", Role: RoleAuditor},
}

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	Exp      int64  `json:"exp"`
}

var jwtSecret = []byte("antigravity-entity-matcher-secret-key-2026")

func generateToken(user User) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsObj := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Name:     user.Name,
		Role:     user.Role,
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}

	claimsBytes, _ := json.Marshal(claimsObj)
	claims := base64.RawURLEncoding.EncodeToString(claimsBytes)

	unsignedToken := header + "." + claims
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(unsignedToken))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return unsignedToken + "." + signature, nil
}

func parseToken(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	unsignedToken := parts[0] + "." + parts[1]
	h := hmac.New(sha256.New, jwtSecret)
	h.Write([]byte(unsignedToken))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, err
	}

	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	u, exists := sampleUsers[req.Username]
	if !exists || (req.Password != "password123" && req.Password != u.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := generateToken(u)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: token,
		User:  u,
	})
}

func (s *Server) HandleAuthMe(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		// Default to guest admin for demo
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleUsers["admin"])
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := parseToken(tokenStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       claims.UserID,
		"username": claims.Username,
		"name":     claims.Name,
		"role":     claims.Role,
	})
}
