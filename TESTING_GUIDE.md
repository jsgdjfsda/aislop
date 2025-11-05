# Complete Testing Guide

Comprehensive guide to test all features of the Smart Home IoT Management System.

## Prerequisites

Ensure all services are running:

1. Infrastructure (Docker Compose):
   ```bash
   docker-compose up -d
   ```

2. Microservices:
   ```bash
   # Terminal 1: Device Registry
   cd services/device-registry && go run main.go

   # Terminal 2: Data Ingestion
   cd services/data-ingestion && go run main.go

   # Terminal 3: Rules Engine
   cd services/rules-engine && python main.py

   # Terminal 4: Analytics
   cd services/analytics && python main.py

   # Terminal 5: Notification
   cd services/notification && npm start

   # Terminal 6: User Management
   cd services/user-management && go run main.go

   # Terminal 7: API Gateway
   cd services/api-gateway && go run main.go

   # Terminal 8: Simulator
   cd simulator && python simulator.py
   ```

## Test 1: User Registration & Authentication

### 1.1 Register New User

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@smarthome.com",
    "password": "SecurePass123!",
    "full_name": "Test User",
    "phone": "+1234567890"
  }'
```

**Expected:** HTTP 201, returns `user_id` and `token`

### 1.2 Login

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@smarthome.com",
    "password": "SecurePass123!"
  }'
```

**Expected:** HTTP 200, returns JWT `token`

### 1.3 Save Token

```bash
# Save token for subsequent requests
export TOKEN="<token-from-login-response>"
```

### 1.4 Get Profile

```bash
curl http://localhost:8080/api/users/profile \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, returns user profile

### 1.5 Test Invalid Token

```bash
curl http://localhost:8080/api/devices \
  -H "Authorization: Bearer invalid-token"
```

**Expected:** HTTP 401 Unauthorized

---

## Test 2: Device Management

### 2.1 Create Devices

```bash
# Temperature Sensor
curl -X POST http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Living Room Temperature",
    "type": "temperature_sensor",
    "location": "Living Room",
    "capabilities": {"metrics": ["temperature", "humidity"]}
  }'

# Smart Light
curl -X POST http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Bedroom Light",
    "type": "smart_light",
    "location": "Bedroom",
    "capabilities": {"commands": ["turn_on", "turn_off", "set_brightness"]}
  }'
```

**Expected:** HTTP 201, returns device details with `id` and `auth_token`

### 2.2 List Devices

```bash
curl http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, returns array of devices

### 2.3 Get Device by ID

```bash
curl http://localhost:8080/api/devices/{device-id} \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, returns device details

### 2.4 Update Device

```bash
curl -X PUT http://localhost:8080/api/devices/{device-id} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Living Room Temp Sensor (Updated)",
    "location": "Living Room - Window"
  }'
```

**Expected:** HTTP 200, device updated

---

## Test 3: Real-Time Telemetry

### 3.1 Subscribe to MQTT Telemetry

```bash
# In new terminal
mosquitto_sub -h localhost -t devices/+/telemetry -v
```

**Expected:** See telemetry messages from simulator every 5 seconds

### 3.2 Send Test Telemetry

```bash
mosquitto_pub -h localhost -t devices/test-device/telemetry \
  -m '{"device_id":"test-device","timestamp":"2025-11-05T12:00:00Z","metrics":{"temperature":25.5,"humidity":50}}'
```

**Expected:** Message appears in Data Ingestion Service logs

### 3.3 Verify TimescaleDB Storage

```bash
docker exec -it smarthome-timescaledb psql -U telemetry -d telemetry_db
```

```sql
-- Check recent telemetry
SELECT device_id, metric_name, metric_value, time
FROM device_telemetry
ORDER BY time DESC
LIMIT 20;

-- Count data points
SELECT COUNT(*) FROM device_telemetry;

-- Devices reporting
SELECT DISTINCT device_id FROM device_telemetry;
```

**Expected:** Data from simulator visible in database

### 3.4 Check RabbitMQ

1. Open http://localhost:15672
2. Login: `smarthome` / `smarthome_dev_password`
3. Go to Queues → `telemetry.raw`
4. Click "Get Messages"

**Expected:** See telemetry messages

---

## Test 4: Automation Rules

### 4.1 Create Rule (Turn on light when temperature > 25°C)

```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Auto light on high temp",
    "conditions": {
      "all": [
        {
          "device": "temp-sensor-living-room",
          "metric": "temperature",
          "operator": ">",
          "value": 25
        }
      ]
    },
    "actions": [
      {
        "device": "light-living-room",
        "command": "turn_on",
        "params": {"brightness": 80}
      }
    ]
  }'
