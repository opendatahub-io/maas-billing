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
      userAgent: req.get('User-Agent'),
      qosMode: req.headers['x-enable-qos'],
      customerTier: req.headers['x-customer-tier'],
      demoMode: req.headers['x-demo-mode']
    });

    const { model, messages, max_tokens, tier } = req.body;
    
    // Check if QoS mode is enabled
    const useQoS = req.headers['x-enable-qos'] === 'true';
    const customerTier = req.headers['x-customer-tier'] as string || 'free';
    const demoMode = req.headers['x-demo-mode'] as string;
    
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
      apiKey: apiKey.substring(0, 8) + '...',
      useQoS,
      customerTier,
      demoMode
    });

    // Configuration constants
    const QOS_SERVICE_URL = process.env.QOS_SERVICE_URL || 'http://localhost:3005';
    const REQUEST_TIMEOUT = 30000;

    // Route through QoS service if enabled
    if (useQoS) {
      return await handleQoSRequest(req, res, {
        model,
        messages,
        max_tokens,
        customerTier,
        demoMode,
        apiKey,
        authHeader,
        startTime
      });
    }
    
    // Get model endpoint dynamically from cluster
    let targetEndpoint: string;
    try {
      const { modelService } = await import('../services/modelService');
      targetEndpoint = await modelService.getModelEndpoint(model);
    } catch (error) {
      logger.error('Failed to get model endpoint:', {
        model,
        error: (error as any).message
      });
      
      return res.status(404).json({
        success: false,
        error: `Model '${model}' not found`,
        details: (error as any).message
      });
    }
    
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
        // Parse OpenShift HTML error responses for better user experience
        let errorMessage = `Kuadrant returned ${kuadrantResponse.status}: ${kuadrantResponse.statusText}`;
        let details = kuadrantResponse.data;
        
        // Check if response contains OpenShift HTML error page
        if (typeof kuadrantResponse.data === 'string' && kuadrantResponse.data.includes('<html>')) {
          const htmlContent = kuadrantResponse.data;
          
          // Extract meaningful error messages from OpenShift HTML
          if (htmlContent.includes('<h1>Application is not available</h1>')) {
            errorMessage = 'Application is not available';
            details = 'The model service is not running or route does not exist. Check if the model deployment is active and all pods are running.';
          } else if (htmlContent.includes('The host doesn\'t exist')) {
            errorMessage = 'Host not found';
            details = 'Route configuration error - the hostname does not exist in the cluster.';
          } else if (htmlContent.includes('Route and path matches, but all pods are down')) {
            errorMessage = 'Service unavailable';
            details = 'All model service pods are down. Check deployment status and pod health.';
          } else if (htmlContent.includes('<h1>')) {
            // Extract any h1 tag content as the error message
            const h1Match = htmlContent.match(/<h1>([^<]+)<\/h1>/);
            if (h1Match) {
              errorMessage = h1Match[1];
              details = 'Check model service status and deployment configuration.';
            }
          }
        }
        
        return res.status(kuadrantResponse.status).json({
          success: false,
          error: errorMessage,
          details: details,
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

      // Forward the successful response from Kuadrant with additional metadata
      const responseData = {
        ...kuadrantResponse.data,
        _simulator_metadata: {
          route_endpoint: targetEndpoint,
          duration_ms: duration,
          kuadrant_status: kuadrantResponse.status,
          processed_via: 'kuadrant'
        }
      };
      
      res.json(responseData);

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

// Handle QoS-enabled requests
async function handleQoSRequest(req: express.Request, res: express.Response, params: {
  model: string;
  messages: any[];
  max_tokens?: number;
  customerTier: string;
  demoMode?: string;
  apiKey: string;
  authHeader?: string;
  startTime: number;
}) {
  const { model, messages, max_tokens, customerTier, demoMode, authHeader, startTime } = params;
  const QOS_SERVICE_URL = process.env.QOS_SERVICE_URL || 'http://localhost:3005';
  const REQUEST_TIMEOUT = 30000;

  try {
    // Generate x-auth-identity header for QoS service
    const authIdentity = {
      metadata: {
        annotations: {
          'kuadrant.io/groups': customerTier,
          'secret.kuadrant.io/user-id': `${customerTier.toUpperCase()}-${Date.now()}`
        },
        sla: customerTier === 'enterprise' ? 'guaranteed' : 
             customerTier === 'premium' ? 'standard' : 'best_effort'
      }
    };

    // Prepare headers for QoS service
    const qosHeaders: Record<string, string> = {
      'Content-Type': 'application/json',
      'x-auth-identity': JSON.stringify(authIdentity)
    };

    // Add demo mode header if specified
    if (demoMode) {
      qosHeaders['x-demo-mode'] = demoMode;
    }

    // Forward authorization header if present
    if (authHeader) {
      qosHeaders['Authorization'] = authHeader;
    }

    logger.info('Routing request through QoS service:', {
      qosUrl: `${QOS_SERVICE_URL}/v1/chat/completions`,
      customerTier,
      demoMode,
      authIdentity: authIdentity.metadata.annotations
    });

    // Send request to QoS service
    const qosResponse = await axios({
      method: 'POST',
      url: `${QOS_SERVICE_URL}/v1/chat/completions`,
      headers: qosHeaders,
      data: {
        model,
        messages,
        max_tokens: max_tokens || 100,
        temperature: 0.7
      },
      timeout: REQUEST_TIMEOUT,
      validateStatus: () => true // Don't throw on HTTP error status codes
    });

    // Log the response for debugging
    logger.info('QoS service response:', {
      status: qosResponse.status,
      statusText: qosResponse.statusText,
      hasData: !!qosResponse.data,
      queueMetrics: qosResponse.data?.queue_metrics
    });

    // Calculate total duration
    const duration = Date.now() - startTime;

    // Handle QoS service errors
    if (qosResponse.status !== 200) {
      logger.error('QoS service error:', {
        status: qosResponse.status,
        data: qosResponse.data,
        duration: `${duration}ms`
      });

      // Parse OpenShift HTML error responses for better user experience
      let errorMessage = `QoS service returned ${qosResponse.status}: ${qosResponse.statusText}`;
      let details = qosResponse.data;
      
      // Check if response contains OpenShift HTML error page
      if (typeof qosResponse.data === 'string' && qosResponse.data.includes('<html>')) {
        const htmlContent = qosResponse.data;
        
        // Extract meaningful error messages from OpenShift HTML
        if (htmlContent.includes('<h1>Application is not available</h1>')) {
          errorMessage = 'Application is not available';
          details = 'The model service is not running or route does not exist. Check if the model deployment is active and all pods are running.';
        } else if (htmlContent.includes('The host doesn\'t exist')) {
          errorMessage = 'Host not found';
          details = 'Route configuration error - the hostname does not exist in the cluster.';
        } else if (htmlContent.includes('Route and path matches, but all pods are down')) {
          errorMessage = 'Service unavailable';
          details = 'All model service pods are down. Check deployment status and pod health.';
        } else if (htmlContent.includes('<h1>')) {
          // Extract any h1 tag content as the error message
          const h1Match = htmlContent.match(/<h1>([^<]+)<\/h1>/);
          if (h1Match) {
            errorMessage = h1Match[1];
            details = 'Check model service status and deployment configuration.';
          }
        }
      }

      return res.status(qosResponse.status).json({
        success: false,
        error: errorMessage,
        details: details,
        qos_status: qosResponse.status,
        duration: `${duration}ms`,
        customer_tier: customerTier
      });
    }

    // Log successful QoS processing
    logger.info('QoS request completed successfully:', {
      model,
      customerTier,
      duration: `${duration}ms`,
      status: qosResponse.status
    });

    // Return the QoS service response with additional metadata
    const responseData = {
      ...qosResponse.data,
      qos_metadata: {
        customer_tier: customerTier,
        processing_mode: demoMode || 'auto',
        duration_ms: duration,
        processed_via: 'qos_service'
      }
    };

    res.json(responseData);

  } catch (error: any) {
    const duration = Date.now() - startTime;
    logger.error('Error communicating with QoS service:', {
      error: error.message,
      customerTier,
      duration: `${duration}ms`,
      qosUrl: QOS_SERVICE_URL
    });
    
    // Return error indicating QoS service unavailable
    res.status(503).json({
      success: false,
      error: 'QoS service unavailable',
      details: error.message,
      customer_tier: customerTier,
      duration_ms: duration,
      fallback_available: false
    });
  }
}

export default router;