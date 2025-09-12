import express from 'express';
import { logger } from '../utils/logger';
import axios from 'axios';

const router: express.Router = express.Router();


// Simulate chat completions endpoint
router.post('/chat/completions', async (req, res) => {
  const startTime = Date.now();
  
  try {
    // Log request with minimal sensitive data
    logger.info('Simulator request received:', {
      model: req.body?.model,
      messageCount: req.body?.messages?.length,
      hasAuth: !!req.headers.authorization,
      userAgent: req.get('User-Agent')
    });

    const { model, messages, max_tokens, tier } = req.body;
    
    // Extract authorization from headers
    const authHeader = req.headers.authorization;
    const apiKey = authHeader?.replace(/^(Bearer|APIKEY)\s+/i, '');
    
    if (!apiKey) {
      logger.error('Simulator validation failed: No API key', {
        authHeader,
        headers: req.headers
      });
      return res.status(401).json({
        success: false,
        error: 'Authorization header is required',
        details: 'Please provide a valid API key'
      });
    }

    if (!model || !messages || !Array.isArray(messages)) {
      logger.error('Simulator validation failed: Missing required parameters', {
        model,
        messages,
        messagesType: typeof messages,
        messagesIsArray: Array.isArray(messages),
        body: req.body
      });
      return res.status(400).json({
        success: false,
        error: 'Missing required parameters',
        details: 'model and messages are required'
      });
    }

    logger.info('Simulator request:', {
      model,
      tier,
      messageCount: messages.length,
      maxTokens: max_tokens,
      apiKey: apiKey.substring(0, 8) + '...'
    });

    // Configuration constants
    const CLUSTER_DOMAIN = process.env.CLUSTER_DOMAIN || 'apps.your-cluster.example.com';
    const REQUEST_TIMEOUT = 30000;
    
    // Map model to endpoint URL (these go through Kuadrant gateway)
    const modelEndpoints: Record<string, string> = {
      'qwen3-0-6b-instruct': `http://qwen3-llm.${CLUSTER_DOMAIN}/v1/chat/completions`,
      'vllm-simulator': `http://simulator-llm.${CLUSTER_DOMAIN}/v1/chat/completions`,
      // Add more models as needed
    };

    const targetEndpoint = modelEndpoints[model] || modelEndpoints['qwen3-0-6b-instruct'];
    
    logger.info('Proxying request to Kuadrant endpoint:', {
      endpoint: targetEndpoint,
      model,
      apiKey: apiKey.substring(0, 8) + '...'
    });

    try {
      // Forward the request to the actual Kuadrant-enabled model endpoint
      const kuadrantResponse = await axios({
        method: 'POST',
        url: targetEndpoint,
        headers: {
          'Authorization': req.headers.authorization,
          'Content-Type': 'application/json',
          'User-Agent': 'MaaS-Backend-Simulator/1.0'
        },
        data: {
          model,
          messages,
          max_tokens: max_tokens || 100,
          temperature: 0.7
        },
        timeout: REQUEST_TIMEOUT,
        httpsAgent: new (require('https').Agent)({
          rejectUnauthorized: false
        }),
        validateStatus: () => true // Don't throw on HTTP error status codes
      });

      // Log the response for debugging
      logger.info('Kuadrant response:', {
        status: kuadrantResponse.status,
        statusText: kuadrantResponse.statusText,
        hasData: !!kuadrantResponse.data
      });

      // If rate limited or other error, return that status
      if (kuadrantResponse.status !== 200) {
        return res.status(kuadrantResponse.status).json({
          success: false,
          error: `Kuadrant returned ${kuadrantResponse.status}: ${kuadrantResponse.statusText}`,
          details: kuadrantResponse.data,
          kuadrant_status: kuadrantResponse.status,
          rate_limited: kuadrantResponse.status === 429
        });
      }

      // Log performance metrics
      const duration = Date.now() - startTime;
      logger.info('Simulator request completed successfully:', {
        model,
        duration: `${duration}ms`,
        status: kuadrantResponse.status
      });

      // Forward the successful response from Kuadrant
      res.json(kuadrantResponse.data);

    } catch (error: any) {
      logger.error('Error proxying to Kuadrant:', error);
      
      // Return error without fallback - all requests must go through Kuadrant
      res.status(503).json({
        success: false,
        error: 'Kuadrant endpoint unavailable',
        details: error.message,
        kuadrant_required: true
      });
    }

  } catch (error: any) {
    logger.error('Simulator error:', error);
    res.status(500).json({
      success: false,
      error: 'Simulation failed',
      details: error.message
    });
  }
});


export default router;