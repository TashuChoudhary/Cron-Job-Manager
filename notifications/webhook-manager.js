const axios = require('axios');
const crypto = require('crypto');

class WebhookManager {
  constructor() {
    this.webhooks = [];
    this.deliveryHistory = [];
    this.retryQueue = [];
    this.templates = {
      discord: this.getDiscordTemplate(),
      teams: this.getTeamsTemplate(),
      generic: this.getGenericTemplate()
    };
  }

  /**
   * Add new webhook configuration
   */
  addWebhook(config) {
    const webhook = {
      id: Date.now(),
      name: config.name,
      url: config.url,
      method: config.method || 'POST',
      headers: config.headers || {},
      template: config.template || 'generic',
      events: config.events || ['failure'],
      enabled: true,
      retryConfig: {
        maxRetries: config.maxRetries || 3,
        retryDelay: config.retryDelay || 1000,
        backoffMultiplier: config.backoffMultiplier || 2,
        timeout: config.timeout || 10000
      },
      authentication: config.authentication || null,
      conditions: config.conditions || {},
      createdAt: new Date().toISOString(),
      lastExecuted: null,
      successCount: 0,
      failureCount: 0
    };

    this.webhooks.push(webhook);
    return webhook;
  }

  /**
   * Update webhook configuration
   */
  updateWebhook(id, updates) {
    const index = this.webhooks.findIndex(w => w.id === id);
    if (index !== -1) {
      this.webhooks[index] = { ...this.webhooks[index], ...updates };
      return this.webhooks[index];
    }
    return null;
  }

  /**
   * Delete webhook
   */
  deleteWebhook(id) {
    const index = this.webhooks.findIndex(w => w.id === id);
    if (index !== -1) {
      return this.webhooks.splice(index, 1)[0];
    }
    return null;
  }

  /**
   * Send webhook notification
   */
  async sendWebhook(webhookId, jobData, eventType) {
    const webhook = this.webhooks.find(w => w.id === webhookId);
    if (!webhook || !webhook.enabled) {
      throw new Error('Webhook not found or disabled');
    }

    // Check if webhook should be triggered for this event
    if (!webhook.events.includes(eventType)) {
      return { skipped: true, reason: 'Event not configured for this webhook' };
    }

    // Check conditions
    if (!this.evaluateConditions(webhook.conditions, jobData, eventType)) {
      return { skipped: true, reason: 'Conditions not met' };
    }

    const payload = this.generatePayload(webhook, jobData, eventType);
    const headers = this.prepareHeaders(webhook);

    try {
      const response = await this.executeWebhook(webhook, payload, headers);
      
      // Update webhook stats
      webhook.successCount++;
      webhook.lastExecuted = new Date().toISOString();
      
      // Log delivery
      this.logDelivery(webhookId, jobData.name, 'success', response.status, payload);
      
      return {
        success: true,
        status: response.status,
        response: response.data
      };
    } catch (error) {
      webhook.failureCount++;
      this.logDelivery(webhookId, jobData.name, 'failed', null, payload, error.message);
      
      // Add to retry queue if retries are configured
      if (webhook.retryConfig.maxRetries > 0) {
        this.addToRetryQueue(webhookId, jobData, eventType, payload, headers, 1);
      }
      
      throw error;
    }
  }

  /**
   * Execute webhook with timeout and error handling
   */
  async executeWebhook(webhook, payload, headers) {
    const config = {
      method: webhook.method,
      url: webhook.url,
      headers,
      timeout: webhook.retryConfig.timeout,
      validateStatus: (status) => status < 400 // Consider 2xx and 3xx as success
    };

    // Add payload for non-GET requests
    if (webhook.method.toLowerCase() !== 'get') {
      config.data = payload;
    } else {
      // For GET requests, add data as query parameters
      config.params = payload;
    }

    return axios(config);
  }

  /**
   * Prepare headers including authentication
   */
  prepareHeaders(webhook) {
    let headers = {
      'Content-Type': 'application/json',
      'User-Agent': 'CronJob-Manager-Webhook/1.0',
      ...webhook.headers
    };

    // Add authentication headers
    if (webhook.authentication) {
      switch (webhook.authentication.type) {
        case 'bearer':
          headers.Authorization = `Bearer ${webhook.authentication.token}`;
          break;
        case 'basic':
          const credentials = Buffer.from(
            `${webhook.authentication.username}:${webhook.authentication.password}`
          ).toString('base64');
          headers.Authorization = `Basic ${credentials}`;
          break;
        case 'api_key':
          headers[webhook.authentication.headerName || 'X-API-Key'] = webhook.authentication.apiKey;
          break;
        case 'custom':
          headers = { ...headers, ...webhook.authentication.customHeaders };
          break;
      }
    }

    return headers;
  }

