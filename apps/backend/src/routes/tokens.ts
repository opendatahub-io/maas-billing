import express from 'express';
import { logger } from '../utils/logger';
import axios from 'axios';

const router: express.Router = express.Router();

// Key Manager API configuration
const CLUSTER_DOMAIN = process.env.CLUSTER_DOMAIN || 'your-cluster.example.com';
const KEY_MANAGER_BASE_URL = process.env.KEY_MANAGER_BASE_URL || `https://key-manager-route-platform-services.${CLUSTER_DOMAIN}`;
const ADMIN_KEY = process.env.KEY_MANAGER_ADMIN_KEY || process.env.ADMIN_KEY;

// Validate required environment variables
if (!ADMIN_KEY) {
  logger.warn('ADMIN_KEY not configured - key manager operations may fail');
}

// Helper function to make authenticated requests to key manager
interface KeyManagerRequestOptions {
  method?: string;
  headers?: Record<string, string>;
  data?: any;
  [key: string]: any;
}

const makeKeyManagerRequest = async (endpoint: string, options: KeyManagerRequestOptions = {}) => {
  const url = `${KEY_MANAGER_BASE_URL}${endpoint}`;
  const headers = {
    'Authorization': `ADMIN ${ADMIN_KEY}`,
    'Content-Type': 'application/json',
    ...options.headers
  };
  
  return axios({
    url,
    method: options.method || 'GET',
    headers,
    data: options.data,
    timeout: 30000,
    httpsAgent: new (require('https').Agent)({
      rejectUnauthorized: false
    }),
    ...options
  });
};

// Simulate Kubernetes API calls for token management
// In a real implementation, these would use the actual Kubernetes API

