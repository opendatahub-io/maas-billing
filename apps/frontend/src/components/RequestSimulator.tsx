import React, { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
  Alert,
  Paper,
  Chip,
  CircularProgress,
  Stack,
  LinearProgress,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Switch,
  FormControlLabel,
  Tooltip,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Stop as StopIcon,
  ExpandMore as ExpandMoreIcon,
  CheckCircle as SuccessIcon,
  Error as ErrorIcon,
  Speed as QoSIcon,
  DirectionsRun as DirectIcon,
} from '@mui/icons-material';

import { Model } from '../types';
import apiService from '../services/api';
import { useExperimental } from '../contexts/ExperimentalContext';

interface SimulationRequest {
  model: string;
  queryText: string;
  count: number;
  maxTokens: number;
  authPrefix: 'Bearer' | 'APIKEY';
  selectedToken: string; // Token name/id to use for requests
  // QoS-specific fields
  enableQoS: boolean;
  customerTier: 'enterprise' | 'premium' | 'free';
  demoMode: 'simulation' | 'advanced' | 'auto';
  loadTestMode: 'single-tier' | 'multi-tier-basic' | 'multi-tier-advanced';
}

interface SimulationResult {
  id: string;
  timestamp: string;
  request: {
    model: string;
    tier: string;
    messages: Array<{ role: string; content: string }>;
    maxTokens: number;
    headers: Record<string, string>;
    body: any;
  };
  response: {
    status: number;
    statusText: string;
    headers: Record<string, string>;
    body: any;
    error?: string;
  };
  success: boolean;
  duration: number;
}

