const API_CONFIG = {
    BASE_URL: 'http://localhost:5000/api', 
    TIMEOUT: 30000, 
    RETRY_ATTEMPTS: 3
};

// API Client Class
class APIClient {
    constructor() {
        this.baseURL = API_CONFIG.BASE_URL;
        this.timeout = API_CONFIG.TIMEOUT;
    }

    // Generic request handler
    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json'
            },
            timeout: this.timeout
        };

        const config = { ...defaultOptions, ...options };

        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), this.timeout);

            const response = await fetch(url, {
                ...config,
                signal: controller.signal
            });

            clearTimeout(timeoutId);

            if (!response.ok) {
                throw new Error(`HTTP Error: ${response.status} ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error(`API Error [${endpoint}]:`, error);
            throw error;
        }
    }

    // GET request
    async get(endpoint) {
        return this.request(endpoint, { method: 'GET' });
    }

    // POST request
    async post(endpoint, data) {
        return this.request(endpoint, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    }

    // PUT request
    async put(endpoint, data) {
        return this.request(endpoint, {
            method: 'PUT',
            body: JSON.stringify(data)
        });
    }

    // DELETE request
    async delete(endpoint) {
        return this.request(endpoint, { method: 'DELETE' });
    }
}


const api = new APIClient();




async function getAllJobs() {
    try {
        const response = await api.get('/jobs');
        return response.jobs || [];
    } catch (error) {
        showError('Failed to fetch jobs');
        return [];
    }
}


async function getJobById(jobId) {
    try {
        return await api.get(`/jobs/${jobId}`);
    } catch (error) {
        showError(`Failed to fetch job ${jobId}`);
        return null;
    }
}

async function createJob(jobData) {
    try {
        const response = await api.post('/jobs', jobData);
        showSuccess('Job created successfully!');
        return response;
    } catch (error) {
        showError('Failed to create job');
        throw error;
    }
}


async function updateJob(jobId, jobData) {
    try {
        const response = await api.put(`/jobs/${jobId}`, jobData);
        showSuccess('Job updated successfully!');
        return response;
    } catch (error) {
        showError('Failed to update job');
        throw error;
    }
}


async function deleteJob(jobId) {
    try {
        await api.delete(`/jobs/${jobId}`);
        showSuccess('Job deleted successfully!');
        return true;
    } catch (error) {
        showError('Failed to delete job');
        return false;
    }
}


async function startJob(jobId) {
    try {
        const response = await api.post(`/jobs/${jobId}/start`, {});
        showSuccess('Job started successfully!');
        return response;
    } catch (error) {
        showError('Failed to start job');
        throw error;
    }
}


async function stopJob(jobId) {
    try {
        const response = await api.post(`/jobs/${jobId}/stop`, {});
        showSuccess('Job stopped successfully!');
        return response;
    } catch (error) {
        showError('Failed to stop job');
        throw error;
    }
}


async function getJobHistory(jobId, limit = 50) {
    try {
        return await api.get(`/jobs/${jobId}/history?limit=${limit}`);
    } catch (error) {
        showError('Failed to fetch job history');
        return [];
    }
}


async function getJobLogs(jobId, limit = 100) {
    try {
        return await api.get(`/jobs/${jobId}/logs?limit=${limit}`);
    } catch (error) {
        showError('Failed to fetch job logs');
        return [];
    }
}


async function getAllWorkflows() {
    try {
        const response = await api.get('/workflows');
        return response.workflows || [];
    } catch (error) {
        showError('Failed to fetch workflows');
        return [];
    }
}


async function getWorkflowById(workflowId) {
    try {
        return await api.get(`/workflows/${workflowId}`);
    } catch (error) {
        showError(`Failed to fetch workflow ${workflowId}`);
        return null;
    }
}

async function createWorkflow(workflowData) {
    try {
        const response = await api.post('/workflows', workflowData);
        showSuccess('Workflow created successfully!');
        return response;
    } catch (error) {
        showError('Failed to create workflow');
        throw error;
    }
}


async function updateWorkflow(workflowId, workflowData) {
    try {
        const response = await api.put(`/workflows/${workflowId}`, workflowData);
        showSuccess('Workflow updated successfully!');
        return response;
    } catch (error) {
        showError('Failed to update workflow');
        throw error;
    }
}


async function deleteWorkflow(workflowId) {
    try {
        await api.delete(`/workflows/${workflowId}`);
        showSuccess('Workflow deleted successfully!');
        return true;
    } catch (error) {
        showError('Failed to delete workflow');
        return false;
    }
}


async function executeWorkflow(workflowId) {
    try {
        const response = await api.post(`/workflows/${workflowId}/execute`, {});
        showSuccess('Workflow execution started!');
        return response;
    } catch (error) {
        showError('Failed to execute workflow');
        throw error;
    }
}


async function getWorkflowStatus(executionId) {
    try {
        return await api.get(`/workflows/executions/${executionId}`);
    } catch (error) {
        showError('Failed to fetch workflow status');
        return null;
    }
}


async function getDashboardStats() {
    try {
        return await api.get('/stats/dashboard');
    } catch (error) {
        showError('Failed to fetch dashboard statistics');
        return {
            total_jobs: 0,
            active_jobs: 0,
            success_rate: 0,
            failed_jobs: 0
        };
    }
}


async function getJobStats(period = '24h') {
    try {
        return await api.get(`/stats/jobs?period=${period}`);
    } catch (error) {
        showError('Failed to fetch job statistics');
        return [];
    }
}


async function getPerformanceMetrics() {
    try {
        return await api.get('/stats/performance');
    } catch (error) {
        showError('Failed to fetch performance metrics');
        return [];
    }
}


async function getNotificationSettings() {
    try {
        return await api.get('/notifications/settings');
    } catch (error) {
        showError('Failed to fetch notification settings');
        return {};
    }
}


async function updateNotificationSettings(settings) {
    try {
        const response = await api.put('/notifications/settings', settings);
        showSuccess('Notification settings updated!');
        return response;
    } catch (error) {
        showError('Failed to update notification settings');
        throw error;
    }
}


async function testNotification(channel, config) {
    try {
        await api.post('/notifications/test', { channel, config });
        showSuccess(`Test notification sent to ${channel}!`);
        return true;
    } catch (error) {
        showError(`Failed to send test notification to ${channel}`);
        return false;
    }
}


class WebSocketClient {
    constructor() {
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 3000;
        this.listeners = {};
    }

    connect() {
        const wsURL = API_CONFIG.BASE_URL.replace('http', 'ws') + '/ws';
        
        try {
            this.ws = new WebSocket(wsURL);

            this.ws.onopen = () => {
                console.log('✅ WebSocket connected');
                this.reconnectAttempts = 0;
                this.updateConnectionStatus(true);
            };

            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleMessage(data);
                } catch (error) {
                    console.error('WebSocket message parse error:', error);
                }
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.updateConnectionStatus(false);
            };

            this.ws.onclose = () => {
                console.log('WebSocket disconnected');
                this.updateConnectionStatus(false);
                this.attemptReconnect();
            };
        } catch (error) {
            console.error('WebSocket connection error:', error);
            this.updateConnectionStatus(false);
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }

    attemptReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            console.log(`Reconnecting... Attempt ${this.reconnectAttempts}`);
            setTimeout(() => this.connect(), this.reconnectDelay);
        } else {
            console.error('Max reconnection attempts reached');
            showError('Lost connection to server. Please refresh the page.');
        }
    }

    handleMessage(data) {
        const { type, payload } = data;
        
        // Emit to registered listeners
        if (this.listeners[type]) {
            this.listeners[type].forEach(callback => callback(payload));
        }

        
        switch(type) {
            case 'job_started':
                console.log('Job started:', payload);
                break;
            case 'job_completed':
                console.log('Job completed:', payload);
                break;
            case 'job_failed':
                console.log('Job failed:', payload);
                showError(`Job "${payload.name}" failed`);
                break;
            case 'log_entry':
                console.log('New log:', payload);
                break;
            default:
                console.log('Unknown message type:', type);
        }
    }

    on(eventType, callback) {
        if (!this.listeners[eventType]) {
            this.listeners[eventType] = [];
        }
        this.listeners[eventType].push(callback);
    }

    off(eventType, callback) {
        if (this.listeners[eventType]) {
            this.listeners[eventType] = this.listeners[eventType].filter(cb => cb !== callback);
        }
    }

    send(type, payload) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type, payload }));
        } else {
            console.error('WebSocket is not connected');
        }
    }

    updateConnectionStatus(connected) {
        const statusElement = document.getElementById('connection-status');
        if (statusElement) {
            if (connected) {
                statusElement.className = 'connection-status status-connected';
                statusElement.innerHTML = '<div class="pulse-dot"></div><span>Connected</span>';
            } else {
                statusElement.className = 'connection-status status-disconnected';
                statusElement.innerHTML = '<div class="pulse-dot"></div><span>Disconnected</span>';
            }
        }
    }
}


const wsClient = new WebSocketClient();


function showSuccess(message) {
    showNotification(message, 'success');
}

function showError(message) {
    showNotification(message, 'error');
}


function showInfo(message) {
    showNotification(message, 'info');
}


function showNotification(message, type = 'info') {
   
    let container = document.getElementById('notification-container');
    
    if (!container) {
        container = document.createElement('div');
        container.id = 'notification-container';
        container.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 9999;
            display: flex;
            flex-direction: column;
            gap: 10px;
        `;
        document.body.appendChild(container);
    }

    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    
    const colors = {
        success: '#10b981',
        error: '#ef4444',
        info: '#3b82f6',
        warning: '#f59e0b'
    };

    const icons = {
        success: '✓',
        error: '✕',
        info: 'ℹ',
        warning: '⚠'
    };

    notification.style.cssText = `
        background: #1e293b;
        color: #e2e8f0;
        padding: 1rem 1.5rem;
        border-radius: 8px;
        border-left: 4px solid ${colors[type]};
        box-shadow: 0 5px 20px rgba(0,0,0,0.3);
        display: flex;
        align-items: center;
        gap: 1rem;
        min-width: 300px;
        animation: slideIn 0.3s ease-out;
    `;

    notification.innerHTML = `
        <span style="font-size: 1.5rem;">${icons[type]}</span>
        <span style="flex: 1;">${message}</span>
        <button onclick="this.parentElement.remove()" style="
            background: none;
            border: none;
            color: #94a3b8;
            cursor: pointer;
            font-size: 1.2rem;
            padding: 0;
            line-height: 1;
        ">×</button>
    `;

    container.appendChild(notification);

   
    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease-out';
        setTimeout(() => notification.remove(), 300);
    }, 5000);
}


