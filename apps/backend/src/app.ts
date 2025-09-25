import 'dotenv/config';
import express from 'express';
import cors from 'cors';
import { createServer } from 'http';
import { Server as SocketIOServer } from 'socket.io';
import { io as SocketIOClient } from 'socket.io-client';
import { logger } from './utils/logger';
import metricsRoutes from './routes/metrics';
import policiesRoutes from './routes/policies';
import tokensRoutes from './routes/tokens';
import simulatorRoutes from './routes/simulator';

// Function to get groups from configmap for group membership checking
async function getGroupsFromConfigMap(): Promise<string[]> {
  try {
    const { exec } = require('child_process');
    const { promisify } = require('util');
    const execAsync = promisify(exec);
    
    const configMapName = process.env.TIER_GROUP_CONFIGMAP_NAME || (() => { throw new Error('TIER_GROUP_CONFIGMAP_NAME environment variable is required'); })();
    const configMapNamespace = process.env.TIER_GROUP_CONFIGMAP_NAMESPACE || (() => { throw new Error('TIER_GROUP_CONFIGMAP_NAMESPACE environment variable is required'); })();
    
    const { stdout } = await execAsync(`oc get configmap ${configMapName} -n ${configMapNamespace} -o jsonpath='{.data.tiers}' 2>/dev/null`);
    
    if (stdout) {
      // Parse YAML format from the ConfigMap
      const yaml = require('js-yaml');
      const tiers = yaml.load(stdout);
      const groups: string[] = [];
      
      if (Array.isArray(tiers)) {
        tiers.forEach((tier: any) => {
          if (tier.groups && Array.isArray(tier.groups)) {
            groups.push(...tier.groups);
          }
        });
      }
      
      return [...new Set(groups)]; // Remove duplicates
    }
    
    logger.warn(`ConfigMap ${configMapName} not found or empty, falling back to default groups`);
    return ['system:authenticated']; // Minimal fallback
  } catch (error) {
    logger.error('Failed to get groups from configmap:', error);
    return ['system:authenticated']; // Minimal fallback
  }
}

// Function to get tier from MaaS API using user groups
async function getTierFromMaasApi(userGroups: string[]): Promise<string> {
  try {
    const maasApiUrl = process.env.MAAS_API_URL || 'http://localhost:8080';
    const response = await fetch(`${maasApiUrl}/v1/tiers/lookup`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ groups: userGroups })
    });

    if (response.ok) {
      const data = await response.json();
      return data.tier || 'free';
    } else {
      logger.warn(`MaaS API tier lookup failed with status ${response.status}, falling back to 'free'`);
      return 'free';
    }
  } catch (error) {
    logger.error('Failed to get tier from MaaS API:', error);
    return 'free'; // Fallback
  }
}

const app: express.Application = express();
const server = createServer(app);
const io = new SocketIOServer(server, {
  cors: {
    origin: process.env.FRONTEND_URL || (() => { throw new Error('FRONTEND_URL environment variable is required'); })(),
    methods: ['GET', 'POST'],
    credentials: true
  }
});
const PORT = process.env.PORT || (() => { throw new Error('PORT environment variable is required'); })();
const QOS_SERVICE_URL = process.env.QOS_SERVICE_URL || (() => { throw new Error('QOS_SERVICE_URL environment variable is required'); })();

// Middleware
app.use(cors({
  origin: process.env.FRONTEND_URL || (() => { throw new Error('FRONTEND_URL environment variable is required'); })(),
  credentials: true
}));
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Request logging
app.use((req, res, next) => {
  logger.info(`${req.method} ${req.path}`, {
    query: req.query,
    userAgent: req.get('User-Agent')
  });
  next();
});

// Health check
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    service: 'maas-backend'
  });
});

// API routes
app.use('/api/v1/metrics', metricsRoutes);
app.use('/api/v1/policies', policiesRoutes);
app.use('/api/v1/tokens', tokensRoutes);
app.use('/api/v1/simulator', simulatorRoutes);