  /**
   * Generate payload based on template
   */
  generatePayload(webhook, jobData, eventType) {
    const template = this.templates[webhook.template] || this.templates.generic;
    return template(jobData, eventType, webhook);
  }

  /**
   * Generic webhook template
   */
  getGenericTemplate() {
    return (jobData, eventType, webhook) => ({
      event: eventType,
      timestamp: new Date().toISOString(),
      webhook: {
        id: webhook.id,
        name: webhook.name
      },
      job: {
        id: jobData.id,
        name: jobData.name,
        status: jobData.status,
        schedule: jobData.schedule,
        duration: jobData.duration,
        exitCode: jobData.exitCode,
        startTime: jobData.startTime,
        endTime: jobData.endTime,
        ...(jobData.error && { error: jobData.error }),
        ...(jobData.output && { output: jobData.output })
      },
      links: {
        dashboard: `https://cronjobmanager.com/dashboard`,
        job: `https://cronjobmanager.com/jobs/${jobData.id}`,
        logs: `https://cronjobmanager.com/jobs/${jobData.id}/logs`
      }
    });
  }

  /**
   * Discord webhook template
   */
  getDiscordTemplate() {
    return (jobData, eventType, webhook) => {
      const color = this.getEventColor(eventType, jobData.status);
      const statusIcon = this.getStatusIcon(jobData.status);
      
      return {
        embeds: [{
          title: `${statusIcon} CronJob ${eventType === 'failure' ? 'Failed' : 'Update'}`,
          description: `**${jobData.name}**`,
          color: color,
          fields: [
            {
              name: '📅 Schedule',
              value: jobData.schedule,
              inline: true
            },
            {
              name: '⏱️ Duration',
              value: jobData.duration,
              inline: true
            },
            {
              name: '🔢 Exit Code',
              value: jobData.exitCode.toString(),
              inline: true
            },
            ...(jobData.error ? [{
              name: '❌ Error',
              value: `\`\`\`${jobData.error.substring(0, 500)}${jobData.error.length > 500 ? '...' : ''}\`\`\``,
              inline: false
            }] : [])
          ],
          timestamp: new Date().toISOString(),
          footer: {
            text: 'CronJob Manager',
            icon_url: 'https://cronjobmanager.com/favicon.ico'
          },
          author: {
            name: 'Job Notification',
            icon_url: 'https://cronjobmanager.com/favicon.ico'
          }
        }],
        components: [{
          type: 1,
          components: [
            {
              type: 2,
              style: 5,
              label: 'View Logs',
              url: `https://cronjobmanager.com/jobs/${jobData.id}/logs`
            },
            {
              type: 2,
              style: 5,
              label: 'Dashboard',
              url: 'https://cronjobmanager.com/dashboard'
            }
          ]
        }]
      };
    };
  }

  /**
   * Microsoft Teams webhook template
   */
  getTeamsTemplate() {
    return (jobData, eventType, webhook) => {
      const color = this.getEventColor(eventType, jobData.status);
      const statusIcon = this.getStatusIcon(jobData.status);
      
      return {
        '@type': 'MessageCard',
        '@context': 'http://schema.org/extensions',
        themeColor: color.toString(16).padStart(6, '0'),
        summary: `CronJob ${eventType}`,
        sections: [{
          activityTitle: `${statusIcon} CronJob ${eventType === 'failure' ? 'Failed' : 'Update'}`,
          activitySubtitle: `Job: ${jobData.name}`,
          facts: [
            { name: 'Schedule', value: jobData.schedule },
            { name: 'Duration', value: jobData.duration },
            { name: 'Exit Code', value: jobData.exitCode.toString() },
            { name: 'Timestamp', value: new Date().toLocaleString() }
          ],
          ...(jobData.error && {
            text: `**Error:** ${jobData.error.substring(0, 500)}${jobData.error.length > 500 ? '...' : ''}`
          })
        }],
        potentialAction: [
          {
            '@type': 'OpenUri',
            name: 'View Logs',
            targets: [{
              os: 'default',
              uri: `https://cronjobmanager.com/jobs/${jobData.id}/logs`
            }]
          },
          {
            '@type': 'OpenUri',
            name: 'Dashboard',
            targets: [{
              os: 'default',
              uri: 'https://cronjobmanager.com/dashboard'
            }]
          }
        ]
      };
    };
  }