const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from {
            opacity: 0;
            transform: translateX(100px);
        }
        to {
            opacity: 1;
            transform: translateX(0);
        }
    }
    
    @keyframes slideOut {
        from {
            opacity: 1;
            transform: translateX(0);
        }
        to {
            opacity: 0;
            transform: translateX(100px);
        }
    }
`;
document.head.appendChild(style);


window.addEventListener('DOMContentLoaded', () => {
    console.log('🚀 API Client initialized');
    console.log('Base URL:', API_CONFIG.BASE_URL);
    
    // Connect to WebSocket for real-time updates
    wsClient.connect();
});

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    wsClient.disconnect();
});

// Export for use in other files
window.API = {
    
    getAllJobs,
    getJobById,
    createJob,
    updateJob,
    deleteJob,
    startJob,
    stopJob,
    getJobHistory,
    getJobLogs,
    
    // Workflow Management
    getAllWorkflows,
    getWorkflowById,
    createWorkflow,
    updateWorkflow,
    deleteWorkflow,
    executeWorkflow,
    getWorkflowStatus,
    
    // Statistics
    getDashboardStats,
    getJobStats,
    getPerformanceMetrics,
    
    // Notifications
    getNotificationSettings,
    updateNotificationSettings,
    testNotification,
    
    // WebSocket
    ws: wsClient,
    
    // Utilities
    showSuccess,
    showError,
    showInfo
};