// QoS proxy endpoints
app.get('/api/v1/qos/metrics', async (req, res) => {
  try {
    const response = await fetch(`${QOS_SERVICE_URL}/metrics`);
    const data = await response.json();
    res.json({ success: true, data });
  } catch (error) {
    logger.error('Failed to fetch QoS metrics:', error);
    res.status(500).json({ success: false, error: 'QoS service unavailable' });
  }
});

app.get('/api/v1/qos/health', async (req, res) => {
  try {
    const response = await fetch(`${QOS_SERVICE_URL}/health`);
    const data = await response.json();
    res.json({ success: true, data });
  } catch (error) {
    logger.error('Failed to fetch QoS health:', error);
    res.status(500).json({ success: false, error: 'QoS service unavailable' });
  }
});

// Models endpoint - dynamically fetch from cluster
app.get('/api/v1/models', async (req, res) => {
  try {
    const { modelService } = await import('./services/modelService');
    const models = await modelService.getModels();
    
    res.json({
      success: true,
      data: models
    });
  } catch (error: any) {
    logger.error('Failed to fetch models:', error);
    res.status(503).json({
      success: false,
      error: 'Unable to fetch models from cluster',
      details: error.message
    });
  }
});

// Cluster status endpoint for authentication dialog
app.get('/api/v1/cluster/status', async (req, res) => {
  try {
    // Try to get the actual authenticated user
    const { exec } = require('child_process');
    const { promisify } = require('util');
    const execAsync = promisify(exec);
    
    let user = 'system:anonymous';
    let connected = true;
    try {
      const { stdout } = await execAsync('oc whoami');
      user = stdout.trim();
      // If oc whoami succeeds, user is authenticated
    } catch (error: any) {
      logger.warn('Could not get authenticated user via oc whoami:', error.message);
      // Check if it's an authentication error
      if (error.message && (error.message.includes('Unauthorized') || error.message.includes('must be logged in'))) {
        connected = false;
        user = 'system:anonymous';
      } else {
        // Other errors (network, etc.) - still set fallback
        user = 'authenticated-user';
      }
    }
    
    res.json({
      success: true,
      data: {
        connected: connected,
        user: user,
        cluster: process.env.CLUSTER_DOMAIN || (() => { throw new Error('CLUSTER_DOMAIN environment variable is required'); })(),
        loginUrl: process.env.REACT_APP_CONSOLE_URL || (() => { throw new Error('REACT_APP_CONSOLE_URL environment variable is required'); })()
      },
      timestamp: new Date().toISOString()
    });
  } catch (error) {
    logger.error('Error getting cluster status:', error);
    res.status(500).json({
      success: false,
      error: 'Could not get cluster status'
    });
  }
});

// User information endpoint for Service Account tokens
app.get('/api/v1/user', async (req, res) => {
  try {
    // For Service Account tokens, we extract user info from the OpenShift context
    const { exec } = require('child_process');
    const { promisify } = require('util');
    const execAsync = promisify(exec);
    
    let username = 'default-user';
    let email = 'default@example.com';
    let tier = 'unknown'; // Will be determined dynamically from ConfigMap
    let groups: string[] = [];
    let cluster = process.env.CLUSTER_DOMAIN || (() => { throw new Error('CLUSTER_DOMAIN environment variable is required'); })();
    
    try {
      // Get the current user
      const { stdout: whoamiOutput } = await execAsync('oc whoami');
      username = whoamiOutput.trim();
      email = `${username}@${cluster}`;
      
      // Get user groups by checking group membership directly from configmap
      const checkGroups = await getGroupsFromConfigMap();
      for (const groupName of checkGroups) {
        try {
          const { stdout } = await execAsync(`oc get group ${groupName} -o jsonpath='{.users[*]}' 2>/dev/null`);
          if (stdout && stdout.includes(username)) {
            groups.push(groupName);
            logger.info(`User ${username} is member of group: ${groupName}`);
          }
        } catch (e) {
          // Group doesn't exist or no access - this is normal
        }
      }
      
      // Also check system:authenticated as it's typically included
      groups.push('system:authenticated');
      logger.info(`Found user groups for ${username}: ${groups.join(', ')}`);
      
      // Dynamic tier determination using MaaS API
      tier = await getTierFromMaasApi(groups);
      logger.info(`Final tier determination for ${username}: ${tier} (groups: ${groups.join(', ')})`);
      
    } catch (error) {
      logger.warn('Could not get authenticated user info:', error);
    }
    
    res.json({
      success: true,
      data: {
        name: username,
        email: email,
        tier: tier,
        groups: groups,
        cluster: cluster,
        isAuthenticated: true
      }
    });
  } catch (error) {
    logger.error('Error getting user info:', error);
    res.status(500).json({
      success: false,
      error: 'Could not get user information'
    });
  }
});