  /**
   * Get color based on event type and status
   */
  getEventColor(eventType, status) {
    const colors = {
      success: 0x00FF00,    // Green
      failure: 0xFF0000,    // Red
      warning: 0xFFA500,    // Orange
      info: 0x0099FF        // Blue
    };
    
    if (status === 'success') return colors.success;
    if (status === 'failed') return colors.failure;
    if (eventType === 'warning') return colors.warning;
    return colors.info;
  }

  /**
   * Get status icon
   */
  getStatusIcon(status) {
    const icons = {
      success: '✅',
      failed: '🚨',
      warning: '⚠️',
      info: 'ℹ️'
    };
    return icons[status] || icons.info;
  }

  /**
   * Evaluate webhook conditions
   */
  evaluateConditions(conditions, jobData, eventType) {
    if (!conditions || Object.keys(conditions).length === 0) {
      return true;
    }

    // Check job name patterns
    if (conditions.jobNamePatterns) {
      const matches = conditions.jobNamePatterns.some(pattern => {
        const regex = new RegExp(pattern, 'i');
        return regex.test(jobData.name);
      });
      if (!matches) return false;
    }

    // Check excluded job patterns
    if (conditions.excludeJobPatterns) {
      const excluded = conditions.excludeJobPatterns.some(pattern => {
        const regex = new RegExp(pattern, 'i');
        return regex.test(jobData.name);
      });
      if (excluded) return false;
    }

    // Check consecutive failures
    if (conditions.consecutiveFailures && eventType === 'failure') {
      // This would need to be implemented with job history
      // For now, always pass
    }

    // Check business hours
    if (conditions.businessHoursOnly) {
      const hour = new Date().getHours();
      if (hour < 9 || hour > 17) return false;
    }

    // Check maintenance windows
    if (conditions.maintenanceWindows) {
      const now = new Date();
      const isInMaintenance = conditions.maintenanceWindows.some(window => {
        const start = new Date(window.start);
        const end = new Date(window.end);
        return now >= start && now <= end;
      });
      if (isInMaintenance) return false;
    }

    return true;
  }

  /**
   * Add webhook to retry queue
   */
  addToRetryQueue(webhookId, jobData, eventType, payload, headers, attempt) {
    const webhook = this.webhooks.find(w => w.id === webhookId);
    if (!webhook || attempt > webhook.retryConfig.maxRetries) {
      return;
    }

    const delay = webhook.retryConfig.retryDelay * Math.pow(webhook.retryConfig.backoffMultiplier, attempt - 1);
    
    const retryItem = {
      id: `${webhookId}-${Date.now()}-${attempt}`,
      webhookId,
      jobData,
      eventType,
      payload,
      headers,
      attempt,
      scheduledFor: new Date(Date.now() + delay).toISOString(),
      createdAt: new Date().toISOString()
    };

    this.retryQueue.push(retryItem);
    
    // Schedule retry
    setTimeout(() => this.executeRetry(retryItem.id), delay);
  }

  /**
   * Execute retry from queue
   */
  async executeRetry(retryId) {
    const retryIndex = this.retryQueue.findIndex(r => r.id === retryId);
    if (retryIndex === -1) return;

    const retry = this.retryQueue[retryIndex];
    const webhook = this.webhooks.find(w => w.id === retry.webhookId);
    
    if (!webhook || !webhook.enabled) {
      this.retryQueue.splice(retryIndex, 1);
      return;
    }

    try {
      const response = await this.executeWebhook(webhook, retry.payload, retry.headers);
      
      // Success - remove from queue and update stats
      this.retryQueue.splice(retryIndex, 1);
      webhook.successCount++;
      this.logDelivery(retry.webhookId, retry.jobData.name, 'success', response.status, retry.payload, null, retry.attempt);
      
    } catch (error) {
      // Failed - try again or give up
      this.retryQueue.splice(retryIndex, 1);
      
      if (retry.attempt < webhook.retryConfig.maxRetries) {
        this.addToRetryQueue(retry.webhookId, retry.jobData, retry.eventType, retry.payload, retry.headers, retry.attempt + 1);
      } else {
        // Final failure
        webhook.failureCount++;
        this.logDelivery(retry.webhookId, retry.jobData.name, 'failed_final', null, retry.payload, error.message, retry.attempt);
      }
    }
  }

