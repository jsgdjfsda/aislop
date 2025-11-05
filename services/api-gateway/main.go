package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

var (
	jwtSecret    = []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production"))
	redisClient  *redis.Client
	ctx          = context.Background()
	rateLimiters = make(map[string]*rate.Limiter)

	// Prometheus metrics
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_http_requests_total",
			Help: "Total HTTP requests through gateway",
		},
		[]string{"service", "method", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_gateway_http_duration_seconds",
			Help: "HTTP request latencies",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)
	rateLimitExceeded = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_gateway_rate_limit_exceeded_total",
			Help: "Total requests that exceeded rate limit",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
	prometheus.MustRegister(rateLimitExceeded)
}

// Service endpoints configuration
type ServiceEndpoint struct {
	Name string
	URL  string
}

var services = map[string]ServiceEndpoint{
	"device-registry":   {Name: "device-registry", URL: "http://localhost:8081"},
	"data-ingestion":    {Name: "data-ingestion", URL: "http://localhost:8082"},
	"rules-engine":      {Name: "rules-engine", URL: "http://localhost:8083"},
	"analytics":         {Name: "analytics", URL: "http://localhost:8084"},
	"notification":      {Name: "notification", URL: "http://localhost:8085"},
	"user-management":   {Name: "user-management", URL: "http://localhost:8086"},
}

// JWT Claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func main() {
	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize Redis
	initRedis()

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())
	router.Use(corsMiddleware())
	router.Use(rateLimitMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
			"services": map[string]bool{
				"device-registry":   checkServiceHealth("device-registry"),
				"data-ingestion":    checkServiceHealth("data-ingestion"),
				"rules-engine":      checkServiceHealth("rules-engine"),
				"analytics":         checkServiceHealth("analytics"),
				"notification":      checkServiceHealth("notification"),
				"user-management":   checkServiceHealth("user-management"),
			},
		})
	})

	// Metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public routes (no auth required)
	router.POST("/api/auth/register", proxyTo("user-management"))
	router.POST("/api/auth/login", proxyTo("user-management"))
	router.POST("/api/auth/refresh", proxyTo("user-management"))

	// Protected routes (auth required)
	protected := router.Group("/api")
	protected.Use(authMiddleware())
	{
		// Device Registry
		protected.Any("/devices/*path", proxyTo("device-registry"))

		// Rules Engine
		protected.Any("/rules/*path", proxyTo("rules-engine"))

		// Analytics
		protected.Any("/analytics/*path", proxyTo("analytics"))

		// Notifications
		protected.Any("/notifications/*path", proxyTo("notification"))

		// User Management
		protected.Any("/users/*path", proxyTo("user-management"))
	}

	// Start server
	port := getEnv("API_GATEWAY_PORT", "8080")
	log.Printf("API Gateway starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initRedis() {
	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: "",
		DB:       0,
	})

	// Test connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v (rate limiting disabled)", err)
		redisClient = nil
	} else {
		log.Println("Connected to Redis")
	}
}

// Proxy requests to backend service
func proxyTo(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, exists := services[serviceName]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
			return
		}

		// Track metrics
		timer := httpDuration.Labels(service.Name, c.Request.Method).Timer()
		defer timer.ObserveDuration()

		// Parse backend URL
		target, err := url.Parse(service.URL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid service URL"})
			return
		}

		// Create reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(target)

		// Modify request
		originalPath := c.Request.URL.Path
		c.Request.URL.Host = target.Host
		c.Request.URL.Scheme = target.Scheme
		c.Request.Host = target.Host

		// For routes like /api/devices/*, forward as /api/devices/*
		// The backend services expect the full path

		// Add user context headers
		if userID, exists := c.Get("user_id"); exists {
			c.Request.Header.Set("X-User-ID", userID.(string))
		}
		if email, exists := c.Get("email"); exists {
			c.Request.Header.Set("X-User-Email", email.(string))
		}

		// Custom director to handle path properly
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = originalPath
		}

		// Error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", serviceName, err)
			httpRequests.WithLabelValues(service.Name, c.Request.Method, "502").Inc()
			c.JSON(http.StatusBadGateway, gin.H{"error": "Service unavailable"})
		}

		// Modify response
		proxy.ModifyResponse = func(resp *http.Response) error {
			httpRequests.WithLabelValues(service.Name, c.Request.Method, fmt.Sprintf("%d", resp.StatusCode)).Inc()
			return nil
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// Check if service is healthy
func checkServiceHealth(serviceName string) bool {
	service, exists := services[serviceName]
	if !exists {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(service.URL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
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

// Rate limit middleware
func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if redisClient == nil {
			// Skip rate limiting if Redis unavailable
			c.Next()
			return
		}

		// Get client identifier (IP or user ID)
		clientID := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			clientID = userID.(string)
		}

		// Rate limit: 100 requests per minute
		key := fmt.Sprintf("rate_limit:%s", clientID)
		count, err := redisClient.Incr(ctx, key).Result()
		if err != nil {
			log.Printf("Redis error: %v", err)
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			redisClient.Expire(ctx, key, time.Minute)
		}

		// Check limit
		if count > 100 {
			rateLimitExceeded.Inc()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "60 seconds",
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Writer.Header().Set("X-RateLimit-Limit", "100")
		c.Writer.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", 100-count))

		c.Next()
	}
}

// CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// Logging middleware
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log request
		duration := time.Since(start)
		log.Printf("[%s] %s %s %d %v",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
			c.Writer.Status(),
			duration,
		)
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
