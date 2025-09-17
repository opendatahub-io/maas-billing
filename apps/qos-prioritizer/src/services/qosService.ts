import PQueue from 'p-queue';
import axios from 'axios';
import { EventEmitter } from 'events';
import { Server as SocketIOServer } from 'socket.io';
import { logger } from '../utils/logger';

interface QueueRequest {
  id: string;
  tier: 'enterprise' | 'premium' | 'free';
  prompt: string;
  timestamp: number;
  resolve: (response: any) => void;
  reject: (error: Error) => void;
  isAdvancedDemo?: boolean;
  isSimulationDemo?: boolean;
  priority?: number;
  aging?: number;
}

interface ProcessingRequest {
  id: string;
  tier: 'enterprise' | 'premium' | 'free';
  startTime: number;
  state: 'queued' | 'processing' | 'completed';
  priority: number;
  request: QueueRequest;
}

interface CapacityInfo {
  maxConcurrent: number;
  currentProcessing: number;
  available: number;
  avgResponseTime: number;
  errorRate: number;
}

interface QoSMetrics {
  totalRequests: number;
  activeRequests: number;
  enterpriseQueue: number;
  premiumQueue: number;
  freeQueue: number;
  avgResponseTime: number;
  // New metrics for immediate processing tracking
  immediateProcessing: {
    enterprise: number;
    premium: number;
    free: number;
  };
  totalImmediate: number;
  totalQueued: number;
}

interface DetailedQoSStats {
  timestamp: string;
  queues: {
    enterprise: {
      size: number;
      pending: number;
      concurrency: number;
      isPaused: boolean;
    };
    premium: {
      size: number;
      pending: number;
      concurrency: number;
      isPaused: boolean;
    };
    free: {
      size: number;
      pending: number;
      concurrency: number;
      isPaused: boolean;
    };
  };
  performance: {
    totalProcessed: number;
    processingRate: number;
    avgWaitTime: number;
    avgProcessingTime: number;
  };
  activeRequests: Array<{
    id: string;
    tier: string;
    startTime: number;
    waitTime: number;
  }>;
}

class WorkConservingScheduler {
  private queuedRequests = new Map<string, ProcessingRequest>();
  private tierWeights = { enterprise: 0.7, premium: 0.2, free: 0.1 };
  private basePriorities = { enterprise: 100, premium: 50, free: 10 };
  
  enqueueRequest(request: QueueRequest): void {
    const priority = this.calculatePriority(request.tier, request.timestamp);
    const processingRequest: ProcessingRequest = {
      id: request.id,
      tier: request.tier,
      startTime: Date.now(),
      state: 'queued',
      priority,
      request: { ...request, priority }
    };
    
    this.queuedRequests.set(request.id, processingRequest);
  }
  
  getNextRequest(capacityInfo: CapacityInfo): QueueRequest | null {
    // Work-conserving priority: Always prefer higher business priority
    const queuedByPriority = Array.from(this.queuedRequests.values())
      .filter(req => req.state === 'queued')
      .sort((a, b) => b.priority - a.priority);
    
    if (queuedByPriority.length === 0) return null;
    
    // Apply proportional fairness if needed
    const nextRequest = this.applyProportionalFairness(queuedByPriority, capacityInfo);
    if (nextRequest) {
      this.queuedRequests.get(nextRequest.id)!.state = 'processing';
      return nextRequest.request;
    }
    
    return null;
  }
  
  private calculatePriority(tier: string, timestamp: number): number {
    const basePriority = this.basePriorities[tier as keyof typeof this.basePriorities] || 10;
    
    // Dynamic aging to prevent starvation
    const age = Date.now() - timestamp;
    const agingBonus = Math.min(age / 1000, 20); // Max 20 point bonus over 20 seconds
    
    return basePriority + agingBonus;
  }
  
  private applyProportionalFairness(requests: ProcessingRequest[], capacityInfo: CapacityInfo): ProcessingRequest | null {
    // Simple proportional fairness: check if any tier is over its fair share
    const processingByTier = this.getProcessingCountByTier();
    const targetShares = this.calculateTargetShares(capacityInfo.maxConcurrent);
    
    // Find first tier that's under its fair share
    for (const tier of ['enterprise', 'premium', 'free']) {
      const currentProcessing = processingByTier[tier] || 0;
      const targetShare = targetShares[tier];
      
      if (currentProcessing < targetShare) {
        const tierRequest = requests.find(req => req.tier === tier);
        if (tierRequest) return tierRequest;
      }
    }
    
    // If all tiers are at their fair share, use strict priority
    return requests[0];
  }
  
