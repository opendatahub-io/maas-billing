import { exec } from 'child_process';
import { promisify } from 'util';
import axios from 'axios';
import { logger } from '../utils/logger';

const execAsync = promisify(exec);

// MaaS API response interfaces
interface MaasApiModel {
  name: string;
  namespace: string;
  url: string;
  ready: boolean;
}

interface MaasApiModelsResponse {
  models: MaasApiModel[];
}

export interface Model {
  id: string;
  name: string;
  provider: string;
  description: string;
  endpoint: string;
  namespace: string;
}

export class ModelService {
  private models: Model[] = [];
  private lastFetch = 0;
  private readonly CACHE_TTL = 60000; // 1 minute cache

  async getModels(): Promise<Model[]> {
    const now = Date.now();
    
    // Return cached models if still fresh
    if (this.models.length > 0 && (now - this.lastFetch) < this.CACHE_TTL) {
      return this.models;
    }

    try {
      // Try MaaS API first
      logger.info('Fetching models from MaaS API...');
      this.models = await this.fetchModelsFromMaasApi();
      this.lastFetch = now;
      
      logger.info(`Retrieved ${this.models.length} models from MaaS API`, {
        models: this.models.map(m => ({ id: m.id, namespace: m.namespace }))
      });
      
      return this.models;
    } catch (maasError) {
      logger.warn('Failed to fetch models from MaaS API, falling back to direct cluster access:', maasError);
      
      try {
        // Fallback to direct cluster access
        logger.info('Fetching models from cluster directly...');
        this.models = await this.fetchModelsFromCluster();
        this.lastFetch = now;
        
        logger.info(`Retrieved ${this.models.length} models from cluster`, {
          models: this.models.map(m => ({ id: m.id, namespace: m.namespace }))
        });
        
        return this.models;
      } catch (clusterError) {
        logger.error('Failed to fetch models from both MaaS API and direct cluster access:', clusterError);
        
        // If we have cached models, return them as fallback
        if (this.models.length > 0) {
          logger.warn('Using cached models due to all fetch methods failing');
          return this.models;
        }
        
        // No cache available, throw error
        throw new Error(`No models available: MaaS API failed (${maasError instanceof Error ? maasError.message : maasError}), direct cluster access failed (${clusterError instanceof Error ? clusterError.message : clusterError})`);
      }
    }
  }

  async getModelById(modelId: string): Promise<Model | null> {
    const models = await this.getModels();
    return models.find(model => model.id === modelId) || null;
  }

  async getModelEndpoint(modelId: string): Promise<string> {
    const model = await this.getModelById(modelId);
    if (!model) {
      throw new Error(`Model '${modelId}' not found in cluster. Available models: ${(await this.getModels()).map(m => m.id).join(', ')}`);
    }
    return model.endpoint;
  }

