import axios from 'axios';
import { logger } from '../utils/logger';

// OpenAI-compatible v1 API response interfaces
interface OpenAIModel {
  id: string;
  created: number;
  object: string;
  owned_by: string;
  ready: boolean;
  url?: string; // URL field from MaaS API
}

interface OpenAIModelsResponse {
  data: OpenAIModel[];
  object: string;
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

    // Fetch models from MaaS API only
    logger.info('Fetching models from MaaS API...');
    this.models = await this.fetchModelsFromMaasApi();
    this.lastFetch = now;
    
    logger.info(`Retrieved ${this.models.length} models from MaaS API`, {
      models: this.models.map(m => ({ id: m.id, namespace: m.namespace }))
    });
    
    return this.models;
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
      
      logger.info('Fetching models from MaaS API...', { url: `${maasApiUrl}/v1/models` });
      
      const response = await axios.get<OpenAIModelsResponse>(`${maasApiUrl}/v1/models`, {
        timeout: 30000,
        headers: {
          'Accept': 'application/json',
        }
      });

      if (!response.data || !Array.isArray(response.data.data)) {
        throw new Error('Invalid response format from MaaS API v1/models endpoint');
      }

      logger.info(`Retrieved ${response.data.data.length} models from MaaS API v1/models`, {
        models: response.data.data.map(m => ({ id: m.id, owned_by: m.owned_by, ready: m.ready, url: m.url }))
      });
      
      const openAIModels = response.data.data;
      
      // Use URLs directly from MaaS API response
      const models: Model[] = [];
      for (const openAIModel of openAIModels) {
        // Check if MaaS API provided a URL
        if (!openAIModel.url) {
          logger.warn(`Model ${openAIModel.id} has no URL in MaaS API response, skipping`);
          continue;
        }
        
        // Convert MaaS API URL to chat completions endpoint
        const endpoint = openAIModel.url.endsWith('/v1/chat/completions') 
          ? openAIModel.url 
          : `${openAIModel.url.replace(/\/$/, '')}/v1/chat/completions`;
        
        models.push({
          id: openAIModel.id,
          name: this.formatModelName(openAIModel.id),
          provider: 'KServe LLM',
          description: `LLM model served via KServe (from MaaS API v1)`,
          endpoint,
          namespace: openAIModel.owned_by
        });
        
        logger.info(`Added model ${openAIModel.id} with endpoint: ${endpoint}`);
      }

      return models;
    } catch (error: any) {
      logger.error('Failed to fetch models from MaaS API:', error);
      
      if (error.code === 'ECONNREFUSED') {
        throw new Error(`MaaS API service is not available at ${process.env.MAAS_API_URL}. Please ensure the service is running.`);
      }
      
      throw new Error(`Failed to fetch models from MaaS API: ${error.message}`);
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