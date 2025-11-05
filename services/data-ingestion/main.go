package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

type TelemetryData struct {
	DeviceID  string                 `json:"device_id"`
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type DataIngestionService struct {
	mqttClient   mqtt.Client
	db           *sql.DB
	rabbitConn   *amqp.Connection
	rabbitCh     *amqp.Channel
	rabbitQueue  string
	rabbitExchange string
}

var (
	// Prometheus metrics
	telemetryReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_ingestion_telemetry_received_total",
			Help: "Total number of telemetry messages received",
		},
		[]string{"device_id", "status"},
	)
	telemetryProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "data_ingestion_processing_duration_seconds",
			Help:    "Time taken to process telemetry",
			Buckets: prometheus.DefBuckets,
		},
	)
	mqttConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "data_ingestion_mqtt_connected",
			Help: "MQTT connection status (1=connected, 0=disconnected)",
		},
	)
)

func init() {
	prometheus.MustRegister(telemetryReceived)
	prometheus.MustRegister(telemetryProcessingDuration)
	prometheus.MustRegister(mqttConnected)
}

func main() {
	// Load environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize TimescaleDB connection
	db, err := initTimescaleDB()
	if err != nil {
		log.Fatalf("Failed to connect to TimescaleDB: %v", err)
	}
	defer db.Close()

	// Initialize RabbitMQ
	rabbitConn, rabbitCh, err := initRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()
	defer rabbitCh.Close()

	service := &DataIngestionService{
		db:             db,
		rabbitConn:     rabbitConn,
		rabbitCh:       rabbitCh,
		rabbitExchange: "device.telemetry",
		rabbitQueue:    "telemetry.raw",
	}

	// Setup RabbitMQ exchange and queue
	if err := service.setupRabbitMQ(); err != nil {
		log.Fatalf("Failed to setup RabbitMQ: %v", err)
	}

	// Initialize MQTT client
	mqttClient, err := initMQTTClient(service)
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Disconnect(250)

	service.mqttClient = mqttClient

	// Subscribe to device telemetry topics
	if err := service.subscribeMQTT(); err != nil {
		log.Fatalf("Failed to subscribe to MQTT topics: %v", err)
	}

	// Start HTTP server for health checks and metrics
	go startHTTPServer()

	log.Println("Data Ingestion Service started successfully")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Data Ingestion Service...")
}

func initTimescaleDB() (*sql.DB, error) {
	host := getEnv("TIMESCALE_HOST", "localhost")
	port := getEnv("TIMESCALE_PORT", "5433")
	user := getEnv("TIMESCALE_USER", "telemetry")
	password := getEnv("TIMESCALE_PASSWORD", "telemetry_dev_password")
	dbname := getEnv("TIMESCALE_DB", "telemetry_db")

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

	log.Println("Successfully connected to TimescaleDB")
	return db, nil
}

func initRabbitMQ() (*amqp.Connection, *amqp.Channel, error) {
	host := getEnv("RABBITMQ_HOST", "localhost")
	port := getEnv("RABBITMQ_PORT", "5672")
	user := getEnv("RABBITMQ_USER", "smarthome")
	password := getEnv("RABBITMQ_PASSWORD", "smarthome_dev_password")

	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, password, host, port)

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}

	log.Println("Successfully connected to RabbitMQ")
	return conn, ch, nil
}

