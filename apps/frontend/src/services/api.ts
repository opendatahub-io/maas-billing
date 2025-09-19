const API_BASE_URL = process.env.REACT_APP_USE_CLUSTER_BACKEND === 'true'
  ? `https://maas-backend-route-llm.${process.env.REACT_APP_CLUSTER_DOMAIN || 'your-cluster.example.com'}/api/v1`
  : process.env.NODE_ENV === 'production' 
    ? `https://maas-backend-route-llm.${process.env.REACT_APP_CLUSTER_DOMAIN || 'your-cluster.example.com'}/api/v1`
    : 'http://localhost:3001/api/v1';

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
      
      // Handle 204/205 responses with no content
      const hasBody = response.status !== 204 && response.status !== 205;
      const contentType = response.headers.get('content-type') || '';
      
      let body: any = null;
      if (hasBody) {
        const rawBody = await response.text();
        
        // Try to parse as JSON if content-type suggests it
        if (contentType.includes('application/json') && rawBody.trim()) {
          try {
            body = JSON.parse(rawBody);
          } catch (e) {
            // If JSON parsing fails, keep raw text
            body = rawBody;
          }
        } else {
          body = rawBody;
        }
      }

      if (!response.ok) {
        console.error(`❌ API Error: ${response.status} ${response.statusText}`, body);
        const error: any = new Error(`HTTP ${response.status} ${response.statusText}`);
        error.status = response.status;
        error.response = {
          status: response.status,
          statusText: response.statusText,
          headers: Object.fromEntries(response.headers.entries()),
          body: body
        };
        throw error;
      }

      console.log(`✅ API Response received:`, body);
      
      // Handle success response format
      if (body && typeof body === 'object' && 'success' in body) {
        return body.success ? body.data : body;
      }
      
      return body;
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
    apiKey: string;
    authPrefix?: string;
  }) {
    const authPrefix = params.authPrefix || 'Bearer';
    return this.fetch('/simulator/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `${authPrefix} ${params.apiKey}`,
      },
      body: JSON.stringify({
        model: params.model,
        messages: params.messages,
        max_tokens: params.max_tokens || 100
      }),
    });
  }

  // Token Management APIs

  async createToken(params: {
    expiration?: string; // Optional expiration, e.g. '1h', '4h', '24h'. If not provided, uses MaaS API default (4h)
  }) {
    return this.fetch('/tokens/create', {
      method: 'POST',
      body: JSON.stringify(params),
    });
  }

  async deleteTokens() {
    return this.fetch('/tokens/delete', {
      method: 'DELETE',
    });
  }


  // OAuth APIs
  async exchangeOAuthCode(code: string, redirectUri?: string) {
    return this.fetch('/auth/oauth/exchange', {
      method: 'POST',
      body: JSON.stringify({
        code: code,
        redirect_uri: redirectUri || window.location.origin + '/auth/callback'
      }),
    });
  }

  async getClusterStatus() {
    return this.fetch('/cluster/status');
  }

  async getUserInfo() {
    return this.fetch('/user');
  }

}

const apiService = new ApiService();
export default apiService;