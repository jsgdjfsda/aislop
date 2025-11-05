#!/usr/bin/env node

/**
 * Notification Service
 *
 * Multi-channel notification system supporting email, SMS, and push notifications.
 * Consumes from RabbitMQ and provides REST API for sending notifications.
 */

const express = require('express');
const { Pool } = require('pg');
const nodemailer = require('nodemailer');
const twilio = require('twilio');
const amqp = require('amqplib');
const promClient = require('prom-client');
const winston = require('winston');
require('dotenv').config({ path: '../../.env' });

// Configuration
const PORT = process.env.NOTIFICATION_PORT || 8085;
const DB_HOST = process.env.POSTGRES_HOST || 'localhost';
const DB_PORT = process.env.POSTGRES_PORT || 5432;
const DB_USER = process.env.POSTGRES_USER || 'smarthome';
const DB_PASSWORD = process.env.POSTGRES_PASSWORD || 'smarthome_dev_password';
const DB_NAME = 'notification_db';

const RABBITMQ_HOST = process.env.RABBITMQ_HOST || 'localhost';
const RABBITMQ_PORT = process.env.RABBITMQ_PORT || 5672;
const RABBITMQ_USER = process.env.RABBITMQ_USER || 'smarthome';
const RABBITMQ_PASSWORD = process.env.RABBITMQ_PASSWORD || 'smarthome_dev_password';

// SMTP Configuration
const SMTP_HOST = process.env.SMTP_HOST || 'smtp.gmail.com';
const SMTP_PORT = process.env.SMTP_PORT || 587;
const SMTP_USER = process.env.SMTP_USER;
const SMTP_PASSWORD = process.env.SMTP_PASSWORD;
const SMTP_FROM = process.env.SMTP_FROM || 'noreply@smarthome.local';

// Twilio Configuration
const TWILIO_ACCOUNT_SID = process.env.TWILIO_ACCOUNT_SID;
const TWILIO_AUTH_TOKEN = process.env.TWILIO_AUTH_TOKEN;
const TWILIO_PHONE_NUMBER = process.env.TWILIO_PHONE_NUMBER;

// Logger setup
const logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    transports: [
        new winston.transports.Console({
            format: winston.format.combine(
                winston.format.colorize(),
                winston.format.simple()
            )
        })
    ]
});

// Prometheus metrics
const register = new promClient.Registry();
promClient.collectDefaultMetrics({ register });

const notificationsSent = new promClient.Counter({
    name: 'notifications_sent_total',
    help: 'Total notifications sent',
    labelNames: ['channel', 'status'],
    registers: [register]
});

const notificationDuration = new promClient.Histogram({
    name: 'notification_duration_seconds',
    help: 'Notification processing time',
    labelNames: ['channel'],
    registers: [register]
});

// Database connection pool
const pool = new Pool({
    host: DB_HOST,
    port: DB_PORT,
    user: DB_USER,
    password: DB_PASSWORD,
    database: DB_NAME,
    max: 20,
    idleTimeoutMillis: 30000,
    connectionTimeoutMillis: 2000,
});

// Email transporter
let emailTransporter = null;
if (SMTP_USER && SMTP_PASSWORD) {
    emailTransporter = nodemailer.createTransport({
        host: SMTP_HOST,
        port: SMTP_PORT,
        secure: false,
        auth: {
            user: SMTP_USER,
            pass: SMTP_PASSWORD,
        },
    });
    logger.info('Email transporter configured');
} else {
    logger.warn('SMTP credentials not provided, email notifications disabled');
}

// Twilio client
let twilioClient = null;
if (TWILIO_ACCOUNT_SID && TWILIO_AUTH_TOKEN) {
    twilioClient = twilio(TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN);
    logger.info('Twilio client configured');
} else {
    logger.warn('Twilio credentials not provided, SMS notifications disabled');
}

// Express app
const app = express();
app.use(express.json());

