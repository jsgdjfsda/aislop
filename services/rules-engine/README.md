# Rules Engine Service

Automation engine that evaluates rules against incoming device telemetry and executes actions.

## Features

- Real-time rule evaluation on telemetry events
- Complex condition logic (AND/OR)
- Multiple action types (device commands, notifications, webhooks)
- Time-based conditions (after, before, between)
- Day of week conditions
- REST API for rule management
- Automatic rule reloading
- Prometheus metrics

## Rule Structure

### Complete Rule Example

```json
{
  "name": "Evening automation",
  "conditions": {
    "all": [
      {
        "device": "light-sensor-1",
        "metric": "illuminance",
        "operator": "<",
        "value": 100
      },
      {
        "time": "after",
        "value": "18:00"
      }
    ]
  },
  "actions": [
    {
      "device": "smart-light-1",
      "command": "turn_on",
      "params": {
        "brightness": 80
      }
    },
    {
      "notify": "user@example.com",
      "message": "Evening lights turned on"
    }
  ],
  "enabled": true
}
```

## Condition Types

### Device Metric Conditions

```json
{
  "device": "temp-sensor-living-room",
  "metric": "temperature",
  "operator": "<",
  "value": 18
}
```

**Supported operators:** `==`, `!=`, `>`, `<`, `>=`, `<=`, `in`, `not_in`

### Time-Based Conditions

**After specific time:**
```json
{
  "time": "after",
  "value": "18:00"
}
```

**Before specific time:**
```json
{
  "time": "before",
  "value": "06:00"
}
```

**Between times:**
```json
{
  "time": "between",
  "value": {
    "start": "08:00",
    "end": "22:00"
  }
}
```

### Day of Week Conditions

```json
{
  "day_of_week": ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]
}
```

## Condition Logic

### AND Logic (all conditions must match)

```json
{
  "conditions": {
    "all": [
      {"device": "motion-1", "metric": "motion_detected", "operator": "==", "value": true},
      {"time": "after", "value": "22:00"}
    ]
  }
}
```

### OR Logic (any condition can match)

```json
{
  "conditions": {
    "any": [
      {"device": "door-front", "metric": "state", "operator": "==", "value": "open"},
      {"device": "door-back", "metric": "state", "operator": "==", "value": "open"}
    ]
  }
}
```

## Action Types

### Device Command

```json
{
  "device": "smart-light-1",
  "command": "turn_on",
  "params": {
    "brightness": 100,
    "color": "#FFFFFF"
  }
}
```

### Notification

```json
{
  "notify": "user@example.com",
  "message": "Motion detected in living room",
  "channels": ["email", "sms", "push"]
}
```

### Webhook

```json
{
  "webhook": "https://example.com/api/notify",
  "method": "POST",
  "payload": {
    "event": "rule_triggered",
    "details": "..."
  }
}
```

## API Endpoints

### Create Rule

```bash
POST /api/rules
Content-Type: application/json
X-User-ID: user-123

{
  "name": "Turn on heater when cold",
  "conditions": {
    "all": [
      {"device": "temp-sensor-1", "metric": "temperature", "operator": "<", "value": 18}
    ]
  },
  "actions": [
    {"device": "heater-1", "command": "turn_on"}
  ]
}
```

### List Rules

```bash
GET /api/rules
X-User-ID: user-123
```

### Update Rule

```bash
PUT /api/rules/{rule_id}
Content-Type: application/json

{
  "name": "Updated rule name",
  "enabled": true,
  "conditions": {...},
  "actions": [...]
}
```

### Delete Rule

```bash
DELETE /api/rules/{rule_id}
```

## Running

```bash
cd services/rules-engine
pip install -r requirements.txt
python main.py
```

Service starts on port **8083** by default.

## Environment Variables

- `POSTGRES_HOST` - PostgreSQL host
- `POSTGRES_PORT` - PostgreSQL port
- `RABBITMQ_HOST` - RabbitMQ host
- `RABBITMQ_PORT` - RabbitMQ port
- `MQTT_BROKER` - MQTT broker host
- `MQTT_PORT` - MQTT port
- `RULES_ENGINE_PORT` - Service port (default: 8083)

## Data Flow

1. Device sends telemetry → MQTT
2. Data Ingestion Service → RabbitMQ
3. **Rules Engine** consumes from RabbitMQ
4. Evaluates all active rules
5. If rule matches → Execute actions
6. Actions: Send MQTT commands, trigger notifications, call webhooks

## Example Use Cases

### 1. Temperature Control

```json
{
  "name": "Auto heating",
  "conditions": {
    "all": [
      {"device": "temp-sensor-main", "metric": "temperature", "operator": "<", "value": 18},
      {"time": "between", "value": {"start": "06:00", "end": "23:00"}}
    ]
  },
  "actions": [
    {"device": "thermostat-main", "command": "set_mode", "params": {"mode": "heat"}}
  ]
}
```

### 2. Security Alert

```json
{
  "name": "Night security",
  "conditions": {
    "all": [
      {"device": "motion-hallway", "metric": "motion_detected", "operator": "==", "value": true},
      {"time": "between", "value": {"start": "23:00", "end": "06:00"}}
    ]
  },
  "actions": [
    {"device": "light-hallway", "command": "turn_on", "params": {"brightness": 100}},
    {"notify": "owner@example.com", "message": "Motion detected at night!"}
  ]
}
```

### 3. Energy Saving

```json
{
  "name": "Auto lights off",
  "conditions": {
    "all": [
      {"device": "motion-living-room", "metric": "seconds_since_motion", "operator": ">", "value": 300}
    ]
  },
  "actions": [
    {"device": "light-living-room", "command": "turn_off"}
  ]
}
```

### 4. Welcome Home

```json
{
  "name": "Welcome home",
  "conditions": {
    "all": [
      {"device": "door-front", "metric": "state", "operator": "==", "value": "open"},
      {"time": "after", "value": "17:00"}
    ]
  },
  "actions": [
    {"device": "light-hallway", "command": "turn_on"},
    {"device": "thermostat-main", "command": "set_temperature", "params": {"temperature": 22}}
  ]
}
```

## Metrics

Prometheus metrics available at `/metrics`:

- `rules_engine_rules_evaluated_total` - Total rules evaluated (by rule_id, result)
- `rules_engine_execution_duration_seconds` - Rule execution time
- `rules_engine_actions_executed_total` - Total actions executed (by type, status)
- `rules_engine_active_rules` - Number of active rules

## Testing

### Create a test rule

```bash
curl -X POST http://localhost:8083/api/rules \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user" \
  -d '{
    "name": "Test rule",
    "conditions": {
      "all": [
        {"device": "temp-sensor-living-room", "metric": "temperature", "operator": ">", "value": 25}
      ]
    },
    "actions": [
      {"device": "light-living-room", "command": "turn_on"}
    ]
  }'
```

### List rules

```bash
curl http://localhost:8083/api/rules -H "X-User-ID: test-user"
```

### Monitor rule execution

Check logs for rule matches:
```bash
python main.py
# Watch for: "Rule 'Test rule' matched for device temp-sensor-living-room"
```

## Troubleshooting

### Rules not triggering

1. Check rule is enabled: `enabled: true`
2. Verify device_id matches exactly in conditions
3. Check metric names are correct
4. Review logs for evaluation errors

### Actions not executing

1. Check MQTT broker connection
2. Verify device exists and is online
3. Review action execution logs
4. Check Prometheus metrics for action failures
