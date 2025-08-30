import express from 'express';
import cors from 'cors';
import { logger } from './utils/logger';
import metricsRoutes from './routes/metrics';
import policiesRoutes from './routes/policies';
import simulatorRoutes from './routes/simulator';

const app: express.Application = express();
const PORT = process.env.PORT || 3002;

// Middleware
app.use(cors({
  origin: process.env.FRONTEND_URL || 'http://localhost:3000',
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
app.use('/api/v1/simulator', simulatorRoutes);

// Models endpoint for compatibility
app.get('/api/v1/models', (req, res) => {
  res.json({
    success: true,
    data: [
      {
        id: 'vllm-simulator',
        name: 'vLLM Simulator',
        provider: 'KServe',
        description: 'Test model for policy enforcement'
      },
      {
        id: 'qwen3-0-6b-instruct',
        name: 'Qwen3 0.6B Instruct',
        provider: 'KServe',
        description: 'Qwen3 model with vLLM runtime'
      }
    ]
  });
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

// Start server
if (require.main === module) {
  app.listen(PORT, () => {
    logger.info(`Server running on port ${PORT}`);
    logger.info('Available endpoints:');
    logger.info('  GET /health - Health check');
    logger.info('  GET /api/v1/models - Available models');
    logger.info('  GET /api/v1/metrics - General metrics');
    logger.info('  GET /api/v1/metrics/live-requests - Live request data with policy enforcement');
    logger.info('  GET /api/v1/metrics/dashboard - Dashboard statistics');
    logger.info('  GET /api/v1/policies - Get all policies');
    logger.info('  POST /api/v1/policies - Create new policy');
    logger.info('  PUT /api/v1/policies/:id - Update policy');
    logger.info('  DELETE /api/v1/policies/:id - Delete policy');
    logger.info('  POST /api/v1/simulator/chat/completions - Simulator proxy endpoint');
  });
}

export default app;