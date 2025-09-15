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
    const CLUSTER_DOMAIN = process.env.CLUSTER_DOMAIN || 'apps.your-cluster.example.com';
    
    try {
      // Get InferenceServices from all namespaces
      const { stdout } = await execAsync(
        `kubectl get inferenceservices -A -o jsonpath='{range .items[*]}{.metadata.name}{"\\t"}{.metadata.namespace}{"\\t"}{.spec.predictor.model.modelFormat.name}{"\\n"}{end}'`
      );
      
      if (!stdout.trim()) {
        logger.warn('No InferenceServices found in cluster');
        return [];
      }

      const models: Model[] = [];
      const lines = stdout.trim().split('\n');
      
      for (const line of lines) {
        const [name, namespace, modelFormat] = line.split('\t');
        
        if (!name || !namespace) {
          logger.warn('Skipping invalid InferenceService entry:', line);
          continue;
        }

        // Construct endpoint URL based on naming convention
        const endpoint = `http://${name}-llm.${CLUSTER_DOMAIN}/v1/chat/completions`;
        
        // Extract display name (remove -llm suffix if present)
        const displayName = name.replace(/-llm$/, '');
        
        models.push({
          id: name,
          name: this.formatModelName(displayName),
          provider: 'KServe',
          description: `${modelFormat || 'LLM'} model served via KServe`,
          endpoint,
          namespace
        });
      }

      return models;
    } catch (error) {
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

  // Clear cache (useful for testing)
  clearCache(): void {
    this.models = [];
    this.lastFetch = 0;
    logger.info('Model cache cleared');
  }
}

// Export singleton instance
export const modelService = new ModelService();