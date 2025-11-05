package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Device struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name" binding:"required"`
	Type         string                 `json:"type" binding:"required"`
	Location     string                 `json:"location"`
	UserID       string                 `json:"user_id"`
	AuthToken    string                 `json:"auth_token,omitempty"`
	Capabilities map[string]interface{} `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	Status       string                 `json:"status"`
	LastSeen     *time.Time             `json:"last_seen"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type DeviceService struct {
	db *sql.DB
}

var (
	// Prometheus metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "device_registry_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "device_registry_http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	devicesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "device_registry_devices_total",
			Help: "Total number of registered devices",
		},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(devicesTotal)
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

	service := &DeviceService{db: db}

	// Initialize Gin router
	router := gin.Default()

	// Middleware
	router.Use(metricsMiddleware())
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "device-registry"})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	api := router.Group("/api/devices")
	{
		api.POST("", service.CreateDevice)
		api.GET("", service.ListDevices)
		api.GET("/:id", service.GetDevice)
		api.PUT("/:id", service.UpdateDevice)
		api.DELETE("/:id", service.DeleteDevice)
		api.POST("/:id/status", service.UpdateDeviceStatus)
	}

	// Start server
	port := getEnv("DEVICE_REGISTRY_PORT", "8081")
	log.Printf("Device Registry Service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*sql.DB, error) {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "smarthome")
	password := getEnv("POSTGRES_PASSWORD", "smarthome_dev_password")
	dbname := "device_registry_db"

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Successfully connected to database")
	return db, nil
}

func (s *DeviceService) CreateDevice(c *gin.Context) {
	var device Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context (set by auth middleware in API Gateway)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = c.GetHeader("X-User-ID") // Fallback for testing
		if userID == "" {
			userID = uuid.New().String() // Default for demo
		}
	}

	device.ID = uuid.New().String()
	device.UserID = userID
	device.AuthToken = generateAuthToken()
	device.Status = "offline"
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	capabilitiesJSON, _ := json.Marshal(device.Capabilities)
	metadataJSON, _ := json.Marshal(device.Metadata)

	query := `
		INSERT INTO devices (id, name, type, location, user_id, auth_token, capabilities, metadata, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := s.db.Exec(query, device.ID, device.Name, device.Type, device.Location,
		device.UserID, device.AuthToken, capabilitiesJSON, metadataJSON,
		device.Status, device.CreatedAt, device.UpdatedAt)

	if err != nil {
		log.Printf("Error creating device: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create device"})
		return
	}

	// Update metrics
	devicesTotal.Inc()

	c.JSON(http.StatusCreated, device)
}

func (s *DeviceService) ListDevices(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	deviceType := c.Query("type")
	status := c.Query("status")

	query := "SELECT id, name, type, location, user_id, capabilities, metadata, status, last_seen, created_at, updated_at FROM devices WHERE 1=1"
	args := []interface{}{}
	argCount := 1

	if userID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}

	if deviceType != "" {
		query += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, deviceType)
		argCount++
	}

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Error listing devices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list devices"})
		return
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var device Device
		var capabilitiesJSON, metadataJSON []byte
		var lastSeen sql.NullTime

		err := rows.Scan(&device.ID, &device.Name, &device.Type, &device.Location,
			&device.UserID, &capabilitiesJSON, &metadataJSON, &device.Status,
			&lastSeen, &device.CreatedAt, &device.UpdatedAt)

		if err != nil {
			log.Printf("Error scanning device: %v", err)
			continue
		}

		if lastSeen.Valid {
			device.LastSeen = &lastSeen.Time
		}

		json.Unmarshal(capabilitiesJSON, &device.Capabilities)
		json.Unmarshal(metadataJSON, &device.Metadata)

		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices, "count": len(devices)})
}

func (s *DeviceService) GetDevice(c *gin.Context) {
	id := c.Param("id")

	var device Device
	var capabilitiesJSON, metadataJSON []byte
	var lastSeen sql.NullTime

	query := `
		SELECT id, name, type, location, user_id, auth_token, capabilities, metadata, status, last_seen, created_at, updated_at
		FROM devices WHERE id = $1
	`

	err := s.db.QueryRow(query, id).Scan(&device.ID, &device.Name, &device.Type,
		&device.Location, &device.UserID, &device.AuthToken, &capabilitiesJSON,
		&metadataJSON, &device.Status, &lastSeen, &device.CreatedAt, &device.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	if err != nil {
		log.Printf("Error getting device: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device"})
		return
	}

	if lastSeen.Valid {
		device.LastSeen = &lastSeen.Time
	}

	json.Unmarshal(capabilitiesJSON, &device.Capabilities)
	json.Unmarshal(metadataJSON, &device.Metadata)

	c.JSON(http.StatusOK, device)
}

func (s *DeviceService) UpdateDevice(c *gin.Context) {
	id := c.Param("id")

	var updates Device
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	capabilitiesJSON, _ := json.Marshal(updates.Capabilities)
	metadataJSON, _ := json.Marshal(updates.Metadata)

	query := `
		UPDATE devices
		SET name = $1, location = $2, capabilities = $3, metadata = $4, updated_at = $5
		WHERE id = $6
	`

	result, err := s.db.Exec(query, updates.Name, updates.Location, capabilitiesJSON,
		metadataJSON, time.Now(), id)

	if err != nil {
		log.Printf("Error updating device: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update device"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device updated successfully"})
}

func (s *DeviceService) DeleteDevice(c *gin.Context) {
	id := c.Param("id")

	result, err := s.db.Exec("DELETE FROM devices WHERE id = $1", id)
	if err != nil {
		log.Printf("Error deleting device: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete device"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	// Update metrics
	devicesTotal.Dec()

	c.JSON(http.StatusOK, gin.H{"message": "Device deleted successfully"})
}

func (s *DeviceService) UpdateDeviceStatus(c *gin.Context) {
	id := c.Param("id")

	var statusUpdate struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&statusUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := "UPDATE devices SET status = $1, last_seen = $2, updated_at = $3 WHERE id = $4"
	result, err := s.db.Exec(query, statusUpdate.Status, time.Now(), time.Now(), id)

	if err != nil {
		log.Printf("Error updating device status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device status updated"})
}

// Middleware
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
	}
}

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

// Helper functions
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func generateAuthToken() string {
	return fmt.Sprintf("device_%s", uuid.New().String())
}