// Notification handlers
class NotificationService {
    /**
     * Send email notification
     */
    static async sendEmail(userId, subject, message, metadata = {}) {
        const timer = notificationDuration.startTimer({ channel: 'email' });

        try {
            if (!emailTransporter) {
                throw new Error('Email transporter not configured');
            }

            // Get user email from preferences
            const userEmail = await this.getUserEmail(userId);
            if (!userEmail) {
                throw new Error('User email not found');
            }

            const mailOptions = {
                from: SMTP_FROM,
                to: userEmail,
                subject: subject,
                text: message,
                html: this.formatEmailHtml(subject, message, metadata),
            };

            await emailTransporter.sendMail(mailOptions);

            // Record notification
            await this.recordNotification(userId, 'email', 'email', subject, message, metadata, 'sent');

            notificationsSent.labels('email', 'success').inc();
            logger.info(`Email sent to user ${userId}`);

            return { success: true, channel: 'email' };

        } catch (error) {
            logger.error(`Email send failed: ${error.message}`);
            await this.recordNotification(userId, 'email', 'email', subject, message, metadata, 'failed');
            notificationsSent.labels('email', 'error').inc();
            throw error;
        } finally {
            timer();
        }
    }

    /**
     * Send SMS notification
     */
    static async sendSMS(userId, message, metadata = {}) {
        const timer = notificationDuration.startTimer({ channel: 'sms' });

        try {
            if (!twilioClient) {
                throw new Error('Twilio client not configured');
            }

            // Get user phone from preferences
            const userPhone = await this.getUserPhone(userId);
            if (!userPhone) {
                throw new Error('User phone not found');
            }

            await twilioClient.messages.create({
                body: message,
                from: TWILIO_PHONE_NUMBER,
                to: userPhone,
            });

            // Record notification
            await this.recordNotification(userId, 'sms', 'sms', null, message, metadata, 'sent');

            notificationsSent.labels('sms', 'success').inc();
            logger.info(`SMS sent to user ${userId}`);

            return { success: true, channel: 'sms' };

        } catch (error) {
            logger.error(`SMS send failed: ${error.message}`);
            await this.recordNotification(userId, 'sms', 'sms', null, message, metadata, 'failed');
            notificationsSent.labels('sms', 'error').inc();
            throw error;
        } finally {
            timer();
        }
    }

    /**
     * Send push notification (placeholder)
     */
    static async sendPush(userId, title, message, metadata = {}) {
        const timer = notificationDuration.startTimer({ channel: 'push' });

        try {
            // TODO: Implement FCM or other push notification service
            logger.info(`Push notification (stub): ${title} - ${message}`);

            await this.recordNotification(userId, 'push', 'push', title, message, metadata, 'sent');
            notificationsSent.labels('push', 'success').inc();

            return { success: true, channel: 'push', note: 'Push notifications not yet implemented' };

        } finally {
            timer();
        }
    }

    /**
     * Send multi-channel notification
     */
    static async sendNotification(userId, type, channels, subject, message, metadata = {}) {
        const results = [];

        // Check user preferences
        const preferences = await this.getUserPreferences(userId);

        for (const channel of channels) {
            // Skip if user has disabled this channel
            if (!preferences[`${channel}_enabled`]) {
                logger.info(`Skipping ${channel} for user ${userId} (disabled in preferences)`);
                continue;
            }

            // Check quiet hours
            if (this.isQuietHours(preferences)) {
                logger.info(`Skipping notification for user ${userId} (quiet hours)`);
                continue;
            }

            try {
                let result;
                switch (channel) {
                    case 'email':
                        result = await this.sendEmail(userId, subject, message, metadata);
                        break;
                    case 'sms':
                        result = await this.sendSMS(userId, message, metadata);
                        break;
                    case 'push':
                        result = await this.sendPush(userId, subject, message, metadata);
                        break;
                    default:
                        logger.warn(`Unknown channel: ${channel}`);
                        continue;
                }
                results.push(result);
            } catch (error) {
                results.push({ success: false, channel, error: error.message });
            }
        }

        return results;
    }

