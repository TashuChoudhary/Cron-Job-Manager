const nodemailer = require('nodemailer');
const axios = require('axios');
const twilio = require('twilio');

class NotificationService {
  constructor() {
    this.emailTransporter = null;
    this.twilioClient = null;
    this.notificationHistory = [];
    this.notificationRules = [];
  }

  // ==================== EMAIL SERVICE ====================
  
  /**
   * Configure SMTP Email Transport
   */
  configureEmail(config) {
    try {
      this.emailTransporter = nodemailer.createTransporter({
        host: config.smtp_host,
        port: config.smtp_port,
        secure: config.smtp_port === 465, 
        auth: {
          user: config.username,
          pass: config.password,
        },
        tls: {
          rejectUnauthorized: false 
        }
      });
      
      return { success: true, message: 'Email configuration saved successfully' };
    } catch (error) {
      return { success: false, message: `Email configuration failed: ${error.message}` };
    }
  }

  
  async sendEmail(jobData, recipients, template = 'modern') {
    if (!this.emailTransporter) {
      throw new Error('Email not configured');
    }

    const emailTemplate = this.getEmailTemplate(jobData, template);
    
    try {
      const results = [];
      
      for (const recipient of recipients) {
        const mailOptions = {
          from: process.env.FROM_EMAIL || 'noreply@cronjobmanager.com',
          to: recipient,
          subject: emailTemplate.subject,
          html: emailTemplate.html,
          text: emailTemplate.text
        };

        const result = await this.emailTransporter.sendMail(mailOptions);
        
        results.push({
          recipient,
          success: true,
          messageId: result.messageId
        });

        // Log to history
        this.addToHistory('email', recipient, jobData.name, 'sent', result.messageId);
      }

      return { success: true, results };
    } catch (error) {
      this.addToHistory('email', recipients[0], jobData.name, 'failed', error.message);
      throw error;
    }
  }

  /**
   * Test Email Configuration
   */
  async testEmail(config, testRecipient) {
    const testData = {
      name: 'Test Notification Job',
      status: 'failed',
      duration: '00:02:30',
      exitCode: 1,
      error: 'This is a test notification to verify your email configuration.',
      schedule: '*/5 * * * *',
      timestamp: new Date().toISOString()
    };

    try {
      // Temporarily configure for test
      const originalTransporter = this.emailTransporter;
      this.configureEmail(config);
      
      await this.sendEmail(testData, [testRecipient]);
      
      // Restore original transporter
      this.emailTransporter = originalTransporter;
      
      return { success: true, message: 'Test email sent successfully' };
    } catch (error) {
      return { success: false, message: `Test email failed: ${error.message}` };
    }
  }

  // ==================== SLACK SERVICE ====================
  
  /**
   * Send Slack Notification
   */
  async sendSlack(jobData, webhookUrl, channel = '#general', username = 'CronJob Manager') {
    const slackMessage = this.getSlackMessage(jobData, channel, username);
    
    try {
      const response = await axios.post(webhookUrl, slackMessage, {
        headers: { 'Content-Type': 'application/json' },
        timeout: 10000
      });

      this.addToHistory('slack', channel, jobData.name, 'sent', response.data);
      return { success: true, response: response.data };
    } catch (error) {
      this.addToHistory('slack', channel, jobData.name, 'failed', error.message);
      throw new Error(`Slack notification failed: ${error.message}`);
    }
  }

  /**
   * Test Slack Configuration
   */
  async testSlack(config) {
    const testData = {
      name: 'Test Notification Job',
      status: 'success',
      duration: '00:01:15',
      exitCode: 0,
      schedule: '*/5 * * * *',
      timestamp: new Date().toISOString()
    };

    try {
      await this.sendSlack(testData, config.webhook_url, config.channel, config.username);
      return { success: true, message: 'Test Slack message sent successfully' };
    } catch (error) {
      return { success: false, message: error.message };
    }
  }

  // ==================== SMS SERVICE (TWILIO) ====================
  