  private getProcessingCountByTier(): Record<string, number> {
    const counts = { enterprise: 0, premium: 0, free: 0 };
    for (const req of this.queuedRequests.values()) {
      if (req.state === 'processing') {
        counts[req.tier as keyof typeof counts]++;
      }
    }
    return counts;
  }
  
  private calculateTargetShares(maxConcurrent: number): Record<string, number> {
    return {
      enterprise: Math.floor(maxConcurrent * this.tierWeights.enterprise),
      premium: Math.floor(maxConcurrent * this.tierWeights.premium),
      free: Math.floor(maxConcurrent * this.tierWeights.free)
    };
  }
  
  completeRequest(requestId: string): void {
    const request = this.queuedRequests.get(requestId);
    if (request) {
      request.state = 'completed';
      this.queuedRequests.delete(requestId);
    }
  }
  
  getQueueStats(): { queued: Record<string, number>; processing: Record<string, number> } {
    const queued = { enterprise: 0, premium: 0, free: 0 };
    const processing = { enterprise: 0, premium: 0, free: 0 };
    
    for (const req of this.queuedRequests.values()) {
      if (req.state === 'queued') {
        queued[req.tier as keyof typeof queued]++;
      } else if (req.state === 'processing') {
        processing[req.tier as keyof typeof processing]++;
      }
    }
    
    return { queued, processing };
  }
}

class CapacityManager {
  private capacityHistory: Array<{ timestamp: number; concurrent: number; responseTime: number }> = [];
  private errorHistory: Array<{ timestamp: number; errors: number; total: number }> = [];
  
  recordRequestStart(): void {
    // Track concurrent requests for capacity detection
    const now = Date.now();
    this.capacityHistory.push({ timestamp: now, concurrent: 1, responseTime: 0 });
    this.cleanupHistory();
  }
  
  recordRequestComplete(responseTime: number, isError: boolean): void {
    const now = Date.now();
    
    // Update response time in latest entry
    if (this.capacityHistory.length > 0) {
      const latest = this.capacityHistory[this.capacityHistory.length - 1];
      latest.responseTime = responseTime;
    }
    
    // Track error rates
    this.errorHistory.push({ timestamp: now, errors: isError ? 1 : 0, total: 1 });
    this.cleanupHistory();
  }
  
  getCurrentCapacity(): CapacityInfo {
    const recentHistory = this.capacityHistory.filter(h => Date.now() - h.timestamp < 60000); // Last minute
    const recentErrors = this.errorHistory.filter(h => Date.now() - h.timestamp < 60000);
    
    const avgResponseTime = recentHistory.length > 0 
      ? recentHistory.reduce((sum, h) => sum + h.responseTime, 0) / recentHistory.length
      : 1000;
    
    const errorRate = recentErrors.length > 0
      ? recentErrors.reduce((sum, h) => sum + h.errors, 0) / recentErrors.reduce((sum, h) => sum + h.total, 0)
      : 0;
    
    // Dynamic capacity calculation based on performance
    const baseCapacity = this.calculateOptimalConcurrency(avgResponseTime);
    
    return {
      maxConcurrent: baseCapacity,
      currentProcessing: 0, // Will be set by caller
      available: baseCapacity,
      avgResponseTime,
      errorRate
    };
  }
  
  private calculateOptimalConcurrency(avgResponseTime: number): number {
    // Dynamic concurrency based on response time
    // Fast responses -> allow more concurrency
    // Slow responses -> limit concurrency
    if (avgResponseTime < 1000) return 6;      // Very fast: 6 concurrent
    if (avgResponseTime < 5000) return 4;      // Fast: 4 concurrent  
    if (avgResponseTime < 15000) return 2;     // Medium: 2 concurrent
    return 1;                                  // Slow: 1 concurrent
  }
  
  private cleanupHistory(): void {
    const cutoff = Date.now() - 300000; // Keep 5 minutes of history
    this.capacityHistory = this.capacityHistory.filter(h => h.timestamp > cutoff);
    this.errorHistory = this.errorHistory.filter(h => h.timestamp > cutoff);
  }
}