// Get available teams
router.get('/teams', async (_req, res) => {
  try {
    logger.info('Fetching available teams from key manager...');
    
    const teamsResponse = await makeKeyManagerRequest('/teams');
    const teams = teamsResponse.data?.teams || [];
    
    res.json({
      success: true,
      data: teams,
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    logger.error('Failed to fetch teams from key manager:', error);
    
    res.status(503).json({
      success: false,
      error: 'Key manager service is unavailable',
      details: error.message || 'Unable to connect to key manager service',
      timestamp: new Date().toISOString()
    });
  }
});

// Get user tier/policy information
router.get('/user/tier', async (_req, res) => {
  try {
    logger.info('Fetching user tier information from key manager...');
    
    // Get teams from key manager to determine user's primary team/policy
    const teamsResponse = await makeKeyManagerRequest('/teams');
    const teams = teamsResponse.data?.teams || [];
    
    // For now, get the first team or default team as the user's primary tier
    // In a real implementation, this would be based on authenticated user context
    const primaryTeam = teams.find((team: any) => team.team_id === 'default') || teams[0];
    
    if (!primaryTeam) {
      throw new Error('No teams found for user');
    }

    const userTier = {
      name: primaryTeam.team_name || 'Default User',
      policy: primaryTeam.policy || 'unlimited-policy',
      usage: 0, // Would need to get from usage endpoint
      limit: primaryTeam.token_limit || 100000,
      models: ['vllm-simulator', 'qwen3-0-6b-instruct'], // Would need to get from models endpoint
      team_id: primaryTeam.team_id,
      team_name: primaryTeam.team_name
    };

    res.json({
      success: true,
      data: userTier,
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    logger.error('Failed to fetch user tier from key manager:', error);
    
    res.status(503).json({
      success: false,
      error: 'Key manager service is unavailable',
      details: error.message || 'Unable to connect to key manager service',
      timestamp: new Date().toISOString()
    });
  }
});

// Get user's API tokens
router.get('/', async (_req, res) => {
  try {
    logger.info('Fetching user tokens from key manager...');
    
    // Get all teams first to aggregate user's tokens across teams
    const teamsResponse = await makeKeyManagerRequest('/teams');
    const teams = teamsResponse.data?.teams || [];
    
    const allTokens: any[] = [];
    
    // Get current user ID - in a real implementation, this would come from authentication
    // For now, we'll use a default user or extract from environment
    const currentUserId = process.env.DEFAULT_USER_ID || 'noyitz';
    
    // For each team, get the keys and filter for current user
    for (const team of teams) {
      try {
        const keysResponse = await makeKeyManagerRequest(`/teams/${team.team_id}/keys`);
        const teamKeys = keysResponse.data?.keys || [];
        
        // Filter keys to only include those belonging to the current user
        const userKeys = teamKeys.filter((key: any) => key.user_id === currentUserId);
        
        // Transform key manager format to our expected format
        const transformedKeys = await Promise.all(userKeys.map(async (key: any) => {
          // Try to extract display name from description if it follows our pattern
          let displayName = key.display_name;
          if (!displayName && key.description && key.description.startsWith('Token: ')) {
            displayName = key.description.replace('Token: ', '');
          }
          
          // Fallback to formatted secret name
          if (!displayName) {
            displayName = key.secret_name?.replace(/^apikey-/, '').replace(/-/g, ' ');
          }
          
          // Get the actual API key value from the Kubernetes secret
          let actualApiKey = key.secret_name; // Fallback to secret name
          try {
            const { exec } = require('child_process');
            const { promisify } = require('util');
            const execAsync = promisify(exec);
            
            // Get the secret value using oc command
            const { stdout } = await execAsync(`oc get secret ${key.secret_name} -o jsonpath='{.data.api_key}' 2>/dev/null`);
            if (stdout && stdout.trim()) {
              // Decode the base64 value
              actualApiKey = Buffer.from(stdout.trim(), 'base64').toString('utf-8');
              logger.info(`Successfully retrieved API key value for ${key.secret_name}`);
            } else {
              logger.warn(`No API key data found in secret ${key.secret_name}`);
            }
          } catch (error) {
            logger.warn(`Failed to get API key value for ${key.secret_name}:`, error);
          }
          
          return {
            name: key.secret_name,
            displayName: displayName,
            alias: key.secret_name,
            created: key.created_at,
            lastUsed: null, // Key manager doesn't track last used in this endpoint
            usage: 0, // Would need to get from usage endpoints
            status: key.status || 'active',
            actualApiKey: actualApiKey, // The actual API key value
            team_id: team.team_id,
            team_name: team.team_name,
            policy: key.policy || team.policy,
            secret_name: key.secret_name,
            user_id: key.user_id,
            user_email: key.user_email,
            role: key.role || 'member'
          };
        }));
        
        allTokens.push(...transformedKeys);
      } catch (keyError) {
        logger.warn(`Failed to fetch keys for team ${team.team_id}:`, keyError);
      }
    }

    res.json({
      success: true,
      data: allTokens,
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    logger.error('Failed to fetch tokens from key manager:', error);
    
    res.status(503).json({
      success: false,
      error: 'Key manager service is unavailable',
      details: error.message || 'Unable to connect to key manager service',
      timestamp: new Date().toISOString()
    });
  }
});

// Create a new API token
router.post('/create', async (req, res) => {
  try {
    const { name, description, team_id } = req.body;
    
    if (!name || !name.trim()) {
      return res.status(400).json({
        success: false,
        error: 'Token name is required'
      });
    }

    logger.info('Creating new token via key manager:', { name, description, team_id });
    
    // Use default team if none specified
    const targetTeamId = team_id || 'default';
    
    // Use current authenticated user ID instead of generating from token name
    const currentUserId = process.env.DEFAULT_USER_ID || 'noyitz';
    
    // Create the token via key manager
    const createTokenData = {
      user_id: currentUserId,
      user_email: `${currentUserId}@generated.local`,
      display_name: name, // Preserve original user-chosen name
      description: description || `Token: ${name}`
    };
    
    const response = await makeKeyManagerRequest(`/teams/${targetTeamId}/keys`, {
      method: 'POST',
      data: createTokenData
    });
    
    const tokenData = response.data;
    
    res.status(201).json({
      success: true,
      data: {
        name: tokenData.secret_name || `apikey-${currentUserId}-${targetTeamId}`,
        token: tokenData.api_key || tokenData.secret_name,
        created: new Date().toISOString(),
        team_id: targetTeamId,
        user_id: currentUserId
      },
      message: 'Token created successfully via key manager'
    });
  } catch (error: any) {
    logger.error('Failed to create token via key manager:', error);
    
    // Return error instead of fallback creation
    res.status(503).json({
      success: false,
      error: 'Key manager service is unavailable',
      details: error.message || 'Unable to connect to key manager service',
      timestamp: new Date().toISOString()
    });
  }
});

// Revoke/delete a token
router.delete('/:tokenName', async (req, res) => {
  try {
    const { tokenName } = req.params;
    
    logger.info('Revoking token via key manager:', tokenName);
    
    // Use the correct key manager endpoint: DELETE /keys/:key_name
    await makeKeyManagerRequest(`/keys/${tokenName}`, {
      method: 'DELETE'
    });
    
    res.json({
      success: true,
      message: `Token ${tokenName} has been revoked successfully via key manager`
    });
  } catch (error: any) {
    logger.error('Failed to revoke token via key manager:', error);
    
    // Return error instead of fallback response
    res.status(503).json({
      success: false,
      error: 'Key manager service is unavailable',
      details: error.message || 'Unable to connect to key manager service',
      timestamp: new Date().toISOString()
    });
  }
});

// Test a token by making a request to a model
router.post('/test', async (req, res) => {
  try {
    const { token, model, message } = req.body;
    
    if (!token || !model || !message) {
      return res.status(400).json({
        success: false,
        error: 'Token, model, and message are required'
      });
    }

    logger.info('Testing token:', { 
      token: token.substring(0, 10) + '...', 
      model, 
      message: message.substring(0, 50) + '...' 
    });
    
    // Get the actual model service URL from environment or use default
    const modelServiceUrl = process.env.MODEL_SERVICE_URL || `https://qwen3-0-6b-instruct-llm.apps.${CLUSTER_DOMAIN}`;
    
    // Construct the request
    const testRequest = {
      url: `${modelServiceUrl}/v1/chat/completions`,
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
        'User-Agent': 'MaaS-Backend-TokenTest/1.0'
      },
      body: {
        model: model,
        messages: [
          {
            role: 'user',
            content: message
          }
        ],
        max_tokens: 100,
        temperature: 0.7
      }
    };

    // Make the actual request to test the token
    try {
      const response = await axios({
        method: testRequest.method,
        url: testRequest.url,
        headers: testRequest.headers,
        data: testRequest.body,
        timeout: 30000,
        validateStatus: () => true // Don't throw on HTTP error status codes
      });

      // Return detailed response regardless of success/failure
      const result = {
        success: response.status >= 200 && response.status < 300,
        statusCode: response.status,
        message: response.status >= 200 && response.status < 300 ? 
          'Token authentication successful!' : 
          `Authentication failed (HTTP ${response.status})`,
        request: {
          url: testRequest.url,
          method: testRequest.method,
          headers: {
            ...testRequest.headers,
            'Authorization': `Bearer ${token.substring(0, 12)}...${token.substring(token.length - 6)}`
          },
          body: testRequest.body
        },
        response: {
          status: response.status,
          headers: response.headers,
          body: response.data
        }
      };

      res.status(200).json({
        success: true,
        data: result
      });

    } catch (networkError: any) {
      logger.error('Network error during token test:', networkError);
      
      res.status(200).json({
        success: true,
        data: {
          success: false,
          statusCode: 0,
          message: 'Network error occurred during token test',
          error: networkError.message,
          request: {
            url: testRequest.url,
            method: testRequest.method,
            headers: {
              ...testRequest.headers,
              'Authorization': `Bearer ${token.substring(0, 12)}...${token.substring(token.length - 6)}`
            },
            body: testRequest.body
          },
          response: null
        }
      });
    }

  } catch (error: any) {
    logger.error('Failed to test token:', error);
    res.status(500).json({
      success: false,
      error: 'Failed to test token'
    });
  }
});

export default router;