  /**
   * Log delivery attempt
   */
  logDelivery(webhookId, jobName, status, responseStatus, payload, error = null, attempt = 1) {
    const delivery = {
      id: Date.now(),
      webhookId,
      jobName,
      status,
      responseStatus,
      payload,
      error,
      attempt,
      timestamp: new Date().toISOString()
    };

    this.deliveryHistory.unshift(delivery);
    
    // Keep only last 1000 deliveries
    if (this.deliveryHistory.length > 1000) {
      this.deliveryHistory = this.deliveryHistory.slice(0, 1000);
    }
  }

  /**
   * Test webhook configuration
   */
  async testWebhook(webhookId) {
    const webhook = this.webhooks.find(w => w.id === webhookId);
    if (!webhook) {
      throw new Error('Webhook not found');
    }

    const testJobData = {
      id: 'test-webhook-job',
      name: 'Test Webhook Job',
      status: 'success',
      schedule: '*/5 * * * *',
      duration: '00:01:30',
      exitCode: 0,
      startTime: new Date(Date.now() - 90000).toISOString(),
      endTime: new Date().toISOString(),
      output: 'Test webhook notification sent successfully'
    };

    try {
      const result = await this.sendWebhook(webhookId, testJobData, 'success');
      return { 
        success: true, 
        message: 'Test webhook sent successfully',
        result 
      };
    } catch (error) {
      return { 
        success: false, 
        message: `Test webhook failed: ${error.message}`,
        error: error.message 
      };
    }
  }

  /**
   * Bulk send webhooks for job event
   */
  async sendWebhooksForEvent(jobData, eventType) {
    const applicableWebhooks = this.webhooks.filter(w => 
      w.enabled && w.events.includes(eventType)
    );

    const results = [];

    for (const webhook of applicableWebhooks) {
      try {
        const result = await this.sendWebhook(webhook.id, jobData, eventType);
        results.push({
          webhookId: webhook.id,
          webhookName: webhook.name,
          success: true,
          result
        });
      } catch (error) {
        results.push({
          webhookId: webhook.id,
          webhookName: webhook.name,
          success: false,
          error: error.message
        });
      }
    }

    return results;
  }

  /**
   * Get webhook statistics
   */
  getWebhookStats() {
    const totalDeliveries = this.deliveryHistory.length;
    const successfulDeliveries = this.deliveryHistory.filter(d => d.status === 'success').length;
    const failedDeliveries = totalDeliveries - successfulDeliveries;

    const last24Hours = this.deliveryHistory.filter(d => {
      const deliveryTime = new Date(d.timestamp);
      const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
      return deliveryTime > oneDayAgo;
    }).length;

    return {
      totalWebhooks: this.webhooks.length,
      activeWebhooks: this.webhooks.filter(w => w.enabled).length,
      totalDeliveries,
      successfulDeliveries,
      failedDeliveries,
      successRate: totalDeliveries > 0 ? ((successfulDeliveries / totalDeliveries) * 100).toFixed(1) : 0,
      last24Hours,
      retryQueueSize: this.retryQueue.length
    };
  }

  /**
   * Get delivery history
   */
  getDeliveryHistory(limit = 50, webhookId = null) {
    let history = this.deliveryHistory;
    
    if (webhookId) {
      history = history.filter(d => d.webhookId === webhookId);
    }

    return history.slice(0, limit);
  }

  /**
   * Get all webhooks
   */
  getWebhooks() {
    return this.webhooks;
  }

  /**
   * Get webhook by ID
   */
  getWebhook(id) {
    return this.webhooks.find(w => w.id === id);
  }

  /**
   * Generate webhook signature for verification
   */
  generateSignature(payload, secret) {
    return crypto
      .createHmac('sha256', secret)
      .update(JSON.stringify(payload))
      .digest('hex');
  }

  /**
   * Verify webhook signature
   */
  verifySignature(payload, signature, secret) {
    const expectedSignature = this.generateSignature(payload, secret);
    return crypto.timingSafeEqual(
      Buffer.from(signature, 'hex'),
      Buffer.from(expectedSignature, 'hex')
    );
  }

  /**
   * Clean up old delivery history and retry queue
   */
  cleanup() {
    const now = Date.now();
    const oneWeekAgo = now - (7 * 24 * 60 * 60 * 1000);

    // Clean delivery history older than 1 week
    this.deliveryHistory = this.deliveryHistory.filter(d => 
      new Date(d.timestamp).getTime() > oneWeekAgo
    );

    // Clean expired retry queue items
    this.retryQueue = this.retryQueue.filter(r => 
      new Date(r.scheduledFor).getTime() > now - (24 * 60 * 60 * 1000)
    );

    console.log('🧹 Webhook cleanup completed');
  }
}

module.exports = WebhookManager;