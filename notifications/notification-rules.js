class NotificationRulesEngine {
    constructor() {
      this.rules = [];
      this.jobHistory = [];
      this.maintenanceWindows = [];
      this.escalationPolicies = [];
      this.suppressions = [];
    }
  
    /**
     * Create a new notification rule
     */
    createRule(config) {
      const rule = {
        id: Date.now(),
        name: config.name,
        description: config.description,
        enabled: config.enabled !== false,
        priority: config.priority || 'normal', // low, normal, high, critical
        
        // Trigger conditions
        triggers: {
          events: config.events || ['failure'], // success, failure, warning, timeout, start, complete
          jobPatterns: config.jobPatterns || [], // regex patterns to match job names
          excludePatterns: config.excludePatterns || [],
          schedulePatterns: config.schedulePatterns || [],
          exitCodes: config.exitCodes || [], // specific exit codes to trigger on
          duration: config.duration || {} // min/max duration conditions
        },
        
        // Smart conditions
        conditions: {
          consecutiveFailures: config.consecutiveFailures || null,
          failureRate: config.failureRate || null, // trigger if failure rate exceeds X% in Y minutes
          businessHoursOnly: config.businessHoursOnly || false,
          timeWindows: config.timeWindows || [], // specific time windows
          cooldownPeriod: config.cooldownPeriod || 300, // seconds between notifications for same job
          suppressionRules: config.suppressionRules || []
        },
        
        // Notification channels
        channels: config.channels || [], // array of channel configs
        
        // Escalation
        escalation: config.escalation || {
          enabled: false,
          levels: [],
          timeouts: []
        },
        
        // Metadata
        createdAt: new Date().toISOString(),
        createdBy: config.createdBy || 'system',
        lastModified: new Date().toISOString(),
        
        // Stats
        stats: {
          triggered: 0,
          sent: 0,
          failed: 0,
          suppressed: 0,
          lastTriggered: null
        }
      };
  
      this.rules.push(rule);
      return rule;
    }
  
    /**
     * Process a job event and determine which notifications to send
     */
    async processJobEvent(jobData, eventType) {
      console.log(`🔍 Processing job event: ${jobData.name} - ${eventType}`);
      
      // Add to job history for pattern analysis
      this.addToJobHistory(jobData, eventType);
      
      // Find applicable rules
      const applicableRules = this.findApplicableRules(jobData, eventType);
      
      const results = [];
      
      for (const rule of applicableRules) {
        try {
          // Check if rule should be triggered
          const shouldTrigger = await this.evaluateRule(rule, jobData, eventType);
          
          if (shouldTrigger.trigger) {
            console.log(`✅ Rule "${rule.name}" triggered for job ${jobData.name}`);
            
            // Update rule stats
            rule.stats.triggered++;
            rule.stats.lastTriggered = new Date().toISOString();
            
            // Process notifications for this rule
            const notificationResults = await this.executeRuleNotifications(rule, jobData, eventType);
            
            results.push({
              ruleId: rule.id,
              ruleName: rule.name,
              triggered: true,
              notifications: notificationResults
            });
            
            // Handle escalation if configured
            if (rule.escalation.enabled) {
              this.initiateEscalation(rule, jobData, eventType);
            }
            
          } else {
            console.log(`⏭️ Rule "${rule.name}" skipped: ${shouldTrigger.reason}`);
            
            if (shouldTrigger.reason.includes('suppressed')) {
              rule.stats.suppressed++;
            }
            
            results.push({
              ruleId: rule.id,
              ruleName: rule.name,
              triggered: false,
              reason: shouldTrigger.reason
            });
          }
          
        } catch (error) {
          console.error(`❌ Error processing rule "${rule.name}":`, error);
          
          rule.stats.failed++;
          
          results.push({
            ruleId: rule.id,
            ruleName: rule.name,
            triggered: false,
            error: error.message
          });
        }
      }
      
      return {
        jobId: jobData.id,
        jobName: jobData.name,
        eventType,
        rulesProcessed: applicableRules.length,
        results
      };
    }
  
    /**
     * Find rules that apply to the current job event
     */
    findApplicableRules(jobData, eventType) {
      return this.rules.filter(rule => {
        if (!rule.enabled) return false;
        
        // Check if event type matches
        if (!rule.triggers.events.includes(eventType)) return false;
        
        // Check job name patterns
        if (rule.triggers.jobPatterns.length > 0) {
          const matches = rule.triggers.jobPatterns.some(pattern => {
            const regex = new RegExp(pattern, 'i');
            return regex.test(jobData.name);
          });
          if (!matches) return false;
        }
        
        // Check exclude patterns
        if (rule.triggers.excludePatterns.length > 0) {
          const excluded = rule.triggers.excludePatterns.some(pattern => {
            const regex = new RegExp(pattern, 'i');
            return regex.test(jobData.name);
          });
          if (excluded) return false;
        }
        
        // Check schedule patterns
        if (rule.triggers.schedulePatterns.length > 0 && jobData.schedule) {
          const matches = rule.triggers.schedulePatterns.some(pattern => {
            const regex = new RegExp(pattern, 'i');
            return regex.test(jobData.schedule);
          });
          if (!matches) return false;
        }
        
        // Check exit codes
        if (rule.triggers.exitCodes.length > 0 && jobData.exitCode !== undefined) {
          if (!rule.triggers.exitCodes.includes(jobData.exitCode)) return false;
        }
        
        return true;
      });
    }
  
    /**
     * Evaluate if a rule should trigger
     */
    async evaluateRule(rule, jobData, eventType) {
      // Check consecutive failures
      if (rule.conditions.consecutiveFailures && eventType === 'failure') {
        const consecutiveFailures = this.getConsecutiveFailures(jobData.name);
        if (consecutiveFailures < rule.conditions.consecutiveFailures) {
          return { 
            trigger: false, 
            reason: `Consecutive failures (${consecutiveFailures}) below threshold (${rule.conditions.consecutiveFailures})` 
          };
        }
      }
      
      // Check failure rate
      if (rule.conditions.failureRate) {
        const failureRate = this.calculateFailureRate(
          jobData.name, 
          rule.conditions.failureRate.timeWindow || 3600 // default 1 hour
        );
        
        if (failureRate < rule.conditions.failureRate.threshold) {
          return { 
            trigger: false, 
            reason: `Failure rate (${failureRate.toFixed(1)}%) below threshold (${rule.conditions.failureRate.threshold}%)` 
          };
        }
      }
      
      // Check business hours
      if (rule.conditions.businessHoursOnly) {
        const now = new Date();
        const hour = now.getHours();
        const day = now.getDay(); // 0 = Sunday
        
        const isWeekend = day === 0 || day === 6;
        const isBusinessHour = hour >= 9 && hour <= 17;
        
        if (isWeekend || !isBusinessHour) {
          return { 
            trigger: false, 
            reason: 'Outside business hours' 
          };
        }
      }
      
      // Check time windows
      if (rule.conditions.timeWindows.length > 0) {
        const now = new Date();
        const currentTime = now.getHours() * 60 + now.getMinutes();
        
        const inTimeWindow = rule.conditions.timeWindows.some(window => {
          const start = this.parseTime(window.start);
          const end = this.parseTime(window.end);
          
          if (start <= end) {
            return currentTime >= start && currentTime <= end;
          } else {
            // Crosses midnight
            return currentTime >= start || currentTime <= end;
          }
        });
        
        if (!inTimeWindow) {
          return { 
            trigger: false, 
            reason: 'Outside configured time windows' 
          };
        }
      }
      
      // Check cooldown period
      if (rule.conditions.cooldownPeriod > 0) {
        const lastNotification = this.getLastNotificationTime(rule.id, jobData.name);
        if (lastNotification) {
          const timeSinceLastNotification = (Date.now() - lastNotification.getTime()) / 1000;
          if (timeSinceLastNotification < rule.conditions.cooldownPeriod) {
            return { 
              trigger: false, 
              reason: `Cooldown period active (${Math.round(rule.conditions.cooldownPeriod - timeSinceLastNotification)}s remaining)` 
            };
          }
        }
      }
      
      // Check maintenance windows
      if (this.isInMaintenanceWindow(jobData.name)) {
        return { 
          trigger: false, 
          reason: 'Job in maintenance window' 
        };
      }
      
      // Check suppression rules
      if (this.isSuppressed(rule, jobData, eventType)) {
        return { 
          trigger: false, 
          reason: 'Notification suppressed by active suppression rule' 
        };
      }
      
      // Check duration conditions
      if (rule.triggers.duration && jobData.duration) {
        const durationSeconds = this.parseDuration(jobData.duration);
        
        if (rule.triggers.duration.min && durationSeconds < rule.triggers.duration.min) {
          return { 
            trigger: false, 
            reason: `Job duration (${durationSeconds}s) below minimum (${rule.triggers.duration.min}s)` 
          };
        }
        
        if (rule.triggers.duration.max && durationSeconds > rule.triggers.duration.max) {
          return { 
            trigger: false, 
            reason: `Job duration (${durationSeconds}s) above maximum (${rule.triggers.duration.max}s)` 
          };
        }
      }
      
      return { trigger: true, reason: 'All conditions met' };
    }
  
    /**
     * Execute notifications for a triggered rule
     */
    async executeRuleNotifications(rule, jobData, eventType) {
      const results = [];
      
      for (const channel of rule.channels) {
        try {
          // This would integrate with the NotificationService
          // For now, we'll simulate the notification
          const result = await this.sendNotification(channel, rule, jobData, eventType);
          
          results.push({
            channel: channel.type,
            recipient: channel.recipient || channel.url,
            success: true,
            result
          });
          
          rule.stats.sent++;
          
          // Record notification time for cooldown tracking
          this.recordNotificationTime(rule.id, jobData.name);
          
        } catch (error) {
          console.error(`Failed to send ${channel.type} notification:`, error);
          
          results.push({
            channel: channel.type,
            recipient: channel.recipient || channel.url,
            success: false,
            error: error.message
          });
          
          rule.stats.failed++;
        }
      }
      
      return results;
    }
  
    /**
     * Send notification through specified channel
     */
    async sendNotification(channel, rule, jobData, eventType) {
      // This would integrate with your actual notification services
      // For now, return a mock response
      return {
        channelType: channel.type,
        recipient: channel.recipient,
        timestamp: new Date().toISOString(),
        messageId: `msg_${Date.now()}`
      };
    }
  
    /**
     * Helper methods
     */
    
    addToJobHistory(jobData, eventType) {
      this.jobHistory.unshift({
        jobName: jobData.name,
        eventType,
        status: jobData.status,
        timestamp: new Date().toISOString(),
        duration: jobData.duration,
        exitCode: jobData.exitCode
      });
      
      // Keep only last 10000 entries
      if (this.jobHistory.length > 10000) {
        this.jobHistory = this.jobHistory.slice(0, 10000);
      }
    }
    
    getConsecutiveFailures(jobName) {
      let consecutiveFailures = 0;
      
      for (const entry of this.jobHistory) {
        if (entry.jobName === jobName) {
          if (entry.eventType === 'failure') {
            consecutiveFailures++;
          } else if (entry.eventType === 'success') {
            break;
          }
        }
      }
      
      return consecutiveFailures;
    }
    
    calculateFailureRate(jobName, timeWindowSeconds) {
      const cutoff = new Date(Date.now() - (timeWindowSeconds * 1000));
      
      const recentEntries = this.jobHistory.filter(entry => 
        entry.jobName === jobName && 
        new Date(entry.timestamp) > cutoff
      );
      
      if (recentEntries.length === 0) return 0;
      
      const failures = recentEntries.filter(entry => entry.eventType === 'failure').length;
      return (failures / recentEntries.length) * 100;
    }
    
    parseTime(timeString) {
      const [hours, minutes] = timeString.split(':').map(Number);
      return hours * 60 + minutes;
    }
    
    parseDuration(durationString) {
      // Parse duration string like "00:05:30" to seconds
      const parts = durationString.split(':').map(Number);
      if (parts.length === 3) {
        return parts[0] * 3600 + parts[1] * 60 + parts[2];
      }
      return 0;
    }
    
    getLastNotificationTime(ruleId, jobName) {
      // This would be stored in a database in a real implementation
      // For now, return null to allow all notifications
      return null;
    }
    
    recordNotificationTime(ruleId, jobName) {
      // Record the notification time for cooldown tracking
      // This would be stored in a database in a real implementation
      console.log(`📝 Recorded notification time for rule ${ruleId}, job ${jobName}`);
    }
    
    isInMaintenanceWindow(jobName) {
      const now = new Date();
      
      return this.maintenanceWindows.some(window => {
        if (!window.active) return false;
        
        // Check if job matches maintenance window
        const jobMatches = window.jobPatterns.some(pattern => {
          const regex = new RegExp(pattern, 'i');
          return regex.test(jobName);
        });
        
        if (!jobMatches) return false;
        
        // Check if current time is within maintenance window
        const start = new Date(window.startTime);
        const end = new Date(window.endTime);
        
        return now >= start && now <= end;
      });
    }
    
    isSuppressed(rule, jobData, eventType) {
      const now = new Date();
      
      return this.suppressions.some(suppression => {
        if (!suppression.active) return false;
        if (suppression.endTime && now > new Date(suppression.endTime)) return false;
        
        // Check if suppression applies to this rule
        if (suppression.ruleIds && !suppression.ruleIds.includes(rule.id)) return false;
        
        // Check job patterns
        if (suppression.jobPatterns.length > 0) {
          const matches = suppression.jobPatterns.some(pattern => {
            const regex = new RegExp(pattern, 'i');
            return regex.test(jobData.name);
          });
          if (!matches) return false;
        }
        
        // Check event types
        if (suppression.eventTypes && !suppression.eventTypes.includes(eventType)) return false;
        
        return true;
      });
    }
  
    /**
     * Escalation Management
     */
    
    initiateEscalation(rule, jobData, eventType) {
      if (!rule.escalation.enabled || rule.escalation.levels.length === 0) return;
      
      const escalationId = `${rule.id}-${jobData.name}-${Date.now()}`;
      
      const escalation = {
        id: escalationId,
        ruleId: rule.id,
        jobData,
        eventType,
        currentLevel: 0,
        startTime: new Date().toISOString(),
        acknowledged: false,
        resolved: false,
        history: []
      };
      
      this.escalationPolicies.push(escalation);
      
      // Start first escalation level
      this.processEscalationLevel(escalation, 0);
      
      console.log(`🚨 Escalation initiated: ${escalationId}`);
    }
    
    processEscalationLevel(escalation, level) {
      const rule = this.rules.find(r => r.id === escalation.ruleId);
      if (!rule || level >= rule.escalation.levels.length) return;
      
      const escalationLevel = rule.escalation.levels[level];
      const timeout = rule.escalation.timeouts[level] || 1800; // 30 minutes default
      
      // Send notifications for this escalation level
      escalationLevel.channels.forEach(async (channel) => {
        try {
          await this.sendEscalationNotification(channel, escalation, level);
          
          escalation.history.push({
            level,
            channel: channel.type,
            recipient: channel.recipient,
            status: 'sent',
            timestamp: new Date().toISOString()
          });
        } catch (error) {
          escalation.history.push({
            level,
            channel: channel.type,
            recipient: channel.recipient,
            status: 'failed',
            error: error.message,
            timestamp: new Date().toISOString()
          });
        }
      });
      
      // Schedule next level if not acknowledged
      setTimeout(() => {
        if (!escalation.acknowledged && !escalation.resolved) {
          escalation.currentLevel = level + 1;
          this.processEscalationLevel(escalation, level + 1);
        }
      }, timeout * 1000);
    }
    
    sendEscalationNotification(channel, escalation, level) {
      // Enhanced notification for escalation
      const enhancedJobData = {
        ...escalation.jobData,
        escalationLevel: level + 1,
        escalationId: escalation.id,
        escalationStartTime: escalation.startTime
      };
      
      return this.sendNotification(channel, { id: escalation.ruleId }, enhancedJobData, escalation.eventType);
    }
    
    acknowledgeEscalation(escalationId, acknowledgedBy) {
      const escalation = this.escalationPolicies.find(e => e.id === escalationId);
      if (escalation) {
        escalation.acknowledged = true;
        escalation.acknowledgedBy = acknowledgedBy;
        escalation.acknowledgedAt = new Date().toISOString();
        
        console.log(`✅ Escalation acknowledged: ${escalationId} by ${acknowledgedBy}`);
        return true;
      }
      return false;
    }
    
    resolveEscalation(escalationId, resolvedBy, resolution) {
      const escalation = this.escalationPolicies.find(e => e.id === escalationId);
      if (escalation) {
        escalation.resolved = true;
        escalation.resolvedBy = resolvedBy;
        escalation.resolvedAt = new Date().toISOString();
        escalation.resolution = resolution;
        
        console.log(`🔧 Escalation resolved: ${escalationId} by ${resolvedBy}`);
        return true;
      }
      return false;
    }
  
    /**
     * Maintenance Window Management
     */
    
    createMaintenanceWindow(config) {
      const window = {
        id: Date.now(),
        name: config.name,
        description: config.description,
        startTime: config.startTime,
        endTime: config.endTime,
        jobPatterns: config.jobPatterns || [],
        active: true,
        createdAt: new Date().toISOString(),
        createdBy: config.createdBy || 'system'
      };
      
      this.maintenanceWindows.push(window);
      console.log(`🔧 Maintenance window created: ${window.name}`);
      return window;
    }
    
    updateMaintenanceWindow(id, updates) {
      const index = this.maintenanceWindows.findIndex(w => w.id === id);
      if (index !== -1) {
        this.maintenanceWindows[index] = { ...this.maintenanceWindows[index], ...updates };
        return this.maintenanceWindows[index];
      }
      return null;
    }
    
    deleteMaintenanceWindow(id) {
      const index = this.maintenanceWindows.findIndex(w => w.id === id);
      if (index !== -1) {
        return this.maintenanceWindows.splice(index, 1)[0];
      }
      return null;
    }
  
    /**
     * Suppression Management
     */
    
    createSuppression(config) {
      const suppression = {
        id: Date.now(),
        name: config.name,
        description: config.description,
        jobPatterns: config.jobPatterns || [],
        ruleIds: config.ruleIds || [],
        eventTypes: config.eventTypes || [],
        startTime: config.startTime || new Date().toISOString(),
        endTime: config.endTime,
        active: true,
        createdAt: new Date().toISOString(),
        createdBy: config.createdBy || 'system'
      };
      
      this.suppressions.push(suppression);
      console.log(`🔇 Suppression created: ${suppression.name}`);
      return suppression;
    }
    
    updateSuppression(id, updates) {
      const index = this.suppressions.findIndex(s => s.id === id);
      if (index !== -1) {
        this.suppressions[index] = { ...this.suppressions[index], ...updates };
        return this.suppressions[index];
      }
      return null;
    }
    
    deleteSuppression(id) {
      const index = this.suppressions.findIndex(s => s.id === id);
      if (index !== -1) {
        return this.suppressions.splice(index, 1)[0];
      }
      return null;
    }
  
    /**
     * Rule Management
     */
    
    updateRule(id, updates) {
      const index = this.rules.findIndex(r => r.id === id);
      if (index !== -1) {
        this.rules[index] = { 
          ...this.rules[index], 
          ...updates, 
          lastModified: new Date().toISOString() 
        };
        return this.rules[index];
      }
      return null;
    }
    
    deleteRule(id) {
      const index = this.rules.findIndex(r => r.id === id);
      if (index !== -1) {
        return this.rules.splice(index, 1)[0];
      }
      return null;
    }
    
    toggleRule(id) {
      const rule = this.rules.find(r => r.id === id);
      if (rule) {
        rule.enabled = !rule.enabled;
        rule.lastModified = new Date().toISOString();
        return rule;
      }
      return null;
    }
  
    /**
     * Analytics and Reporting
     */
    
    getRuleStats() {
      const totalRules = this.rules.length;
      const activeRules = this.rules.filter(r => r.enabled).length;
      const totalTriggered = this.rules.reduce((sum, r) => sum + r.stats.triggered, 0);
      const totalSent = this.rules.reduce((sum, r) => sum + r.stats.sent, 0);
      const totalFailed = this.rules.reduce((sum, r) => sum + r.stats.failed, 0);
      const totalSuppressed = this.rules.reduce((sum, r) => sum + r.stats.suppressed, 0);
      
      return {
        totalRules,
        activeRules,
        inactiveRules: totalRules - activeRules,
        totalTriggered,
        totalSent,
        totalFailed,
        totalSuppressed,
        successRate: totalSent > 0 ? ((totalSent / (totalSent + totalFailed)) * 100).toFixed(1) : 0,
        suppressionRate: totalTriggered > 0 ? ((totalSuppressed / totalTriggered) * 100).toFixed(1) : 0
      };
    }
    
    getTopTriggeredRules(limit = 10) {
      return this.rules
        .sort((a, b) => b.stats.triggered - a.stats.triggered)
        .slice(0, limit)
        .map(rule => ({
          id: rule.id,
          name: rule.name,
          triggered: rule.stats.triggered,
          sent: rule.stats.sent,
          failed: rule.stats.failed,
          successRate: rule.stats.sent > 0 ? 
            ((rule.stats.sent / (rule.stats.sent + rule.stats.failed)) * 100).toFixed(1) : 0
        }));
    }
    
    getJobEventStats(timeWindow = 86400) { // 24 hours default
      const cutoff = new Date(Date.now() - (timeWindow * 1000));
      const recentEvents = this.jobHistory.filter(entry => 
        new Date(entry.timestamp) > cutoff
      );
      
      const eventTypes = {};
      const jobStats = {};
      
      recentEvents.forEach(entry => {
        // Count event types
        eventTypes[entry.eventType] = (eventTypes[entry.eventType] || 0) + 1;
        
        // Count per job
        if (!jobStats[entry.jobName]) {
          jobStats[entry.jobName] = { total: 0, success: 0, failure: 0 };
        }
        jobStats[entry.jobName].total++;
        jobStats[entry.jobName][entry.eventType] = (jobStats[entry.jobName][entry.eventType] || 0) + 1;
      });
      
      return {
        timeWindow: timeWindow,
        totalEvents: recentEvents.length,
        eventTypes,
        jobStats: Object.keys(jobStats).map(jobName => ({
          jobName,
          ...jobStats[jobName],
          failureRate: jobStats[jobName].total > 0 ? 
            ((jobStats[jobName].failure || 0) / jobStats[jobName].total * 100).toFixed(1) : 0
        }))
      };
    }
  
    /**
     * Export/Import Configuration
     */
    
    exportConfiguration() {
      return {
        rules: this.rules,
        maintenanceWindows: this.maintenanceWindows,
        suppressions: this.suppressions.filter(s => s.active),
        exportedAt: new Date().toISOString(),
        version: '1.0'
      };
    }
    
    importConfiguration(config) {
      if (config.version !== '1.0') {
        throw new Error('Unsupported configuration version');
      }
      
      // Backup current config
      const backup = this.exportConfiguration();
      
      try {
        // Import rules
        if (config.rules) {
          config.rules.forEach(rule => {
            rule.id = Date.now() + Math.random(); // Generate new ID
            rule.stats = rule.stats || { triggered: 0, sent: 0, failed: 0, suppressed: 0 };
            this.rules.push(rule);
          });
        }
        
        // Import maintenance windows
        if (config.maintenanceWindows) {
          config.maintenanceWindows.forEach(window => {
            window.id = Date.now() + Math.random();
            this.maintenanceWindows.push(window);
          });
        }
        
        // Import suppressions
        if (config.suppressions) {
          config.suppressions.forEach(suppression => {
            suppression.id = Date.now() + Math.random();
            this.suppressions.push(suppression);
          });
        }
        
        console.log('✅ Configuration imported successfully');
        return { success: true, backup };
        
      } catch (error) {
        console.error('❌ Configuration import failed:', error);
        return { success: false, error: error.message, backup };
      }
    }
  
    /**
     * Cleanup and Maintenance
     */
    
    cleanup() {
      const now = Date.now();
      const oneWeekAgo = now - (7 * 24 * 60 * 60 * 1000);
      const oneMonthAgo = now - (30 * 24 * 60 * 60 * 1000);
      
      // Clean old job history (keep last 30 days)
      this.jobHistory = this.jobHistory.filter(entry => 
        new Date(entry.timestamp).getTime() > oneMonthAgo
      );
      
      // Clean completed escalations (keep last 7 days)
      this.escalationPolicies = this.escalationPolicies.filter(escalation => 
        !escalation.resolved || 
        new Date(escalation.resolvedAt).getTime() > oneWeekAgo
      );
      
      // Deactivate expired suppressions
      this.suppressions.forEach(suppression => {
        if (suppression.endTime && new Date(suppression.endTime).getTime() < now) {
          suppression.active = false;
        }
      });
      
      // Deactivate expired maintenance windows
      this.maintenanceWindows.forEach(window => {
        if (new Date(window.endTime).getTime() < now) {
          window.active = false;
        }
      });
      
      console.log('🧹 Notification rules engine cleanup completed');
    }
  
    /**
     * Health Check
     */
    
    healthCheck() {
      return {
        status: 'healthy',
        timestamp: new Date().toISOString(),
        stats: this.getRuleStats(),
        activeMaintenanceWindows: this.maintenanceWindows.filter(w => w.active).length,
        activeSuppressions: this.suppressions.filter(s => s.active).length,
        activeEscalations: this.escalationPolicies.filter(e => !e.resolved).length,
        jobHistorySize: this.jobHistory.length
      };
    }
  
    // Getter methods
    getRules() { return this.rules; }
    getRule(id) { return this.rules.find(r => r.id === id); }
    getMaintenanceWindows() { return this.maintenanceWindows; }
    getSuppressions() { return this.suppressions; }
    getEscalationPolicies() { return this.escalationPolicies; }
    getJobHistory(limit = 100) { return this.jobHistory.slice(0, limit); }
  }
  
  module.exports = NotificationRulesEngine;