class UniversalCircuitBreaker {
  private state: 'CLOSED' | 'OPEN' | 'HALF_OPEN' = 'CLOSED';
  private errorCounts = { enterprise: 0, premium: 0, free: 0 };
  private lastFailureTime = 0;
  private successCount = 0;
  
  private readonly ERROR_THRESHOLDS = { enterprise: 0.01, premium: 0.05, free: 0.20 };
  private readonly TIMEOUT_WINDOW = 30000; // 30 seconds
  private readonly SUCCESS_THRESHOLD = 3;
  
  canProcess(tier: string, capacityInfo: CapacityInfo): boolean {
    if (this.state === 'OPEN') {
      if (Date.now() - this.lastFailureTime > this.TIMEOUT_WINDOW) {
        this.state = 'HALF_OPEN';
        this.successCount = 0;
      } else {
        return false;
      }
    }
    
    // Check tier-specific error thresholds
    const threshold = this.ERROR_THRESHOLDS[tier as keyof typeof this.ERROR_THRESHOLDS] || 0.20;
    return capacityInfo.errorRate <= threshold;
  }
  
  recordSuccess(): void {
    if (this.state === 'HALF_OPEN') {
      this.successCount++;
      if (this.successCount >= this.SUCCESS_THRESHOLD) {
        this.state = 'CLOSED';
        this.errorCounts = { enterprise: 0, premium: 0, free: 0 };
      }
    }
  }
  
  recordFailure(tier: string): void {
    this.errorCounts[tier as keyof typeof this.errorCounts]++;
    this.lastFailureTime = Date.now();
    
    // Trip circuit breaker if too many failures
    const totalFailures = Object.values(this.errorCounts).reduce((sum, count) => sum + count, 0);
    if (totalFailures >= 5) {
      this.state = 'OPEN';
    }
  }
  
  getState(): string {
    return this.state;
  }
}

export class QoSService extends EventEmitter {
  // Keep existing p-queue implementation for backward compatibility
  private enterpriseQueue: PQueue;
  private premiumQueue: PQueue;
  private freeQueue: PQueue;
  private metrics: QoSMetrics;
  private simulationMode: boolean;
  private modelEndpoint: string;
  private io?: SocketIOServer;
  private activeRequests: Map<string, { id: string; tier: string; startTime: number; waitTime: number }>;
  private performanceHistory: Array<{ timestamp: number; processingTime: number; waitTime: number }>;
  private metricsInterval?: NodeJS.Timeout;
  
  // New environment-agnostic algorithm components
  private scheduler: WorkConservingScheduler;
  private capacityManager: CapacityManager;
  private circuitBreaker: UniversalCircuitBreaker;
  private processingRequests = new Map<string, ProcessingRequest>();
  private useNewAlgorithm: boolean;
  
  constructor(io?: SocketIOServer) {
    super();
    this.io = io;
    this.simulationMode = process.env.SIMULATION_MODE === 'true';
    this.modelEndpoint = 'http://localhost:8004';
    this.activeRequests = new Map();
    this.performanceHistory = [];
    
    // Initialize new algorithm components
    this.scheduler = new WorkConservingScheduler();
    this.capacityManager = new CapacityManager();
    this.circuitBreaker = new UniversalCircuitBreaker();
    this.useNewAlgorithm = process.env.USE_NEW_ALGORITHM !== 'false'; // Default to new algorithm
    
    // Create priority queues with different concurrency
    this.enterpriseQueue = new PQueue({ 
      concurrency: 3         // Enterprise gets most resources
    });
    
    this.premiumQueue = new PQueue({ 
      concurrency: 2         // Premium gets medium resources  
    });
    
    this.freeQueue = new PQueue({ 
      concurrency: 1         // Free gets least resources
    });
    
    this.metrics = {
      totalRequests: 0,
      activeRequests: 0,
      enterpriseQueue: 0,
      premiumQueue: 0,
      freeQueue: 0,
      avgResponseTime: 0,
      immediateProcessing: {
        enterprise: 0,
        premium: 0,
        free: 0
      },
      totalImmediate: 0,
      totalQueued: 0
    };
    
    this.setupEventHandlers();
    this.startMetricsBroadcast();
    
    logger.info('QoS Service initialized with p-queue', {
      simulationMode: this.simulationMode,
      endpoint: this.modelEndpoint,
      socketIO: !!this.io
    });
  }
  