  /**
   * Configure Twilio SMS
   */
  configureSMS(accountSid, authToken) {
    try {
      this.twilioClient = twilio(accountSid, authToken);
      return { success: true, message: 'SMS configuration saved successfully' };
    } catch (error) {
      return { success: false, message: `SMS configuration failed: ${error.message}` };
    }
  }

  /**
   * Send SMS Notification
   */
  async sendSMS(jobData, recipients, fromNumber) {
    if (!this.twilioClient) {
      throw new Error('SMS not configured');
    }

    const message = this.getSMSMessage(jobData);
    
    try {
      const results = [];
      
      for (const recipient of recipients) {
        const result = await this.twilioClient.messages.create({
          body: message,
          from: fromNumber,
          to: recipient
        });

        results.push({
          recipient,
          success: true,
          sid: result.sid
        });

        this.addToHistory('sms', recipient, jobData.name, 'sent', result.sid);
      }

      return { success: true, results };
    } catch (error) {
      this.addToHistory('sms', recipients[0], jobData.name, 'failed', error.message);
      throw error;
    }
  }

  // ==================== WEBHOOK SERVICE ====================
  
  /**
   * Send Custom Webhook
   */
  async sendWebhook(jobData, webhookConfig) {
    const payload = {
      job: jobData,
      timestamp: new Date().toISOString(),
      notification_type: 'job_status',
      ...webhookConfig.customPayload
    };

    try {
      const response = await axios({
        method: webhookConfig.method || 'POST',
        url: webhookConfig.url,
        data: payload,
        headers: {
          'Content-Type': 'application/json',
          ...webhookConfig.headers
        },
        timeout: webhookConfig.timeout || 10000
      });

      this.addToHistory('webhook', webhookConfig.url, jobData.name, 'sent', response.status);
      return { success: true, response: response.data };
    } catch (error) {
      this.addToHistory('webhook', webhookConfig.url, jobData.name, 'failed', error.message);
      
      // Retry logic with exponential backoff
      if (webhookConfig.retryCount > 0) {
        await this.sleep(webhookConfig.retryDelay || 1000);
        return this.sendWebhook(jobData, {
          ...webhookConfig,
          retryCount: webhookConfig.retryCount - 1,
          retryDelay: (webhookConfig.retryDelay || 1000) * 2
        });
      }
      
      throw error;
    }
  }

  // ==================== NOTIFICATION RULES ENGINE ====================
  
  /**
   * Process Job Event and Send Notifications
   */
  async processJobEvent(jobData, eventType = 'failure') {
    const applicableRules = this.notificationRules.filter(rule => 
      rule.enabled && 
      rule.events.includes(eventType) &&
      this.evaluateRuleConditions(rule, jobData)
    );

    const results = [];

    for (const rule of applicableRules) {
      try {
        let result;
        
        switch (rule.type) {
          case 'email':
            result = await this.sendEmail(jobData, rule.recipients, rule.template);
            break;
          case 'slack':
            result = await this.sendSlack(jobData, rule.webhookUrl, rule.channel, rule.username);
            break;
          case 'sms':
            result = await this.sendSMS(jobData, rule.recipients, rule.fromNumber);
            break;
          case 'webhook':
            result = await this.sendWebhook(jobData, rule.config);
            break;
        }
        
        results.push({ rule: rule.id, success: true, result });
      } catch (error) {
        results.push({ rule: rule.id, success: false, error: error.message });
      }
    }

    return results;
  }

  /**
   * Evaluate Rule Conditions
   */
  evaluateRuleConditions(rule, jobData) {
    if (!rule.conditions) return true;

    // Check consecutive failures
    if (rule.conditions.consecutiveFailures) {
      const recentFailures = this.getRecentFailures(jobData.name);
      if (recentFailures < rule.conditions.consecutiveFailures) {
        return false;
      }
    }

    // Check business hours
    if (rule.conditions.businessHoursOnly) {
      const now = new Date();
      const hour = now.getHours();
      if (hour < 9 || hour > 17) {
        return false;
      }
    }

    // Check maintenance window
    if (rule.conditions.skipMaintenanceWindow) {
      if (this.isMaintenanceWindow()) {
        return false;
      }
    }

    return true;
  }