```

**Expected:** HTTP 201, rule created

### 4.2 List Rules

```bash
curl http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, returns rules array

### 4.3 Create Time-Based Rule

```bash
curl -X POST http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Evening automation",
    "conditions": {
      "all": [
        {"time": "after", "value": "18:00"},
        {"time": "before", "value": "23:00"}
      ]
    },
    "actions": [
      {"device": "light-hallway", "command": "turn_on"}
    ]
  }'
```

**Expected:** Rule matches during specified time window

### 4.4 Watch Rule Execution

Monitor Rules Engine logs for:
```
Rule 'Auto light on high temp' matched for device temp-sensor-living-room
Sent command 'turn_on' to device light-living-room
```

---

## Test 5: Analytics

### 5.1 Device Statistics

```bash
curl "http://localhost:8080/api/analytics/devices/temp-sensor-living-room/stats?metric=temperature&hours=24" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, returns avg, min, max, std_dev

### 5.2 Time-Series Data

```bash
curl "http://localhost:8080/api/analytics/devices/temp-sensor-living-room/timeseries?metric=temperature&hours=6&interval=30 minutes" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** Array of time-bucketed data points

### 5.3 Anomaly Detection

```bash
curl "http://localhost:8080/api/analytics/devices/temp-sensor-living-room/anomalies?metric=temperature&hours=168" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** List of detected anomalies (if any)

### 5.4 Energy Consumption

```bash
curl "http://localhost:8080/api/analytics/energy?hours=24" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** Energy consumption summary with cost estimate

### 5.5 Device Uptime

```bash
curl "http://localhost:8080/api/analytics/devices/temp-sensor-living-room/uptime?days=7" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** Uptime percentage

### 5.6 Trend Analysis

```bash
curl "http://localhost:8080/api/analytics/devices/temp-sensor-living-room/trend?metric=temperature&hours=168" \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** Trend direction (increasing/decreasing/stable)

---

## Test 6: Notifications

### 6.1 Send Notification (requires SMTP config)

```bash
curl -X POST http://localhost:8080/api/notifications/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test-user",
    "type": "alert",
    "channels": ["email"],
    "subject": "Test Alert",
    "message": "This is a test notification from Smart Home IoT",
    "metadata": {
      "device_id": "test-device",
      "timestamp": "2025-11-05T12:00:00Z"
    }
  }'
```

**Expected:** HTTP 200, notification sent (check email)

### 6.2 Get Notification History

```bash
curl http://localhost:8080/api/notifications/history \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** HTTP 200, list of sent notifications

### 6.3 Update Preferences

```bash
curl -X PUT http://localhost:8080/api/notifications/preferences \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email_enabled": true,
    "sms_enabled": false,
    "push_enabled": true,
    "quiet_hours_start": "22:00",
    "quiet_hours_end": "08:00"
  }'
```

**Expected:** Preferences saved

---

## Test 7: Rate Limiting

### 7.1 Test Rate Limit

```bash
# Send 101 requests quickly (limit is 100/min)
for i in {1..101}; do
  curl -s http://localhost:8080/api/devices \
    -H "Authorization: Bearer $TOKEN" \
    -o /dev/null -w "Request $i: %{http_code}\n"
  sleep 0.1
done
```

**Expected:** First 100 return 200, 101st returns 429 (Too Many Requests)

### 7.2 Check Rate Limit Headers

```bash
curl -I http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN"
```

**Expected Headers:**
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
```

---

## Test 8: System Health

### 8.1 API Gateway Health

```bash
curl http://localhost:8080/health | jq
```

**Expected:** All services show `true`

### 8.2 Individual Service Health

```bash
curl http://localhost:8081/health  # Device Registry
curl http://localhost:8082/health  # Data Ingestion
curl http://localhost:8083/health  # Rules Engine
curl http://localhost:8084/health  # Analytics
curl http://localhost:8085/health  # Notification
curl http://localhost:8086/health  # User Management
```

**Expected:** All return `{"status":"healthy"}`

---

## Test 9: Monitoring & Metrics

### 9.1 Prometheus Metrics

```bash
# Gateway metrics
curl http://localhost:8080/metrics

# Device Registry metrics
curl http://localhost:8081/metrics

# Check specific metric
curl http://localhost:8081/metrics | grep device_registry_devices_total
```

**Expected:** Prometheus format metrics

### 9.2 Prometheus UI

1. Open http://localhost:9090
2. Query: `api_gateway_http_requests_total`
3. Query: `device_registry_devices_total`

**Expected:** See metrics graphs

### 9.3 Grafana

1. Open http://localhost:3000
2. Login: `admin` / `admin`
3. Add Prometheus datasource: http://prometheus:9090
4. Create dashboard with queries

---

