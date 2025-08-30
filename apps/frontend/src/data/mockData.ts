import { Team, Model, Policy } from '../types';

export const teams: Team[] = [
  { id: 'engineering', name: 'Engineering', color: '#1976d2' },
  { id: 'product', name: 'Product', color: '#388e3c' },
  { id: 'marketing', name: 'Marketing', color: '#f57c00' },
  { id: 'cto', name: 'CTO', color: '#7b1fa2' },
];

export const models: Model[] = [
  {
    id: 'vllm-simulator',
    name: 'vLLM Simulator',
    provider: 'KServe',
    description: 'Test model for policy enforcement - RUNNING'
  },
  // Note: These models are not currently deployed in your local environment
  // Uncomment when they are deployed:
  // {
  //   id: 'qwen3-0-6b-instruct',
  //   name: 'Qwen3 0.6B Instruct (NOT DEPLOYED)',
  //   provider: 'KServe',
  //   description: 'Qwen3 model with vLLM runtime'
  // },
];

// This will be replaced with real Kuadrant policies
export const initialPolicies: Policy[] = [
  {
    id: 'policy-1',
    name: 'Engineering Team Access',
    description: 'Full access for engineering team during business hours',
    items: [
      {
        id: 'item-1',
        type: 'tier',
        value: 'premium',
        isApprove: true
      }
    ],
    requestLimits: {
      tokenLimit: 10000,
      timePeriod: 'hour'
    },
    timeRange: {
      startTime: '09:00',
      endTime: '17:00',
      unlimited: false
    },
    created: new Date().toISOString(),
    modified: new Date().toISOString(),
    // Kuadrant mapping
    type: 'auth',
    isActive: true
  },
  {
    id: 'policy-2',
    name: 'Rate Limit Policy',
    description: 'General rate limiting for all teams',
    items: [
      {
        id: 'item-2',
        type: 'tier',
        value: 'free',
        isApprove: true
      },
      {
        id: 'item-3',
        type: 'tier',
        value: 'enterprise',
        isApprove: true
      }
    ],
    requestLimits: {
      tokenLimit: 5000,
      timePeriod: 'hour'
    },
    timeRange: {
      startTime: '00:00',
      endTime: '23:59',
      unlimited: true
    },
    created: new Date().toISOString(),
    modified: new Date().toISOString(),
    // Kuadrant mapping
    type: 'rateLimit',
    isActive: true
  }
];