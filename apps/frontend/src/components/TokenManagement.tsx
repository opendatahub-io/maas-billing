import React, { useState, useEffect } from 'react';
import {
  Box, Card, CardContent, Typography, Button,
  TextField, FormControl, Select, MenuItem,
  Chip, Alert, Dialog, DialogContent, DialogTitle,
  DialogActions, IconButton, Tooltip, Stack,
  Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, Paper, CircularProgress
} from '@mui/material';
import {
  ContentCopy as CopyIcon,
  Delete as DeleteIcon,
  Add as AddIcon,
  Key as KeyIcon,
  Security as SecurityIcon,
  Assessment as TestIcon,
  Code as CodeIcon,
  Http as HttpIcon
} from '@mui/icons-material';
import apiService from '../services/api';

interface UserToken {
  name: string;
  displayName?: string;
  created: string;
  lastUsed: string;
  usage: number;
  status: 'active' | 'unused' | 'expired';
  tokenValue?: string; // Only available immediately after creation
  actualApiKey?: string; // The actual API key value from Kubernetes secret
  team_id?: string;
  team_name?: string;
  policy?: string;
  alias?: string;
  secret_name?: string;
}

interface UserTier {
  name: string;
  usage: number;
  limit: number;
  models: string[];
  team_id?: string;
  team_name?: string;
  policy?: string;
}

interface TestResult {
  success: boolean;
  response?: string;
  error?: string;
  statusCode?: number;
  request?: {
    url: string;
    method: string;
    headers: Record<string, string>;
    body: any;
  };
  responseDetails?: {
    status: number;
    headers: Record<string, string>;
    body: any;
  };
}