  // ==================== TEMPLATE GENERATORS ====================
  
  /**
   * Get Email Template
   */
  getEmailTemplate(jobData, template = 'modern') {
    const statusIcon = jobData.status === 'success' ? '✅' : '🚨';
    const statusColor = jobData.status === 'success' ? '#10B981' : '#EF4444';
    
    const subject = `${statusIcon} CronJob ${jobData.status === 'success' ? 'Success' : 'Failed'}: ${jobData.name}`;
    
    const html = `
      <!DOCTYPE html>
      <html>
      <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>CronJob Notification</title>
      </head>
      <body style="font-family: Arial, sans-serif; line-height: 1.6; margin: 0; padding: 20px; background-color: #f4f4f4;">
        <div style="max-width: 600px; margin: 0 auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
          <div style="background: linear-gradient(135deg, ${statusColor}, ${statusColor}dd); color: white; padding: 20px; text-align: center;">
            <h1 style="margin: 0; font-size: 24px;">${statusIcon} Job ${jobData.status === 'success' ? 'Completed' : 'Failed'}</h1>
            <p style="margin: 5px 0 0 0; opacity: 0.9;">CronJob Manager Alert • ${new Date(jobData.timestamp).toLocaleString()}</p>
          </div>
          
          <div style="padding: 30px;">
            <h2 style="color: #333; margin-top: 0;">Job Details</h2>
            <table style="width: 100%; border-collapse: collapse; margin-bottom: 20px;">
              <tr><td style="padding: 8px; border-bottom: 1px solid #eee; font-weight: bold;">Name:</td><td style="padding: 8px; border-bottom: 1px solid #eee;">${jobData.name}</td></tr>
              <tr><td style="padding: 8px; border-bottom: 1px solid #eee; font-weight: bold;">Schedule:</td><td style="padding: 8px; border-bottom: 1px solid #eee;">${jobData.schedule}</td></tr>
              <tr><td style="padding: 8px; border-bottom: 1px solid #eee; font-weight: bold;">Duration:</td><td style="padding: 8px; border-bottom: 1px solid #eee;">${jobData.duration}</td></tr>
              <tr><td style="padding: 8px; border-bottom: 1px solid #eee; font-weight: bold;">Exit Code:</td><td style="padding: 8px; border-bottom: 1px solid #eee;">${jobData.exitCode}</td></tr>
            </table>
            
            ${jobData.error ? `
              <h3 style="color: #EF4444;">Error Details</h3>
              <div style="background: #FEF2F2; border: 1px solid #FECACA; border-radius: 6px; padding: 15px; font-family: monospace; font-size: 14px; margin-bottom: 20px;">
                ${jobData.error}
              </div>
            ` : ''}
            
            <div style="text-align: center; margin-top: 30px;">
              <a href="https://cronjobmanager.com/jobs/${jobData.id}/logs" style="background: #3B82F6; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 0 5px; display: inline-block;">View Logs</a>
              <a href="https://cronjobmanager.com/jobs/${jobData.id}/retry" style="background: #10B981; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; margin: 0 5px; display: inline-block;">Retry Job</a>
            </div>
          </div>
          
          <div style="background: #F9FAFB; padding: 20px; text-align: center; color: #6B7280; font-size: 14px;">
            <p>This notification was sent by CronJob Manager Pro</p>
            <p><a href="https://cronjobmanager.com/notifications/unsubscribe" style="color: #6B7280;">Unsubscribe</a> | <a href="https://cronjobmanager.com/settings" style="color: #6B7280;">Notification Settings</a></p>
          </div>
        </div>
      </body>
      </html>
    `;

    const text = `
${statusIcon} CronJob ${jobData.status === 'success' ? 'Success' : 'Failed'}: ${jobData.name}

Job Details:
- Name: ${jobData.name}
- Schedule: ${jobData.schedule}
- Duration: ${jobData.duration}
- Exit Code: ${jobData.exitCode}

${jobData.error ? `Error: ${jobData.error}` : ''}

View details: https://cronjobmanager.com/jobs/${jobData.id}
    `;

    return { subject, html, text };
  }

