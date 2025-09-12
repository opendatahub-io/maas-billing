export interface Team {
  id: string;
  name: string;
  color: string;
}

export interface Model {
  id: string;
  name: string;
  provider: string;
  description?: string;
}

export interface PolicyItem {
  id: string;
  type: 'tier' | 'model';
  value: string;
  isApprove: boolean; // true for approve policy, false for reject policy
}

export interface RequestLimits {
  tokenLimit: number | null; // null means unlimited
  timePeriod: 'hour' | 'day' | 'week' | 'month';
}

export interface TimeRange {
  startTime: string; // HH:mm format
  endTime: string; // HH:mm format
  unlimited: boolean;
}

export interface Policy {
  id: string;
  name: string;
  description: string;
  items: any[]; // Real Kuadrant policy items (auth rules, rate limit rules, etc.)
  requestLimits?: RequestLimits;
  timeRange?: TimeRange; // Optional since real policies don't have this
  created: string;
  modified: string;
  // Real Kuadrant policy properties
  type: 'auth' | 'rate-limit';
  namespace: string;
  targetRef?: {
    kind: string;
    name: string;
  };
  status?: any;
  fullSpec?: any;
  config?: {
    auth?: {
      type: string;
      required: boolean;
    };
    rateLimit?: {
      requests: number;
      duration: string;
      unit: string;
    };
    rules?: any; // Full Kuadrant rules object
    limits?: any[]; // Limitador limits array
  };
  isActive?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface ModelInferenceData {
  requestId: string;
  modelName: string;
  modelVersion?: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  responseTime: number; // ms
  prompt?: string;
  completion?: string;
  temperature?: number;
  maxTokens?: number;
  stopSequences?: string[];
  finishReason?: 'stop' | 'length' | 'content_filter' | 'error';
}

export interface PolicyDecisionDetails {
  policyId: string;
  policyName: string;
  policyType: 'AuthPolicy' | 'RateLimitPolicy' | 'ContentPolicy' | 'CostPolicy';
  decision: 'allow' | 'deny';
  reason: string;
  ruleTriggered?: string;
  metadata?: Record<string, any>;
  enforcementPoint: 'authorino' | 'limitador' | 'envoy' | 'opa' | 'kuadrant';
  processingTime?: number; // ms
}

export interface AuthenticationDetails {
  method: 'api-key' | 'jwt' | 'oauth' | 'none';
  principal?: string;
  groups?: string[];
  scopes?: string[];
  keyId?: string;
  issuer?: string;
  isValid: boolean;
  validationErrors?: string[];
}

export interface RateLimitDetails {
  limitName: string;
  current: number;
  limit: number;
  window: string;
  remaining: number;
  resetTime: string;
  tier: string;
}

export interface Request {
  id: string;
  timestamp: string;
  
  // Request details
  team: string;
  model: string;
  endpoint: string;
  httpMethod: string;
  userAgent?: string;
  clientIp?: string;
  
  // High-level decision
  decision: 'accept' | 'reject';
  finalReason?: string;
  
  // Authentication data
  authentication?: AuthenticationDetails;
  
  // Policy decisions (can be multiple)
  policyDecisions: PolicyDecisionDetails[];
  
  // Rate limiting info
  rateLimitStatus?: RateLimitDetails;
  
  // Model inference data (only if request was approved and processed)
  modelInference?: ModelInferenceData;
  
  // Request content
  queryText?: string;
  
  // Timing and performance
  totalResponseTime?: number; // ms
  gatewayLatency?: number; // ms
  
  // Cost and billing
  estimatedCost?: number;
  billingTier?: string;
  
  // Source and tracing
  source: 'limitador' | 'authorino' | 'envoy' | 'kuadrant' | 'kserve';
  traceId?: string;
  spanId?: string;
  
  // Legacy fields for compatibility
  policyType?: 'AuthPolicy' | 'RateLimitPolicy' | 'None';
  reason?: string;
  tokens?: number;
  
  // Raw log data from Envoy access logs
  rawLogData?: {
    responseCode: number;
    flags: string;
    route: string;
    bytesReceived: number;
    bytesSent: number;
    host: string;
    upstreamHost: string;
  };
}

export interface SimulationRequest {
  team: string;
  model: string;
  timeOfDay: string;
  queryText: string;
  count: number;
}

export interface SimulationResult {
  id: string;
  timestamp: string;
  request: {
    model: string;
    tier: string;
    messages: Array<{ role: string; content: string }>;
    maxTokens: number;
    headers: Record<string, string>;
    body: any;
  };
  response: {
    status: number;
    statusText: string;
    headers: Record<string, string>;
    body: any;
    error?: string;
  };
  success: boolean;
  duration: number;
}