  private async fetchModelsFromMaasApi(): Promise<Model[]> {
    try {
      const maasApiUrl = process.env.MAAS_API_URL || (() => { throw new Error('MAAS_API_URL environment variable is required'); })();
      
      logger.info('Fetching models from MaaS API...', { url: `${maasApiUrl}/models` });
      
      const response = await axios.get<MaasApiModelsResponse>(`${maasApiUrl}/models`, {
        timeout: 30000,
        headers: {
          'Accept': 'application/json',
        }
      });

      if (!response.data || !Array.isArray(response.data.models)) {
        throw new Error('Invalid response format from MaaS API');
      }

      logger.info(`Retrieved ${response.data.models.length} models from MaaS API`, {
        models: response.data.models.map(m => ({ name: m.name, namespace: m.namespace, ready: m.ready }))
      });
      
      const maasModels = response.data.models;
      
      // TODO: Remove this route discovery workaround once MaaS API is fixed
      // Issue: https://github.com/opendatahub-io/maas-billing/issues/90
      // MaaS API currently returns unusable InferenceService URLs instead of accessible route URLs
      // This workaround discovers OpenShift routes and maps them to InferenceServices
      // Once MaaS API returns proper route URLs, this entire section can be removed
      
      // Get route mapping to convert InferenceService URLs to actual route URLs
      let routeMap = new Map<string, string>();
      try {
        const routeResult = await execAsync(`kubectl get routes -n llm -o jsonpath='{range .items[*]}{.metadata.name}{"\\t"}{.spec.host}{"\\n"}{end}'`);
        if (routeResult.stdout.trim()) {
          const routeLines = routeResult.stdout.trim().split('\n');
          for (const line of routeLines) {
            const [routeName, host] = line.split('\t');
            if (routeName && host) {
              routeMap.set(routeName, host);
              logger.info(`Found route: ${routeName} -> ${host}`);
            }
          }
        }
      } catch (routeError) {
        logger.warn('Failed to get routes, using MaaS API URLs directly:', routeError);
      }
      
      const models: Model[] = maasModels.map((maasModel: MaasApiModel) => {
        let endpoint: string;
        
        // Try to find matching route for better external access
        if (routeMap.size > 0) {
          let routeHost: string | undefined;
          let bestMatch = '';
          let bestScore = 0;
          
          for (const [routeName, host] of routeMap.entries()) {
            const score = this.calculateRouteMatchScore(maasModel.name, routeName);
            if (score > bestScore) {
              bestScore = score;
              bestMatch = routeName;
              routeHost = host;
            }
          }
          
          if (routeHost && bestScore >= 0.3) {
            endpoint = `http://${routeHost}/v1/chat/completions`;
            logger.info(`Matched MaaS model ${maasModel.name} to route ${bestMatch} (score: ${bestScore})`);
          } else {
            // Fallback to MaaS API URL if no good route match
            endpoint = maasModel.url.replace(/\/$/, '') + '/v1/chat/completions';
            logger.warn(`No suitable route found for ${maasModel.name}, using MaaS API URL: ${endpoint}`);
          }
        } else {
          // No routes available, use MaaS API URL
          endpoint = maasModel.url.replace(/\/$/, '') + '/v1/chat/completions';
        }
        
        return {
          id: maasModel.name,
          name: this.formatModelName(maasModel.name),
          provider: 'KServe',
          description: `Model served via KServe (from MaaS API)`,
          endpoint,
          namespace: maasModel.namespace
        };
      });
      // END TODO: Route discovery workaround - remove when MaaS API issue #90 is fixed

      return models;
    } catch (error: any) {
      logger.error('Failed to fetch models from MaaS API:', error);
      
      if (error.code === 'ECONNREFUSED') {
        throw new Error(`MaaS API service is not available at ${process.env.MAAS_API_URL}. Please ensure the service is running.`);
      }
      
      throw new Error(`Failed to fetch models from MaaS API: ${error.message}`);
    }
  }

