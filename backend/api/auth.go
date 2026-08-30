package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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

// sampleUsers is populated at package init with bcrypt hashes of demo passwords.
// Each user's password is hashed with bcrypt cost 10.
var sampleUsers = make(map[string]User)

// jwtSecret is read from environment at package init or generated if not set.
var jwtSecret []byte

// dummyHash is a real bcrypt hash used for timing-safe password comparison
// when a user is not found. Generated at init time.
var dummyHash []byte

// initSampleUsers initializes demo users with bcrypt-hashed passwords.
func initSampleUsers() {
	// Demo passwords (all "password123" for easy testing)
	demoPassword := "password123"
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), 10)
	if err != nil {
		log.Fatalf("Failed to hash demo password: %v", err)
	}

	sampleUsers["admin"] = User{
		ID:       "usr-01",
		Username: "admin",
		Password: string(hash),
		Name:     "Enterprise Admin",
		Role:     RoleAdmin,
	}

	sampleUsers["engineer_alex"] = User{
		ID:       "usr-02",
		Username: "engineer_alex",
		Password: string(hash),
		Name:     "Alex (Data Engineer)",
		Role:     RoleEngineer,
	}

	sampleUsers["reviewer_sarah"] = User{
		ID:       "usr-03",
		Username: "reviewer_sarah",
		Password: string(hash),
		Name:     "Sarah (Review Operator)",
		Role:     RoleReviewer,
	}

	sampleUsers["auditor_mike"] = User{
		ID:       "usr-04",
		Username: "auditor_mike",
		Password: string(hash),
		Name:     "Mike (Compliance Auditor)",
		Role:     RoleAuditor,
	}
}

// initDummyHash generates a real bcrypt hash for timing-safe user-not-found comparison.
func initDummyHash() {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy"), 10)
	if err != nil {
		log.Fatalf("Failed to generate dummy hash: %v", err)
	}
	dummyHash = hash
}

// initJWTSecret reads JWT_SECRET from environment or generates a random key.
func initJWTSecret() {
	envSecret := os.Getenv("JWT_SECRET")
	if envSecret != "" {
		jwtSecret = []byte(envSecret)
		return
	}

	// Generate random 32-byte key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("Failed to generate random JWT secret: %v", err)
	}
	jwtSecret = key

	log.Printf("WARNING: JWT_SECRET not set. Generated random key at startup. Tokens will NOT survive server restart.")
}

// init runs at package load time to set up users, dummy hash, and JWT secret.
func init() {
	initSampleUsers()
	initDummyHash()
	initJWTSecret()
}

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     Role   `json:"role"`
	Exp      int64  `json:"exp"`
}


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
	if !exists {
		// Constant-time comparison: always do password check even if user not found
		// Use real dummy hash to avoid timing attacks and user enumeration
		bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Compare password using bcrypt (constant-time)
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := parseToken(tokenStr)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
