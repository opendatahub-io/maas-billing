import { exec } from 'child_process';
import { promisify } from 'util';
import { logger } from '../utils/logger';

const execAsync = promisify(exec);

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
      logger.info('Fetching models from cluster...');
      this.models = await this.fetchModelsFromCluster();
      this.lastFetch = now;
      
      logger.info(`Retrieved ${this.models.length} models from cluster`, {
        models: this.models.map(m => ({ id: m.id, namespace: m.namespace }))
      });
      
      return this.models;
    } catch (error) {
      logger.error('Failed to fetch models from cluster:', error);
      
      // If we have cached models, return them as fallback
      if (this.models.length > 0) {
        logger.warn('Using cached models due to fetch error');
        return this.models;
      }
      
      // No cache available, throw error
      throw new Error('No models available from cluster and no cached models');
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