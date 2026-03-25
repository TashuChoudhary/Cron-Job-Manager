const twilio = require('twilio');

class SMSService {
  constructor() {
    this.client = null;
    this.config = {
      accountSid: null,
      authToken: null,
      fromNumber: null,
      rateLimits: {
        maxPerHour: 100,
        maxPerDay: 1000,
        cooldownMinutes: 5
      },
      emergencyContacts: []
    };
    this.sentMessages = [];
  }

  /**
   * Initialize Twilio Client
   */
  initialize(accountSid, authToken, fromNumber) {
    try {
      this.client = twilio(accountSid, authToken);
      this.config.accountSid = accountSid;
      this.config.authToken = authToken;
      this.config.fromNumber = fromNumber;
      
      console.log('✅ Twilio SMS service initialized successfully');
      return { success: true, message: 'SMS service configured' };
    } catch (error) {
      console.error('❌ Twilio initialization failed:', error);
      return { success: false, message: `SMS configuration failed: ${error.message}` };
    }
  }

  /**
   * Send SMS with rate limiting and error handling
   */
  async sendSMS(phoneNumber, message, priority = 'normal') {
    if (!this.client) {
      throw new Error('SMS service not configured');
    }

    // Check rate limits
    if (!this.checkRateLimit(phoneNumber, priority)) {
      throw new Error('SMS rate limit exceeded');
    }

    // Validate phone number format
    const formattedNumber = this.formatPhoneNumber(phoneNumber);
    if (!formattedNumber) {
      throw new Error('Invalid phone number format');
    }

    try {
      const messageData = {
        body: message,
        from: this.config.fromNumber,
        to: formattedNumber
      };

      const result = await this.client.messages.create(messageData);
      
      // Log successful send
      this.logMessage(formattedNumber, message, 'sent', result.sid, priority);
      
      return {
        success: true,
        messageSid: result.sid,
        status: result.status,
        recipient: formattedNumber
      };
    } catch (error) {
      // Log failed send
      this.logMessage(formattedNumber, message, 'failed', null, priority, error.message);
      
      throw new Error(`SMS sending failed: ${error.message}`);
    }
  }

  /**
   * Send Critical Job Failure SMS with Escalation
   */
  async sendCriticalAlert(jobData, emergencyContacts) {
    const message = this.formatCriticalMessage(jobData);
    const results = [];

    for (const contact of emergencyContacts) {
      try {
        const result = await this.sendSMS(contact.phone, message, 'critical');
        results.push({
          contact: contact.name,
          phone: contact.phone,
          success: true,
          result
        });
      } catch (error) {
        results.push({
          contact: contact.name,
          phone: contact.phone,
          success: false,
          error: error.message
        });
      }
    }

    return results;
  }

  /**
   * Send SMS Digest (Multiple job statuses)
   */
  async sendDigest(phoneNumber, jobSummaries) {
    if (!jobSummaries || jobSummaries.length === 0) {
      return { success: false, message: 'No jobs to report' };
    }

    const message = this.formatDigestMessage(jobSummaries);
    
    try {
      const result = await this.sendSMS(phoneNumber, message, 'normal');
      return { success: true, result };
    } catch (error) {
      return { success: false, error: error.message };
    }
  }

  /**
   * Test SMS Configuration
   */
  async testSMS(phoneNumber) {
    const testMessage = `🤖 CronJob Manager Test SMS\n\nYour SMS notifications are working correctly!\n\nTime: ${new Date().toLocaleString()}\n\nThis is a test message.`;
    
    try {
      const result = await this.sendSMS(phoneNumber, testMessage, 'test');
      return { 
        success: true, 
        message: 'Test SMS sent successfully',
        messageSid: result.messageSid 
      };
    } catch (error) {
      return { 
        success: false, 
        message: `Test SMS failed: ${error.message}` 
      };
    }
  }

  /**
   * Format phone number to E.164 format
   */
  formatPhoneNumber(phoneNumber) {
    // Remove all non-digit characters
    const cleaned = phoneNumber.replace(/\D/g, '');
    
    // Handle US numbers
    if (cleaned.length === 10) {
      return `+1${cleaned}`;
    }
    
    // Handle international numbers
    if (cleaned.length > 10 && cleaned.startsWith('1')) {
      return `+${cleaned}`;
    }
    
    if (cleaned.length > 10) {
      return `+${cleaned}`;
    }
    
    return null; // Invalid format
  }