  /**
   * Process request with environment-agnostic priority queuing
   */
  async processRequest(
    id: string,
    tier: 'enterprise' | 'premium' | 'free',
    prompt: string,
    userContext: any
  ): Promise<any> {
    const startTime = Date.now();
    
    logger.info('Processing QoS request', {
      requestId: id,
      tier,
      algorithm: this.useNewAlgorithm ? 'work-conserving' : 'p-queue',
      queueSizes: this.getQueueSizes()
    });
    
    this.metrics.totalRequests++;
    
    // Track the request
    this.activeRequests.set(id, {
      id,
      tier,
      startTime,
      waitTime: 0
    });
    
    // Emit request started event
    this.broadcastEvent('request_queued', {
      requestId: id,
      tier,
      queueSizes: this.getQueueSizes(),
      timestamp: new Date().toISOString()
    });
    
    return new Promise((resolve, reject) => {
      const queueRequest: QueueRequest = {
        id,
        tier,
        prompt,
        timestamp: startTime,
        resolve,
        reject,
        isAdvancedDemo: userContext?.isAdvancedDemo || false,
        isSimulationDemo: userContext?.isSimulationDemo || false
      };
      
      if (this.useNewAlgorithm) {
        // Use new environment-agnostic algorithm
        this.processWithNewAlgorithm(queueRequest);
      } else {
        // Fallback to existing p-queue implementation
        const queue = this.selectQueue(tier);
        const priority = this.calculatePriority(tier, startTime);
        
        queue.add(
          () => this.executeRequest(queueRequest),
          { priority }
        ).then(resolve).catch(reject);
      }
      
      this.updateQueueMetrics();
      this.broadcastQueueUpdate();
    });
  }
  
  /**
   * Process request using the new Work-Conserving Priority algorithm
   */
  private async processWithNewAlgorithm(request: QueueRequest): Promise<void> {
    // Get current system capacity
    const capacityInfo = this.capacityManager.getCurrentCapacity();
    capacityInfo.currentProcessing = this.processingRequests.size;
    capacityInfo.available = Math.max(0, capacityInfo.maxConcurrent - capacityInfo.currentProcessing);
    
    // Check circuit breaker
    if (!this.circuitBreaker.canProcess(request.tier, capacityInfo)) {
      const error = new Error(`Circuit breaker open for tier: ${request.tier}`);
      request.reject(error);
      logger.warn('Request rejected by circuit breaker', {
        requestId: request.id,
        tier: request.tier,
        circuitState: this.circuitBreaker.getState()
      });
      return;
    }
    
    // If we have available capacity, process immediately
    if (capacityInfo.available > 0) {
      // Track immediate processing
      this.metrics.immediateProcessing[request.tier as keyof typeof this.metrics.immediateProcessing]++;
      this.metrics.totalImmediate++;
      
      logger.info('Request processed immediately (bypassed queue)', {
        requestId: request.id,
        tier: request.tier,
        availableCapacity: capacityInfo.available,
        algorithm: 'work-conserving'
      });
      
      this.dispatchRequest(request);
    } else {
      // Track queued processing
      this.metrics.totalQueued++;
      
      logger.info('Request added to queue (capacity full)', {
        requestId: request.id,
        tier: request.tier,
        availableCapacity: capacityInfo.available,
        algorithm: 'work-conserving'
      });
      
      // Add to scheduler queue
      this.scheduler.enqueueRequest(request);
      this.processNextAvailable();
    }
  }
  
  /**
   * Dispatch request immediately to backend
   */
  private async dispatchRequest(request: QueueRequest): Promise<void> {
    const processingRequest: ProcessingRequest = {
      id: request.id,
      tier: request.tier,
      startTime: Date.now(),
      state: 'processing',
      priority: request.priority || 0,
      request
    };
    
    this.processingRequests.set(request.id, processingRequest);
    this.capacityManager.recordRequestStart();
    
    try {
      const response = await this.executeRequest(request);
      request.resolve(response);
      this.onRequestCompleted(request.id, false);
    } catch (error) {
      request.reject(error instanceof Error ? error : new Error(String(error)));
      this.onRequestCompleted(request.id, true);
    }
  }
  
