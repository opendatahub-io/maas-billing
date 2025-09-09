const API_BASE_URL = process.env.NODE_ENV === 'production' 
  ? 'https://maas-backend-route-llm.apps.summit-gpu.octo-emerging.redhataicoe.com/api/v1'
  : 'http://localhost:3003/api/v1';

class ApiService {
  private async fetch(endpoint: string, options: RequestInit = {}) {
    const url = `${API_BASE_URL}${endpoint}`;
    
    try {
      console.log(`🌐 Making API request to: ${url}`);
      
      const response = await fetch(url, {
        headers: {
          'Content-Type': 'application/json',
          ...options.headers,
        },
        ...options,
      });

      console.log(`📡 Response status: ${response.status} ${response.statusText}`);

      if (!response.ok) {
        const errorText = await response.text();
        console.error(`❌ API Error: ${response.status} ${response.statusText}`, errorText);
        throw new Error(`HTTP error! status: ${response.status} - ${response.statusText}`);
      }

      const data = await response.json();
      console.log(`✅ API Response received:`, data);
      return data.success ? data.data : data;
    } catch (error) {
      console.error(`💥 Network error for ${url}:`, error);
      if (error instanceof TypeError && error.message.includes('Failed to fetch')) {
        throw new Error(`Network error: Unable to connect to backend. This might be due to SSL certificate issues or CORS restrictions. Check browser console for details.`);
      }
      throw error;
    }
  }

  async getModels() {
    return this.fetch('/models');
  }

  async getLiveRequests() {
    return this.fetch('/metrics/live-requests');
  }

  async getDashboardStats() {
    return this.fetch('/metrics/dashboard');
  }

  async getMetrics(timeRange: string = '1h') {
    return this.fetch(`/metrics?timeRange=${timeRange}`);
  }

  async getPolicies() {
    return this.fetch('/policies');
  }

  async createPolicy(policy: any) {
    return this.fetch('/policies', {
      method: 'POST',
      body: JSON.stringify(policy),
    });
  }

  async updatePolicy(id: string, policy: any) {
    return this.fetch(`/policies/${id}`, {
      method: 'PUT',
      body: JSON.stringify(policy),
    });
  }

  async deletePolicy(id: string) {
    return this.fetch(`/policies/${id}`, {
      method: 'DELETE',
    });
  }

  async getRequestDetails(id: string) {
    return this.fetch(`/metrics/requests/${id}`);
  }

  async getPolicyStats() {
    return this.fetch('/metrics/policy-stats');
  }

  async simulateRequest(params: {
    model: string;
    messages: Array<{role: string, content: string}>;
    max_tokens?: number;
    tier: string;
    apiKey: string;
  }) {
    return this.fetch('/simulator/chat/completions', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${params.apiKey}`,
      },
      body: JSON.stringify({
        model: params.model,
        messages: params.messages,
        max_tokens: params.max_tokens || 100,
        tier: params.tier
      }),
    });
  }

  // Token Management APIs
  async getUserTier() {
    return this.fetch('/tokens/user/tier');
  }

  async getUserTokens() {
    return this.fetch('/tokens');
  }

  async createToken(params: {
    name: string;
    description: string;
  }) {
    return this.fetch('/tokens/create', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }

  async revokeToken(tokenName: string) {
    return this.fetch(`/tokens/${tokenName}`, {
      method: 'DELETE',
    });
  }

  async testToken(params: {
    token: string;
    model: string;
    message: string;
  }) {
    return this.fetch('/tokens/test', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }
}

const apiService = new ApiService();
export default apiService;