package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"
)

// User represents a system user
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email" binding:"required,email"`
	Password  string    `json:"password,omitempty" binding:"required,min=8"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Home represents a home/location
type Home struct {
	ID        string    `json:"id"`
	Name      string    `json:"name" binding:"required"`
	OwnerID   string    `json:"owner_id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

// JWT Claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// Login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

var (
	jwtSecret = []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production"))

	// Prometheus metrics
	authAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_management_auth_attempts_total",
			Help: "Total authentication attempts",
		},
		[]string{"method", "status"},
	)
	registrations = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_management_registrations_total",
			Help: "Total user registrations",
		},
	)
)

func init() {
	prometheus.MustRegister(authAttempts)
	prometheus.MustRegister(registrations)
}

type UserService struct {
	db *sql.DB
}

func main() {
	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize database
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	service := &UserService{db: db}

	// Initialize Gin router
	router := gin.Default()
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "user-management"})
	})

	// Metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public routes (no auth required)
	public := router.Group("/api/auth")
	{
		public.POST("/register", service.Register)
		public.POST("/login", service.Login)
		public.POST("/refresh", service.RefreshToken)
	}

	// Protected routes (auth required)
	protected := router.Group("/api/users")
	protected.Use(authMiddleware())
	{
		protected.GET("/profile", service.GetProfile)
		protected.PUT("/profile", service.UpdateProfile)
		protected.GET("/homes", service.ListHomes)
		protected.POST("/homes", service.CreateHome)
	}

	// Start server
	port := getEnv("USER_MANAGEMENT_PORT", "8086")
	log.Printf("User Management Service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*sql.DB, error) {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "smarthome")
	password := getEnv("POSTGRES_PASSWORD", "smarthome_dev_password")
	dbname := "user_management_db"

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Successfully connected to database")
	return db, nil
}

// Register a new user
func (s *UserService) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email already exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if exists {
		authAttempts.WithLabelValues("register", "duplicate").Inc()
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	userID := uuid.New().String()
	query := `
		INSERT INTO users (id, email, password_hash, full_name, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	now := time.Now()
	_, err = s.db.Exec(query, userID, req.Email, string(hashedPassword), req.FullName, req.Phone, now, now)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, err := generateToken(userID, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	registrations.Inc()
	authAttempts.WithLabelValues("register", "success").Inc()

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": userID,
		"token":   token,
	})
}

// Login user
func (s *UserService) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	var user User
	var passwordHash string

	query := "SELECT id, email, password_hash, full_name FROM users WHERE email = $1"
	err := s.db.QueryRow(query, req.Email).Scan(&user.ID, &user.Email, &passwordHash, &user.FullName)

	if err == sql.ErrNoRows {
		authAttempts.WithLabelValues("login", "invalid_email").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if err != nil {
		log.Printf("Error querying user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		authAttempts.WithLabelValues("login", "invalid_password").Inc()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := generateToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	authAttempts.WithLabelValues("login", "success").Inc()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Login successful",
		"token":     token,
		"user_id":   user.ID,
		"email":     user.Email,
		"full_name": user.FullName,
	})
}

// Refresh token
func (s *UserService) RefreshToken(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
		return
	}

	// Remove "Bearer " prefix
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Parse token (even if expired)
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		return
	}

	// Generate new token
	newToken, err := generateToken(claims.UserID, claims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": newToken,
	})
}

// Get user profile
func (s *UserService) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var user User
	query := "SELECT id, email, full_name, phone, created_at, updated_at FROM users WHERE id = $1"
	err := s.db.QueryRow(query, userID).Scan(&user.ID, &user.Email, &user.FullName, &user.Phone, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err != nil {
		log.Printf("Error getting user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Update user profile
func (s *UserService) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var updates struct {
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := "UPDATE users SET full_name = $1, phone = $2, updated_at = $3 WHERE id = $4"
	_, err := s.db.Exec(query, updates.FullName, updates.Phone, time.Now(), userID)

	if err != nil {
		log.Printf("Error updating user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

// List user's homes
func (s *UserService) ListHomes(c *gin.Context) {
	userID := c.GetString("user_id")

	query := `
		SELECT h.id, h.name, h.owner_id, h.address, h.created_at
		FROM homes h
		JOIN user_homes uh ON h.id = uh.home_id
		WHERE uh.user_id = $1
		ORDER BY h.created_at DESC
	`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		log.Printf("Error listing homes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list homes"})
		return
	}
	defer rows.Close()

	homes := []Home{}
	for rows.Next() {
		var home Home
		err := rows.Scan(&home.ID, &home.Name, &home.OwnerID, &home.Address, &home.CreatedAt)
		if err != nil {
			log.Printf("Error scanning home: %v", err)
			continue
		}
		homes = append(homes, home)
	}

	c.JSON(http.StatusOK, gin.H{"homes": homes, "count": len(homes)})
}

// Create home
func (s *UserService) CreateHome(c *gin.Context) {
	userID := c.GetString("user_id")

	var home Home
	if err := c.ShouldBindJSON(&home); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	homeID := uuid.New().String()

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create home"})
		return
	}
	defer tx.Rollback()

	// Create home
	query := "INSERT INTO homes (id, name, owner_id, address, created_at) VALUES ($1, $2, $3, $4, $5)"
	_, err = tx.Exec(query, homeID, home.Name, userID, home.Address, time.Now())
	if err != nil {
		log.Printf("Error creating home: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create home"})
		return
	}

	// Add user as owner
	query = "INSERT INTO user_homes (user_id, home_id, role) VALUES ($1, $2, $3)"
	_, err = tx.Exec(query, userID, homeID, "owner")
	if err != nil {
		log.Printf("Error adding user to home: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create home"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create home"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Home created successfully",
		"home_id": homeID,
	})
}

// Generate JWT token
func generateToken(userID, email string) (string, error) {
	expiryStr := getEnv("JWT_EXPIRY", "24h")
	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		expiry = 24 * time.Hour
	}

	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Auth middleware
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No authorization token provided"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		// Parse token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)

		c.Next()
	}
}

// CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// Helper function
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