func initMQTTClient(service *DataIngestionService) (mqtt.Client, error) {
	broker := getEnv("MQTT_BROKER", "localhost")
	port := getEnv("MQTT_PORT", "1883")
	clientID := fmt.Sprintf("data-ingestion-%d", time.Now().Unix())

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	opts.OnConnect = func(client mqtt.Client) {
		log.Println("Connected to MQTT broker")
		mqttConnected.Set(1)
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
		mqttConnected.Set(0)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

func (s *DataIngestionService) subscribeMQTT() error {
	// Subscribe to all device telemetry
	topic := "devices/+/telemetry"

	token := s.mqttClient.Subscribe(topic, 1, s.handleTelemetryMessage)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	log.Printf("Subscribed to MQTT topic: %s", topic)

	// Subscribe to device status updates
	statusTopic := "devices/+/status"
	token = s.mqttClient.Subscribe(statusTopic, 1, s.handleStatusMessage)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	log.Printf("Subscribed to MQTT topic: %s", statusTopic)

	return nil
}

func (s *DataIngestionService) handleTelemetryMessage(client mqtt.Client, msg mqtt.Message) {
	start := time.Now()

	var telemetry TelemetryData
	if err := json.Unmarshal(msg.Payload(), &telemetry); err != nil {
		log.Printf("Error parsing telemetry message: %v", err)
		telemetryReceived.WithLabelValues("unknown", "error").Inc()
		return
	}

	// Set timestamp if not provided
	if telemetry.Timestamp.IsZero() {
		telemetry.Timestamp = time.Now()
	}

	// Store in TimescaleDB
	if err := s.storeTelemetry(telemetry); err != nil {
		log.Printf("Error storing telemetry: %v", err)
		telemetryReceived.WithLabelValues(telemetry.DeviceID, "storage_error").Inc()
		return
	}

	// Publish to RabbitMQ for downstream processing
	if err := s.publishToRabbitMQ(telemetry); err != nil {
		log.Printf("Error publishing to RabbitMQ: %v", err)
	}

	// Update metrics
	telemetryReceived.WithLabelValues(telemetry.DeviceID, "success").Inc()
	telemetryProcessingDuration.Observe(time.Since(start).Seconds())

	log.Printf("Processed telemetry from device %s", telemetry.DeviceID)
}

func (s *DataIngestionService) handleStatusMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received status update: %s = %s", msg.Topic(), string(msg.Payload()))
	// Status updates can be stored or forwarded as needed
}

func (s *DataIngestionService) storeTelemetry(telemetry TelemetryData) error {
	// Insert each metric as a separate row
	query := `
		INSERT INTO device_telemetry (time, device_id, metric_name, metric_value, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`

	metadataJSON, _ := json.Marshal(telemetry.Metadata)

	for metricName, metricValue := range telemetry.Metrics {
		var value float64

		// Convert metric value to float64
		switch v := metricValue.(type) {
		case float64:
			value = v
		case float32:
			value = float64(v)
		case int:
			value = float64(v)
		case int64:
			value = float64(v)
		case bool:
			if v {
				value = 1.0
			} else {
				value = 0.0
			}
		default:
			log.Printf("Unsupported metric value type for %s: %T", metricName, metricValue)
			continue
		}

		_, err := s.db.Exec(query, telemetry.Timestamp, telemetry.DeviceID, metricName, value, metadataJSON)
		if err != nil {
			return fmt.Errorf("failed to insert metric %s: %w", metricName, err)
		}
	}

	return nil
}

func (s *DataIngestionService) setupRabbitMQ() error {
	// Declare exchange
	err := s.rabbitCh.ExchangeDeclare(
		s.rabbitExchange, // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return err
	}

	// Declare queue
	_, err = s.rabbitCh.QueueDeclare(
		s.rabbitQueue, // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		return err
	}

	// Bind queue to exchange
	err = s.rabbitCh.QueueBind(
		s.rabbitQueue,    // queue name
		"telemetry.#",    // routing key
		s.rabbitExchange, // exchange
		false,
		nil,
	)

	log.Println("RabbitMQ exchange and queue setup complete")
	return err
}

func (s *DataIngestionService) publishToRabbitMQ(telemetry TelemetryData) error {
	body, err := json.Marshal(telemetry)
	if err != nil {
		return err
	}

	return s.rabbitCh.Publish(
		s.rabbitExchange,                // exchange
		fmt.Sprintf("telemetry.%s", telemetry.DeviceID), // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

func startHTTPServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "data-ingestion",
		})
	})

	http.Handle("/metrics", promhttp.Handler())

	port := getEnv("DATA_INGESTION_PORT", "8082")
	log.Printf("HTTP server listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
