# Architecture Documentation

## System Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                          IoT Devices Layer                          │
│  (Temperature Sensors, Smart Lights, Door Locks, Cameras, etc.)    │
└────────────────────────────┬────────────────────────────────────────┘
                             │ MQTT Protocol
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        MQTT Broker (Mosquitto)                      │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Data Ingestion Service (Go)                    │
│  - Subscribe to MQTT topics                                         │
│  - Validate and enrich data                                         │
│  - Publish to message queue                                         │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        RabbitMQ (Message Broker)                    │
│  Exchanges: device.telemetry, device.events, notifications          │
└───────┬──────────────┬─────────────────┬──────────────┬─────────────┘
        │              │                 │              │
        ▼              ▼                 ▼              ▼
   ┌────────┐    ┌──────────┐    ┌──────────┐   ┌─────────────┐
   │ Rules  │    │Analytics │    │  Notif.  │   │   Other     │
   │ Engine │    │ Service  │    │ Service  │   │  Consumers  │
   └────────┘    └──────────┘    └──────────┘   └─────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                          API Gateway (Go)                           │
│  - Authentication & Authorization                                   │
│  - Request routing                                                  │
│  - Rate limiting                                                    │
│  - Load balancing                                                   │
└────────────────────────────┬────────────────────────────────────────┘
                             │ HTTP/REST
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Device     │    │     User     │    │    Rules     │
│  Registry    │    │  Management  │    │   Engine     │
│  Service     │    │   Service    │    │   Service    │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ PostgreSQL   │    │ PostgreSQL   │    │ PostgreSQL   │
│  (Devices)   │    │   (Users)    │    │   (Rules)    │
└──────────────┘    └──────────────┘    └──────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     TimescaleDB (Time-Series Data)                  │
│  - Device telemetry history                                         │
│  - Analytics aggregations                                           │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         Redis (Cache & Pub/Sub)                     │
│  - Session caching                                                  │
│  - Real-time updates                                                │
│  - Rate limiting counters                                           │
└─────────────────────────────────────────────────────────────────────┘
```

## Service Responsibilities

### 1. Device Registry Service
**Port**: 8081
**Language**: Go
**Database**: PostgreSQL

**Responsibilities:**
- CRUD operations for devices
- Device metadata management (name, type, location, capabilities)
- Device authentication tokens
- Multi-tenant device organization
- Device status tracking

**API Endpoints:**
- `POST /devices` - Register new device
- `GET /devices` - List devices (with filtering)
- `GET /devices/{id}` - Get device details
- `PUT /devices/{id}` - Update device
- `DELETE /devices/{id}` - Remove device
- `GET /devices/{id}/telemetry` - Get recent telemetry

### 2. Data Ingestion Service
**Port**: 8082
**Language**: Go
**Protocol**: MQTT

**Responsibilities:**
- Subscribe to MQTT topics (devices/+/telemetry)
- Validate incoming data
- Enrich data with device metadata
- Store in TimescaleDB
- Publish to RabbitMQ for downstream processing
- Handle high-throughput data streams

**MQTT Topics:**
- `devices/{device_id}/telemetry` - Device data
- `devices/{device_id}/status` - Device status (online/offline)
- `devices/{device_id}/commands` - Commands to devices

### 3. Rules Engine Service
**Port**: 8083
**Language**: Python
**Database**: PostgreSQL

**Responsibilities:**
- Define automation rules (JSON-based DSL)
- Real-time rule evaluation on incoming events
- Execute actions (send commands, trigger notifications)
- Scene management (groups of actions)
- Scheduling support (time-based rules)

**Rule Structure:**
```json
{
  "id": "rule-123",
  "name": "Turn on lights when dark",
  "conditions": {
    "all": [
      {"device": "light-sensor-1", "metric": "illuminance", "operator": "<", "value": 100},
      {"time": "after", "value": "18:00"}
    ]
  },
  "actions": [
    {"device": "smart-light-1", "command": "turn_on"},
    {"notify": "user@example.com", "message": "Lights turned on"}
  ]
}
```

### 4. Analytics Service
**Port**: 8084
**Language**: Python
**Database**: TimescaleDB

**Responsibilities:**
- Process telemetry data
- Generate statistics and aggregations
- Anomaly detection (ML-based)
- Historical data analysis
- Energy consumption tracking
- Predictive maintenance

**API Endpoints:**
- `GET /analytics/devices/{id}/stats` - Device statistics
- `GET /analytics/energy` - Energy consumption
- `GET /analytics/anomalies` - Detected anomalies
- `GET /analytics/trends` - Trend analysis

### 5. Notification Service
**Port**: 8085
**Language**: Node.js
**Database**: PostgreSQL

**Responsibilities:**
- Send notifications (email, SMS, push)
- Template management
- User preferences
- Notification history
- Delivery tracking

**Notification Channels:**
- Email (SMTP)
- SMS (Twilio/SNS)
- Push notifications (FCM)
- WebSocket (real-time browser notifications)

### 6. User Management Service
**Port**: 8086
**Language**: Go
**Database**: PostgreSQL

**Responsibilities:**
- User registration and authentication
- JWT token generation and validation
- Multi-tenant support (homes/locations)
- Role-based access control (RBAC)
- User preferences

**API Endpoints:**
- `POST /auth/register` - User registration
- `POST /auth/login` - User login (returns JWT)
- `POST /auth/refresh` - Refresh token
- `GET /users/profile` - Get user profile
- `PUT /users/profile` - Update profile

### 7. API Gateway
**Port**: 8080
**Language**: Go

**Responsibilities:**
- Single entry point for all client requests
- Authentication middleware (JWT validation)
- Request routing to appropriate services
- Rate limiting (Redis-based)
- Request/response logging
- CORS handling

**Routing:**
- `/api/devices/*` → Device Registry Service
- `/api/rules/*` → Rules Engine Service
- `/api/analytics/*` → Analytics Service
- `/api/auth/*` → User Management Service
- `/api/notifications/*` → Notification Service

## Data Flow

### Device Telemetry Flow

1. **IoT Device** sends telemetry via MQTT
   ```
   Topic: devices/temp-sensor-1/telemetry
   Payload: {"temperature": 22.5, "humidity": 45, "timestamp": "2025-11-05T12:00:00Z"}
   ```

2. **Data Ingestion Service** receives and processes:
   - Validates data format
   - Enriches with device metadata
   - Stores in TimescaleDB
   - Publishes to RabbitMQ exchange `device.telemetry`

3. **Rules Engine** evaluates rules:
   - Checks if data matches any rule conditions
   - Executes actions if conditions met

4. **Analytics Service** processes data:
   - Updates aggregations
   - Checks for anomalies
   - Updates trends

### Rule Execution Flow

1. User creates rule via API Gateway
2. Gateway routes to Rules Engine Service
3. Rules Engine stores rule in PostgreSQL
4. On telemetry event, Rules Engine evaluates:
   - Fetches relevant rules
   - Evaluates conditions
   - If match, executes actions
5. Actions may include:
   - Send MQTT command to device
   - Trigger notification
   - Call webhook

## Communication Patterns

### Synchronous Communication (REST)
- Client ↔ API Gateway
- API Gateway ↔ Services (for reads)
- Used for: CRUD operations, queries

### Asynchronous Communication (RabbitMQ)
- Data Ingestion → Rules Engine
- Data Ingestion → Analytics
- Rules Engine → Notification Service
- Used for: Event processing, decoupling services

### Pub/Sub (MQTT)
- IoT Devices → MQTT Broker → Data Ingestion
- Data Ingestion → MQTT Broker → IoT Devices (commands)

## Data Storage Strategy

### PostgreSQL Databases
- **device_registry_db**: Device metadata, capabilities
- **user_management_db**: Users, authentication, permissions
- **rules_engine_db**: Rules, scenes, schedules
- **notification_db**: Notification templates, history

### TimescaleDB (PostgreSQL Extension)
- **telemetry_db**: Time-series device data
- Hypertables with automatic partitioning
- Retention policies for old data
- Continuous aggregates for analytics

### Redis
- Session cache (JWT blacklist)
- Rate limiting counters
- Real-time presence data
- Pub/sub for WebSocket updates

## Security Considerations

1. **Authentication**: JWT-based authentication for API
2. **Authorization**: RBAC for multi-tenant access control
3. **Device Authentication**: Device tokens for MQTT
4. **Data Encryption**: TLS for all communications
5. **Secrets Management**: Environment variables, not hardcoded
6. **API Rate Limiting**: Prevent abuse
7. **Input Validation**: All services validate inputs

## Scalability Considerations

1. **Horizontal Scaling**: All services are stateless (can run multiple instances)
2. **Database Partitioning**: TimescaleDB handles time-series partitioning
3. **Message Queue**: RabbitMQ for load distribution
4. **Caching**: Redis reduces database load
5. **Load Balancing**: API Gateway can run behind load balancer

## Monitoring & Observability

1. **Logging**: Structured JSON logs from all services
2. **Metrics**: Prometheus metrics exposed by each service
3. **Tracing**: Distributed tracing with correlation IDs
4. **Health Checks**: `/health` endpoint on each service
5. **Dashboards**: Grafana for visualization

## Deployment

### Docker Compose (Development)
- All services in single compose file
- Shared network
- Volume persistence

### Kubernetes (Production)
- Each service as Deployment
- Services for networking
- ConfigMaps for configuration
- Secrets for sensitive data
- Ingress for external access