  private async fetchModelsFromCluster(): Promise<Model[]> {
    try {
      // Get InferenceServices and their corresponding routes
      const [inferenceResult, routeResult] = await Promise.all([
        execAsync(`kubectl get inferenceservices -A -o jsonpath='{range .items[*]}{.metadata.name}{"\\t"}{.metadata.namespace}{"\\t"}{.spec.predictor.model.modelFormat.name}{"\\n"}{end}'`),
        execAsync(`kubectl get routes -n llm -o jsonpath='{range .items[*]}{.metadata.name}{"\\t"}{.spec.host}{"\\n"}{end}'`)
      ]);
      
      if (!inferenceResult.stdout.trim()) {
        logger.warn('No InferenceServices found in cluster');
        return [];
      }

      // Build route mapping - purely data-driven
      const routeMap = new Map<string, string>();
      if (routeResult.stdout.trim()) {
        const routeLines = routeResult.stdout.trim().split('\n');
        for (const line of routeLines) {
          const [routeName, host] = line.split('\t');
          if (routeName && host) {
            // Store route exactly as it appears in the cluster
            routeMap.set(routeName, host);
            logger.info(`Found route: ${routeName} -> ${host}`);
          }
        }
      }

      const models: Model[] = [];
      const lines = inferenceResult.stdout.trim().split('\n');
      
      for (const line of lines) {
        const [name, namespace, modelFormat] = line.split('\t');
        
        if (!name || !namespace) {
          logger.warn('Skipping invalid InferenceService entry:', line);
          continue;
        }

        // Find matching route using generic pattern matching algorithms
        let routeHost: string | undefined;
        let bestMatch = '';
        let bestScore = 0;
        
        for (const [routeName, host] of routeMap.entries()) {
          const score = this.calculateRouteMatchScore(name, routeName);
          if (score > bestScore) {
            bestScore = score;
            bestMatch = routeName;
            routeHost = host;
          }
        }
        
        if (!routeHost || bestScore < 0.3) { // Minimum threshold for matching
          logger.warn(`No suitable route found for InferenceService ${name} (best match: ${bestMatch}, score: ${bestScore})`);
          logger.warn(`Available routes: ${Array.from(routeMap.keys()).join(', ')}`);
          continue;
        }
        
        logger.info(`Matched InferenceService ${name} to route ${bestMatch} (score: ${bestScore})`);
      

        const endpoint = `http://${routeHost}/v1/chat/completions`;
        
        // Extract display name
        const displayName = name.replace(/-llm$/, '');
        
        models.push({
          id: name,
          name: this.formatModelName(displayName),
          provider: 'KServe',
          description: `${modelFormat || 'LLM'} model served via KServe`,
          endpoint,
          namespace
        });
        
        logger.info(`Found model: ${name} with endpoint: ${endpoint}`);
      }

      return models;
    } catch (error: any) {
      logger.error('Error executing kubectl command:', error);
      throw new Error(`Failed to fetch models from cluster: ${error.message}`);
    }
  }

  private formatModelName(modelId: string): string {
    // Convert model ID to human-readable name
    return modelId
      .split('-')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  }

  private calculateRouteMatchScore(inferenceServiceName: string, routeName: string): number {
    // Normalize names for comparison
    const normalizeString = (str: string) => str.toLowerCase().replace(/[-_]/g, '');
    const normalizedService = normalizeString(inferenceServiceName);
    const normalizedRoute = normalizeString(routeName);
    
    // Remove common suffixes/prefixes for better matching
    const cleanService = normalizedService.replace(/^(inference|service|model)/, '').replace(/(service|model)$/, '');
    const cleanRoute = normalizedRoute.replace(/route$/, '').replace(/^(api|service)/, '');
    
    // Calculate similarity scores using multiple algorithms
    let score = 0;
    
    // 1. Exact match (highest score)
    if (cleanService === cleanRoute) {
      return 1.0;
    }
    
    // 2. One contains the other
    if (cleanService.includes(cleanRoute) || cleanRoute.includes(cleanService)) {
      score += 0.8;
    }
    
    // 3. Check for common word segments
    const serviceWords = cleanService.split(/[^a-z0-9]+/).filter(w => w.length > 2);
    const routeWords = cleanRoute.split(/[^a-z0-9]+/).filter(w => w.length > 2);
    
    let wordMatches = 0;
    for (const serviceWord of serviceWords) {
      for (const routeWord of routeWords) {
        if (serviceWord === routeWord || serviceWord.includes(routeWord) || routeWord.includes(serviceWord)) {
          wordMatches++;
          break;
        }
      }
    }
    
    if (serviceWords.length > 0) {
      score += (wordMatches / serviceWords.length) * 0.6;
    }
    
    // 4. Check for prefix matching
    const maxPrefixLength = Math.min(cleanService.length, cleanRoute.length);
    let prefixLength = 0;
    for (let i = 0; i < maxPrefixLength; i++) {
      if (cleanService[i] === cleanRoute[i]) {
        prefixLength++;
      } else {
        break;
      }
    }
    
    if (prefixLength >= 3) { // At least 3 character prefix match
      score += (prefixLength / maxPrefixLength) * 0.4;
    }
    
    return Math.min(score, 1.0); // Cap at 1.0
  }

  // Clear cache (useful for testing)
  clearCache(): void {
    this.models = [];
    this.lastFetch = 0;
    logger.info('Model cache cleared');
  }
}

// Export singleton instance
export const modelService = new ModelService();