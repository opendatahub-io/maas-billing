import express from 'express';
import { logger } from '../utils/logger';
import axios from 'axios';

const router: express.Router = express.Router();

// Simulate chat completions endpoint
router.post('/chat/completions', async (req, res) => {
  // Log ALL requests at the very beginning (before try block)
  logger.info('=== SIMULATOR REQUEST START ===', {
    method: req.method,
    url: req.url,
    headers: req.headers,
    body: req.body,
    bodyType: typeof req.body,
    bodyKeys: req.body ? Object.keys(req.body) : 'no body',
    timestamp: new Date().toISOString(),
    userAgent: req.get('User-Agent')
  });

  try {
    // Log the complete request for debugging
    logger.info('Full simulator request received:', {
      headers: req.headers,
      body: req.body,
      timestamp: new Date().toISOString()
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

    // Get cluster domain from environment
    const CLUSTER_DOMAIN = process.env.CLUSTER_DOMAIN || 'apps.your-cluster.example.com';
    
    // Map model to endpoint URL (these go through Kuadrant gateway)
    const modelEndpoints: { [key: string]: string } = {
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
        timeout: 30000,
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

// Generate mock responses based on input
function generateMockResponse(userMessage: string, model: string, maxTokens: number = 100): string {
  const responses = [
    "I'm a simulated AI assistant! This is a test response from the MaaS platform simulator.",
    "Hello! I'm running on the vLLM simulator. This demonstrates how your Kuadrant policies would handle real requests.",
    "This is a mock response to test rate limiting and authentication. Your request was processed successfully!",
    "I'm here to help! This simulated response shows that your API token and policies are working correctly.",
    "Great question! This is a simulated response from the model. In a real deployment, this would be generated by the actual AI model."
  ];

  let baseResponse = responses[Math.floor(Math.random() * responses.length)];
  
  // Add context about the user's message
  if (userMessage.toLowerCase().includes('code') || userMessage.toLowerCase().includes('programming')) {
    baseResponse += "\n\nFor coding questions, I would normally provide detailed code examples and explanations.";
  } else if (userMessage.toLowerCase().includes('help')) {
    baseResponse += "\n\nI'm here to assist you with any questions you might have!";
  }

  // Add model-specific information
  baseResponse += `\n\n[Simulated by: ${model}]`;
  
  // Truncate if needed based on max_tokens (rough approximation: 1 token ≈ 4 characters)
  const maxChars = maxTokens * 4;
  if (baseResponse.length > maxChars) {
    baseResponse = baseResponse.substring(0, maxChars - 10) + '...';
  }

  return baseResponse;
}

export default router;