  /**
   * Handle request completion and trigger next request
   */
  private onRequestCompleted(requestId: string, isError: boolean): void {
    const processingRequest = this.processingRequests.get(requestId);
    if (processingRequest) {
      const responseTime = Date.now() - processingRequest.startTime;
      
      // Record metrics
      this.capacityManager.recordRequestComplete(responseTime, isError);
      
      if (isError) {
        this.circuitBreaker.recordFailure(processingRequest.tier);
      } else {
        this.circuitBreaker.recordSuccess();
      }
      
      // Remove from processing
      this.processingRequests.delete(requestId);
      this.scheduler.completeRequest(requestId);
      
      // Process next queued request
      this.processNextAvailable();
    }
  }
  
  /**
   * Process next available request from scheduler
   */
  private processNextAvailable(): void {
    const capacityInfo = this.capacityManager.getCurrentCapacity();
    capacityInfo.currentProcessing = this.processingRequests.size;
    capacityInfo.available = Math.max(0, capacityInfo.maxConcurrent - capacityInfo.currentProcessing);
    
    if (capacityInfo.available > 0) {
      const nextRequest = this.scheduler.getNextRequest(capacityInfo);
      if (nextRequest) {
        this.dispatchRequest(nextRequest);
      }
    }
  }
  
  /**
   * Execute the actual request (simulation or real LLM)
   */
  private async executeRequest(request: QueueRequest): Promise<any> {
    const startTime = Date.now();
    this.metrics.activeRequests++;
    
    try {
      let response;
      
      // Check demo mode flags
      const isAdvancedDemo = request.isAdvancedDemo || false;
      const isSimulationDemo = request.isSimulationDemo || false;
      
      // Determine processing mode:
      // - isSimulationDemo=true: Force simulation (regardless of service mode)
      // - isAdvancedDemo=true: Force real LLM (regardless of service mode)  
      // - Otherwise: Use service default (simulationMode setting)
      if (isSimulationDemo || (this.simulationMode && !isAdvancedDemo)) {
        response = await this.simulateResponse(request);
      } else {
        response = await this.callLLM(request);
      }
      
      const processingTime = Date.now() - startTime;
      const queueTime = startTime - request.timestamp;
      
      // Store performance data
      this.performanceHistory.push({
        timestamp: Date.now(),
        processingTime,
        waitTime: queueTime
      });
      
      // Keep only last 100 entries
      if (this.performanceHistory.length > 100) {
        this.performanceHistory.shift();
      }
      
      logger.info('Request completed', {
        requestId: request.id,
        tier: request.tier,
        processingTime,
        queueTime,
        simulation: this.simulationMode
      });
      
      // Emit completion event
      this.broadcastEvent('request_completed', {
        requestId: request.id,
        tier: request.tier,
        processingTime,
        queueTime,
        queueSizes: this.getQueueSizes(),
        timestamp: new Date().toISOString()
      });
      
      this.updateMetrics(processingTime);
      this.broadcastQueueUpdate();
      return response;
      
    } finally {
      this.metrics.activeRequests--;
      this.activeRequests.delete(request.id);
    }
  }
  
  /**
   * Simulate LLM response for testing
   */
  private async simulateResponse(request: QueueRequest): Promise<any> {
    // Same processing time for all tiers to demonstrate pure QoS prioritization
    // (Real QoS benefits come from queue management, not faster hardware)
    const delay = 4000;
    
    await new Promise(resolve => setTimeout(resolve, delay));
    
    return {
      id: `sim-${Date.now()}`,
      object: 'chat.completion',
      created: Math.floor(Date.now() / 1000),
      model: 'gpt2-medium-sim',
      choices: [{
        index: 0,
        message: {
          role: 'assistant',
          content: `Simulated ${request.tier.toUpperCase()} response: ${request.prompt.substring(0, 50)}... [Priority processing completed]`
        },
        finish_reason: 'stop'
      }],
      usage: { prompt_tokens: 10, completion_tokens: 20, total_tokens: 30 }
    };
  }
  