  /**
   * Get Slack Message Format
   */
  getSlackMessage(jobData, channel, username) {
    const statusColor = jobData.status === 'success' ? 'good' : 'danger';
    const statusIcon = jobData.status === 'success' ? '✅' : '🚨';
    
    return {
      channel,
      username,
      icon_emoji: ':robot_face:',
      attachments: [{
        color: statusColor,
        title: `${statusIcon} Job ${jobData.status === 'success' ? 'Completed' : 'Failed'}: ${jobData.name}`,
        fields: [
          { title: 'Schedule', value: jobData.schedule, short: true },
          { title: 'Duration', value: jobData.duration, short: true },
          { title: 'Exit Code', value: jobData.exitCode.toString(), short: true },
          { title: 'Timestamp', value: new Date(jobData.timestamp).toLocaleString(), short: true }
        ],
        ...(jobData.error && { text: `*Error:* \`${jobData.error}\`` }),
        actions: [
          { type: 'button', text: 'View Logs', url: `https://cronjobmanager.com/jobs/${jobData.id}/logs` },
          { type: 'button', text: 'Retry Job', url: `https://cronjobmanager.com/jobs/${jobData.id}/retry` }
        ],
        ts: Math.floor(Date.now() / 1000)
      }]
    };
  }

  /**
   * Get SMS Message
   */
  getSMSMessage(jobData) {
    const statusIcon = jobData.status === 'success' ? '✅' : '🚨';
    return `${statusIcon} CronJob Alert: ${jobData.name} ${jobData.status === 'success' ? 'completed' : 'failed'}. Duration: ${jobData.duration}. View: https://cronjobmanager.com/jobs/${jobData.id}`;
  }

  // ==================== HELPER METHODS ====================
  
  addToHistory(type, recipient, jobName, status, details) {
    this.notificationHistory.unshift({
      id: Date.now(),
      type,
      recipient,
      jobName,
      status,
      details,
      timestamp: new Date().toISOString()
    });
    
    // Keep only last 1000 entries
    if (this.notificationHistory.length > 1000) {
      this.notificationHistory = this.notificationHistory.slice(0, 1000);
    }
  }

  getRecentFailures(jobName) {
    const last24Hours = Date.now() - (24 * 60 * 60 * 1000);
    return this.notificationHistory.filter(item => 
      item.jobName === jobName && 
      item.status === 'failed' &&
      new Date(item.timestamp).getTime() > last24Hours
    ).length;
  }

  isMaintenanceWindow() {
    // Example: Check if current time is in maintenance window
    // This should be configurable based on user preferences
    return false;
  }

  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  // ==================== API METHODS ====================
  
  getNotificationHistory(limit = 50) {
    return this.notificationHistory.slice(0, limit);
  }

  getNotificationStats() {
    const total = this.notificationHistory.length;
    const sent = this.notificationHistory.filter(item => item.status === 'sent').length;
    const failed = total - sent;
    
    return {
      total,
      sent,
      failed,
      successRate: total > 0 ? (sent / total * 100).toFixed(1) : 0
    };
  }

  addNotificationRule(rule) {
    const newRule = {
      ...rule,
      id: Date.now(),
      enabled: true,
      createdAt: new Date().toISOString()
    };
    
    this.notificationRules.push(newRule);
    return newRule;
  }

  updateNotificationRule(id, updates) {
    const index = this.notificationRules.findIndex(rule => rule.id === id);
    if (index !== -1) {
      this.notificationRules[index] = { ...this.notificationRules[index], ...updates };
      return this.notificationRules[index];
    }
    return null;
  }

  deleteNotificationRule(id) {
    const index = this.notificationRules.findIndex(rule => rule.id === id);
    if (index !== -1) {
      return this.notificationRules.splice(index, 1)[0];
    }
    return null;
  }

  getNotificationRules() {
    return this.notificationRules;
  }
}

module.exports = NotificationService;