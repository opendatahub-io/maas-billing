import * as k8s from '@kubernetes/client-node';
import { logger } from '../utils/logger';

export interface KuadrantPolicy {
  id: string;
  name: string;
  description: string;
  type: 'auth' | 'rateLimit';
  namespace: string;
  targetRef: {
    group: string;
    kind: string;
    name: string;
  };
  config: any;
  status: {
    conditions: Array<{
      type: string;
      status: string;
      reason: string;
      message: string;
      lastTransitionTime: string;
    }>;
  };
  created: string;
  modified: string;
  isActive: boolean;
  items?: Array<{
    id: string;
    type: string;
    config: any;
    description?: string;
    rates?: any[];
    counters?: any[];
    conditions?: any[];
    allowedGroups?: string[];
  }>;
}

class KuadrantService {
  private kc: k8s.KubeConfig;
  private k8sApi: k8s.CoreV1Api;
  private customObjectsApi: k8s.CustomObjectsApi;

  constructor() {
    this.kc = new k8s.KubeConfig();
    this.kc.loadFromCluster();
    this.k8sApi = this.kc.makeApiClient(k8s.CoreV1Api);
    this.customObjectsApi = this.kc.makeApiClient(k8s.CustomObjectsApi);
  }

  async getAuthPolicies(): Promise<KuadrantPolicy[]> {
    try {
      logger.info('Fetching AuthPolicies from Kubernetes API...');
      
      const policies: KuadrantPolicy[] = [];
      
      // Try to get AuthPolicies from current namespace first
      const currentNamespace = process.env.NAMESPACE || 'llm';
      
      try {
        const response = await this.customObjectsApi.listNamespacedCustomObject(
          'kuadrant.io',
          'v1',
          currentNamespace,
          'authpolicies'
        );
        
        const authPolicies = (response.body as any).items || [];
        
        for (const policy of authPolicies) {
          policies.push(this.transformAuthPolicy(policy));
        }
        
        logger.info(`Found ${authPolicies.length} AuthPolicies in namespace ${currentNamespace}`);
      } catch (nsError: any) {
        logger.warn(`Failed to get AuthPolicies from namespace ${currentNamespace}: ${nsError.message}`);
        
        // Try cluster-wide access if namespace access fails
        try {
          const clusterResponse = await this.customObjectsApi.listClusterCustomObject(
            'kuadrant.io',
            'v1',
            'authpolicies'
          );
          
          const clusterPolicies = (clusterResponse.body as any).items || [];
          
          for (const policy of clusterPolicies) {
            policies.push(this.transformAuthPolicy(policy));
          }
          
          logger.info(`Found ${clusterPolicies.length} AuthPolicies cluster-wide`);
        } catch (clusterError: any) {
          logger.error(`Failed to get AuthPolicies cluster-wide: ${clusterError.message}`);
        }
      }

      return policies;
    } catch (error: any) {
      logger.error('Error fetching AuthPolicies:', error);
      return [];
    }
  }

  async getRateLimitPolicies(): Promise<KuadrantPolicy[]> {
    try {
      logger.info('Fetching RateLimitPolicies from Kubernetes API...');
      
      const policies: KuadrantPolicy[] = [];
      const currentNamespace = process.env.NAMESPACE || 'llm';
      
      try {
        const response = await this.customObjectsApi.listNamespacedCustomObject(
          'kuadrant.io',
          'v1beta2',
          currentNamespace,
          'ratelimitpolicies'
        );
        
        const rateLimitPolicies = (response.body as any).items || [];
        
        for (const policy of rateLimitPolicies) {
          policies.push(this.transformRateLimitPolicy(policy));
        }
        
        logger.info(`Found ${rateLimitPolicies.length} RateLimitPolicies in namespace ${currentNamespace}`);
      } catch (nsError: any) {
        logger.warn(`Failed to get RateLimitPolicies from namespace ${currentNamespace}: ${nsError.message}`);
        
        // Try cluster-wide access if namespace access fails
        try {
          const clusterResponse = await this.customObjectsApi.listClusterCustomObject(
            'kuadrant.io',
            'v1beta2',
            'ratelimitpolicies'
          );
          
          const clusterPolicies = (clusterResponse.body as any).items || [];
          
          for (const policy of clusterPolicies) {
            policies.push(this.transformRateLimitPolicy(policy));
          }
          
          logger.info(`Found ${clusterPolicies.length} RateLimitPolicies cluster-wide`);
        } catch (clusterError: any) {
          logger.error(`Failed to get RateLimitPolicies cluster-wide: ${clusterError.message}`);
        }
      }

      return policies;
    } catch (error: any) {
      logger.error('Error fetching RateLimitPolicies:', error);
      return [];
    }
  }