  /**
   * Call real LLM model
   */
  private async callLLM(request: QueueRequest): Promise<any> {
    try {
      const response = await axios.post(`${this.modelEndpoint}/v1/chat/completions`, {
        model: 'gpt2-medium',
        messages: [{ role: 'user', content: request.prompt }],
        max_tokens: 50,
        temperature: 0.7
      }, {
        headers: { 'Content-Type': 'application/json' },
        timeout: 180000 // 3 minutes
      });
      
      return response.data;
      
    } catch (error: any) {
      logger.error('LLM request failed', {
        requestId: request.id,
        error: error.message,
        endpoint: this.modelEndpoint
      });
      throw error;
    }
  }
  
  /**
   * Select appropriate queue based on tier
   */
  private selectQueue(tier: string): PQueue {
    switch (tier) {
      case 'enterprise': return this.enterpriseQueue;
      case 'premium': return this.premiumQueue;  
      case 'free': return this.freeQueue;
      default: return this.freeQueue;
    }
  }
  
  /**
   * Calculate priority score (higher = more important)
   */
  private calculatePriority(tier: string, timestamp: number): number {
    const basePriority = {
      enterprise: 100,
      premium: 50,
      free: 10
    }[tier] || 10;
    
    // Add slight aging bonus to prevent starvation
    const age = Date.now() - timestamp;
    const agingBonus = Math.min(age / 1000, 20); // Max 20 point bonus
    
    return basePriority + agingBonus;
  }
  
  /**
   * Update queue size metrics (includes both waiting and processing)
   */
  private updateQueueMetrics(): void {
    if (this.useNewAlgorithm) {
      // Use new algorithm queue stats
      const stats = this.scheduler.getQueueStats();
      this.metrics.enterpriseQueue = stats.queued.enterprise + stats.processing.enterprise;
      this.metrics.premiumQueue = stats.queued.premium + stats.processing.premium;
      this.metrics.freeQueue = stats.queued.free + stats.processing.free;
    } else {
      // Include both queued (size) and currently processing (pending) requests
      this.metrics.enterpriseQueue = this.enterpriseQueue.size + this.enterpriseQueue.pending;
      this.metrics.premiumQueue = this.premiumQueue.size + this.premiumQueue.pending;
      this.metrics.freeQueue = this.freeQueue.size + this.freeQueue.pending;
    }
  }
  
  /**
   * Update performance metrics
   */
  private updateMetrics(processingTime: number): void {
    // Simple moving average for response time
    this.metrics.avgResponseTime = 
      (this.metrics.avgResponseTime * 0.9) + (processingTime * 0.1);
  }
  
  /**
   * Get current queue sizes (includes both waiting and processing)
   */
  private getQueueSizes(): Record<string, number> {
    if (this.useNewAlgorithm) {
      // Use new algorithm queue stats
      const stats = this.scheduler.getQueueStats();
      return {
        enterprise: stats.queued.enterprise + stats.processing.enterprise,
        premium: stats.queued.premium + stats.processing.premium,
        free: stats.queued.free + stats.processing.free
      };
    } else {
      // Use existing p-queue stats
      return {
        enterprise: this.enterpriseQueue.size + this.enterpriseQueue.pending,
        premium: this.premiumQueue.size + this.premiumQueue.pending,
        free: this.freeQueue.size + this.freeQueue.pending
      };
    }
  }
  
  /**
   * Get comprehensive metrics
   */
  getMetrics(): QoSMetrics {
    this.updateQueueMetrics();
    return { ...this.metrics };
  }