// Error handling
app.use((err: any, req: express.Request, res: express.Response, next: express.NextFunction) => {
  logger.error('Unhandled error:', err);
  res.status(500).json({
    success: false,
    error: 'Internal server error'
  });
});

// 404 handler
app.use('*', (req, res) => {
  res.status(404).json({
    success: false,
    error: 'Route not found'
  });
});

// QoS Service integration
let qosClient: any = null;

function connectToQoSService() {
  try {
    qosClient = SocketIOClient(QOS_SERVICE_URL);
    
    qosClient.on('connect', () => {
      logger.info('Connected to QoS service', { url: QOS_SERVICE_URL });
    });
    
    qosClient.on('disconnect', () => {
      logger.warn('Disconnected from QoS service');
    });
    
    qosClient.on('connect_error', (error: any) => {
      logger.error('QoS service connection error:', error);
    });
    
    // Forward QoS events to frontend clients
    qosClient.on('queue_update', (data: any) => {
      io.emit('qos_queue_update', data);
    });
    
    qosClient.on('queue_stats', (data: any) => {
      io.emit('qos_queue_stats', data);
    });
    
    qosClient.on('request_queued', (data: any) => {
      io.emit('qos_request_queued', data);
    });
    
    qosClient.on('request_completed', (data: any) => {
      io.emit('qos_request_completed', data);
    });
    
  } catch (error) {
    logger.error('Failed to connect to QoS service:', error);
  }
}

// Socket.IO connection handling
io.on('connection', (socket) => {
  logger.info('Frontend client connected', { 
    socketId: socket.id,
    clientIP: socket.handshake.address 
  });
  
  // Handle QoS monitoring requests
  socket.on('subscribe_qos', () => {
    logger.info('Client subscribed to QoS updates', { socketId: socket.id });
    if (qosClient?.connected) {
      qosClient.emit('get_metrics');
      qosClient.emit('get_detailed_stats');
    }
  });
  
  socket.on('unsubscribe_qos', () => {
    logger.info('Client unsubscribed from QoS updates', { socketId: socket.id });
  });
  
  socket.on('disconnect', () => {
    logger.info('Frontend client disconnected', { socketId: socket.id });
  });
});


// Start server
if (require.main === module) {
  server.listen(PORT, () => {
    logger.info(`Server running on port ${PORT}`);
    logger.info('Available endpoints:');
    logger.info('  GET /health - Health check');
    logger.info('  GET /api/v1/models - Available models');
    logger.info('  GET /api/v1/cluster/status - Cluster authentication status');
    logger.info('  GET /api/v1/user - Current user information');
    logger.info('  GET /api/v1/metrics - General metrics');
    logger.info('  GET /api/v1/metrics/live-requests - Live request data with policy enforcement');
    logger.info('  GET /api/v1/metrics/dashboard - Dashboard statistics');
    logger.info('  GET /api/v1/policies - Get all policies');
    logger.info('  POST /api/v1/policies - Create new policy');
    logger.info('  PUT /api/v1/policies/:id - Update policy');
    logger.info('  DELETE /api/v1/policies/:id - Delete policy');
    logger.info('  POST /api/v1/tokens/create - Create new token');
    logger.info('  DELETE /api/v1/tokens/delete - Delete all tokens');
    logger.info('  POST /api/v1/simulator/chat/completions - Simulate requests');
    logger.info('  GET /api/v1/qos/metrics - QoS metrics proxy');
    logger.info('  GET /api/v1/qos/health - QoS health proxy');
    logger.info('Socket.IO server enabled for real-time updates');
    
    // Connect to QoS service after server starts
    connectToQoSService();
  });
}

export default app;