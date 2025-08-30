import express from 'express';
import { request } from 'undici';
import { logger } from '../utils/logger';
import metricsService from '../services/metricsService';

const router: express.Router = express.Router();

// Simulator proxy endpoint
router.post('/chat/completions', async (req, res) => {
  const startTime = Date.now(); // Track request start time
  try {
    const { model, messages, max_tokens, tier } = req.body;
    
    // Get authorization header from request
    const authHeader = req.headers.authorization;
    if (!authHeader) {
      return res.status(401).json({
        success: false,
        error: 'Authorization header required'
      });
    }

    // Determine model and host based on request
    const modelKey = model?.includes('qwen') ? 'qwen3' : 'simulator';
    const hostHeader = modelKey === 'qwen3' ? 'qwen3.maas.local' : 'simulator.maas.local';
    const actualModel = modelKey === 'qwen3' ? 'qwen3-0-6b-instruct' : 'simulator-model';
    
    const requestBody = {
      model: actualModel,
      messages,
      max_tokens
    };

    const requestHeaders = {
      'Authorization': authHeader,
      'Content-Type': 'application/json',
      'Host': hostHeader
    };

    // Connect to the working Kuadrant-protected services
    const baseUrl = modelKey === 'qwen3' 
      ? (process.env.QWEN3_URL || 'http://qwen3-llm.apps.summit-gpu.octo-emerging.redhataicoe.com')
      : (process.env.SIMULATOR_URL || 'http://simulator-llm.apps.summit-gpu.octo-emerging.redhataicoe.com');
    const targetUrl = `${baseUrl}/v1/chat/completions`;

    logger.info('Proxying request to simulator', {
      targetUrl,
      hostHeader,
      model: actualModel,
      tier
    });

    // Use undici to make request to simulator
    const requestOptions = {
      method: 'POST',
      headers: {
        'Authorization': authHeader,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(requestBody)
    };

    logger.info('Making request with Host header using undici', {
      targetUrl,
      hostHeader,
      headers: requestOptions.headers
    });

    // Make request to Kuadrant gateway using undici
    const response = await request(targetUrl, requestOptions);

    const responseText = await response.body.text();
    let responseData;
    
    try {
      responseData = JSON.parse(responseText);
    } catch (e) {
      responseData = { error: responseText };
    }

    // Log the response for debugging
    logger.info('Kuadrant response', {
      status: response.statusCode,
      dataLength: responseText.length
    });

    // Note: Real metrics will be captured by Kuadrant if the request reaches the gateway

    // Return response with same status code
    res.status(response.statusCode).json({
      success: response.statusCode >= 200 && response.statusCode < 300,
      data: responseData,
      // Include debug info for simulator
      debug: {
        requestUrl: targetUrl,
        requestHeaders: requestOptions.headers,
        requestBody,
        responseStatus: response.statusCode,
        responseHeaders: response.headers,
        note: 'Using undici with proper Host header support'
      }
    });

  } catch (error) {
    logger.error('Simulator proxy error:', error);
    res.status(500).json({
      success: false,
      error: 'Failed to proxy request to model server',
      details: error instanceof Error ? error.message : String(error)
    });
  }
});

export default router;