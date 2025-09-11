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
  Groups as TeamsIcon,
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

interface Team {
  team_id: string;
  team_name: string;
  policy: string;
  description: string;
  rate_limit: {
    limit: number | string;
    window: string;
    description: string;
  };
}

const TokenManagement: React.FC = () => {
  const [tokens, setTokens] = useState<UserToken[]>([]);
  const [userTier, setUserTier] = useState<UserTier | null>(null);
  const [newTokenName, setNewTokenName] = useState('');
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Team-based token creation state
  const [availableTeams, setAvailableTeams] = useState<Team[]>([]);
  const [selectedTeam, setSelectedTeam] = useState('');
  const [teamsLoading, setTeamsLoading] = useState(false);

  useEffect(() => {
    loadUserData();
    loadAvailableTeams();
  }, []);

  // Group tokens by team for display
  const getTokensByTeam = () => {
    const tokensByTeam: { [teamId: string]: UserToken[] } = {};
    
    tokens.forEach(token => {
      const teamId = token.team_id || 'unknown';
      if (!tokensByTeam[teamId]) {
        tokensByTeam[teamId] = [];
      }
      tokensByTeam[teamId].push(token);
    });
    
    return tokensByTeam;
  };

  // Create team data from actual tokens and policies
  const createTeamDataFromTokens = async (): Promise<Team[]> => {
    const uniqueTeams = new Set<string>();
    const teamPolicies: { [teamId: string]: string } = {};
    
    // Extract unique teams from tokens
    tokens.forEach(token => {
      if (token.team_id) {
        uniqueTeams.add(token.team_id);
        if (token.policy) {
          teamPolicies[token.team_id] = token.policy;
        }
      }
    });

    // Get rate limit info from policies with proper typing
    const rateLimit: { [key: string]: { limit: number | string; window: string; description: string } } = {
      'unlimited-policy': { limit: 100000, window: '1h', description: '100,000 tokens per hour' },
      'test-tokens': { limit: 'No specific limit', window: 'N/A', description: 'Testing tier - inherits default limits' },
      'premium': { limit: 50000, window: '1m', description: '50,000 tokens per minute' },
      'free': { limit: 10000, window: '1m', description: '10,000 tokens per minute' },
      'enterprise': { limit: 'Unlimited', window: 'N/A', description: 'Enterprise tier - no limits' }
    };

    return Array.from(uniqueTeams).map(teamId => {
      const policy = teamPolicies[teamId] || 'unknown';
      const rateLimitInfo = rateLimit[policy] || { limit: 'Unknown', window: 'N/A', description: 'Policy information not available' };
      
      return {
        team_id: teamId,
        team_name: teamId === 'default' ? 'Default Team' : 
                   teamId === 'test-team' ? 'Test Team' :
                   teamId.replace('-', ' ').replace(/\b\w/g, l => l.toUpperCase()),
        policy: policy,
        description: teamId === 'default' ? 'Default team for all users' :
                     teamId === 'test-team' ? 'Testing team' :
                     `${teamId} team`,
        rate_limit: rateLimitInfo
      };
    });
  };

  // Get team info for a team ID
  const getTeamInfo = (teamId: string) => {
    return availableTeams.find(team => team.team_id === teamId) || {
      team_id: teamId,
      team_name: teamId === 'default' ? 'Default Team' : teamId.replace('-', ' ').replace(/\b\w/g, l => l.toUpperCase()),
      policy: tokens.find(t => t.team_id === teamId)?.policy || 'unknown',
      description: '',
      rate_limit: {
        limit: 'Unknown',
        window: 'N/A',
        description: 'Policy information not available'
      }
    };
  };

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

  const loadAvailableTeams = async () => {
    try {
      setTeamsLoading(true);
      const teams = await apiService.getTeams();
      console.log('Loaded teams:', teams);
      
      // Create team data based on actual tokens and policies
      const teamsData = teams && teams.length > 0 ? teams : await createTeamDataFromTokens();
      
      setAvailableTeams(teamsData);
      
      // Auto-select default team if none selected
      if (!selectedTeam && teamsData.length > 0) {
        setSelectedTeam(teamsData[0].team_id);
      }
    } catch (error) {
      console.error('Error loading teams:', error);
      setError('Failed to load teams');
    } finally {
      setTeamsLoading(false);
    }
  };

  const handleGenerateToken = async () => {
    if (!newTokenName.trim()) {
      setError('Token name is required');
      return;
    }

    if (!selectedTeam) {
      setError('Please select a team');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      
      const response = await apiService.createTeamToken(selectedTeam, {
        user_id: 'noyitz', // TODO: Get from auth context
        alias: newTokenName.trim(),
      });
      
      console.log('Token creation response:', response);
      
      // Extract the API key from the response
      const apiKey = response?.data?.api_key || response?.api_key || response?.actualApiKey;
      if (apiKey) {
        setGeneratedToken(apiKey);
      } else {
        console.warn('No API key found in response:', response);
        setGeneratedToken('Token created successfully, but API key not returned');
      }
      setNewTokenName('');
      
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
      setError(null);
      console.log('🗑️ Attempting to delete token:', tokenName);
      
      const result = await apiService.revokeToken(tokenName);
      console.log('🗑️ Delete result:', result);
      
      // Refresh the token list to reflect the deletion
      await loadUserData();
      console.log('✅ Token list refreshed after deletion');
    } catch (error: any) {
      console.error('❌ Token deletion failed:', error);
      setError(error.message || 'Failed to revoke token');
    } finally {
      setLoading(false);
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
      case 'expired': return 'default'; // Changed from 'error' to avoid red
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

      {/* Multi-Team Account Information */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <SecurityIcon />
            Team Memberships & Rate Limits
          </Typography>
          
          {/* Summary Stats */}
          {!loading && tokens.length > 0 && (
            <Box sx={{ mb: 3, p: 2, bgcolor: 'primary.50', borderRadius: 1, border: '1px solid', borderColor: 'primary.200' }}>
              <Stack direction="row" spacing={3} sx={{ flexWrap: 'wrap', gap: 2 }}>
                <Box>
                  <Typography variant="h6" color="primary.main">
                    {Object.keys(getTokensByTeam()).length}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Team{Object.keys(getTokensByTeam()).length !== 1 ? 's' : ''}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="h6" color="primary.main">
                    {tokens.length}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Total Token{tokens.length !== 1 ? 's' : ''}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="h6" color="primary.main">
                    {new Set(tokens.map(t => t.policy)).size}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Unique Polic{new Set(tokens.map(t => t.policy)).size !== 1 ? 'ies' : 'y'}
                  </Typography>
                </Box>
              </Stack>
            </Box>
          )}
          
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Your active team memberships with policies, rate limits, and token counts.
          </Typography>
          
          {loading ? (
            <Box display="flex" justifyContent="center" p={2}>
              <CircularProgress size={20} />
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table size="small">
                <TableHead>
                  <TableRow sx={{ bgcolor: 'grey.50' }}>
                    <TableCell sx={{ fontWeight: 600 }}>Team</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Policy & Rate Limits</TableCell>
                    <TableCell align="center" sx={{ fontWeight: 600 }}>Tokens</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Active Tokens</TableCell>
                    <TableCell align="center" sx={{ fontWeight: 600 }}>Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {Object.entries(getTokensByTeam()).map(([teamId, teamTokens]) => {
                    const teamInfo = getTeamInfo(teamId);
                    return (
                      <TableRow key={teamId} sx={{ '&:hover': { bgcolor: 'grey.50' } }}>
                        <TableCell>
                          <Box>
                            <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                              {teamInfo.team_name}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              ID: {teamId}
                            </Typography>
                          </Box>
                        </TableCell>
                        <TableCell>
                          <Stack spacing={1}>
                            <Box>
                              <Chip 
                                label={teamInfo.policy}
                                color="primary" 
                                size="small"
                                variant="outlined"
                              />
                            </Box>
                            <Typography variant="caption" color="text.secondary">
                              {teamInfo.rate_limit.description}
                            </Typography>
                          </Stack>
                        </TableCell>
                        <TableCell align="center">
                          <Chip 
                            label={teamTokens.length}
                            color="info" 
                            size="small"
                          />
                        </TableCell>
                        <TableCell>
                          <Stack spacing={0.5}>
                            {teamTokens.slice(0, 3).map(token => (
                              <Box key={token.name} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Typography variant="body2" sx={{ fontSize: '0.875rem' }}>
                                  {token.displayName || token.alias || token.name}
                                </Typography>
                                <Chip 
                                  label={token.status}
                                  color={getStatusColor(token.status) as any}
                                  size="small"
                                  sx={{ height: 16, fontSize: '0.7rem' }}
                                />
                              </Box>
                            ))}
                            {teamTokens.length > 3 && (
                              <Typography variant="caption" color="text.secondary">
                                +{teamTokens.length - 3} more...
                              </Typography>
                            )}
                          </Stack>
                        </TableCell>
                        <TableCell align="center">
                          <Stack direction="row" spacing={0.5} justifyContent="center">
                            {teamTokens.slice(0, 2).map(token => (
                              <Tooltip key={token.name} title={`Copy ${token.displayName || token.alias || token.name} API key`}>
                                <IconButton 
                                  size="small" 
                                  onClick={() => copyToClipboard(token.actualApiKey || token.name)}
                                >
                                  <CopyIcon fontSize="small" />
                                </IconButton>
                              </Tooltip>
                            ))}
                            {teamTokens.length > 2 && (
                              <Typography variant="caption" color="text.secondary">
                                +{teamTokens.length - 2}
                              </Typography>
                            )}
                          </Stack>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
              
              {Object.keys(getTokensByTeam()).length === 0 && (
                <Box sx={{ p: 3, textAlign: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    No team memberships found. Create your first token below.
                  </Typography>
                </Box>
              )}
            </TableContainer>
          )}
        </CardContent>
      </Card>

      {/* Team-Based Token Creation */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <TeamsIcon />
            Create Token by Team
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Select a team to create a token with the team's policy and rate limits.
          </Typography>
          
          <Stack spacing={3}>
            {/* Team Selection */}
            <FormControl fullWidth>
              <Typography variant="body2" sx={{ mb: 1 }}>Select Team *</Typography>
              <Select
                value={selectedTeam}
                onChange={(e) => setSelectedTeam(e.target.value)}
                disabled={teamsLoading}
              >
                {teamsLoading ? (
                  <MenuItem disabled>Loading teams...</MenuItem>
                ) : availableTeams.length === 0 ? (
                  <MenuItem disabled>No teams available</MenuItem>
                ) : (
                  availableTeams.map((team) => (
                    <MenuItem key={team.team_id} value={team.team_id}>
                      <Box sx={{ width: '100%' }}>
                        <Typography variant="body2" sx={{ fontWeight: 600 }}>
                          {team.team_name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          Policy: {team.policy} • {team.rate_limit.description}
                        </Typography>
                      </Box>
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>


            {/* Token Name Input */}
            <TextField
              fullWidth
              label="Token Name/Alias *"
              value={newTokenName}
              onChange={(e) => setNewTokenName(e.target.value)}
              placeholder="e.g., my-test-token, premium-api-key"
              disabled={loading}
              helperText="Choose a descriptive name for your token"
            />

            {/* Create Button */}
            <Button
              variant="contained"
              onClick={handleGenerateToken}
              disabled={loading || !selectedTeam || !newTokenName.trim()}
              startIcon={loading ? <CircularProgress size={16} /> : <AddIcon />}
              size="large"
            >
              {loading ? 'Creating Token...' : 'Create Team Token'}
            </Button>
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
          🎉 Token Generated Successfully!
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

      {/* Individual Token Management */}
      {tokens.length > 0 && (
        <Card sx={{ mt: 3 }}>
          <CardContent>
            <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <KeyIcon />
              Token Management
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              Detailed view and management of all your API tokens.
            </Typography>
            
            <TableContainer component={Paper} variant="outlined">
              <Table size="small">
                <TableHead>
                  <TableRow sx={{ bgcolor: 'grey.50' }}>
                    <TableCell sx={{ fontWeight: 600 }}>Token Name</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Team & Policy</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Status</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Created</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>API Key</TableCell>
                    <TableCell align="center" sx={{ fontWeight: 600 }}>Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {tokens.map((token) => (
                    <TableRow key={token.name} sx={{ '&:hover': { bgcolor: 'grey.50' } }}>
                      <TableCell>
                        <Box>
                          <Typography variant="subtitle2">
                            {token.displayName || token.alias || token.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {token.secret_name || token.name}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Stack spacing={0.5}>
                          <Chip 
                            label={token.team_name || 'Unknown Team'}
                            color="secondary" 
                            size="small"
                            variant="outlined"
                          />
                          <Chip 
                            label={token.policy || 'No Policy'}
                            color="primary"
                            size="small"
                            variant="outlined"
                          />
                        </Stack>
                      </TableCell>
                      <TableCell>
                        <Chip 
                          label={token.status}
                          color={getStatusColor(token.status) as any}
                          size="small"
                        />
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2">
                            {formatDate(token.created)}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {token.usage} uses
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        {token.actualApiKey ? (
                          <Box sx={{ 
                            fontFamily: 'monospace', 
                            fontSize: '0.75rem',
                            p: 1,
                            bgcolor: 'grey.100',
                            borderRadius: 1,
                            border: '1px solid',
                            borderColor: 'grey.300',
                            maxWidth: 200,
                            overflow: 'hidden'
                          }}>
                            {token.actualApiKey.substring(0, 16)}...
                          </Box>
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            Not available
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell align="center">
                        <Stack direction="row" spacing={1} justifyContent="center">
                          <Tooltip title="Copy full API key">
                            <IconButton 
                              size="small" 
                              onClick={() => copyToClipboard(token.actualApiKey || token.name)}
                              disabled={!token.actualApiKey}
                            >
                              <CopyIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Revoke token">
                            <IconButton 
                              size="small" 
                              sx={{ color: 'text.secondary', '&:hover': { color: 'warning.main', bgcolor: 'warning.50' } }}
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
          </CardContent>
        </Card>
      )}

    </Box>
  );
};

export default TokenManagement;