  async getTokenRateLimitPolicies(): Promise<KuadrantPolicy[]> {
    try {
      logger.info('Fetching TokenRateLimitPolicies from Kubernetes API...');
      
      const policies: KuadrantPolicy[] = [];
      const currentNamespace = process.env.NAMESPACE || 'llm';
      
      try {
        const response = await this.customObjectsApi.listNamespacedCustomObject(
          'kuadrant.io',
          'v1beta1',
          currentNamespace,
          'tokenratelimitpolicies'
        );
        
        const tokenPolicies = (response.body as any).items || [];
        
        for (const policy of tokenPolicies) {
          policies.push(this.transformTokenRateLimitPolicy(policy));
        }
        
        logger.info(`Found ${tokenPolicies.length} TokenRateLimitPolicies in namespace ${currentNamespace}`);
      } catch (nsError: any) {
        logger.warn(`Failed to get TokenRateLimitPolicies from namespace ${currentNamespace}: ${nsError.message}`);
        
        try {
          const clusterResponse = await this.customObjectsApi.listClusterCustomObject(
            'kuadrant.io',
            'v1beta1',
            'tokenratelimitpolicies'
          );
          
          const clusterPolicies = (clusterResponse.body as any).items || [];
          
          for (const policy of clusterPolicies) {
            policies.push(this.transformTokenRateLimitPolicy(policy));
          }
          
          logger.info(`Found ${clusterPolicies.length} TokenRateLimitPolicies cluster-wide`);
        } catch (clusterError: any) {
          logger.error(`Failed to get TokenRateLimitPolicies cluster-wide: ${clusterError.message}`);
        }
      }

      return policies;
    } catch (error: any) {
      logger.error('Error fetching TokenRateLimitPolicies:', error);
      return [];
    }
  }

  async getAllPolicies(): Promise<KuadrantPolicy[]> {
    const [authPolicies, rateLimitPolicies, tokenPolicies] = await Promise.all([
      this.getAuthPolicies(),
      this.getRateLimitPolicies(),
      this.getTokenRateLimitPolicies()
    ]);

    return [...authPolicies, ...rateLimitPolicies, ...tokenPolicies];
  }

  private transformAuthPolicy(policy: any): KuadrantPolicy {
    return {
      id: `${policy.metadata?.namespace}/${policy.metadata?.name}`,
      name: policy.metadata?.name || 'Unknown',
      description: policy.spec?.description || `AuthPolicy for ${policy.spec?.targetRef?.name || 'unknown target'}`,
      type: 'auth',
      namespace: policy.metadata?.namespace || 'default',
      targetRef: policy.spec?.targetRef || {},
      config: policy.spec || {},
      status: {
        conditions: policy.status?.conditions || []
      },
      created: policy.metadata?.creationTimestamp || new Date().toISOString(),
      modified: policy.metadata?.resourceVersion || new Date().toISOString(),
      isActive: this.isPolicyActive(policy),
      items: this.extractAuthPolicyItems(policy.spec)
    };
  }

  private transformRateLimitPolicy(policy: any): KuadrantPolicy {
    return {
      id: `${policy.metadata?.namespace}/${policy.metadata?.name}`,
      name: policy.metadata?.name || 'Unknown',
      description: policy.spec?.description || `RateLimitPolicy for ${policy.spec?.targetRef?.name || 'unknown target'}`,
      type: 'rateLimit',
      namespace: policy.metadata?.namespace || 'default',
      targetRef: policy.spec?.targetRef || {},
      config: policy.spec || {},
      status: {
        conditions: policy.status?.conditions || []
      },
      created: policy.metadata?.creationTimestamp || new Date().toISOString(),
      modified: policy.metadata?.resourceVersion || new Date().toISOString(),
      isActive: this.isPolicyActive(policy),
      items: this.extractRateLimitPolicyItems(policy.spec, policy.metadata?.name)
    };
  }

  private transformTokenRateLimitPolicy(policy: any): KuadrantPolicy {
    return {
      id: `${policy.metadata?.namespace}/${policy.metadata?.name}`,
      name: policy.metadata?.name || 'Unknown',
      description: policy.spec?.description || `TokenRateLimitPolicy for ${policy.spec?.targetRef?.name || 'unknown target'}`,
      type: 'rateLimit',
      namespace: policy.metadata?.namespace || 'default',
      targetRef: policy.spec?.targetRef || {},
      config: policy.spec || {},
      status: {
        conditions: policy.status?.conditions || []
      },
      created: policy.metadata?.creationTimestamp || new Date().toISOString(),
      modified: policy.metadata?.resourceVersion || new Date().toISOString(),
      isActive: this.isPolicyActive(policy),
      items: this.extractTokenRateLimitPolicyItems(policy.spec)
    };
  }

