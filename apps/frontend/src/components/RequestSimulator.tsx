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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Chip,
  CircularProgress,
  Collapse,
  IconButton,
  Divider,
  Stack,
  LinearProgress,
  Accordion,
  AccordionSummary,
  AccordionDetails,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Stop as StopIcon,
  ExpandMore as ExpandMoreIcon,
  CheckCircle as SuccessIcon,
  Error as ErrorIcon,
  AccessTime as PendingIcon,
  Add as AddIcon,
  Person as PersonIcon,
  Key as KeyIcon,
  ContentCopy as CopyIcon,
  Delete as DeleteIcon,
} from '@mui/icons-material';

import { Model, Policy } from '../types';
import apiService from '../services/api';

interface SimulationRequest {
  model: string;
  queryText: string;
  count: number;
  maxTokens: number;
  selectedToken: string; // JWT token to use for requests
}

interface SimulationResult {
  id: string;
  timestamp: string;
  request: {
    model: string;
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

// Helper function to calculate expiration time from duration string
const calculateExpirationTime = (durationStr: string): string => {
  try {
    // Parse duration string like "4h", "30m", "1h30m", etc.
    const now = new Date();
    let totalMs = 0;
    
    // Match patterns like "1h", "30m", "45s", "1h30m45s"
    const matches = durationStr.match(/(\d+(?:\.\d+)?[hms])/g);
    if (!matches) return 'Unknown';
    
    for (const match of matches) {
      const value = parseFloat(match.slice(0, -1));
      const unit = match.slice(-1);
      
      switch (unit) {
        case 'h':
          totalMs += value * 60 * 60 * 1000;
          break;
        case 'm':
          totalMs += value * 60 * 1000;
          break;
        case 's':
          totalMs += value * 1000;
          break;
      }
    }
    
    const expirationTime = new Date(now.getTime() + totalMs);
    return expirationTime.toLocaleString();
  } catch (error) {
    return 'Unknown';
  }
};

const RequestSimulator: React.FC = () => {
  // Form state
  const [simulationForm, setSimulationForm] = useState<SimulationRequest>({
    model: '',
    queryText: 'Hello, can you help me with a coding question?',
    count: 1,
    maxTokens: 100,
    selectedToken: '', // JWT token will be pasted here
  });

  // Data state
  const [models, setModels] = useState<Model[]>([]);
  const [userInfo, setUserInfo] = useState<any>(null);
  
  // UI state
  const [loading, setLoading] = useState(true);
  const [isRunning, setIsRunning] = useState(false);
  const [results, setResults] = useState<SimulationResult[]>([]);
  const [expandedResults, setExpandedResults] = useState<Set<string>>(new Set());
  const [currentRequest, setCurrentRequest] = useState(0);
  const [error, setError] = useState<string | null>(null);
  
  // Token creation state
  const [showTokenCreate, setShowTokenCreate] = useState(false);
  const [selectedTTL, setSelectedTTL] = useState('default'); // 'default', '1h', '4h', '24h', 'custom'
  const [customTTL, setCustomTTL] = useState('');
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [createdTokenInfo, setCreatedTokenInfo] = useState<{token: string, expiration: string} | null>(null);
  const [tokenCreating, setTokenCreating] = useState(false);
  const [tokenDeleting, setTokenDeleting] = useState(false);

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, []);

  const loadInitialData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Load models and user info in parallel
      const [modelsData, userInfoData] = await Promise.all([
        apiService.getModels().catch(() => []),
        apiService.getUserInfo().catch(() => null),
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
      
      // Set user info
      if (userInfoData) {
        console.log('Loaded user info:', userInfoData);
        setUserInfo(userInfoData);
      }
    } catch (err) {
      console.error('Failed to load initial data:', err);
      setError('Failed to load models and tiers. Please check your connection.');
      // Set fallback data
      setModels([
        { id: 'vllm-simulator', name: 'vLLM Simulator', provider: 'KServe', description: 'Test model' },
        { id: 'qwen3-0.6b-instruct', name: 'Qwen3 0.6B Instruct', provider: 'KServe', description: 'Qwen3 model' }
      ]);
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

  // Get tier from user info (since tokens are stateless)
  const getSelectedTokenTier = () => {
    // For stateless tokens, use the user's tier from user info
    return userInfo?.tier || 'free';
  };

  const getApiKey = () => {
    // Use the token directly from the text field
    if (simulationForm.selectedToken?.trim()) {
      console.log('🔑 Using provided token:', `${simulationForm.selectedToken.substring(0, 20)}...`);
      return simulationForm.selectedToken.trim();
    }
    
    // No fallback since we require manual token input
    console.log('🔑 No token provided');
    return '';
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
      apiKey: getApiKey(),
    };

    // Service Account tokens use Bearer authentication
    const requestHeaders = {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${requestData.apiKey}`,
    };

    try {
      console.log(`🚀 Running simulation request ${requestIndex + 1}:`, requestData);
      const maskedToken = requestData.apiKey.length > 8 ? 
        `${requestData.apiKey.substring(0, 4)}...${requestData.apiKey.substring(requestData.apiKey.length - 4)}` : 
        requestData.apiKey;
      console.log(`🔐 Authorization header: Bearer ${maskedToken}`);
      
      const response = await apiService.simulateRequest(requestData);
      const duration = Date.now() - startTime;

      return {
        id: requestId,
        timestamp: new Date().toISOString(),
        request: {
          model: requestData.model,
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
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
          messages: requestData.messages,
          maxTokens: requestData.max_tokens,
          headers: requestHeaders,
          body: {
            model: requestData.model,
            messages: requestData.messages,
            max_tokens: requestData.max_tokens,
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
    if (!simulationForm.model || !simulationForm.queryText || !simulationForm.selectedToken?.trim()) {
      setError('Please fill in all required fields (Model, Token, and Query Text)');
      return;
    }
    
    // Validate token format (basic JWT validation)
    const token = simulationForm.selectedToken.trim();
    if (!token.includes('.') || token.split('.').length !== 3) {
      setError('Invalid token format. Please paste a valid JWT Service Account token.');
      return;
    }

    const tokenTier = getSelectedTokenTier();

    setIsRunning(true);
    setResults([]);
    setCurrentRequest(0);
    setError(null);

    try {
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
    } catch (err) {
      console.error('Simulation failed:', err);
      setError('Simulation failed. Please try again.');
    } finally {
      setIsRunning(false);
      setCurrentRequest(0);
    }
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

  const formatJsonWithMaskedAuth = (obj: any) => {
    if (obj && obj.Authorization) {
      const maskedObj = { ...obj };
      const authValue = maskedObj.Authorization;
      if (typeof authValue === 'string' && authValue.startsWith('Bearer ')) {
        const token = authValue.substring(7); // Remove "Bearer " prefix
        if (token.length > 8) {
          maskedObj.Authorization = `Bearer ${token.substring(0, 4)}...${token.substring(token.length - 4)}`;
        }
      }
      return JSON.stringify(maskedObj, null, 2);
    }
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

  // Token creation functionality
  const handleCreateToken = async () => {
    try {
      setTokenCreating(true);
      setError(null);
      
      // Determine expiration to send
      let expirationToSend: string | undefined;
      if (selectedTTL === 'default') {
        expirationToSend = undefined; // Let MaaS API use default
      } else if (selectedTTL === 'custom') {
        expirationToSend = customTTL.trim();
        if (!expirationToSend) {
          setError('Custom expiration is required when custom option is selected');
          setTokenCreating(false);
          return;
        }
      } else {
        expirationToSend = selectedTTL;
      }
      
      const response = await apiService.createToken({
        expiration: expirationToSend,
      });
      
      console.log('Token creation response:', response);
      
      // Extract the token and metadata from the response
      // API service unwraps { success: true, data: {...} } responses automatically
      const apiKey = response?.token;
      
      if (apiKey) {
        setCreatedToken(apiKey);
        setCreatedTokenInfo({
          token: apiKey,
          expiration: response?.expiration || 'Unknown'
        });
        
        
        // Auto-paste the new token into the textbox
        setSimulationForm(prev => ({ 
          ...prev, 
          selectedToken: apiKey
        }));
        
        console.log('🎯 Auto-pasted new token into textbox');
      } else {
        console.warn('No API key found in response:', response);
        setError('Token created but API key not returned');
      }
      setSelectedTTL('default');
      setCustomTTL('');
      setShowTokenCreate(false);
    } catch (error: any) {
      console.error('Token creation failed:', error);
      if (error.status === 503) {
        setError('MaaS API service is currently unavailable. Cannot create tokens at this time.');
      } else if (error.message?.includes('service is unavailable')) {
        setError('MaaS API service is currently unavailable. Cannot create tokens at this time.');
      } else {
        setError(error.message || 'Failed to create token');
      }
    } finally {
      setTokenCreating(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    // Could add a snackbar notification here
  };

  // Delete all tokens functionality
  const handleDeleteTokens = async () => {
    try {
      setTokenDeleting(true);
      setError(null);
      
      console.log('🗑️ Deleting all tokens...');
      
      const response = await apiService.deleteTokens();
      
      console.log('Token deletion response:', response);
      
      // Clear the current token from the text field since they've been deleted
      setSimulationForm(prev => ({ 
        ...prev, 
        selectedToken: ''
      }));
      
      // Clear any previously created token display
      setCreatedToken(null);
      setCreatedTokenInfo(null);
      
      console.log('🎯 All tokens deleted successfully');
      
      // Could add success notification here
      
    } catch (error: any) {
      console.error('Token deletion failed:', error);
      if (error.status === 503) {
        setError('MaaS API service is currently unavailable. Cannot delete tokens at this time.');
      } else if (error.message?.includes('service is unavailable')) {
        setError('MaaS API service is currently unavailable. Cannot delete tokens at this time.');
      } else if (error.status === 401) {
        setError('Authentication failed. Unable to delete tokens.');
      } else {
        setError(error.message || 'Failed to delete tokens');
      }
    } finally {
      setTokenDeleting(false);
    }
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
      {/* User Info Header */}
      {userInfo && (
        <Card sx={{ mb: 3, bgcolor: 'primary.50', border: '1px solid', borderColor: 'primary.200' }}>
          <CardContent>
            <Box display="flex" alignItems="center" justifyContent="space-between">
              <Box display="flex" alignItems="center" gap={2}>
                <PersonIcon color="primary" />
                <Box>
                  <Typography variant="h6" color="primary.main">
                    {userInfo.name}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {userInfo.email} • {userInfo.tier} tier • {userInfo.cluster}
                  </Typography>
                </Box>
              </Box>
              <Box display="flex" gap={1} alignItems="center">
                <Chip 
                  label={`${userInfo.tier} tier`}
                  color="primary"
                  variant="outlined"
                />
                <Chip 
                  label={`${userInfo.groups?.length || 0} groups`}
                  color="secondary"
                  variant="outlined"
                  size="small"
                />
                <Button
                  variant="outlined"
                  color="error"
                  size="small"
                  onClick={handleDeleteTokens}
                  disabled={tokenDeleting}
                  startIcon={tokenDeleting ? <CircularProgress size={16} /> : <DeleteIcon />}
                  sx={{ ml: 1 }}
                >
                  {tokenDeleting ? 'Deleting...' : 'Delete All Tokens'}
                </Button>
              </Box>
            </Box>
          </CardContent>
        </Card>
      )}

      {/* Header */}
      <Typography variant="h4" component="h1" gutterBottom>
        Playground
      </Typography>
      
      <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
        Create tokens and test your Kuadrant policies by sending real requests to your models. 
        Models and user info are retrieved from the MaaS API.
      </Typography>
      
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        <strong>Authentication:</strong> Uses ephemeral Service Account JWT tokens (4-hour TTL) with Bearer authentication.
        {simulationForm.selectedToken?.trim() ? 
          ` Using provided token (${simulationForm.selectedToken.substring(0, 20)}...)` : 
          ' Paste JWT token below to authenticate requests.'
        }
      </Typography>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 4 }}>
        <strong>Tier & Model Access:</strong> Your tier ({userInfo?.tier || 'free'}) determines model access and rate limits.
        Service Account tokens are stateless and don't store policy information - tier is inherited from your OpenShift groups.
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Token Creation Section */}
      <Card sx={{ mb: 4 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <KeyIcon />
            API Token Management
          </Typography>
          
          <Box display="flex" gap={2} alignItems="end" sx={{ mb: 2 }}>
            <FormControl size="small" sx={{ minWidth: 150 }}>
              <InputLabel>Token Expiration</InputLabel>
              <Select
                value={selectedTTL}
                onChange={(e) => setSelectedTTL(e.target.value)}
                label="Token Expiration"
                disabled={tokenCreating}
              >
                <MenuItem value="default">Default (4h)</MenuItem>
                <MenuItem value="1h">1 hour</MenuItem>
                <MenuItem value="4h">4 hours</MenuItem>
                <MenuItem value="24h">24 hours</MenuItem>
                <MenuItem value="custom">Custom</MenuItem>
              </Select>
            </FormControl>
            
            {selectedTTL === 'custom' && (
              <TextField
                label="Custom Expiration"
                value={customTTL}
                onChange={(e) => setCustomTTL(e.target.value)}
                placeholder="e.g., 2h, 30m, 3d"
                disabled={tokenCreating}
                size="small"
                sx={{ minWidth: 120 }}
                helperText="Format: 1h, 30m, 2d"
              />
            )}
            
            <Button
              variant="contained"
              onClick={handleCreateToken}
              disabled={tokenCreating || (selectedTTL === 'custom' && !customTTL.trim())}
              startIcon={tokenCreating ? <CircularProgress size={16} /> : <AddIcon />}
            >
              {tokenCreating ? 'Creating...' : 'Create Token'}
            </Button>
          </Box>
          
        </CardContent>
      </Card>

      {/* Token Display Dialog */}
      {createdToken && (
        <Alert 
          severity="success" 
          sx={{ mb: 3 }}
          action={
            <Box>
              <Button 
                color="inherit" 
                size="small" 
                onClick={() => copyToClipboard(createdToken)}
                startIcon={<CopyIcon />}
              >
                Copy
              </Button>
              <Button 
                color="inherit" 
                size="small" 
                onClick={() => {
                  setCreatedToken(null);
                  setCreatedTokenInfo(null);
                }}
              >
                Close
              </Button>
            </Box>
          }
        >
          <strong>Token Created Successfully!</strong>
          {createdTokenInfo && (
            <Box sx={{ mt: 1, mb: 1 }}>
              <Typography variant="body2" color="success.dark">
                <strong>Expiration:</strong> {createdTokenInfo.expiration} • <strong>Expires:</strong> {calculateExpirationTime(createdTokenInfo.expiration)}
              </Typography>
            </Box>
          )}
          <Box 
            component="code" 
            sx={{ 
              display: 'block', 
              mt: 1, 
              p: 1, 
              bgcolor: 'success.light', 
              borderRadius: 1,
              fontFamily: 'monospace',
              fontSize: '0.875rem',
              wordBreak: 'break-all'
            }}
          >
            {createdToken}
          </Box>
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

            <Grid item xs={12} sm={6} md={4}>
              <TextField
                fullWidth
                label="API Token *"
                value={simulationForm.selectedToken}
                onChange={(e) => {
                  console.log('Token entered:', e.target.value.substring(0, 10) + '...');
                  handleInputChange('selectedToken', e.target.value);
                }}
                placeholder="Paste your Service Account token here..."
                disabled={isRunning || loading}
                multiline
                rows={2}
                helperText="Paste the JWT token from the creation dialog above"
              />
            </Grid>


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
                Running request {currentRequest} of {simulationForm.count}...
              </Typography>
              <LinearProgress 
                variant="determinate" 
                value={(currentRequest / simulationForm.count) * 100} 
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
                        Request to {result.request.model}
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
                            {formatJsonWithMaskedAuth(result.request.headers)}
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
