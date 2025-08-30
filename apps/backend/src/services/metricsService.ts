import axios from 'axios';
import * as k8s from '@kubernetes/client-node';
import { logger } from '../utils/logger';
import fs from 'fs';

export interface ModelInferenceData {
  requestId: string;
  modelName: string;
  modelVersion?: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  responseTime: number;
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
  processingTime?: number;
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
}

export interface RequestLogEntry {
  timestamp: string;
  requestId: string;
  method: string;
  path: string;
  statusCode: number;
  responseTime: number;
  userAgent?: string;
  sourceIP?: string;
  policyDecisions?: PolicyDecisionDetails[];
  authentication?: AuthenticationDetails;
  rateLimits?: RateLimitDetails[];
  decision: 'accept' | 'reject';
  policyType?: 'AuthPolicy' | 'RateLimitPolicy';
  modelInference?: ModelInferenceData;
  namespace?: string;
  service?: string;
  headers?: Record<string, string>;
}

class MetricsService {
  private kc: k8s.KubeConfig;
  private k8sApi: k8s.CoreV1Api;
  private serviceAccountToken: string | null = null;
  
  constructor() {
    this.kc = new k8s.KubeConfig();
    this.kc.loadFromCluster();
    this.k8sApi = this.kc.makeApiClient(k8s.CoreV1Api);
    
    // Load service account token for making authenticated requests
    try {
      this.serviceAccountToken = fs.readFileSync('/var/run/secrets/kubernetes.io/serviceaccount/token', 'utf8');
    } catch (error) {
      logger.warn('Could not read service account token:', error);
    }
  }

  // Core Prometheus metrics fetching with HTTP API
  async fetchPrometheusMetrics(): Promise<any> {
    try {
      logger.info('Fetching metrics directly from Prometheus...');
      
      const [istioRequests, limitadorRejected, authorinoResponses] = await Promise.all([
        this.fetchFromPrometheus('istio_requests_total'),
        this.fetchFromPrometheus('limitador_limit_checks_total'),
        this.fetchFromPrometheus('http_requests_total{job=~".*authorino.*"}')
      ]);

      // Parse and aggregate results
      const totalRequests = this.sumMetricValues(istioRequests);
      const authFailedRequests = this.sumMetricValues(authorinoResponses.filter((m: any) => 
        m.metric?.status_code && parseInt(m.metric.status_code) === 401
      ));
      const rateLimitedRequests = this.sumMetricValues(limitadorRejected.filter((m: any) => 
        m.metric?.result === 'over_limit'
      ));
      const successRequests = totalRequests - authFailedRequests - rateLimitedRequests;

      logger.info(`Prometheus metrics fetched: ${totalRequests} total, ${successRequests} success, ${authFailedRequests} auth failures, ${rateLimitedRequests} rate limited`);

      return {
        totalRequests,
        successRequests,
        authFailedRequests,
        rateLimitedRequests,
        authByNamespace: this.groupMetricsByNamespace(authorinoResponses),
        limitsByNamespace: this.groupMetricsByNamespace(limitadorRejected)
      };
    } catch (error: any) {
      logger.error('Failed to fetch Prometheus metrics:', error.message);
      return null;
    }
  }

  // Fetch metrics from Prometheus with multiple endpoint fallbacks
  private async fetchFromPrometheus(query: string): Promise<any[]> {
    const prometheusEndpoints = [
      'http://thanos-querier.openshift-monitoring.svc.cluster.local:9091',
      'http://prometheus-k8s.openshift-monitoring.svc.cluster.local:9090',
      'http://prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9091'
    ];

    for (const endpoint of prometheusEndpoints) {
      try {
        const url = `${endpoint}/api/v1/query?query=${encodeURIComponent(query)}`;
        
        const headers: Record<string, string> = {
          'Accept': 'application/json'
        };

        // Add authorization header if we have a service account token
        if (this.serviceAccountToken) {
          headers['Authorization'] = `Bearer ${this.serviceAccountToken}`;
        }

        const response = await axios.get(url, {
          headers,
          timeout: 10000
        });

        if (response.data && response.data.status === 'success') {
          return response.data.data.result || [];
        }
      } catch (error: any) {
        logger.warn(`Failed to connect to ${endpoint}: ${error.message}`);
      }
    }

    logger.error('All Prometheus endpoints failed');
    return [];
  }