const TokenManagement: React.FC = () => {
  const [tokens, setTokens] = useState<UserToken[]>([]);
  const [userTier, setUserTier] = useState<UserTier | null>(null);
  const [newTokenName, setNewTokenName] = useState('');
  const [newTokenDescription, setNewTokenDescription] = useState('');
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Test functionality state
  const [testModel, setTestModel] = useState('vllm-simulator');
  const [testMessage, setTestMessage] = useState('Hello, test my token!');
  const [selectedTokenForTest, setSelectedTokenForTest] = useState('');
  const [customTokenValue, setCustomTokenValue] = useState('');
  const [useCustomToken, setUseCustomToken] = useState(false);
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [testLoading, setTestLoading] = useState(false);
  
  // Available models
  const [availableModels, setAvailableModels] = useState<string[]>([]);

  useEffect(() => {
    loadUserData();
    loadAvailableModels();
  }, []);

  const loadUserData = async () => {
    try {
      setLoading(true);
      // Load user tier information
      const tierResponse = await apiService.getUserTier();
      console.log('Tier response:', tierResponse);
      setUserTier(tierResponse);
      
      // Load user tokens
      const tokensResponse = await apiService.getUserTokens();
      console.log('Tokens response:', tokensResponse);
      
      // Ensure tokens is always an array
      if (Array.isArray(tokensResponse)) {
        setTokens(tokensResponse);
      } else if (tokensResponse && Array.isArray(tokensResponse.data)) {
        setTokens(tokensResponse.data);
      } else {
        console.warn('Unexpected tokens response format:', tokensResponse);
        setTokens([]);
      }
    } catch (error) {
      setError('Failed to load user data');
      console.error('Error loading user data:', error);
      // Set empty arrays on error to prevent map errors
      setTokens([]);
      setUserTier(null);
    } finally {
      setLoading(false);
    }
  };

  const loadAvailableModels = async () => {
    try {
      const models = await apiService.getModels();
      setAvailableModels(models.map((m: any) => m.name));
    } catch (error) {
      console.error('Error loading models:', error);
      setAvailableModels(['vllm-simulator', 'qwen3-0.6b-instruct']);
    }
  };

  const handleGenerateToken = async () => {
    if (!newTokenName.trim()) {
      setError('Token name is required');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      
      const response = await apiService.createToken({
        name: newTokenName.trim(),
        description: newTokenDescription.trim() || 'Generated via UI'
      });
      
      setGeneratedToken(response.token);
      setNewTokenName('');
      setNewTokenDescription('');
      
      // Refresh token list
      await loadUserData();
    } catch (error: any) {
      setError(error.message || 'Failed to generate token');
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeToken = async (tokenName: string) => {
    if (!window.confirm(`Are you sure you want to revoke token "${tokenName}"? This action cannot be undone.`)) {
      return;
    }

    try {
      setLoading(true);
      await apiService.revokeToken(tokenName);
      await loadUserData(); // Refresh list
    } catch (error: any) {
      setError(error.message || 'Failed to revoke token');
    } finally {
      setLoading(false);
    }
  };

  const handleTestToken = async () => {
    let tokenToTest: string;
    
    if (useCustomToken) {
      tokenToTest = customTokenValue;
    } else {
      // Find the selected token and use its actual API key value
      const selectedToken = tokens.find(t => t.name === selectedTokenForTest);
      if (!selectedToken?.actualApiKey) {
        setError('Selected token does not have a valid API key value. Please try another token or use a custom token.');
        return;
      }
      tokenToTest = selectedToken.actualApiKey;
    }
    
    if (!tokenToTest || !testMessage.trim()) {
      setError('Please select a token (or enter a custom one) and enter a test message');
      return;
    }

    try {
      setTestLoading(true);
      setTestResult(null);
      
      console.log('Testing token:', tokenToTest.substring(0, 10) + '...');
      
      const response = await apiService.testToken({
        token: tokenToTest,
        model: testModel,
        message: testMessage.trim()
      });
      
      console.log('Test response received:', response);
      
      setTestResult({
        success: true,
        response: response.message || 'Test successful!',
        statusCode: response.statusCode || 200,
        request: response.request || null,
        responseDetails: response.response || null
      });
    } catch (error: any) {
      console.error('Token test error:', error);
      
      // The API service might throw the full response as an error
      // Try to extract the actual response data
      let errorData = null;
      let actualResponse = null;
      
      try {
        // Check if the error contains the response data
        if (error.response) {
          actualResponse = error.response;
        } else if (error.message) {
          // Try to parse error message as JSON
          try {
            actualResponse = JSON.parse(error.message);
          } catch (e) {
            // If not JSON, check if it contains a JSON string
            const jsonMatch = error.message.match(/\{.*\}/s);
            if (jsonMatch) {
              actualResponse = JSON.parse(jsonMatch[0]);
            }
          }
        }
        
        if (actualResponse && actualResponse.data) {
          errorData = actualResponse.data;
        }
      } catch (e) {
        console.warn('Could not parse error response:', e);
      }

      setTestResult({
        success: false,
        error: errorData?.error || errorData?.message || error.message || 'Test failed',
        statusCode: errorData?.statusCode || error.statusCode || 500,
        request: errorData?.request,
        responseDetails: errorData?.responseDetails || errorData?.response
      });
    } finally {
      setTestLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    // Could add a snackbar notification here
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'success';
      case 'unused': return 'warning';
      case 'expired': return 'error';
      default: return 'default';
    }
  };

  return (
    <Box>
      <Typography variant="h4" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <KeyIcon />
        My API Tokens
      </Typography>
      
      {error && (
        <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* User Account Information */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <SecurityIcon />
            Account Information
          </Typography>
          {userTier ? (
            <Box>
              <Stack direction="row" spacing={2} sx={{ mb: 2 }}>
                <Chip 
                  label={`Policy: ${userTier.policy || userTier.name}`} 
                  color="primary" 
                  variant="outlined"
                />
                <Chip 
                  label={`${userTier.usage}/${userTier.limit} tokens this month`}
                  color={userTier.usage > userTier.limit * 0.8 ? 'warning' : 'success'}
                />
                {userTier.team_name && (
                  <Chip 
                    label={`Team: ${userTier.team_name}`} 
                    color="secondary" 
                    variant="outlined"
                  />
                )}
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Available Models: {userTier.models?.join(', ') || 'No models available'}
              </Typography>
              {userTier.team_id && (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  Team ID: {userTier.team_id}
                </Typography>
              )}
            </Box>
          ) : (
            <CircularProgress size={20} />
          )}
        </CardContent>
      </Card>

      {/* Token Generation */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <AddIcon />
            Generate New Token
          </Typography>
          <Stack spacing={2}>
            <TextField
              fullWidth
              label="Token Name"
              value={newTokenName}
              onChange={(e) => setNewTokenName(e.target.value)}
              placeholder="my-project-token"
              required
            />
            <TextField
              fullWidth
              label="Description (Optional)"
              value={newTokenDescription}
              onChange={(e) => setNewTokenDescription(e.target.value)}
              placeholder="Token for ML project"
              multiline
              rows={2}
            />
            <Box>
              <Button 
                variant="contained" 
                onClick={handleGenerateToken}
                disabled={!newTokenName.trim() || loading}
                startIcon={loading ? <CircularProgress size={20} /> : <KeyIcon />}
              >
                {loading ? 'Generating...' : 'Generate Token'}
              </Button>
            </Box>
          </Stack>
        </CardContent>
      </Card>

      {/* Token Display Dialog */}
      <Dialog 
        open={!!generatedToken} 
        onClose={() => setGeneratedToken(null)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>
          Token Generated Successfully!
        </DialogTitle>
        <DialogContent>
          <Alert severity="warning" sx={{ mb: 2 }}>
            <strong>Save this token now!</strong> You won't be able to see it again for security reasons.
          </Alert>
          <TextField
            fullWidth
            multiline
            rows={4}
            value={generatedToken || ''}
            InputProps={{ 
              readOnly: true,
              endAdornment: (
                <Tooltip title="Copy to clipboard">
                  <IconButton onClick={() => copyToClipboard(generatedToken || '')}>
                    <CopyIcon />
                  </IconButton>
                </Tooltip>
              )
            }}
            sx={{ fontFamily: 'monospace' }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => copyToClipboard(generatedToken || '')} startIcon={<CopyIcon />}>
            Copy Token
          </Button>
          <Button onClick={() => setGeneratedToken(null)} variant="contained">
            I've Saved It
          </Button>
        </DialogActions>
      </Dialog>

      {/* Active Tokens List */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Active Tokens
          </Typography>
          {loading ? (
            <Box display="flex" justifyContent="center" p={2}>
              <CircularProgress />
            </Box>
          ) : (tokens || []).length === 0 ? (
            <Typography variant="body2" color="text.secondary" textAlign="center" py={2}>
              No tokens found. Generate your first token above.
            </Typography>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>Name</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell>Created</TableCell>
                    <TableCell>Last Used</TableCell>
                    <TableCell>Usage</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {(tokens || []).map((token) => (
                    <TableRow key={token.secret_name || token.name}>
                      <TableCell>
                        <Typography variant="subtitle2">
                          {token.displayName || token.alias || token.name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          Secret: {token.name}
                        </Typography>
                        {token.team_name && (
                          <Typography variant="caption" color="text.secondary" display="block">
                            Team: {token.team_name}
                          </Typography>
                        )}
                        {token.actualApiKey && (
                          <Typography variant="caption" color="success.main" display="block">
                            ✓ API Key: {token.actualApiKey.substring(0, 10)}...
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Stack spacing={0.5}>
                          <Chip 
                            label={token.status} 
                            color={getStatusColor(token.status) as any}
                            size="small"
                          />
                          {token.policy && (
                            <Chip 
                              label={token.policy} 
                              color="default"
                              size="small"
                              variant="outlined"
                            />
                          )}
                        </Stack>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {formatDate(token.created)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {token.lastUsed ? formatDate(token.lastUsed) : 'Never'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {token.usage} requests
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={1} justifyContent="flex-end">
                          <Tooltip title="Copy secret name">
                            <IconButton size="small" onClick={() => copyToClipboard(token.secret_name || token.name)}>
                              <CopyIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Revoke token">
                            <IconButton 
                              size="small" 
                              color="error"
                              onClick={() => handleRevokeToken(token.secret_name || token.name)}
                            >
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </CardContent>
      </Card>

      {/* Quick Test */}
      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <TestIcon />
            Quick Test
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Test your tokens by sending a request to the selected model.
          </Typography>
          
          <Stack spacing={2}>
            <FormControl fullWidth>
              <Typography variant="body2" sx={{ mb: 1 }}>Model</Typography>
              <Select
                value={testModel}
                onChange={(e) => setTestModel(e.target.value)}
              >
                {(availableModels || []).map((model) => (
                  <MenuItem key={model} value={model}>
                    {model}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            
            <TextField
              fullWidth
              label="Test Message"
              value={testMessage}
              onChange={(e) => setTestMessage(e.target.value)}
              placeholder="Hello, test my token!"
            />
            
            <FormControl fullWidth>
              <Typography variant="body2" sx={{ mb: 1 }}>Token Selection</Typography>
              <Stack spacing={2}>
                <Box>
                  <Button 
                    variant={!useCustomToken ? "contained" : "outlined"} 
                    onClick={() => setUseCustomToken(false)}
                    size="small"
                  >
                    Use My Tokens
                  </Button>
                  <Button 
                    variant={useCustomToken ? "contained" : "outlined"} 
                    onClick={() => setUseCustomToken(true)}
                    size="small"
                    sx={{ ml: 1 }}
                  >
                    Custom Token
                  </Button>
                </Box>
                
                {!useCustomToken ? (
                  <Select
                    value={selectedTokenForTest}
                    onChange={(e) => setSelectedTokenForTest(e.target.value)}
                    displayEmpty
                  >
                    <MenuItem value="">Select a token to test</MenuItem>
                    {(tokens || []).filter(t => t.status === 'active').map((token) => (
                      <MenuItem key={token.name} value={token.name}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                          <Typography variant="body2">
                            {token.displayName || token.alias || token.name} {token.team_name && `(${token.team_name})`}
                          </Typography>
                          {token.actualApiKey ? (
                            <Chip label="✓ Ready" color="success" size="small" />
                          ) : (
                            <Chip label="⚠ No API Key" color="warning" size="small" />
                          )}
                          {token.policy && (
                            <Chip label={token.policy} size="small" />
                          )}
                        </Box>
                      </MenuItem>
                    ))}
                  </Select>
                ) : (
                  <Box>
                    <TextField
                      fullWidth
                      label="Custom Token Value"
                      value={customTokenValue}
                      onChange={(e) => setCustomTokenValue(e.target.value)}
                      placeholder="Enter the actual API key value (not the secret name)"
                      helperText="Use the actual API key value from the Kubernetes secret, not the secret name"
                    />
                    <Box sx={{ mt: 1 }}>
                      <Typography variant="caption" color="text.secondary">
                        Quick test values:
                      </Typography>
                      <Box sx={{ mt: 0.5 }}>
                        <Button 
                          size="small" 
                          variant="outlined" 
                          onClick={() => setCustomTokenValue('invalid-token-123')}
                          sx={{ mr: 1, mb: 0.5 }}
                        >
                          Test Invalid Token
                        </Button>
                        <Button 
                          size="small" 
                          variant="outlined" 
                          onClick={() => setCustomTokenValue('freeuser1_key')}
                          sx={{ mr: 1, mb: 0.5 }}
                        >
                          Test Free User Token
                        </Button>
                        <Button 
                          size="small" 
                          variant="outlined" 
                          onClick={() => setCustomTokenValue('premiumuser1_key')}
                          sx={{ mb: 0.5 }}
                        >
                          Test Premium User Token
                        </Button>
                      </Box>
                    </Box>
                    
                    <Alert severity="info" sx={{ mt: 2 }}>
                      <Typography variant="body2">
                        <strong>💡 Note:</strong> We're working on automatically fetching API key values. For now, use the "Custom Token" option.
                      </Typography>
                    </Alert>
                  </Box>
                )}
              </Stack>
            </FormControl>
            
            <Box>
              <Button 
                variant="outlined"
                onClick={handleTestToken}
                disabled={
                  (!useCustomToken && !selectedTokenForTest) || 
                  (useCustomToken && !customTokenValue.trim()) || 
                  !testMessage.trim() || 
                  testLoading
                }
                startIcon={testLoading ? <CircularProgress size={20} /> : <TestIcon />}
              >
                {testLoading ? 'Testing...' : 'Send Test Request'}
              </Button>
              {useCustomToken && (
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
                  💡 Try testing with invalid tokens to see authentication errors
                </Typography>
              )}
            </Box>
            
            {testResult && (
              <Box sx={{ mt: 2 }}>
                {/* Main Result Alert */}
                <Alert 
                  severity={testResult.success ? 'success' : 'error'}
                  sx={{ mb: 2 }}
                >
                  <Typography variant="subtitle2">
                    {testResult.success ? '✅ Token Authentication Successful!' : '❌ Token Authentication Failed'}
                    {testResult.statusCode && ` (HTTP ${testResult.statusCode})`}
                  </Typography>
                  <Typography variant="body2">
                    {testResult.success ? testResult.response : testResult.error}
                  </Typography>
                </Alert>
                
                {/* Debug Info */}
                <Card variant="outlined" sx={{ mb: 2, bgcolor: 'info.50' }}>
                  <CardContent>
                    <Typography variant="subtitle2" gutterBottom>
                      🐛 Debug Info (Full Response Data):
                    </Typography>
                    <Box sx={{ fontFamily: 'monospace', fontSize: '0.7rem', bgcolor: 'white', p: 1, borderRadius: 1, maxHeight: 150, overflow: 'auto' }}>
                      <pre style={{ margin: 0 }}>
                        {JSON.stringify({ 
                          hasRequest: !!testResult.request,
                          hasResponse: !!testResult.responseDetails,
                          fullResponse: testResult
                        }, null, 2)}
                      </pre>
                    </Box>
                  </CardContent>
                </Card>
                
                {/* Always Show Request Details */}
                <Card variant="outlined" sx={{ mb: 2 }}>
                  <CardContent>
                    <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
                      <HttpIcon color="primary" />
                      <Typography variant="h6">
                        📤 HTTP Request
                      </Typography>
                      {testResult.request && (
                        <IconButton 
                          size="small" 
                          onClick={() => copyToClipboard(JSON.stringify(testResult.request, null, 2))}
                          title="Copy request details"
                        >
                          <CopyIcon fontSize="small" />
                        </IconButton>
                      )}
                    </Stack>
                    
                    {testResult.request ? (
                      <>
                        {/* URL and Method */}
                        <Box sx={{ mb: 2, p: 1.5, bgcolor: 'primary.50', borderRadius: 1, border: '1px solid', borderColor: 'primary.200' }}>
                          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontWeight: 'bold', color: 'primary.main' }}>
                            {testResult.request.method} {testResult.request.url}
                          </Typography>
                        </Box>
                        
                        {/* Headers and Body in a Grid */}
                        <Stack spacing={2} direction={{ xs: 'column', md: 'row' }}>
                          <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <CodeIcon fontSize="small" />
                              Headers:
                            </Typography>
                            <Box sx={{ fontFamily: 'monospace', fontSize: '0.8rem', bgcolor: 'grey.50', p: 1.5, borderRadius: 1, border: '1px solid', borderColor: 'grey.300' }}>
                              {Object.entries(testResult.request.headers).map(([key, value]) => (
                                <Box key={key} sx={{ mb: 0.3 }}>
                                  <strong style={{ color: '#1976d2' }}>{key}:</strong>{' '}
                                  <span style={{ color: key === 'Authorization' ? '#d32f2f' : '#333', wordBreak: 'break-all' }}>
                                    {key === 'Authorization' ? 
                                      `${value.substring(0, 12)}...${value.substring(value.length - 6)}` : 
                                      value
                                    }
                                  </span>
                                </Box>
                              ))}
                            </Box>
                          </Box>
                          
                          <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <CodeIcon fontSize="small" />
                              Body:
                            </Typography>
                            <Box sx={{ fontFamily: 'monospace', fontSize: '0.8rem', bgcolor: 'grey.50', p: 1.5, borderRadius: 1, border: '1px solid', borderColor: 'grey.300' }}>
                              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: '0.8rem' }}>
                                {JSON.stringify(testResult.request.body, null, 2)}
                              </pre>
                            </Box>
                          </Box>
                        </Stack>
                      </>
                    ) : (
                      <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                        Request details not available
                      </Typography>
                    )}
                  </CardContent>
                </Card>
                
                {/* Always Show Response Details */}
                <Card variant="outlined">
                  <CardContent>
                    <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
                      <HttpIcon color={testResult.success ? 'success' : 'error'} />
                      <Typography variant="h6">
                        📥 HTTP Response
                      </Typography>
                      {testResult.responseDetails && (
                        <IconButton 
                          size="small" 
                          onClick={() => copyToClipboard(JSON.stringify(testResult.responseDetails, null, 2))}
                          title="Copy response details"
                        >
                          <CopyIcon fontSize="small" />
                        </IconButton>
                      )}
                    </Stack>
                    
                    {testResult.responseDetails ? (
                      <>
                        {/* Status */}
                        <Box sx={{ 
                          mb: 2, 
                          p: 1.5, 
                          bgcolor: testResult.success ? 'success.50' : 'error.50', 
                          borderRadius: 1, 
                          border: '1px solid', 
                          borderColor: testResult.success ? 'success.200' : 'error.200' 
                        }}>
                          <Typography variant="body2" sx={{ 
                            fontFamily: 'monospace', 
                            fontWeight: 'bold', 
                            color: testResult.success ? 'success.main' : 'error.main' 
                          }}>
                            HTTP {testResult.responseDetails.status} {testResult.success ? 'OK' : 'Error'}
                          </Typography>
                        </Box>
                        
                        {/* Headers and Body in a Grid */}
                        <Stack spacing={2} direction={{ xs: 'column', md: 'row' }}>
                          <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <CodeIcon fontSize="small" />
                              Headers:
                            </Typography>
                            <Box sx={{ fontFamily: 'monospace', fontSize: '0.8rem', bgcolor: 'grey.50', p: 1.5, borderRadius: 1, border: '1px solid', borderColor: 'grey.300', maxHeight: 200, overflow: 'auto' }}>
                              {Object.entries(testResult.responseDetails.headers).map(([key, value]) => (
                                <Box key={key} sx={{ mb: 0.3 }}>
                                  <strong style={{ color: '#1976d2' }}>{key}:</strong>{' '}
                                  <span style={{ color: '#333', wordBreak: 'break-all' }}>{value}</span>
                                </Box>
                              ))}
                            </Box>
                          </Box>
                          
                          <Box sx={{ flex: 1 }}>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <CodeIcon fontSize="small" />
                              Body:
                            </Typography>
                            <Box sx={{ 
                              fontFamily: 'monospace', 
                              fontSize: '0.8rem', 
                              bgcolor: 'grey.50', 
                              p: 1.5, 
                              borderRadius: 1, 
                              border: '1px solid', 
                              borderColor: 'grey.300',
                              maxHeight: 300, 
                              overflow: 'auto' 
                            }}>
                              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: '0.8rem' }}>
                                {testResult.responseDetails.body ? 
                                  JSON.stringify(testResult.responseDetails.body, null, 2) : 
                                  '(empty response body)'
                                }
                              </pre>
                            </Box>
                          </Box>
                        </Stack>
                        
                        {/* Security Analysis for Failed Requests */}
                        {!testResult.success && testResult.responseDetails.headers && (
                          <Box sx={{ mt: 2, p: 2, bgcolor: 'warning.50', borderRadius: 1, border: '1px solid', borderColor: 'warning.200' }}>
                            <Typography variant="subtitle2" gutterBottom color="warning.main">
                              🔒 Security Analysis:
                            </Typography>
                            {testResult.responseDetails.headers['x-ext-auth-reason'] && (
                              <Box sx={{ mb: 1 }}>
                                <Typography variant="body2" sx={{ fontWeight: 'bold', mb: 0.5 }}>
                                  Kuadrant Auth Failure:
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem', bgcolor: 'white', p: 1, borderRadius: 1 }}>
                                  {testResult.responseDetails.headers['x-ext-auth-reason']}
                                </Typography>
                              </Box>
                            )}
                            {testResult.responseDetails.headers['www-authenticate'] && (
                              <Box>
                                <Typography variant="body2" sx={{ fontWeight: 'bold', mb: 0.5 }}>
                                  Available Auth Methods:
                                </Typography>
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem', bgcolor: 'white', p: 1, borderRadius: 1 }}>
                                  {testResult.responseDetails.headers['www-authenticate']}
                                </Typography>
                              </Box>
                            )}
                          </Box>
                        )}
                      </>
                    ) : (
                      <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                        Response details not available
                      </Typography>
                    )}
                  </CardContent>
                </Card>
              </Box>
            )}
          </Stack>
        </CardContent>
      </Card>
    </Box>
  );
};

export default TokenManagement;