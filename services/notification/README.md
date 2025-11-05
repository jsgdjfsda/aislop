# Notification Service

Multi-channel notification system supporting email, SMS, and push notifications with user preferences and quiet hours.

## Features

- **Email notifications** via SMTP (Gmail, SendGrid, etc.)
- **SMS notifications** via Twilio
- **Push notifications** (placeholder for FCM/APNs)
- User preferences (enable/disable channels)
- Quiet hours support
- Notification history
- RabbitMQ consumer for async notifications
- Prometheus metrics

## API Endpoints

### Send Notification

```bash
POST /api/notifications/send
Content-Type: application/json

{
  "user_id": "user-123",
  "type": "alert",
  "channels": ["email", "sms", "push"],
  "subject": "Motion Detected",
  "message": "Motion detected in living room at 10:30 PM",
  "metadata": {
    "device_id": "motion-living-room",
    "timestamp": "2025-11-05T22:30:00Z"
  }
}
```

**Response:**
```json
{
  "success": true,
  "results": [
    { "success": true, "channel": "email" },
    { "success": true, "channel": "sms" },
    { "success": true, "channel": "push", "note": "Push notifications not yet implemented" }
  ]
}
```

### Get Notification History

```bash
GET /api/notifications/history?limit=50
X-User-ID: user-123
```

**Response:**
```json
{
  "notifications": [
    {
      "id": "notif-1",
      "type": "alert",
      "channel": "email",
      "subject": "Motion Detected",
      "message": "Motion detected...",
      "status": "sent",
      "sent_at": "2025-11-05T22:30:00Z",
      "created_at": "2025-11-05T22:30:00Z"
    }
  ],
  "count": 1
}
```

### Get User Preferences

```bash
GET /api/notifications/preferences
X-User-ID: user-123
```

**Response:**
```json
{
  "user_id": "user-123",
  "email_enabled": true,
  "sms_enabled": false,
  "push_enabled": true,
  "quiet_hours_start": "22:00",
  "quiet_hours_end": "07:00"
}
```

### Update User Preferences

```bash
PUT /api/notifications/preferences
X-User-ID: user-123
Content-Type: application/json

{
  "email_enabled": true,
  "sms_enabled": true,
  "push_enabled": true,
  "quiet_hours_start": "23:00",
  "quiet_hours_end": "08:00"
}
```

## Configuration

### Environment Variables

**SMTP (Email):**
```bash
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@smarthome.local
```

**Twilio (SMS):**
```bash
TWILIO_ACCOUNT_SID=your-account-sid
TWILIO_AUTH_TOKEN=your-auth-token
TWILIO_PHONE_NUMBER=+1234567890
```

**Service:**
```bash
NOTIFICATION_PORT=8085
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
```

### Gmail Setup

1. Enable 2-factor authentication on your Google account
2. Generate an App Password:
   - Go to Google Account → Security → 2-Step Verification → App passwords
   - Select "Mail" and generate password
3. Use the generated password as `SMTP_PASSWORD`

### Twilio Setup

1. Sign up at https://www.twilio.com/
2. Get your Account SID and Auth Token from the dashboard
3. Purchase a phone number or use trial number
4. Add recipient numbers to verified list (for trial accounts)

## Running

```bash
cd services/notification
npm install
npm start
```

Service starts on port **8085** by default.

## Notification Channels

### Email

**HTML formatted emails** with:
- Styled header and footer
- Subject line
- Message content
- Device metadata
- Timestamp

### SMS

**Plain text messages** via Twilio:
- Character limit: 160 (standard SMS)
- Supports international numbers
- Delivery receipts available

### Push Notifications

**Placeholder implementation** - ready for:
- Firebase Cloud Messaging (FCM) for Android
- Apple Push Notification Service (APNs) for iOS

## User Preferences

### Channel Control

Users can enable/disable each channel:
```json
{
  "email_enabled": true,
  "sms_enabled": false,
  "push_enabled": true
}
```

### Quiet Hours

Notifications are suppressed during quiet hours:
```json
{
  "quiet_hours_start": "22:00",
  "quiet_hours_end": "07:00"
}
```

## Integration with Other Services

### RabbitMQ Integration

Services can send notifications via RabbitMQ:

```python
# Example from Rules Engine
import pika
import json

connection = pika.BlockingConnection(pika.ConnectionParameters('localhost'))
channel = connection.channel()

message = {
    "user_id": "user-123",
    "type": "alert",
    "channels": ["email"],
    "subject": "Temperature Alert",
    "message": "Temperature exceeded threshold"
}

channel.basic_publish(
    exchange='notifications',
    routing_key='notification.alert',
    body=json.dumps(message)
)
```

### REST API Integration

```bash
curl -X POST http://localhost:8085/api/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "type": "info",
    "channels": ["email"],
    "subject": "Daily Report",
    "message": "Your energy consumption today: 25.3 kWh"
  }'
```