  // Get pod logs using Kubernetes API instead of kubectl
  async getRealLiveRequests(): Promise<RequestLogEntry[]> {
    try {
      logger.info('Fetching live requests from Kubernetes API...');
      
      const namespace = process.env.NAMESPACE || 'llm';
      const requests: RequestLogEntry[] = [];

      // Get gateway pods
      const gatewayPods = await this.findPods(namespace, { 'app': 'gateway' });
      
      for (const pod of gatewayPods) {
        try {
          const logs = await this.getPodLogs(namespace, pod.metadata?.name || '', { tailLines: 100, sinceSeconds: 3600 });
          const parsedRequests = this.parseAccessLogs(logs);
          requests.push(...parsedRequests);
        } catch (error: any) {
          logger.warn(`Failed to get logs from pod ${pod.metadata?.name}: ${error.message}`);
        }
      }

      logger.info(`Parsed ${requests.length} live requests from logs`);
      return requests.slice(0, 100); // Limit to recent 100 requests
    } catch (error: any) {
      logger.error('Failed to fetch live requests:', error.message);
      return [];
    }
  }

  // Get Kuadrant component status using Kubernetes API
  async getMetricsStatus(): Promise<any> {
    try {
      logger.info('Checking Kuadrant components status...');
      
      const status = {
        limitadorConnected: false,
        authorinoConnected: false,
        hasRealTraffic: false,
        lastUpdate: new Date().toISOString()
      };

      // Check Limitador pods
      const limitadorPods = await this.findPods('kuadrant-system', { 'app': 'limitador' });
      status.limitadorConnected = limitadorPods.length > 0 && 
        limitadorPods.some(pod => pod.status?.phase === 'Running');

      // Check Authorino pods  
      const authorinoPods = await this.findPods('kuadrant-system', { 'control-plane': 'controller-manager' });
      status.authorinoConnected = authorinoPods.length > 0 && 
        authorinoPods.some(pod => pod.status?.phase === 'Running');

      // Check for recent traffic by looking at recent requests
      const recentRequests = await this.getRealLiveRequests();
      status.hasRealTraffic = recentRequests.length > 0;

      logger.info('Component status check completed', status);
      return status;
    } catch (error: any) {
      logger.error('Failed to check component status:', error.message);
      return {
        limitadorConnected: false,
        authorinoConnected: false,
        hasRealTraffic: false,
        lastUpdate: new Date().toISOString(),
        error: error.message
      };
    }
  }

  // Helper methods using Kubernetes API

  private async findPods(namespace: string, labels: Record<string, string>): Promise<k8s.V1Pod[]> {
    try {
      const labelSelector = Object.entries(labels)
        .map(([key, value]) => `${key}=${value}`)
        .join(',');

      const response = await this.k8sApi.listNamespacedPod(
        namespace,
        undefined, // pretty
        undefined, // allowWatchBookmarks
        undefined, // continue
        undefined, // fieldSelector
        labelSelector // labelSelector
      );

      return response.body.items || [];
    } catch (error: any) {
      logger.warn(`Failed to find pods in namespace ${namespace} with labels ${JSON.stringify(labels)}: ${error.message}`);
      return [];
    }
  }

  private async getPodLogs(namespace: string, podName: string, options: { tailLines?: number, sinceSeconds?: number } = {}): Promise<string> {
    try {
      const response = await this.k8sApi.readNamespacedPodLog(
        podName,
        namespace,
        undefined, // container
        undefined, // follow
        undefined, // insecureSkipTLSVerifyBackend
        undefined, // limitBytes
        undefined, // pretty
        undefined, // previous
        options.sinceSeconds, // sinceSeconds
        options.tailLines, // tailLines
        undefined // timestamps
      );

      return response.body || '';
    } catch (error: any) {
      logger.warn(`Failed to get logs for pod ${podName}: ${error.message}`);
      return '';
    }
  }

  // Direct HTTP access to component metrics endpoints
  private async fetchComponentMetrics(serviceName: string, namespace: string, port: number, path: string = '/metrics'): Promise<string> {
    try {
      const url = `http://${serviceName}.${namespace}.svc.cluster.local:${port}${path}`;
      
      const headers: Record<string, string> = {};
      if (this.serviceAccountToken) {
        headers['Authorization'] = `Bearer ${this.serviceAccountToken}`;
      }

      const response = await axios.get(url, {
        headers,
        timeout: 5000
      });

      return response.data || '';
    } catch (error: any) {
      logger.warn(`Failed to fetch metrics from ${serviceName}.${namespace}:${port}${path}: ${error.message}`);
      return '';
    }
  }

  async fetchLimitadorMetrics(): Promise<string> {
    return this.fetchComponentMetrics('limitador', 'kuadrant-system', 8080);
  }

