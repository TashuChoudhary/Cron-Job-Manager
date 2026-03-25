const express = require('express');
const router = express.Router();
const NotificationService = require('./notification-service');


const notificationService = new NotificationService();


router.post('/config/email', async (req, res) => {
  try {
    const { smtp_host, smtp_port, username, password, from_email, template } = req.body;
    
    // Validate required fields
    if (!smtp_host || !smtp_port || !username || !password) {
      return res.status(400).json({
        success: false,
        message: 'Missing required email configuration fields'
      });
    }

    const result = notificationService.configureEmail({
      smtp_host,
      smtp_port: parseInt(smtp_port),
      username,
      password,
      from_email,
      template
    });

    // Store configuration in database (implement your DB logic here)
    // await saveEmailConfig(req.user.id, { smtp_host, smtp_port, username, from_email, template });

    res.json(result);
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Email configuration failed: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/config/slack
 * Configure Slack settings
 */
router.post('/config/slack', async (req, res) => {
  try {
    const { webhook_url, channel, username, icon_emoji } = req.body;
    
    if (!webhook_url) {
      return res.status(400).json({
        success: false,
        message: 'Webhook URL is required for Slack configuration'
      });
    }

    // Store configuration in database
    // await saveSlackConfig(req.user.id, { webhook_url, channel, username, icon_emoji });

    res.json({
      success: true,
      message: 'Slack configuration saved successfully'
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Slack configuration failed: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/config/sms
 * Configure SMS/Twilio settings
 */
router.post('/config/sms', async (req, res) => {
  try {
    const { account_sid, auth_token, from_number } = req.body;
    
    if (!account_sid || !auth_token || !from_number) {
      return res.status(400).json({
        success: false,
        message: 'Account SID, Auth Token, and From Number are required'
      });
    }

    const result = notificationService.configureSMS(account_sid, auth_token);
    
    if (result.success) {
      // Store configuration in database
      // await saveSMSConfig(req.user.id, { account_sid, auth_token, from_number });
    }

    res.json(result);
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `SMS configuration failed: ${error.message}`
    });
  }
});

// ==================== TEST ROUTES ====================

/**
 * POST /api/notifications/test/email
 * Test email configuration
 */
router.post('/test/email', async (req, res) => {
  try {
    const { config, test_recipient } = req.body;
    
    if (!test_recipient) {
      return res.status(400).json({
        success: false,
        message: 'Test recipient email is required'
      });
    }

    const result = await notificationService.testEmail(config, test_recipient);
    res.json(result);
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Test email failed: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/test/slack
 * Test Slack configuration
 */
router.post('/test/slack', async (req, res) => {
  try {
    const { webhook_url, channel, username } = req.body;
    
    if (!webhook_url) {
      return res.status(400).json({
        success: false,
        message: 'Webhook URL is required'
      });
    }

    const result = await notificationService.testSlack({
      webhook_url,
      channel,
      username
    });
    
    res.json(result);
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Test Slack message failed: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/test/sms
 * Test SMS configuration
 */
router.post('/test/sms', async (req, res) => {
  try {
    const { test_number, from_number } = req.body;
    
    if (!test_number || !from_number) {
      return res.status(400).json({
        success: false,
        message: 'Test number and from number are required'
      });
    }

    const testData = {
      name: 'SMS Test Notification',
      status: 'success',
      duration: '00:01:00',
      exitCode: 0,
      id: 'test-job-123'
    };

    const result = await notificationService.sendSMS(testData, [test_number], from_number);
    res.json({ success: true, message: 'Test SMS sent successfully', result });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Test SMS failed: ${error.message}`
    });
  }
});

// ==================== NOTIFICATION RULES ROUTES ====================

/**
 * GET /api/notifications/rules
 * Get all notification rules
 */
router.get('/rules', async (req, res) => {
  try {
    const rules = notificationService.getNotificationRules();
    res.json({ success: true, rules });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to fetch rules: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/rules
 * Create new notification rule
 */
router.post('/rules', async (req, res) => {
  try {
    const rule = req.body;
    
    // Validate required fields
    if (!rule.type || !rule.events || !rule.recipients) {
      return res.status(400).json({
        success: false,
        message: 'Type, events, and recipients are required'
      });
    }

    const newRule = notificationService.addNotificationRule(rule);
    
    // Store in database
    // await saveNotificationRule(req.user.id, newRule);
    
    res.status(201).json({ success: true, rule: newRule });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to create rule: ${error.message}`
    });
  }
});

/**
 * PUT /api/notifications/rules/:id
 * Update notification rule
 */
router.put('/rules/:id', async (req, res) => {
  try {
    const { id } = req.params;
    const updates = req.body;
    
    const updatedRule = notificationService.updateNotificationRule(parseInt(id), updates);
    
    if (!updatedRule) {
      return res.status(404).json({
        success: false,
        message: 'Rule not found'
      });
    }
    
    // Update in database
    // await updateNotificationRule(req.user.id, parseInt(id), updates);
    
    res.json({ success: true, rule: updatedRule });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to update rule: ${error.message}`
    });
  }
});

/**
 * DELETE /api/notifications/rules/:id
 * Delete notification rule
 */
router.delete('/rules/:id', async (req, res) => {
  try {
    const { id } = req.params;
    
    const deletedRule = notificationService.deleteNotificationRule(parseInt(id));
    
    if (!deletedRule) {
      return res.status(404).json({
        success: false,
        message: 'Rule not found'
      });
    }
    
    // Delete from database
    // await deleteNotificationRule(req.user.id, parseInt(id));
    
    res.json({ success: true, message: 'Rule deleted successfully' });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to delete rule: ${error.message}`
    });
  }
});

// ==================== NOTIFICATION HISTORY & STATS ====================

/**
 * GET /api/notifications/history
 * Get notification history
 */
router.get('/history', async (req, res) => {
  try {
    const { limit = 50, type, status } = req.query;
    
    let history = notificationService.getNotificationHistory(parseInt(limit));
    
    // Apply filters
    if (type) {
      history = history.filter(item => item.type === type);
    }
    
    if (status) {
      history = history.filter(item => item.status === status);
    }
    
    res.json({ success: true, history });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to fetch history: ${error.message}`
    });
  }
});

/**
 * GET /api/notifications/stats
 * Get notification statistics
 */
router.get('/stats', async (req, res) => {
  try {
    const stats = notificationService.getNotificationStats();
    
    // Add additional stats from database if needed
    const additionalStats = {
      activeRules: notificationService.getNotificationRules().filter(rule => rule.enabled).length,
      totalRules: notificationService.getNotificationRules().length,
      last24Hours: {
        sent: stats.sent, // This could be filtered by date
        failed: stats.failed
      }
    };
    
    res.json({ 
      success: true, 
      stats: { ...stats, ...additionalStats }
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to fetch stats: ${error.message}`
    });
  }
});

// ==================== WEBHOOK ROUTES ====================

/**
 * POST /api/notifications/webhooks
 * Create custom webhook configuration
 */
router.post('/webhooks', async (req, res) => {
  try {
    const { name, url, method, headers, timeout, retryCount, retryDelay } = req.body;
    
    if (!name || !url) {
      return res.status(400).json({
        success: false,
        message: 'Webhook name and URL are required'
      });
    }
    
    const webhookConfig = {
      id: Date.now(),
      name,
      url,
      method: method || 'POST',
      headers: headers || {},
      timeout: timeout || 10000,
      retryCount: retryCount || 3,
      retryDelay: retryDelay || 1000,
      enabled: true,
      createdAt: new Date().toISOString()
    };
    
    // Store in database
    // await saveWebhookConfig(req.user.id, webhookConfig);
    
    res.status(201).json({ success: true, webhook: webhookConfig });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to create webhook: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/webhooks/:id/test
 * Test webhook configuration
 */
router.post('/webhooks/:id/test', async (req, res) => {
  try {
    const { id } = req.params;
    
    // Get webhook config from database
    // const webhookConfig = await getWebhookConfig(req.user.id, parseInt(id));
    
    // For demo, create test config
    const webhookConfig = {
      url: req.body.url,
      method: req.body.method || 'POST',
      headers: req.body.headers || {},
      timeout: 10000,
      retryCount: 0
    };
    
    const testData = {
      name: 'Webhook Test Job',
      status: 'success',
      duration: '00:01:30',
      exitCode: 0,
      id: 'test-webhook-job'
    };
    
    const result = await notificationService.sendWebhook(testData, webhookConfig);
    res.json({ success: true, message: 'Test webhook sent successfully', result });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Test webhook failed: ${error.message}`
    });
  }
});

// ==================== JOB EVENT PROCESSING ====================

/**
 * POST /api/notifications/process-job-event
 * Process job event and trigger notifications
 */
router.post('/process-job-event', async (req, res) => {
  try {
    const { jobData, eventType } = req.body;
    
    if (!jobData || !eventType) {
      return res.status(400).json({
        success: false,
        message: 'Job data and event type are required'
      });
    }
    
    const results = await notificationService.processJobEvent(jobData, eventType);
    
    res.json({ 
      success: true, 
      message: 'Job event processed',
      results 
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to process job event: ${error.message}`
    });
  }
});

/**
 * POST /api/notifications/send
 * Manual send notification (for testing)
 */
router.post('/send', async (req, res) => {
  try {
    const { type, jobData, config } = req.body;
    
    let result;
    
    switch (type) {
      case 'email':
        result = await notificationService.sendEmail(
          jobData, 
          config.recipients, 
          config.template
        );
        break;
        
      case 'slack':
        result = await notificationService.sendSlack(
          jobData, 
          config.webhookUrl, 
          config.channel, 
          config.username
        );
        break;
        
      case 'sms':
        result = await notificationService.sendSMS(
          jobData, 
          config.recipients, 
          config.fromNumber
        );
        break;
        
      case 'webhook':
        result = await notificationService.sendWebhook(jobData, config);
        break;
        
      default:
        return res.status(400).json({
          success: false,
          message: 'Invalid notification type'
        });
    }
    
    res.json({ success: true, result });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to send notification: ${error.message}`
    });
  }
});

// ==================== NOTIFICATION RULES MANAGEMENT ====================

/**
 * POST /api/notifications/rules/bulk
 * Bulk create notification rules
 */
router.post('/rules/bulk', async (req, res) => {
  try {
    const { rules } = req.body;
    
    if (!Array.isArray(rules)) {
      return res.status(400).json({
        success: false,
        message: 'Rules must be an array'
      });
    }
    
    const createdRules = [];
    
    for (const rule of rules) {
      const newRule = notificationService.addNotificationRule(rule);
      createdRules.push(newRule);
    }
    
    res.status(201).json({ 
      success: true, 
      message: `${createdRules.length} rules created successfully`,
      rules: createdRules 
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to create rules: ${error.message}`
    });
  }
});

/**
 * PATCH /api/notifications/rules/:id/toggle
 * Toggle notification rule enabled/disabled
 */
router.patch('/rules/:id/toggle', async (req, res) => {
  try {
    const { id } = req.params;
    
    const rule = notificationService.getNotificationRules().find(r => r.id === parseInt(id));
    
    if (!rule) {
      return res.status(404).json({
        success: false,
        message: 'Rule not found'
      });
    }
    
    const updatedRule = notificationService.updateNotificationRule(parseInt(id), {
      enabled: !rule.enabled
    });
    
    res.json({ 
      success: true, 
      message: `Rule ${updatedRule.enabled ? 'enabled' : 'disabled'}`,
      rule: updatedRule 
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Failed to toggle rule: ${error.message}`
    });
  }
});

// ==================== HEALTH CHECK ====================

/**
 * GET /api/notifications/health
 * Health check for notification service
 */
router.get('/health', async (req, res) => {
  try {
    const health = {
      status: 'healthy',
      timestamp: new Date().toISOString(),
      services: {
        email: notificationService.emailTransporter ? 'configured' : 'not_configured',
        sms: notificationService.twilioClient ? 'configured' : 'not_configured'
      },
      stats: notificationService.getNotificationStats()
    };
    
    res.json({ success: true, health });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: `Health check failed: ${error.message}`
    });
  }
});

// ==================== ERROR HANDLING MIDDLEWARE ====================

router.use((error, req, res, next) => {
  console.error('Notification API Error:', error);
  
  res.status(500).json({
    success: false,
    message: 'Internal server error in notification service',
    ...(process.env.NODE_ENV === 'development' && { stack: error.stack })
  });
});

module.exports = router;