  private isPolicyActive(policy: any): boolean {
    const conditions = policy.status?.conditions || [];
    return conditions.some((condition: any) => 
      condition.type === 'Ready' && condition.status === 'True'
    ) || conditions.length === 0; // Consider active if no conditions (newly created)
  }

  private extractAuthPolicyItems(spec: any): Array<{id: string, type: string, config: any}> {
    const items: Array<{id: string, type: string, config: any}> = [];
    
    if (spec?.rules?.authentication) {
      Object.entries(spec.rules.authentication).forEach(([key, value]: [string, any]) => {
        items.push({
          id: `auth-${key}`,
          type: 'authentication',
          config: value
        });
      });
    }
    
    if (spec?.rules?.authorization) {
      Object.entries(spec.rules.authorization).forEach(([key, value]: [string, any]) => {
        items.push({
          id: `authz-${key}`,
          type: 'authorization',
          config: value
        });
      });
    }
    
    if (spec?.rules?.response) {
      Object.entries(spec.rules.response).forEach(([key, value]: [string, any]) => {
        items.push({
          id: `response-${key}`,
          type: 'response',
          config: value
        });
      });
    }
    
    return items;
  }

  private extractRateLimitPolicyItems(spec: any, policyName?: string): Array<{id: string, type: string, config: any, description?: string, rates?: any[], counters?: any[], conditions?: any[]}> {
    const items: Array<{id: string, type: string, config: any, description?: string, rates?: any[], counters?: any[], conditions?: any[]}> = [];
    
    if (spec?.limits) {
      Object.entries(spec.limits).forEach(([key, value]: [string, any]) => {
        const rates = value.rates || [];
        const counters = value.counters || [];
        const conditions = value.when || [];
        
        // Build description - detect if this is a token-based policy
        const isTokenPolicy = policyName && policyName.toLowerCase().includes('token');
        let description = `Rate limit: ${key}`;
        if (rates.length > 0) {
          const rate = rates[0];
          const unit = isTokenPolicy ? 'tokens' : 'requests';
          description += ` - ${rate.limit} ${unit} per ${rate.window}`;
        }
        if (conditions.length > 0) {
          description += ` (when: ${conditions[0].predicate})`;
        }
        
        items.push({
          id: key,
          type: isTokenPolicy ? 'token-rate-limit' : 'rate-limit',
          config: value,
          description,
          rates,
          counters,
          conditions
        });
      });
    }
    
    return items;
  }

  private extractTokenRateLimitPolicyItems(spec: any): Array<{id: string, type: string, config: any, description?: string, rates?: any[], counters?: any[], conditions?: any[]}> {
    const items: Array<{id: string, type: string, config: any, description?: string, rates?: any[], counters?: any[], conditions?: any[]}> = [];
    
    if (spec?.limits) {
      Object.entries(spec.limits).forEach(([key, value]: [string, any]) => {
        const rates = value.rates || [];
        const counters = value.counters || [];
        const conditions = value.when || [];
        
        // Build description - use "tokens" for token policies
        let description = `Rate limit: ${key}`;
        if (rates.length > 0) {
          const rate = rates[0];
          description += ` - ${rate.limit} tokens per ${rate.window}`;
        }
        if (conditions.length > 0) {
          description += ` (when: ${conditions[0].predicate})`;
        }
        
        items.push({
          id: key,
          type: 'token-rate-limit',
          config: value,
          description,
          rates,
          counters,
          conditions
        });
      });
    }
    
    return items;
  }

  // Additional helper methods for debugging and status
  async getPolicyByName(name: string, namespace?: string): Promise<KuadrantPolicy | null> {
    const allPolicies = await this.getAllPolicies();
    return allPolicies.find(p => 
      p.name === name && (!namespace || p.namespace === namespace)
    ) || null;
  }

  async checkKuadrantConnection(): Promise<{connected: boolean, error?: string}> {
    try {
      // Try to list any Kuadrant resource to check connectivity
      await this.customObjectsApi.listClusterCustomObject(
        'kuadrant.io',
        'v1',
        'authpolicies'
      );
      return { connected: true };
    } catch (error: any) {
      return { connected: false, error: error.message };
    }
  }
}

export default new KuadrantService();