// Environment configuration for the entire application
export const config = {
  // Backend API base URL
  API_BASE_URL: process.env.REACT_APP_API_BASE_URL || 'http://localhost:3002/api/v1',
  
  // Model serving configuration
  MODEL_SERVING: {
    // Deployment mode: 'kuadrant' for local Kuadrant with Host headers, 'direct' for direct endpoints
    mode: process.env.REACT_APP_MODEL_MODE || 'kuadrant',
    
    // Base URL for model requests (Kuadrant gateway or direct endpoints)
    baseUrl: process.env.REACT_APP_MODEL_BASE_URL || 'http://localhost:8080',
    
    // Host headers for Kuadrant domain-based routing
    hosts: {
      simulator: process.env.REACT_APP_SIMULATOR_HOST || 'simulator.maas.local',
      qwen3: process.env.REACT_APP_QWEN3_HOST || 'qwen3.maas.local',
    },
    
    // Model names
    models: {
      simulator: process.env.REACT_APP_SIMULATOR_MODEL || 'simulator-model',
      qwen3: process.env.REACT_APP_QWEN3_MODEL || 'qwen3-0-6b-instruct',
    }
  },
  
  // OpenAI API path
  OPENAI_CHAT_COMPLETIONS_PATH: '/v1/chat/completions',
  
  // Default tier-based API keys (in real deployment these would come from environment)
  API_KEYS: {
    free: process.env.REACT_APP_FREE_API_KEY || 'freeuser1_key',
    premium: process.env.REACT_APP_PREMIUM_API_KEY || 'premiumuser1_key',
    none: process.env.REACT_APP_NONE_API_KEY || '', // Empty key to test auth failures
  }
};

export default config;