    /**
     * Record notification in database
     */
    static async recordNotification(userId, type, channel, subject, message, metadata, status) {
        const query = `
            INSERT INTO notifications (user_id, type, channel, subject, message, metadata, status, sent_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
        `;

        await pool.query(query, [
            userId,
            type,
            channel,
            subject,
            message,
            JSON.stringify(metadata),
            status
        ]);
    }

    /**
     * Get user email
     */
    static async getUserEmail(userId) {
        // In production, this would query user_management_db
        // For now, return test email or metadata
        return SMTP_USER; // Fallback for testing
    }

    /**
     * Get user phone
     */
    static async getUserPhone(userId) {
        // In production, this would query user_management_db
        return null; // Not implemented
    }

    /**
     * Get user notification preferences
     */
    static async getUserPreferences(userId) {
        try {
            const query = 'SELECT * FROM notification_preferences WHERE user_id = $1';
            const result = await pool.query(query, [userId]);

            if (result.rows.length > 0) {
                return result.rows[0];
            }

            // Default preferences
            return {
                email_enabled: true,
                sms_enabled: false,
                push_enabled: true,
                quiet_hours_start: null,
                quiet_hours_end: null,
            };
        } catch (error) {
            logger.error(`Error getting user preferences: ${error.message}`);
            return { email_enabled: true, sms_enabled: false, push_enabled: true };
        }
    }

    /**
     * Check if current time is in quiet hours
     */
    static isQuietHours(preferences) {
        if (!preferences.quiet_hours_start || !preferences.quiet_hours_end) {
            return false;
        }

        const now = new Date();
        const currentTime = now.getHours() * 60 + now.getMinutes();

        const [startHour, startMin] = preferences.quiet_hours_start.split(':').map(Number);
        const [endHour, endMin] = preferences.quiet_hours_end.split(':').map(Number);

        const quietStart = startHour * 60 + startMin;
        const quietEnd = endHour * 60 + endMin;

        return currentTime >= quietStart && currentTime <= quietEnd;
    }

    /**
     * Format email HTML
     */
    static formatEmailHtml(subject, message, metadata) {
        return `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { background: #f9f9f9; padding: 20px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>🏠 Smart Home Alert</h2>
        </div>
        <div class="content">
            <h3>${subject}</h3>
            <p>${message}</p>
            ${metadata.device_id ? `<p><small>Device: ${metadata.device_id}</small></p>` : ''}
            ${metadata.timestamp ? `<p><small>Time: ${metadata.timestamp}</small></p>` : ''}
        </div>
        <div class="footer">
            <p>Smart Home IoT Management System</p>
        </div>
    </div>
</body>
</html>
        `;
    }
}

// API Endpoints

app.get('/health', (req, res) => {
    res.json({ status: 'healthy', service: 'notification' });
});

app.get('/metrics', (req, res) => {
    res.set('Content-Type', register.contentType);
    res.end(register.metrics());
});

/**
 * Send notification
 */
app.post('/api/notifications/send', async (req, res) => {
    try {
        const { user_id, type, channels, subject, message, metadata } = req.body;

        if (!user_id || !channels || !message) {
            return res.status(400).json({ error: 'Missing required fields' });
        }

        const results = await NotificationService.sendNotification(
            user_id,
            type || 'general',
            channels,
            subject || 'Smart Home Notification',
            message,
            metadata || {}
        );

        res.json({ success: true, results });

    } catch (error) {
        logger.error(`Send notification error: ${error.message}`);
        res.status(500).json({ error: error.message });
    }
});

/**
 * Get notification history
 */
