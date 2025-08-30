import express from 'express';
import { logger } from '../utils/logger';
import kuadrantService from '../services/kuadrantService';

const router: express.Router = express.Router();

// Get all policies from Kuadrant
router.get('/', async (req, res) => {
  try {
    logger.info('Fetching real policies from Kuadrant...');
    
    // Fetch real policies from Kuadrant
    const policies = await kuadrantService.getAllPolicies();
    
    logger.info(`Retrieved ${policies.length} policies from Kuadrant`);

    res.json({
      success: true,
      data: policies,
      timestamp: new Date().toISOString()
    });
  } catch (error: any) {
    logger.error('Failed to fetch policies:', error);
    res.status(500).json({
      success: false,
      error: 'Failed to fetch policies'
    });
  }
});

// Create a new policy
router.post('/', async (req, res) => {
  try {
    const policyData = req.body;
    logger.info('Creating policy:', policyData);
    
    // Determine policy type and create accordingly
    let newPolicy;
    if (policyData.type === 'auth') {
      newPolicy = await kuadrantService.createAuthPolicy(policyData);
    } else if (policyData.type === 'rateLimit') {
      newPolicy = await kuadrantService.createRateLimitPolicy(policyData);
    } else {
      return res.status(400).json({
        success: false,
        error: 'Invalid policy type. Must be "auth" or "rateLimit"'
      });
    }
    
    res.status(201).json({
      success: true,
      data: newPolicy
    });
  } catch (error: any) {
    logger.error('Failed to create policy:', error);
    res.status(500).json({
      success: false,
      error: error.message || 'Failed to create policy'
    });
  }
});

// Update a policy
router.put('/:id', async (req, res) => {
  try {
    const policyId = req.params.id;
    const policyData = req.body;
    logger.info(`Updating policy ${policyId}:`, policyData);
    
    const updatedPolicy = await kuadrantService.updatePolicy(policyId, policyData);
    
    res.json({
      success: true,
      data: updatedPolicy
    });
  } catch (error: any) {
    logger.error('Failed to update policy:', error);
    res.status(500).json({
      success: false,
      error: error.message || 'Failed to update policy'
    });
  }
});

// Delete a policy
router.delete('/:id', async (req, res) => {
  try {
    const policyId = req.params.id;
    logger.info(`Deleting policy ${policyId}`);
    
    await kuadrantService.deletePolicy(policyId);
    
    res.json({
      success: true,
      message: `Policy ${policyId} deleted successfully`
    });
  } catch (error: any) {
    logger.error('Failed to delete policy:', error);
    res.status(500).json({
      success: false,
      error: error.message || 'Failed to delete policy'
    });
  }
});

export default router;