  /**
   * Check rate limiting
   */
  checkRateLimit(phoneNumber, priority) {
    const now = new Date();
    const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);
    const oneDayAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000);
    
    // Critical messages bypass most rate limits
    if (priority === 'critical') {
      // Only check for spam prevention (max 5 critical per hour per number)
      const recentCritical = this.sentMessages.filter(msg => 
        msg.phoneNumber === phoneNumber &&
        msg.priority === 'critical' &&
        new Date(msg.timestamp) > oneHourAgo
      );
      return recentCritical.length < 5;
    }

    // Regular rate limit checks
    const recentHourly = this.sentMessages.filter(msg => 
      msg.phoneNumber === phoneNumber &&
      new Date(msg.timestamp) > oneHourAgo
    );
    
    const recentDaily = this.sentMessages.filter(msg => 
      msg.phoneNumber === phoneNumber &&
      new Date(msg.timestamp) > oneDayAgo
    );

    return recentHourly.length < this.config.rateLimits.maxPerHour &&
           recentDaily.length < this.config.rateLimits.maxPerDay;
  }

  /**
   * Log sent message
   */
  logMessage(phoneNumber, message, status, messageSid, priority, error = null) {
    this.sentMessages.push({
      phoneNumber,
      message,
      status,
      messageSid,
      priority,
      error,
      timestamp: new Date().toISOString()
    });

    // Keep only last 1000 messages
    if (this.sentMessages.length > 1000) {
      this.sentMessages = this.sentMessages.slice(-1000);
    }
  }

  /**
   * Format critical job failure message
   */
  formatCriticalMessage(jobData) {
    return `🚨 CRITICAL JOB FAILURE 🚨

Job: ${jobData.name}
Time: ${new Date(jobData.timestamp).toLocaleString()}
Duration: ${jobData.duration}
Exit Code: ${jobData.exitCode}

${jobData.error ? `Error: ${jobData.error.substring(0, 100)}...` : ''}

View details: https://cronjobmanager.com/jobs/${jobData.id}

Reply STOP to opt out.`;
  }

  /**
   * Format job digest message
   */
  formatDigestMessage(jobSummaries) {
    const failed = jobSummaries.filter(job => job.status === 'failed').length;
    const successful = jobSummaries.filter(job => job.status === 'success').length;
    
    let message = `📊 CronJob Daily Digest\n\n`;
    message += `✅ ${successful} successful\n`;
    message += `❌ ${failed} failed\n\n`;
    
    if (failed > 0) {
      message += `Failed Jobs:\n`;
      jobSummaries
        .filter(job => job.status === 'failed')
        .slice(0, 3) // Show max 3 failed jobs
        .forEach(job => {
          message += `• ${job.name}\n`;
        });
      
      if (failed > 3) {
        message += `• +${failed - 3} more\n`;
      }
    }
    
    message += `\nView dashboard: https://cronjobmanager.com`;
    
    return message;
  }

  /**
   * Get SMS statistics
   */
  getStats() {
    const total = this.sentMessages.length;
    const sent = this.sentMessages.filter(msg => msg.status === 'sent').length;
    const failed = total - sent;
    
    const last24Hours = this.sentMessages.filter(msg => {
      const msgTime = new Date(msg.timestamp);
      const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
      return msgTime > oneDayAgo;
    }).length;

    return {
      total,
      sent,
      failed,
      successRate: total > 0 ? ((sent / total) * 100).toFixed(1) : 0,
      last24Hours,
      rateLimitsActive: this.config.rateLimits
    };
  }

  /**
   * Get message history
   */
  getMessageHistory(limit = 50) {
    return this.sentMessages
      .sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp))
      .slice(0, limit);
  }

  /**
   * Update rate limits configuration
   */
  updateRateLimits(rateLimits) {
    this.config.rateLimits = { ...this.config.rateLimits, ...rateLimits };
    return this.config.rateLimits;
  }

  /**
   * Add/Update emergency contact
   */
  addEmergencyContact(name, phone, priority = 1) {
    const formattedPhone = this.formatPhoneNumber(phone);
    if (!formattedPhone) {
      throw new Error('Invalid phone number format');
    }

    const contact = {
      id: Date.now(),
      name,
      phone: formattedPhone,
      priority,
      active: true,
      addedAt: new Date().toISOString()
    };

    this.config.emergencyContacts.push(contact);
    return contact;
  }

  /**
   * Remove emergency contact
   */
  removeEmergencyContact(contactId) {
    const index = this.config.emergencyContacts.findIndex(c => c.id === contactId);
    if (index !== -1) {
      return this.config.emergencyContacts.splice(index, 1)[0];
    }
    return null;
  }

  /**
   * Get emergency contacts sorted by priority
   */
  getEmergencyContacts() {
    return this.config.emergencyContacts
      .filter(contact => contact.active)
      .sort((a, b) => a.priority - b.priority);
  }

  /**
   * Handle SMS opt-out requests (webhook from Twilio)
   */
  handleOptOut(phoneNumber) {
    const formattedNumber = this.formatPhoneNumber(phoneNumber);
    
    // Add to opt-out list (implement your storage)
    console.log(`📱 SMS opt-out request from ${formattedNumber}`);
    
    // Remove from emergency contacts if exists
    this.config.emergencyContacts = this.config.emergencyContacts.filter(
      contact => contact.phone !== formattedNumber
    );
    
    return { success: true, message: 'Phone number opted out successfully' };
  }

  /**
   * Validate Twilio webhook signature (for security)
   */
  validateWebhookSignature(signature, url, params, authToken) {
    return twilio.validateRequest(authToken, signature, url, params);
  }

  /**
   * Check Twilio account balance
   */
  async checkAccountBalance() {
    if (!this.client) {
      throw new Error('SMS service not configured');
    }

    try {
      const account = await this.client.api.accounts(this.config.accountSid).fetch();
      return {
        accountSid: account.sid,
        friendlyName: account.friendlyName,
        status: account.status,
        balance: account.balance,
        currency: account.currency || 'USD'
      };
    } catch (error) {
      throw new Error(`Failed to check account balance: ${error.message}`);
    }
  }

  /**
   * Get phone number info
   */
  async getPhoneNumberInfo() {
    if (!this.client || !this.config.fromNumber) {
      throw new Error('SMS service not configured');
    }

    try {
      const phoneNumbers = await this.client.incomingPhoneNumbers.list();
      const currentNumber = phoneNumbers.find(num => num.phoneNumber === this.config.fromNumber);
      
      return {
        phoneNumber: this.config.fromNumber,
        friendlyName: currentNumber?.friendlyName,
        capabilities: currentNumber?.capabilities,
        status: currentNumber ? 'active' : 'not_found'
      };
    } catch (error) {
      throw new Error(`Failed to get phone number info: ${error.message}`);
    }
  }
}

module.exports = SMSService;