  /**
   * Get detailed statistics for real-time monitoring
   */
  getDetailedStats(): DetailedQoSStats {
    this.updateQueueMetrics();
    
    const now = Date.now();
    const recentHistory = this.performanceHistory.filter(h => now - h.timestamp < 300000); // Last 5 minutes
    
    const avgWaitTime = recentHistory.length > 0 
      ? recentHistory.reduce((sum, h) => sum + h.waitTime, 0) / recentHistory.length 
      : 0;
      
    const avgProcessingTime = recentHistory.length > 0
      ? recentHistory.reduce((sum, h) => sum + h.processingTime, 0) / recentHistory.length
      : 0;
    
    const processingRate = recentHistory.length / 5; // Requests per minute over 5 minutes
    
    if (this.useNewAlgorithm) {
      // Use new algorithm stats
      const stats = this.scheduler.getQueueStats();
      const capacityInfo = this.capacityManager.getCurrentCapacity();
      
      return {
        timestamp: new Date().toISOString(),
        queues: {
          enterprise: {
            size: stats.queued.enterprise,
            pending: stats.processing.enterprise,
            concurrency: Math.floor(capacityInfo.maxConcurrent * 0.7), // 70% allocation
            isPaused: false
          },
          premium: {
            size: stats.queued.premium,
            pending: stats.processing.premium,
            concurrency: Math.floor(capacityInfo.maxConcurrent * 0.2), // 20% allocation
            isPaused: false
          },
          free: {
            size: stats.queued.free,
            pending: stats.processing.free,
            concurrency: Math.floor(capacityInfo.maxConcurrent * 0.1), // 10% allocation
            isPaused: false
          }
        },
        performance: {
          totalProcessed: this.metrics.totalRequests,
          processingRate,
          avgWaitTime,
          avgProcessingTime
        },
        activeRequests: Array.from(this.activeRequests.values()).map(req => ({
          ...req,
          waitTime: Date.now() - req.startTime
        }))
      };
    } else {
      // Use existing p-queue stats
      return {
        timestamp: new Date().toISOString(),
        queues: {
          enterprise: {
            size: this.enterpriseQueue.size,
            pending: this.enterpriseQueue.pending,
            concurrency: this.enterpriseQueue.concurrency,
            isPaused: this.enterpriseQueue.isPaused
          },
          premium: {
            size: this.premiumQueue.size,
            pending: this.premiumQueue.pending,
            concurrency: this.premiumQueue.concurrency,
            isPaused: this.premiumQueue.isPaused
          },
          free: {
            size: this.freeQueue.size,
            pending: this.freeQueue.pending,
            concurrency: this.freeQueue.concurrency,
            isPaused: this.freeQueue.isPaused
          }
        },
        performance: {
          totalProcessed: this.metrics.totalRequests,
          processingRate,
          avgWaitTime,
          avgProcessingTime
        },
        activeRequests: Array.from(this.activeRequests.values()).map(req => ({
          ...req,
          waitTime: Date.now() - req.startTime
        }))
      };
    }
  }

  /**
   * Broadcast event to connected Socket.IO clients
   */
  private broadcastEvent(event: string, data: any): void {
    if (this.io) {
      this.io.emit(event, data);
    }
  }

  /**
   * Broadcast queue update to all clients
   */
  private broadcastQueueUpdate(): void {
    if (this.io) {
      this.io.emit('queue_update', this.getMetrics());
    }
  }

  /**
   * Start periodic metrics broadcasting
   */
  private startMetricsBroadcast(): void {
    if (this.io) {
      this.metricsInterval = setInterval(() => {
        this.broadcastEvent('queue_stats', this.getDetailedStats());
        this.broadcastQueueUpdate();
      }, 1000); // Broadcast every second
    }
  }
  
  /**
   * Setup event handlers for monitoring
   */
  private setupEventHandlers(): void {
    // Monitor queue events
    [this.enterpriseQueue, this.premiumQueue, this.freeQueue].forEach((queue, index) => {
      const tier = ['enterprise', 'premium', 'free'][index];
      
      queue.on('active', () => {
        logger.debug(`${tier} queue processing started`, {
          queueSize: queue.size,
          pending: queue.pending
        });
      });
      
      queue.on('idle', () => {
        logger.debug(`${tier} queue idle`, {
          queueSize: queue.size
        });
      });
    });
  }
  
  /**
   * Health check
   */
  async healthCheck(): Promise<{ status: string; metrics: QoSMetrics }> {
    return {
      status: 'healthy',
      metrics: this.getMetrics()
    };
  }
  
  /**
   * Graceful shutdown
   */
  async shutdown(): Promise<void> {
    logger.info('Shutting down QoS Service...');
    
    // Clear metrics interval
    if (this.metricsInterval) {
      clearInterval(this.metricsInterval);
    }
    
    // Wait for all queues to finish
    await Promise.all([
      this.enterpriseQueue.onIdle(),
      this.premiumQueue.onIdle(), 
      this.freeQueue.onIdle()
    ]);
    
    logger.info('QoS Service shutdown complete');
  }
}