## Use Cases

### 1. Security Alerts

```json
{
  "user_id": "user-123",
  "type": "security",
  "channels": ["email", "sms", "push"],
  "subject": "Security Alert",
  "message": "Door opened while system armed",
  "metadata": {
    "device_id": "door-front",
    "timestamp": "2025-11-05T02:30:00Z",
    "severity": "high"
  }
}
```

### 2. Device Offline Alerts

```json
{
  "user_id": "user-123",
  "type": "warning",
  "channels": ["email"],
  "subject": "Device Offline",
  "message": "Temperature sensor in bedroom has been offline for 30 minutes",
  "metadata": {
    "device_id": "temp-sensor-bedroom",
    "last_seen": "2025-11-05T14:00:00Z"
  }
}
```

### 3. Daily Summaries

```json
{
  "user_id": "user-123",
  "type": "report",
  "channels": ["email"],
  "subject": "Daily Smart Home Report",
  "message": "Energy: 28.5 kWh | Cost: $3.42 | Avg Temp: 22.1°C",
  "metadata": {
    "report_date": "2025-11-05"
  }
}
```

### 4. Automation Confirmations

```json
{
  "user_id": "user-123",
  "type": "info",
  "channels": ["push"],
  "subject": "Automation Triggered",
  "message": "Evening lights turned on automatically",
  "metadata": {
    "rule_id": "rule-evening-lights",
    "devices": ["light-living-room", "light-hallway"]
  }
}
```

## Email Templates

The service uses HTML email templates with Smart Home branding:

```html
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; }
        .header { background: #4CAF50; color: white; padding: 20px; }
        .content { padding: 20px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>🏠 Smart Home Alert</h2>
    </div>
    <div class="content">
        <h3>Subject</h3>
        <p>Message content...</p>
    </div>
</body>
</html>
```

## Metrics

Prometheus metrics at `/metrics`:

- `notifications_sent_total` - Total notifications sent (by channel, status)
- `notification_duration_seconds` - Processing time (by channel)

## Testing

### Test Email Notification

```bash
curl -X POST http://localhost:8085/api/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test-user",
    "channels": ["email"],
    "subject": "Test Notification",
    "message": "This is a test notification from Smart Home IoT"
  }'
```

### Test Quiet Hours

```bash
# Set quiet hours
curl -X PUT http://localhost:8085/api/notifications/preferences \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user" \
  -d '{
    "quiet_hours_start": "22:00",
    "quiet_hours_end": "08:00"
  }'

# Try sending during quiet hours (should be suppressed)
curl -X POST http://localhost:8085/api/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test-user",
    "channels": ["email"],
    "subject": "Test",
    "message": "Should be suppressed during quiet hours"
  }'
```

### View Notification History

```bash
curl http://localhost:8085/api/notifications/history \
  -H "X-User-ID: test-user"
```

## Error Handling

The service handles errors gracefully:

- **SMTP errors**: Logged and recorded with status 'failed'
- **Twilio errors**: Retried once, then marked as failed
- **Network errors**: Automatic retry via RabbitMQ
- **Invalid configuration**: Service starts but channels are disabled

## Troubleshooting

### Email not sending

1. Check SMTP credentials:
   ```bash
   echo $SMTP_USER
   echo $SMTP_PASSWORD
   ```

2. Test SMTP connection:
   ```bash
   telnet smtp.gmail.com 587
   ```

3. Check service logs for errors

4. Verify Gmail app password (not regular password)

### SMS not sending

1. Verify Twilio credentials
2. Check phone number format: `+1234567890`
3. For trial accounts, add recipient to verified numbers
4. Check Twilio console for delivery logs

### Notifications suppressed

1. Check user preferences (channel enabled?)
2. Check quiet hours settings
3. Review notification history for failures

## Future Enhancements

- [ ] Rich push notifications with images
- [ ] Notification templates system
- [ ] Rate limiting per user
- [ ] Batch notifications
- [ ] Notification digest (combine multiple alerts)
- [ ] Custom email templates per notification type
- [ ] Webhook notifications
- [ ] Slack/Discord integrations
- [ ] In-app notifications

## Database Schema

```sql
-- Notifications table
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    type VARCHAR(50) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    subject VARCHAR(255),
    message TEXT NOT NULL,
    metadata JSONB,
    status VARCHAR(20) DEFAULT 'pending',
    sent_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Preferences table
CREATE TABLE notification_preferences (
    user_id UUID PRIMARY KEY,
    email_enabled BOOLEAN DEFAULT true,
    sms_enabled BOOLEAN DEFAULT false,
    push_enabled BOOLEAN DEFAULT true,
    quiet_hours_start TIME,
    quiet_hours_end TIME,
    preferences JSONB
);
```