  async fetchAuthorinoMetrics(): Promise<string> {
    return this.fetchComponentMetrics('authorino-controller-manager-metrics-service', 'kuadrant-system', 8080);
  }

  async fetchIstioMetrics(): Promise<string> {
    // Try to get Istio metrics from the gateway
    const namespace = process.env.NAMESPACE || 'llm';
    return this.fetchComponentMetrics('inference-gateway-istio', namespace, 15090, '/stats/prometheus');
  }

  // Utility methods for parsing metrics data

  private sumMetricValues(metrics: any[]): number {
    return metrics.reduce((sum, metric) => {
      const value = parseFloat(metric.value?.[1] || '0');
      return sum + value;
    }, 0);
  }

  private groupMetricsByNamespace(metrics: any[]): Map<string, number> {
    const byNamespace = new Map<string, number>();
    
    metrics.forEach(metric => {
      const namespace = metric.metric?.namespace || 'unknown';
      const value = parseFloat(metric.value?.[1] || '0');
      byNamespace.set(namespace, (byNamespace.get(namespace) || 0) + value);
    });

    return byNamespace;
  }

  private parseAccessLogs(logs: string): RequestLogEntry[] {
    const entries: RequestLogEntry[] = [];
    const lines = logs.split('\n').filter(line => line.trim());

    for (const line of lines) {
      try {
        // Try to parse as JSON first (structured logs)
        if (line.includes('{') && line.includes('}')) {
          const jsonMatch = line.match(/\{.*\}/);
          if (jsonMatch) {
            const logEntry = JSON.parse(jsonMatch[0]);
            const entry = this.transformLogEntry(logEntry);
            if (entry) entries.push(entry);
          }
        } else {
          // Parse common access log format
          const entry = this.parseCommonLogFormat(line);
          if (entry) entries.push(entry);
        }
      } catch (error) {
        // Skip unparseable log lines
      }
    }

    return entries;
  }

  private transformLogEntry(logEntry: any): RequestLogEntry | null {
    try {
      return {
        timestamp: logEntry.timestamp || logEntry.time || new Date().toISOString(),
        requestId: logEntry.request_id || logEntry.requestId || `req-${Date.now()}`,
        method: logEntry.method || 'GET',
        path: logEntry.path || logEntry.url || '/',
        statusCode: parseInt(logEntry.status_code || logEntry.status || '200'),
        responseTime: parseFloat(logEntry.response_time || logEntry.duration || '0'),
        sourceIP: logEntry.source_ip || logEntry.remote_addr,
        userAgent: logEntry.user_agent,
        decision: logEntry.status_code && parseInt(logEntry.status_code) < 400 ? 'accept' : 'reject',
        headers: logEntry.headers
      };
    } catch (error) {
      return null;
    }
  }

  private parseCommonLogFormat(line: string): RequestLogEntry | null {
    // Parse common access log format: IP - - [timestamp] "METHOD path HTTP/1.1" status size
    const commonLogRegex = /^(\S+) \S+ \S+ \[(.*?)\] "(\S+) (\S+) \S+" (\d+) (\d+|-)/;
    const match = line.match(commonLogRegex);

    if (match) {
      const [, sourceIP, timestamp, method, path, statusCode, size] = match;
      return {
        timestamp: new Date().toISOString(), // Convert log timestamp if needed
        requestId: `req-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        method,
        path,
        statusCode: parseInt(statusCode),
        responseTime: 0, // Not available in basic format
        sourceIP,
        decision: parseInt(statusCode) < 400 ? 'accept' : 'reject'
      };
    }

    return null;
  }

  // Mock data methods for fallback/testing
  generateMockRequests(count: number = 50): RequestLogEntry[] {
    const requests: RequestLogEntry[] = [];
    const now = Date.now();

    for (let i = 0; i < count; i++) {
      const timestamp = new Date(now - (i * 60000)).toISOString(); // Spread over last hour
      const statusCode = Math.random() > 0.8 ? (Math.random() > 0.5 ? 401 : 429) : 200;
      
      requests.push({
        timestamp,
        requestId: `req-${now}-${i}`,
        method: Math.random() > 0.5 ? 'POST' : 'GET',
        path: `/v1/chat/completions`,
        statusCode,
        responseTime: Math.floor(Math.random() * 2000) + 100,
        sourceIP: `10.0.0.${Math.floor(Math.random() * 255)}`,
        decision: statusCode === 200 ? 'accept' : 'reject',
        policyType: statusCode === 401 ? 'AuthPolicy' : statusCode === 429 ? 'RateLimitPolicy' : undefined
      });
    }

    return requests;
  }
}

export default new MetricsService();