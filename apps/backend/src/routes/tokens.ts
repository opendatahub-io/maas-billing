import express from 'express';
import { logger } from '../utils/logger';
import axios from 'axios';

const router: express.Router = express.Router();

// MaaS API Configuration - Required environment variables
const MAAS_API_URL = process.env.MAAS_API_URL || (() => { throw new Error('MAAS_API_URL environment variable is required'); })();

logger.info(`Token service using MaaS API: ${MAAS_API_URL}`);

// Helper to get OpenShift token for authentication
const getOpenShiftToken = async (): Promise<string> => {
  try {
    const { exec } = require('child_process');
    const { promisify } = require('util');
    const execAsync = promisify(exec);
    
    const { stdout } = await execAsync('oc whoami -t');
    return stdout.trim();
  } catch (error) {
    logger.error('Failed to get OpenShift token:', error);
    throw new Error('OpenShift authentication required');
  }
};

// Helper function to make requests to MaaS API
const makeMaasApiRequest = async (endpoint: string, options: any = {}) => {
  const url = `${MAAS_API_URL}${endpoint}`;
  const osToken = await getOpenShiftToken();
  
  const headers = {
    'Authorization': `Bearer ${osToken}`,
    'Content-Type': 'application/json',
    ...options.headers
  };
  
  return axios({
    url,
    method: options.method || 'GET',
    headers,
    data: options.data,
    timeout: 30000,
    ...options
  });
};





// Create a new API token
router.post('/create', async (req, res) => {
  try {
    const { ttl } = req.body;

    logger.info('Creating Service Account token via MaaS API:', { ttl });
    
    // For Service Account tokens, we use TTL
    // If no TTL provided, let MaaS API use its default (4h)
    const requestData = ttl ? { ttl: ttl } : {};
    
    const response = await makeMaasApiRequest('/v1/tokens', {
      method: 'POST',
      data: requestData
    });
    
    const tokenData = response.data;
    
    res.status(201).json({
      success: true,
      data: tokenData, // Return pure MaaS API response
      message: `Service Account token created successfully (TTL: ${tokenData.ttl})`
    });
  } catch (error: any) {
    logger.error('Failed to create Service Account token via MaaS API:', error);
    
    res.status(500).json({
      success: false,
      error: 'Failed to create Service Account token',
      details: error.response?.data || error.message,
      timestamp: new Date().toISOString()
    });
  }
});

// Delete all tokens for current user
router.delete('/delete', async (req, res) => {
  try {
    logger.info('Deleting all tokens via MaaS API');
    
    // Call MaaS API to delete all tokens
    await makeMaasApiRequest('/v1/tokens', {
      method: 'DELETE'
    });
    
    logger.info('Tokens deleted successfully via MaaS API');
    res.json({
      success: true,
      message: 'All tokens revoked successfully',
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    logger.error('Failed to delete tokens via MaaS API:', error);
    
    if (error.response?.status === 401) {
      res.status(401).json({
        success: false,
        error: 'Authentication failed',
        details: 'Unable to authenticate with MaaS API',
        timestamp: new Date().toISOString()
      });
    } else {
      res.status(503).json({
        success: false,
        error: 'MaaS API service is unavailable',
        details: error.message || 'Unable to connect to MaaS API service',
        timestamp: new Date().toISOString()
      });
    }
  }
});

export default router;