app.get('/api/notifications/history', async (req, res) => {
    try {
        const userId = req.headers['x-user-id'];
        const limit = parseInt(req.query.limit) || 50;

        const query = `
            SELECT id, type, channel, subject, message, status, sent_at, created_at
            FROM notifications
            WHERE user_id = $1
            ORDER BY created_at DESC
            LIMIT $2
        `;

        const result = await pool.query(query, [userId, limit]);

        res.json({
            notifications: result.rows,
            count: result.rows.length
        });

    } catch (error) {
        logger.error(`Get history error: ${error.message}`);
        res.status(500).json({ error: error.message });
    }
});

/**
 * Get user preferences
 */
app.get('/api/notifications/preferences', async (req, res) => {
    try {
        const userId = req.headers['x-user-id'];
        const preferences = await NotificationService.getUserPreferences(userId);
        res.json(preferences);

    } catch (error) {
        logger.error(`Get preferences error: ${error.message}`);
        res.status(500).json({ error: error.message });
    }
});

/**
 * Update user preferences
 */
app.put('/api/notifications/preferences', async (req, res) => {
    try {
        const userId = req.headers['x-user-id'];
        const { email_enabled, sms_enabled, push_enabled, quiet_hours_start, quiet_hours_end } = req.body;

        const query = `
            INSERT INTO notification_preferences (user_id, email_enabled, sms_enabled, push_enabled, quiet_hours_start, quiet_hours_end)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (user_id)
            DO UPDATE SET
                email_enabled = EXCLUDED.email_enabled,
                sms_enabled = EXCLUDED.sms_enabled,
                push_enabled = EXCLUDED.push_enabled,
                quiet_hours_start = EXCLUDED.quiet_hours_start,
                quiet_hours_end = EXCLUDED.quiet_hours_end
        `;

        await pool.query(query, [
            userId,
            email_enabled !== undefined ? email_enabled : true,
            sms_enabled !== undefined ? sms_enabled : false,
            push_enabled !== undefined ? push_enabled : true,
            quiet_hours_start,
            quiet_hours_end
        ]);

        res.json({ success: true, message: 'Preferences updated' });

    } catch (error) {
        logger.error(`Update preferences error: ${error.message}`);
        res.status(500).json({ error: error.message });
    }
});

// RabbitMQ Consumer (for receiving notification requests from other services)
async function startRabbitMQConsumer() {
    try {
        const connection = await amqp.connect(
            `amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@${RABBITMQ_HOST}:${RABBITMQ_PORT}`
        );

        const channel = await connection.createChannel();
        const exchange = 'notifications';
        const queue = 'notification.requests';

        await channel.assertExchange(exchange, 'topic', { durable: true });
        await channel.assertQueue(queue, { durable: true });
        await channel.bindQueue(queue, exchange, 'notification.#');

        logger.info('RabbitMQ consumer started');

        channel.consume(queue, async (msg) => {
            if (msg !== null) {
                try {
                    const data = JSON.parse(msg.content.toString());
                    logger.info(`Received notification request: ${JSON.stringify(data)}`);

                    await NotificationService.sendNotification(
                        data.user_id,
                        data.type,
                        data.channels || ['email'],
                        data.subject,
                        data.message,
                        data.metadata
                    );

                    channel.ack(msg);
                } catch (error) {
                    logger.error(`Error processing message: ${error.message}`);
                    channel.nack(msg, false, false);
                }
            }
        });

    } catch (error) {
        logger.error(`RabbitMQ connection error: ${error.message}`);
        // Retry after 5 seconds
        setTimeout(startRabbitMQConsumer, 5000);
    }
}

// Start server
async function start() {
    try {
        // Test database connection
        await pool.query('SELECT NOW()');
        logger.info('Database connection successful');

        // Start RabbitMQ consumer
        startRabbitMQConsumer();

        // Start Express server
        app.listen(PORT, () => {
            logger.info(`Notification Service listening on port ${PORT}`);
        });

    } catch (error) {
        logger.error(`Failed to start service: ${error.message}`);
        process.exit(1);
    }
}

// Graceful shutdown
process.on('SIGINT', async () => {
    logger.info('Shutting down gracefully...');
    await pool.end();
    process.exit(0);
});

start();