## Test 10: End-to-End Workflow

### Complete User Journey

```bash
# 1. Register user
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"e2e@test.com","password":"Test1234!","full_name":"E2E Test"}'

# 2. Login and save token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"e2e@test.com","password":"Test1234!"}' \
  | jq -r '.token')

# 3. Create home
curl -X POST http://localhost:8080/api/users/homes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Smart Home","address":"123 Main St"}'

# 4. Register device
DEVICE_ID=$(curl -s -X POST http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Sensor","type":"temperature_sensor","location":"Living Room"}' \
  | jq -r '.id')

# 5. Wait for simulator data (30 seconds)
sleep 30

# 6. Get device statistics
curl "http://localhost:8080/api/analytics/devices/$DEVICE_ID/stats?metric=temperature" \
  -H "Authorization: Bearer $TOKEN" | jq

# 7. Create automation rule
curl -X POST http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Test Rule\",\"conditions\":{\"all\":[{\"device\":\"$DEVICE_ID\",\"metric\":\"temperature\",\"operator\":\">\",\"value\":20}]},\"actions\":[{\"device\":\"light-1\",\"command\":\"turn_on\"}]}"

# 8. View notification history
curl http://localhost:8080/api/notifications/history \
  -H "Authorization: Bearer $TOKEN" | jq
```

**Expected:** All steps succeed, data flows through system

---

## Test 11: Error Handling

### 11.1 Invalid Credentials

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"wrong@test.com","password":"wrongpass"}'
```

**Expected:** HTTP 401 Unauthorized

### 11.2 Duplicate Registration

```bash
# Register twice with same email
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"duplicate@test.com","password":"Test1234!"}'

curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"duplicate@test.com","password":"Test1234!"}'
```

**Expected:** Second request returns HTTP 409 Conflict

### 11.3 Missing Required Fields

```bash
curl -X POST http://localhost:8080/api/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Incomplete Device"}'
```

**Expected:** HTTP 400 Bad Request

---

## Troubleshooting

### Services Not Starting

```bash
# Check if ports are available
netstat -tuln | grep -E '8080|8081|8082|8083|8084|8085|8086'

# Check infrastructure
docker-compose ps

# View logs
docker-compose logs postgres
docker-compose logs timescaledb
docker-compose logs mosquitto
docker-compose logs rabbitmq
```

### No Telemetry Data

```bash
# Test MQTT
mosquitto_pub -h localhost -t test -m "hello"
mosquitto_sub -h localhost -t test

# Check Data Ingestion logs
# Should see: "Processed telemetry from device..."

# Check TimescaleDB
docker exec -it smarthome-timescaledb psql -U telemetry -d telemetry_db -c "SELECT COUNT(*) FROM device_telemetry;"
```

### Rules Not Triggering

```bash
# Check Rules Engine logs
# Should see: "Loaded X active rules"

# Verify RabbitMQ connection
docker-compose logs rabbitmq

# Test rule manually
curl -X POST http://localhost:8080/api/rules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","conditions":{"all":[{"device":"temp-sensor-living-room","metric":"temperature","operator":">","value":0}]},"actions":[]}'
```

---

## Success Criteria

✅ All 7 microservices running without errors
✅ User registration and authentication working
✅ Devices can be created and managed
✅ Simulator sending telemetry every 5 seconds
✅ Telemetry stored in TimescaleDB
✅ Rules evaluating and actions executing
✅ Analytics returning statistics and insights
✅ Notifications being sent (if configured)
✅ Rate limiting enforcing 100 req/min
✅ All health checks passing
✅ Prometheus metrics available

---

## Performance Benchmarks

### Expected Performance

- **Device Registry:** 1000+ req/sec
- **Data Ingestion:** 10,000+ messages/sec
- **Rules Evaluation:** < 10ms per rule
- **Analytics Queries:** < 500ms for 24h data
- **API Gateway:** < 50ms latency overhead

### Load Testing

```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Test API Gateway
hey -n 1000 -c 10 \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/devices

# Test device creation
hey -n 100 -c 5 \
  -m POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Load Test Device","type":"sensor","location":"Test"}' \
  http://localhost:8080/api/devices
```

**Expected Results:**
- Success rate: > 99%
- Average latency: < 100ms
- No 500 errors

---

## Next Steps

After successful testing:

1. Review Prometheus metrics
2. Create Grafana dashboards
3. Set up alerting rules
4. Configure production SMTP/Twilio
5. Implement additional device types
6. Add more automation rules
7. Create web/mobile frontend
8. Set up CI/CD pipeline
9. Deploy to production (Kubernetes)
10. Monitor and optimize

Congratulations! You've successfully tested a complete microservices-based Smart Home IoT platform! 🎉