const RequestSimulator: React.FC = () => {
  const { experimentalMode } = useExperimental();
  // Form state
  const [simulationForm, setSimulationForm] = useState<SimulationRequest>({
    model: '',
    queryText: 'Hello, can you help me with a coding question?',
    count: 1,
    maxTokens: 100,
    authPrefix: 'APIKEY', // Default to APIKEY as per your cluster config
    selectedToken: '', // Will be auto-selected when tokens load
    // QoS defaults
    enableQoS: false,
    customerTier: 'free',
    demoMode: 'auto',
    loadTestMode: 'single-tier'
  });

  // Disable QoS when experimental mode is turned off
  React.useEffect(() => {
    if (!experimentalMode && simulationForm.enableQoS) {
      setSimulationForm(prev => ({ ...prev, enableQoS: false }));
    }
  }, [experimentalMode, simulationForm.enableQoS]);

  // Data state
  const [models, setModels] = useState<Model[]>([]);
  const [userTokens, setUserTokens] = useState<any[]>([]);
  
  // UI state
  const [loading, setLoading] = useState(true);
  const [isRunning, setIsRunning] = useState(false);
  const [results, setResults] = useState<SimulationResult[]>([]);
  const [expandedResults, setExpandedResults] = useState<Set<string>>(new Set());
  const [currentRequest, setCurrentRequest] = useState(0);
  const [error, setError] = useState<string | null>(null);

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, []);

  const loadInitialData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Load models and user tokens in parallel
      const [modelsData, tokensData] = await Promise.all([
        apiService.getModels().catch(() => []),
        apiService.getUserTokens().catch(() => []),
      ]);

      // Transform models data to match expected structure
      const transformedModels = (modelsData || []).map((model: any) => ({
        id: model.id || model.name,
        name: model.name || model.id,
        provider: 'KServe',
        description: model.description || `${model.name || model.id} Model`,
      }));
      
      console.log('Loaded models:', transformedModels);
      setModels(transformedModels);
      
      const tokens = Array.isArray(tokensData) ? tokensData : tokensData?.data || [];
      console.log('Loaded user tokens:', tokens.length, 'tokens');
      if (tokens.length > 0) {
        console.log('First token structure:', Object.keys(tokens[0]));
        console.log('First token data:', tokens[0]);
      }
      setUserTokens(tokens);

      // No need to extract tiers - they come from the selected token's policy
      
      // Auto-select token based on user's first token if available
      if (tokens.length > 0 && !simulationForm.selectedToken) {
        const firstToken = tokens[0];
        setSimulationForm(prev => ({ 
          ...prev, 
          selectedToken: firstToken.name || firstToken.id
        }));
        console.log('🎯 Auto-selected token:', firstToken.displayName || firstToken.name, 'with policy:', firstToken.policy);
      }
    } catch (err) {
      console.error('Failed to load initial data:', err);
      setError('Failed to load models and tiers. Please check your connection.');
      // Set fallback data
      setModels([
        { id: 'vllm-simulator', name: 'vLLM Simulator', provider: 'KServe', description: 'Test model' },
        { id: 'qwen3-0.6b-instruct', name: 'Qwen3 0.6B Instruct', provider: 'KServe', description: 'Qwen3 model' }
      ]);
      // Fallback data set
      setUserTokens([]);
    } finally {
      setLoading(false);
    }
  };

  const handleInputChange = (field: keyof SimulationRequest, value: any) => {
    setSimulationForm(prev => ({
      ...prev,
      [field]: value,
    }));
  };

  // Get tier from selected token
  const getSelectedTokenTier = () => {
    if (simulationForm.selectedToken && userTokens.length > 0) {
      const selectedToken = userTokens.find(token => 
        (token.name === simulationForm.selectedToken) || 
        (token.id === simulationForm.selectedToken)
      );
      return selectedToken?.policy || 'unknown';
    }
    return 'unknown';
  };

  const getApiKey = () => {
    // Use the selected token if available
    if (simulationForm.selectedToken && userTokens.length > 0) {
      const selectedToken = userTokens.find(token => 
        (token.name === simulationForm.selectedToken) || 
        (token.id === simulationForm.selectedToken)
      );
      
      if (selectedToken) {
        const apiKey = selectedToken.actualApiKey || selectedToken.token || selectedToken.key || selectedToken.api_key || selectedToken.value;
        console.log('🔑 Using selected token:', selectedToken.displayName || selectedToken.name, `${apiKey?.substring(0, 8)}...`);
        return apiKey;
      }
    }
    
    // Fallback to first available token
    if (userTokens.length > 0) {
      const token = userTokens[0];
      const apiKey = token.actualApiKey || token.token || token.key || token.api_key || token.value;
      console.log('🔑 Using first available token:', token.displayName || token.name, `${apiKey?.substring(0, 8)}...`);
      return apiKey;
    }
    
    // Final fallback to test key
    const fallbackKey = 'freeuser1_key';
    console.log('🔑 Using fallback API key:', fallbackKey);
    return fallbackKey;
  };

  const runSingleRequest = async (requestIndex: number): Promise<SimulationResult> => {
    const startTime = Date.now();
    const requestId = `req-${Date.now()}-${requestIndex}`;
    
    const selectedTokenTier = getSelectedTokenTier();
    const requestData = {
      model: simulationForm.model,
      messages: [
        { role: 'user', content: simulationForm.queryText }
      ],
      max_tokens: simulationForm.maxTokens,
      tier: selectedTokenTier,
      apiKey: getApiKey(),
      authPrefix: simulationForm.authPrefix,
      // QoS parameters
      enableQoS: simulationForm.enableQoS,
      customerTier: simulationForm.customerTier,
      demoMode: simulationForm.demoMode,
    };

    // Kuadrant supports both APIKEY and Bearer prefixes
    // Use the selected prefix from the form
    const requestHeaders = {
      'Content-Type': 'application/json',
      'Authorization': `${simulationForm.authPrefix} ${requestData.apiKey}`,
    };

    try {
      console.log(`🚀 Running simulation request ${requestIndex + 1}:`, requestData);
      console.log(`🔐 Authorization header: ${simulationForm.authPrefix} ${requestData.apiKey.substring(0, 8)}...`);
      
      const response = await apiService.simulateRequest(requestData);
      const duration = Date.now() - startTime;

      return {
        id: requestId,
        timestamp: new Date().toISOString(),
        request: {
          model: requestData.model,
          tier: requestData.tier,
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
            tier: requestData.tier,
          },
        },
        response: {
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'application/json' },
          body: response,
        },
        success: true,
        duration,
      };
    } catch (err: any) {
      const duration = Date.now() - startTime;
      console.error(`❌ Request ${requestIndex + 1} failed:`, err);

      return {
        id: requestId,
        timestamp: new Date().toISOString(),
        request: {
          model: requestData.model,
          tier: requestData.tier,
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
            tier: requestData.tier,
          },
        },
        response: {
          status: err.response?.status || 500,
          statusText: err.response?.statusText || 'Internal Server Error',
          headers: err.response?.headers || {},
          body: err.response?.body || { error: err.message },
          error: err.message,
        },
        success: false,
        duration,
      };
    }
  };

  // Multi-tier request runner
  const runMultiTierRequest = async (tier: string, requestIndex: number, tierRequestId: number): Promise<SimulationResult> => {
    const startTime = Date.now();
    const requestId = `req-${Date.now()}-${tier}-${tierRequestId}`;
    
    const requestData = {
      model: simulationForm.model,
      messages: [
        { role: 'user', content: `Hello from ${tier} customer ${tierRequestId}. ${simulationForm.queryText}` }
      ],
      max_tokens: simulationForm.maxTokens,
      tier: tier,
      apiKey: getApiKey(),
      authPrefix: simulationForm.authPrefix,
      // QoS parameters
      enableQoS: simulationForm.enableQoS,
      customerTier: tier,
      demoMode: simulationForm.demoMode,
    };

    const requestHeaders = {
      'Content-Type': 'application/json',
      'Authorization': `${simulationForm.authPrefix} ${requestData.apiKey}`,
    };

    try {
      console.log(`🚀 Running multi-tier request ${requestIndex + 1} (${tier}-${tierRequestId}):`, requestData);
      
      const response = await apiService.simulateRequest(requestData);
      const duration = Date.now() - startTime;

      return {
        id: requestId,
        timestamp: new Date().toISOString(),
        request: {
          model: requestData.model,
          tier: tier,
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
            tier: tier,
          },
        },
        response: {
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'application/json' },
          body: response,
        },
        success: true,
        duration,
      };
    } catch (err: any) {
      const duration = Date.now() - startTime;
      console.error(`❌ Multi-tier request ${requestIndex + 1} (${tier}-${tierRequestId}) failed:`, err);

      return {
        id: requestId,
        timestamp: new Date().toISOString(),
        request: {
          model: requestData.model,
          tier: tier,
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
            tier: tier,
          },
        },
        response: {
          status: err.response?.status || 500,
          statusText: err.response?.statusText || 'Internal Server Error',
          headers: err.response?.headers || {},
          body: err.response?.body || { error: err.message },
          error: err.message,
        },
        success: false,
        duration,
      };
    }
  };

  const handleRunSimulation = async () => {
    if (!simulationForm.model || !simulationForm.queryText || !simulationForm.selectedToken) {
      setError('Please fill in all required fields (Model, Token, and Query Text)');
      return;
    }
    
    // Validate that the selected token exists
    const selectedToken = userTokens.find(t => t.name === simulationForm.selectedToken || t.id === simulationForm.selectedToken);
    if (!selectedToken) {
      setError('Selected token not found. Please choose a valid token.');
      return;
    }

    const tokenTier = getSelectedTokenTier();
    if (tokenTier === 'unknown' && simulationForm.loadTestMode === 'single-tier') {
      setError('Selected token does not have a valid policy/tier.');
      return;
    }

    setIsRunning(true);
    setResults([]);
    setCurrentRequest(0);
    setError(null);

    try {
      if (simulationForm.enableQoS && simulationForm.loadTestMode !== 'single-tier') {
        // Multi-tier load testing
        await runMultiTierLoadTest();
      } else {
        // Single-tier testing (original behavior)
        const requests = Array.from({ length: simulationForm.count }, (_, i) => i);
        
        for (let i = 0; i < requests.length; i++) {
          setCurrentRequest(i + 1);
          const result = await runSingleRequest(i);
          setResults(prev => [...prev, result]);
          
          // Small delay between requests to avoid overwhelming the server
          if (i < requests.length - 1) {
            await new Promise(resolve => setTimeout(resolve, 500));
          }
        }
      }
    } catch (err) {
      console.error('Simulation failed:', err);
      setError('Simulation failed. Please try again.');
    } finally {
      setIsRunning(false);
      setCurrentRequest(0);
    }
  };

  const runMultiTierLoadTest = async () => {
    let requests: { tier: string; tierRequestId: number; delay: number }[] = [];

    if (simulationForm.loadTestMode === 'multi-tier-basic') {
      // 5 requests: 2 Enterprise + 3 Free (like demo.sh basic mode)
      requests = [
        { tier: 'free', tierRequestId: 1, delay: 0 },
        { tier: 'free', tierRequestId: 2, delay: 300 },
        { tier: 'enterprise', tierRequestId: 1, delay: 300 },
        { tier: 'free', tierRequestId: 3, delay: 300 },
        { tier: 'enterprise', tierRequestId: 2, delay: 300 },
      ];
    } else if (simulationForm.loadTestMode === 'multi-tier-advanced') {
      // 30 requests: 3 Enterprise + 9 Premium + 18 Free (like demo.sh advanced mode)
      const advancedPattern = [
        { tier: 'free', tierRequestId: 1 }, { tier: 'free', tierRequestId: 2 }, { tier: 'free', tierRequestId: 3 },
        { tier: 'premium', tierRequestId: 1 }, { tier: 'free', tierRequestId: 4 }, { tier: 'free', tierRequestId: 5 },
        { tier: 'enterprise', tierRequestId: 1 }, { tier: 'free', tierRequestId: 6 }, { tier: 'premium', tierRequestId: 2 },
        { tier: 'free', tierRequestId: 7 }, { tier: 'free', tierRequestId: 8 }, { tier: 'premium', tierRequestId: 3 },
        { tier: 'free', tierRequestId: 9 }, { tier: 'enterprise', tierRequestId: 2 }, { tier: 'free', tierRequestId: 10 },
        { tier: 'premium', tierRequestId: 4 }, { tier: 'free', tierRequestId: 11 }, { tier: 'free', tierRequestId: 12 },
        { tier: 'premium', tierRequestId: 5 }, { tier: 'free', tierRequestId: 13 }, { tier: 'enterprise', tierRequestId: 3 },
        { tier: 'free', tierRequestId: 14 }, { tier: 'premium', tierRequestId: 6 }, { tier: 'free', tierRequestId: 15 },
        { tier: 'premium', tierRequestId: 7 }, { tier: 'free', tierRequestId: 16 }, { tier: 'premium', tierRequestId: 8 },
        { tier: 'free', tierRequestId: 17 }, { tier: 'premium', tierRequestId: 9 }, { tier: 'free', tierRequestId: 18 },
      ];
      requests = advancedPattern.map(item => ({ ...item, delay: 20 })); // Rapid fire with minimal delays
    }

    console.log(`🎯 Starting ${simulationForm.loadTestMode} with ${requests.length} requests`);
    
    // Launch all requests in rapid succession (like demo.sh)
    const runningRequests: Promise<SimulationResult>[] = [];
    
    for (let i = 0; i < requests.length; i++) {
      const { tier, tierRequestId, delay } = requests[i];
      
      if (i > 0 && delay > 0) {
        await new Promise(resolve => setTimeout(resolve, delay));
      }
      
      setCurrentRequest(i + 1);
      
      // Start the request without waiting for completion
      const requestPromise = runMultiTierRequest(tier, i, tierRequestId);
      runningRequests.push(requestPromise);
      
      // Add result as soon as it completes
      requestPromise.then(result => {
        setResults(prev => [...prev, result].sort((a, b) => 
          new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
        ));
      });
    }
    
    // Wait for all requests to complete
    await Promise.all(runningRequests);
    console.log(`✅ All ${requests.length} requests completed`);
  };

  const handleStopSimulation = () => {
    setIsRunning(false);
    setCurrentRequest(0);
  };

  const toggleResultExpansion = (resultId: string) => {
    setExpandedResults(prev => {
      const newSet = new Set(prev);
      if (newSet.has(resultId)) {
        newSet.delete(resultId);
      } else {
        newSet.add(resultId);
      }
      return newSet;
    });
  };

  const formatJson = (obj: any) => {
    return JSON.stringify(obj, null, 2);
  };

  const getResultIcon = (result: SimulationResult) => {
    if (result.success) {
      return <SuccessIcon color="success" />;
    } else {
      return <ErrorIcon color="error" />;
    }
  };

  const getResultColor = (result: SimulationResult) => {
    return result.success ? 'success' : 'error';
  };

  if (loading) {
    return (
      <Box display="flex" flexDirection="column" alignItems="center" justifyContent="center" minHeight="400px">
        <CircularProgress size={40} />
        <Typography sx={{ mt: 2 }}>Loading models and available tiers...</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          Fetching data from Kuadrant policies
        </Typography>
      </Box>
    );
  }

  const canRunSimulation = simulationForm.model && simulationForm.queryText && simulationForm.selectedToken && !isRunning;

  return (
    <Box>
      {/* Header */}
      <Typography variant="h4" component="h1" gutterBottom>
        Request Simulator
      </Typography>
      
      <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
        Test your Kuadrant policies{experimentalMode ? ' with optional QoS prioritization' : ''} by sending real requests to your models. 
        {experimentalMode ? (
          <>Toggle between direct Kuadrant access and QoS-prioritized routing to compare performance.</>
        ) : (
          <>Enable experimental features to access QoS prioritization capabilities.</>
        )}
      </Typography>
      
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        <strong>Authentication:</strong> Uses real API keys from your cluster. 
        Kuadrant supports both "APIKEY" and "Bearer" authorization prefixes.
        {simulationForm.selectedToken && userTokens.length > 0 ? (() => {
          const selectedToken = userTokens.find(t => t.name === simulationForm.selectedToken || t.id === simulationForm.selectedToken);
          return selectedToken ? 
            ` Using token: ${selectedToken.displayName || selectedToken.name} (${selectedToken.policy} policy)` : 
            ' Token not found';
        })() : userTokens.length > 0 ? 
          ` ${userTokens.length} tokens available - select one above` : 
          ' Using fallback test key.'
        }
      </Typography>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 4 }}>
        <strong>Tier & Model Access:</strong> Each token has its own policy that determines model access and rate limits.
        {userTokens.length > 1 && (
          <><br/><strong>Available Tokens:</strong> You have {userTokens.length} active tokens with different policies.</>
        )}
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Simulation Form */}
      <Card sx={{ mb: 4 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Simulation Parameters
          </Typography>
          
          <Grid container spacing={3}>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth>
                <InputLabel>Model *</InputLabel>
                <Select
                  value={simulationForm.model}
                  onChange={(e) => {
                    console.log('Model selected:', e.target.value);
                    handleInputChange('model', e.target.value);
                  }}
                  label="Model *"
                  disabled={isRunning || loading}
                >
                  {models.length === 0 ? (
                    <MenuItem disabled>
                      {loading ? 'Loading models...' : 'No models available'}
                    </MenuItem>
                  ) : (
                    models.map(model => (
                      <MenuItem key={model.id} value={model.id}>
                        {model.name}
                      </MenuItem>
                    ))
                  )}
                </Select>
              </FormControl>
            </Grid>


            {/* Show selected token's policy */}
            {simulationForm.selectedToken && (
              <Grid item xs={12} sm={6} md={3}>
                <Box sx={{ p: 2, bgcolor: 'info.50', borderRadius: 1, border: '1px solid', borderColor: 'info.200' }}>
                  <Typography variant="body2" color="info.main" sx={{ fontWeight: 600 }}>
                    Using Policy: {getSelectedTokenTier()}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Inherited from selected token
                  </Typography>
                </Box>
              </Grid>
            )}

            <Grid item xs={12} sm={6} md={2}>
              <TextField
                fullWidth
                label="Request Count"
                type="number"
                value={simulationForm.count}
                onChange={(e) => handleInputChange('count', Math.max(1, Math.min(10, parseInt(e.target.value) || 1)))}
                inputProps={{ min: 1, max: 10 }}
                disabled={isRunning}
              />
            </Grid>

            <Grid item xs={12} sm={6} md={2}>
              <TextField
                fullWidth
                label="Max Tokens"
                type="number"
                value={simulationForm.maxTokens}
                onChange={(e) => handleInputChange('maxTokens', Math.max(1, parseInt(e.target.value) || 100))}
                inputProps={{ min: 1, max: 2000 }}
                disabled={isRunning}
              />
            </Grid>

            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth>
                <InputLabel>API Token *</InputLabel>
                <Select
                  value={simulationForm.selectedToken}
                  onChange={(e) => {
                    const tokenName = e.target.value;
                    const selectedToken = userTokens.find(t => t.name === tokenName || t.id === tokenName);
                    console.log('Token selected:', tokenName, 'with policy:', selectedToken?.policy);
                    handleInputChange('selectedToken', tokenName);
                  }}
                  label="API Token *"
                  disabled={isRunning || loading}
                >
                  {userTokens.length === 0 ? (
                    <MenuItem disabled>
                      {loading ? 'Loading tokens...' : 'No tokens available'}
                    </MenuItem>
                  ) : (
                    userTokens.map(token => (
                      <MenuItem key={token.name || token.id} value={token.name || token.id}>
                        <Box>
                          <Typography variant="body2">
                            {token.displayName || token.alias || token.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            Policy: {token.policy} • Team: {token.team_name || 'Unknown'} • Created: {new Date(token.created).toLocaleDateString()}
                          </Typography>
                        </Box>
                      </MenuItem>
                    ))
                  )}
                </Select>
              </FormControl>
            </Grid>

            <Grid item xs={12} sm={6} md={2}>
              <FormControl fullWidth>
                <InputLabel>Auth Prefix</InputLabel>
                <Select
                  value={simulationForm.authPrefix}
                  onChange={(e) => handleInputChange('authPrefix', e.target.value)}
                  label="Auth Prefix"
                  disabled={isRunning || loading}
                >
                  <MenuItem value="APIKEY">APIKEY</MenuItem>
                  <MenuItem value="Bearer">Bearer</MenuItem>
                </Select>
              </FormControl>
            </Grid>
          </Grid>

          {/* QoS Configuration Section - Only show if experimental mode is enabled */}
          {experimentalMode && (
            <Box sx={{ mt: 3, p: 2, bgcolor: 'background.paper', borderRadius: 1, border: '1px solid', borderColor: 'divider' }}>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                <Tooltip title="Enable QoS prioritization for request processing">
                  <FormControlLabel
                    control={
                      <Switch
                        checked={simulationForm.enableQoS}
                        onChange={(e) => handleInputChange('enableQoS', e.target.checked)}
                        disabled={isRunning}
                        color="primary"
                      />
                    }
                    label={
                      <Box sx={{ display: 'flex', alignItems: 'center' }}>
                        {simulationForm.enableQoS ? <QoSIcon color="primary" sx={{ mr: 1 }} /> : <DirectIcon color="disabled" sx={{ mr: 1 }} />}
                        <Typography variant="body2" sx={{ fontWeight: 600 }}>
                          {simulationForm.enableQoS ? 'QoS Priority Mode' : 'Direct Mode'}
                        </Typography>
                        <Chip label="EXPERIMENTAL" size="small" color="warning" sx={{ ml: 1 }} />
                      </Box>
                    }
                  />
                </Tooltip>
              </Box>

              {simulationForm.enableQoS && (
                <>
                  <Grid container spacing={2}>
                    <Grid item xs={12} sm={6} md={3}>
                      <FormControl fullWidth size="small">
                        <InputLabel>Load Test Mode</InputLabel>
                        <Select
                          value={simulationForm.loadTestMode}
                          onChange={(e) => handleInputChange('loadTestMode', e.target.value)}
                          label="Load Test Mode"
                          disabled={isRunning}
                        >
                          <MenuItem value="single-tier">
                            <Box>
                              <Typography variant="body2">Single Tier</Typography>
                              <Typography variant="caption" color="text.secondary">
                                Test one customer tier only
                              </Typography>
                            </Box>
                          </MenuItem>
                          <MenuItem value="multi-tier-basic">
                            <Box>
                              <Typography variant="body2">Multi-Tier Basic</Typography>
                              <Typography variant="caption" color="text.secondary">
                                5 requests: 2 Enterprise + 3 Free
                              </Typography>
                            </Box>
                          </MenuItem>
                          <MenuItem value="multi-tier-advanced">
                            <Box>
                              <Typography variant="body2">Multi-Tier Advanced</Typography>
                              <Typography variant="caption" color="text.secondary">
                                30 requests: 3 Enterprise + 9 Premium + 18 Free
                              </Typography>
                            </Box>
                          </MenuItem>
                        </Select>
                      </FormControl>
                    </Grid>

                    {simulationForm.loadTestMode === 'single-tier' && (
                      <Grid item xs={12} sm={6} md={3}>
                        <FormControl fullWidth size="small">
                          <InputLabel>Customer Tier</InputLabel>
                          <Select
                            value={simulationForm.customerTier}
                            onChange={(e) => handleInputChange('customerTier', e.target.value)}
                            label="Customer Tier"
                            disabled={isRunning}
                          >
                            <MenuItem value="enterprise">
                              <Box>
                                <Typography variant="body2" color="success.main">Enterprise</Typography>
                                <Typography variant="caption" color="text.secondary">
                                  Highest priority • 3x concurrency
                                </Typography>
                              </Box>
                            </MenuItem>
                            <MenuItem value="premium">
                              <Box>
                                <Typography variant="body2" color="warning.main">Premium</Typography>
                                <Typography variant="caption" color="text.secondary">
                                  Medium priority • 2x concurrency
                                </Typography>
                              </Box>
                            </MenuItem>
                            <MenuItem value="free">
                              <Box>
                                <Typography variant="body2" color="text.secondary">Free</Typography>
                                <Typography variant="caption" color="text.secondary">
                                  Best effort • 1x concurrency
                                </Typography>
                              </Box>
                            </MenuItem>
                          </Select>
                        </FormControl>
                      </Grid>
                    )}

                    <Grid item xs={12} sm={6} md={3}>
                      <FormControl fullWidth size="small">
                        <InputLabel>Demo Mode</InputLabel>
                        <Select
                          value={simulationForm.demoMode}
                          onChange={(e) => handleInputChange('demoMode', e.target.value)}
                          label="Demo Mode"
                          disabled={isRunning}
                        >
                          <MenuItem value="auto">
                            <Box>
                              <Typography variant="body2">Auto</Typography>
                              <Typography variant="caption" color="text.secondary">
                                Service default mode
                              </Typography>
                            </Box>
                          </MenuItem>
                          <MenuItem value="simulation">
                            <Box>
                              <Typography variant="body2">Simulation</Typography>
                              <Typography variant="caption" color="text.secondary">
                                Fast simulation (4s delay)
                              </Typography>
                            </Box>
                          </MenuItem>
                          <MenuItem value="advanced">
                            <Box>
                              <Typography variant="body2">Advanced</Typography>
                              <Typography variant="caption" color="text.secondary">
                                Real LLM processing
                              </Typography>
                            </Box>
                          </MenuItem>
                        </Select>
                      </FormControl>
                    </Grid>

                    <Grid item xs={12} md={3}>
                      <Alert severity="info" sx={{ height: '100%', display: 'flex', alignItems: 'center' }}>
                        <Typography variant="caption">
                          {simulationForm.loadTestMode === 'single-tier' ? (
                            simulationForm.customerTier === 'enterprise' ? 
                              '💎 Premium queue with guaranteed processing' :
                              simulationForm.customerTier === 'premium' ?
                              '⚡ Priority queue with standard processing' :
                              '⏳ Best-effort queue with aging protection'
                          ) : simulationForm.loadTestMode === 'multi-tier-basic' ? 
                            '🚦 Basic QoS demo: Enterprise vs Free' :
                            '🎯 Advanced QoS demo: Full 3-tier prioritization'
                          }
                        </Typography>
                      </Alert>
                    </Grid>
                  </Grid>
                </>
              )}

              {!simulationForm.enableQoS && (
                <Alert severity="warning" sx={{ mt: 1 }}>
                  Direct mode bypasses QoS prioritization. Requests go directly to Kuadrant endpoints using first-come-first-serve processing.
                </Alert>
              )}
            </Box>
          )}

          <Grid container spacing={3} sx={{ mt: 2 }}>
            <Grid item xs={12} md={2}>
              <Button
                fullWidth
                variant="contained"
                color="primary"
                onClick={canRunSimulation ? handleRunSimulation : handleStopSimulation}
                startIcon={isRunning ? <StopIcon /> : <PlayIcon />}
                disabled={!canRunSimulation && !isRunning}
                sx={{ height: '56px' }}
              >
                {isRunning ? 'Stop' : 'Run Simulation'}
              </Button>
            </Grid>
          </Grid>

          <Box sx={{ mt: 3 }}>
            <TextField
              fullWidth
              label="Query Text *"
              multiline
              rows={3}
              value={simulationForm.queryText}
              onChange={(e) => handleInputChange('queryText', e.target.value)}
              disabled={isRunning}
              placeholder="Enter the text you want to send to the model..."
            />
          </Box>

          {isRunning && (
            <Box sx={{ mt: 3 }}>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                {simulationForm.enableQoS && simulationForm.loadTestMode !== 'single-tier' ? (
                  simulationForm.loadTestMode === 'multi-tier-basic' ? 
                    `Running multi-tier test: ${currentRequest} of 5 requests...` :
                    `Running advanced multi-tier test: ${currentRequest} of 30 requests...`
                ) : (
                  `Running request ${currentRequest} of ${simulationForm.count}...`
                )}
              </Typography>
              <LinearProgress 
                variant="determinate" 
                value={simulationForm.enableQoS && simulationForm.loadTestMode !== 'single-tier' ? (
                  simulationForm.loadTestMode === 'multi-tier-basic' ? 
                    (currentRequest / 5) * 100 :
                    (currentRequest / 30) * 100
                ) : (
                  (currentRequest / simulationForm.count) * 100
                )} 
              />
            </Box>
          )}
        </CardContent>
      </Card>

      {/* Results */}
      {results.length > 0 && (
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Simulation Results ({results.length})
            </Typography>

            <Stack spacing={2}>
              {results.map((result) => (
                <Accordion
                  key={result.id}
                  expanded={expandedResults.has(result.id)}
                  onChange={() => toggleResultExpansion(result.id)}
                >
                  <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                    <Box display="flex" alignItems="center" width="100%">
                      {getResultIcon(result)}
                      <Typography sx={{ ml: 2, flexGrow: 1 }}>
                        Request to {result.request.model} (policy: {result.request.tier})
                        {result.response.body?.qos_metadata && (
                          <Chip 
                            label={`QoS: ${result.response.body.qos_metadata.customer_tier}`}
                            size="small" 
                            color="primary" 
                            sx={{ ml: 1 }}
                          />
                        )}
                      </Typography>
                      <Chip
                        label={result.success ? `${result.response.status} ${result.response.statusText}` : 'Failed'}
                        color={getResultColor(result)}
                        size="small"
                        sx={{ mr: 2 }}
                      />
                      <Typography variant="body2" color="text.secondary">
                        {result.duration}ms
                      </Typography>
                    </Box>
                  </AccordionSummary>
                  
                  <AccordionDetails>
                    <Grid container spacing={3}>
                      {/* Request Details */}
                      <Grid item xs={12} md={6}>
                        <Typography variant="h6" gutterBottom color="primary">
                          📤 Request
                        </Typography>
                        
                        <Typography variant="subtitle2" gutterBottom>
                          Headers:
                        </Typography>
                        <Paper sx={{ p: 2, mb: 2, bgcolor: 'grey.50' }}>
                          <pre style={{ margin: 0, fontSize: '0.875rem', whiteSpace: 'pre-wrap' }}>
                            {formatJson(result.request.headers)}
                          </pre>
                        </Paper>

                        <Typography variant="subtitle2" gutterBottom>
                          Body:
                        </Typography>
                        <Paper sx={{ p: 2, bgcolor: 'grey.50' }}>
                          <pre style={{ margin: 0, fontSize: '0.875rem', whiteSpace: 'pre-wrap' }}>
                            {formatJson(result.request.body)}
                          </pre>
                        </Paper>
                      </Grid>

                      {/* Response Details */}
                      <Grid item xs={12} md={6}>
                        <Typography variant="h6" gutterBottom color="secondary">
                          📥 Response
                        </Typography>
                        
                        <Typography variant="subtitle2" gutterBottom>
                          Status: {result.response.status} {result.response.statusText}
                        </Typography>
                        
                        <Typography variant="subtitle2" gutterBottom>
                          Headers:
                        </Typography>
                        <Paper sx={{ p: 2, mb: 2, bgcolor: 'grey.50' }}>
                          <pre style={{ margin: 0, fontSize: '0.875rem', whiteSpace: 'pre-wrap' }}>
                            {formatJson(result.response.headers)}
                          </pre>
                        </Paper>

                        <Typography variant="subtitle2" gutterBottom>
                          Body:
                        </Typography>
                        <Paper sx={{ p: 2, bgcolor: result.success ? 'success.light' : 'error.light', color: result.success ? 'success.contrastText' : 'error.contrastText' }}>
                          <pre style={{ margin: 0, fontSize: '0.875rem', whiteSpace: 'pre-wrap' }}>
                            {formatJson(result.response.body)}
                          </pre>
                        </Paper>
                      </Grid>
                    </Grid>
                  </AccordionDetails>
                </Accordion>
              ))}
            </Stack>
          </CardContent>
        </Card>
      )}
    </Box>
  );
};

export default RequestSimulator;