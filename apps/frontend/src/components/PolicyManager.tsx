import React, { useState, useEffect } from 'react';
import {
  Box,
  Chip,
  TextField,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Collapse,
  Card,
  CardContent,
  Stack,
} from '@mui/material';
import {
  Search as SearchIcon,
  Security as AuthIcon,
  Speed as RateLimitIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
  Policy as PolicyIcon,
  Group as GroupIcon,
} from '@mui/icons-material';

import { Policy } from '../types';
import apiService from '../services/api';

const PolicyManager: React.FC = () => {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [filteredPolicies, setFilteredPolicies] = useState<Policy[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [expandedPolicies, setExpandedPolicies] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchPolicies = async () => {
      try {
        setLoading(true);
        setError(null);
        console.log('🔄 Fetching policies from API...');
        const data = await apiService.getPolicies();
        console.log('📊 Raw API response:', data);
        
        // Ensure data is always an array
        const policies = Array.isArray(data) ? data : [];
        console.log(`✅ Processed ${policies.length} policies:`, policies);
        
        setPolicies(policies);
        setFilteredPolicies(policies);
      } catch (error) {
        console.error('❌ Failed to fetch policies:', error);
        const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred';
        setError(`Failed to fetch policies: ${errorMessage}`);
        setPolicies([]);
        setFilteredPolicies([]);
      } finally {
        setLoading(false);
      }
    };

    fetchPolicies();
  }, []);

  useEffect(() => {
    const filtered = policies.filter(policy =>
      policy.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      policy.description.toLowerCase().includes(searchTerm.toLowerCase())
    );
    setFilteredPolicies(filtered);
  }, [searchTerm, policies]);


  const togglePolicyExpansion = (policyId: string) => {
    const newExpanded = new Set(expandedPolicies);
    if (newExpanded.has(policyId)) {
      newExpanded.delete(policyId);
    } else {
      newExpanded.add(policyId);
    }
    setExpandedPolicies(newExpanded);
  };

  const renderPolicyDetails = (policy: Policy) => {
    return (
      <Box sx={{ p: 2, bgcolor: 'background.paper' }}>
        <Stack spacing={3}>
          {/* Basic Information */}
          <Card variant="outlined">
            <CardContent>
              <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <PolicyIcon fontSize="small" />
                Policy Details
              </Typography>
              <Stack spacing={1}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">Namespace:</Typography>
                  <Typography variant="body2">{policy.namespace}</Typography>
                </Box>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">Target:</Typography>
                  <Typography variant="body2">
                    {policy.targetRef?.kind}/{policy.targetRef?.name || 'Unknown'}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">Created:</Typography>
                  <Typography variant="body2">{new Date(policy.created).toLocaleString()}</Typography>
                </Box>
              </Stack>
            </CardContent>
          </Card>

          {/* Authentication Policy Details */}
          {policy.type === 'auth' && (
            <Card variant="outlined">
              <CardContent>
                <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <AuthIcon fontSize="small" />
                  Authentication Policy Details
                </Typography>
                <Stack spacing={2}>
                  {/* API Key Configuration */}
                  {policy.config?.auth && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>API Key Authentication</Typography>
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                        <Chip 
                          label={`Method: ${policy.config.auth.type}`} 
                          size="small" 
                          color="primary" 
                          variant="outlined"
                        />
                        <Chip 
                          label={policy.config.auth.required ? 'Required' : 'Optional'} 
                          size="small" 
                          color={policy.config.auth.required ? 'success' : 'default'} 
                          variant="outlined"
                        />
                      </Box>
                    </Box>
                  )}
                  
                  {/* Authentication Rules */}
                  {(policy.config as any)?.rules?.authentication && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>Authentication Methods:</Typography>
                      {Object.entries((policy.config as any).rules.authentication).map(([ruleName, ruleConfig]: [string, any]) => (
                        <Card key={ruleName} variant="outlined" sx={{ bgcolor: 'grey.50', mb: 1 }}>
                          <CardContent sx={{ py: 1 }}>
                            <Typography variant="subtitle2" gutterBottom>
                              {ruleName.replace(/-/g, ' ').toUpperCase()}
                            </Typography>
                            {ruleConfig.credentials?.authorizationHeader && (
                              <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                                Prefix: {ruleConfig.credentials.authorizationHeader.prefix}
                              </Typography>
                            )}
                            <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
                              <Chip label={`Priority: ${ruleConfig.priority || 0}`} size="small" variant="outlined" />
                              <Chip 
                                label={ruleConfig.metrics ? 'Metrics: ON' : 'Metrics: OFF'} 
                                size="small" 
                                color={ruleConfig.metrics ? 'success' : 'default'}
                                variant="outlined"
                              />
                            </Box>
                          </CardContent>
                        </Card>
                      ))}
                    </Box>
                  )}
                  
                  {/* Allowed Groups/Tiers */}
                  {policy.items && policy.items.length > 0 && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <GroupIcon fontSize="small" />
                        Allowed Tiers
                      </Typography>
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                        {policy.items.map((item: any) => (
                          <Chip 
                            key={item.id} 
                            label={item.value} 
                            size="small" 
                            color="primary" 
                            variant="filled"
                            sx={{ backgroundColor: getTierColor(item.value), color: 'white' }}
                          />
                        ))}
                      </Box>
                    </Box>
                  )}
                </Stack>
              </CardContent>
            </Card>
          )}
          
          {/* Rate Limit Policy Details */}
          {policy.type === 'rate-limit' && (
            <Card variant="outlined">
              <CardContent>
                <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <RateLimitIcon fontSize="small" />
                  Rate Limit Policy Details
                </Typography>
                <Stack spacing={2}>
                  {/* Rate Limit Configuration */}
                  {policy.config?.rateLimit && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>Rate Limit Configuration</Typography>
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                        <Chip 
                          label={`${policy.config.rateLimit.requests.toLocaleString()} ${policy.config.rateLimit.unit}`} 
                          size="medium" 
                          color="warning" 
                          variant="filled"
                        />
                        <Chip 
                          label={`Per ${policy.config.rateLimit.duration}`} 
                          size="medium" 
                          color="info" 
                          variant="outlined"
                        />
                      </Box>
                    </Box>
                  )}
                  
                  {/* Token Limits */}
                  {policy.requestLimits && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>Token Limits</Typography>
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                        <Chip 
                          label={`${policy.requestLimits.tokenLimit!.toLocaleString()} tokens`} 
                          size="medium" 
                          color="secondary" 
                          variant="filled"
                        />
                        <Chip 
                          label={`Per ${policy.requestLimits.timePeriod}`} 
                          size="medium" 
                          color="default" 
                          variant="outlined"
                        />
                      </Box>
                    </Box>
                  )}
                  
                  {/* Applied Groups/Tiers */}
                  {policy.items && policy.items.length > 0 && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>Applies To User Groups/Tiers</Typography>
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                        {policy.items.map((item: any) => (
                          <Chip 
                            key={item.id} 
                            label={item.value} 
                            size="medium" 
                            color="primary" 
                            variant="filled"
                            sx={{ backgroundColor: getTierColor(item.value), color: 'white' }}
                          />
                        ))}
                      </Box>
                      {policy.items[0]?.predicate && (
                        <Card variant="outlined" sx={{ bgcolor: 'grey.50', p: 1 }}>
                          <Typography variant="caption" color="text.secondary" gutterBottom>
                            Group Selection Logic:
                          </Typography>
                          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                            {policy.items[0].predicate}
                          </Typography>
                        </Card>
                      )}
                    </Box>
                  )}
                  
                  {/* Limitador Configuration Details */}
                  {(policy.config as any)?.limits && (policy.config as any).limits.length > 0 && (
                    <Box>
                      <Typography variant="subtitle1" gutterBottom>Limitador Configuration</Typography>
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                        This policy is applied to {(policy.config as any).limits.length} service endpoints
                      </Typography>
                      <Box sx={{ maxHeight: '200px', overflow: 'auto' }}>
                        {(policy.config as any).limits.slice(0, 3).map((limit: any, index: number) => (
                          <Card key={index} variant="outlined" sx={{ bgcolor: 'grey.50', mb: 1 }}>
                            <CardContent sx={{ py: 1 }}>
                              <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                                {limit.namespace}
                              </Typography>
                              <Box sx={{ display: 'flex', gap: 1, mt: 0.5 }}>
                                <Chip label={`${limit.max_value} requests`} size="small" variant="outlined" />
                                <Chip label={`${limit.seconds}s window`} size="small" variant="outlined" />
                              </Box>
                            </CardContent>
                          </Card>
                        ))}
                        {(policy.config as any).limits.length > 3 && (
                          <Typography variant="caption" color="text.secondary">
                            ... and {(policy.config as any).limits.length - 3} more endpoints
                          </Typography>
                        )}
                      </Box>
                    </Box>
                  )}
                </Stack>
              </CardContent>
            </Card>
          )}
          
          {/* Policy Status */}
          <Card variant="outlined">
            <CardContent>
              <Typography variant="h6" gutterBottom>Policy Status</Typography>
              <Stack spacing={1}>
                {policy.status?.conditions && policy.status.conditions.map((condition: any, index: number) => (
                  <Box key={index} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Chip 
                      label={condition.type} 
                      size="small" 
                      color={condition.status === 'True' ? 'success' : 'error'}
                      variant="filled"
                    />
                    <Typography variant="body2" color="text.secondary">
                      {condition.message} ({condition.reason})
                    </Typography>
                  </Box>
                ))}
                {(!policy.status?.conditions || policy.status.conditions.length === 0) && (
                  <Typography variant="body2" color="text.secondary">
                    No status conditions available
                  </Typography>
                )}
              </Stack>
            </CardContent>
          </Card>
        </Stack>
      </Box>
    );
  };

  const getTierColor = (tierName: string) => {
    // Handle undefined/null tierName
    if (!tierName) return '#666';
    
    // Define distinct colors for each tier
    const tierColors: { [key: string]: string } = {
      'free': '#4caf50',                    // Green
      'free user tokens': '#4caf50',        // Green
      'premium': '#ff9800',                 // Orange  
      'premium user tokens': '#ff9800',     // Orange
      'enterprise': '#9c27b0',              // Purple
      'basic': '#2196f3',                   // Blue
      'pro': '#f44336',                     // Red
      'ultimate': '#795548',                // Brown
      'rate limit test': '#e91e63',         // Pink
      'unlimited policy': '#607d8b',        // Blue Grey
      'test-tokens': '#ff5722',             // Deep Orange
    };
    return tierColors[tierName.toLowerCase()] || '#666';
  };


  const getPolicyTypeIcon = (type: string) => {
    return type === 'auth' ? <AuthIcon /> : <RateLimitIcon />;
  };

  const getPolicyTypeColor = (type: string) => {
    return type === 'auth' ? 'primary' : 'secondary';
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <Typography>Loading policies...</Typography>
      </Box>
    );
  }

  if (error) {
    return (
      <Box>
        <Typography variant="h4" component="h1" gutterBottom>
          Policy Management
        </Typography>
        <Card sx={{ mt: 3, border: 2, borderColor: 'error.main' }}>
          <CardContent>
            <Typography variant="h6" color="error" gutterBottom>
              Error Loading Policies
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {error}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
              Please check:
            </Typography>
            <Typography variant="body2" component="ul" color="text.secondary" sx={{ mt: 1, pl: 2 }}>
              <li>Backend service is running on port 3001</li>
              <li>You are authenticated with the Kuadrant cluster</li>
              <li>Network connectivity to the cluster</li>
            </Typography>
          </CardContent>
        </Card>
      </Box>
    );
  }

  return (
    <Box>
      {/* Header */}
      <Box sx={{ mb: 3 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Policy Management
        </Typography>
        <Typography variant="body1" color="text.secondary">
          View and manage Kuadrant policies for authentication and rate limiting
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          <strong>Rate limit policies</strong> apply to specific user groups/tiers (free, premium, enterprise). 
          <strong>Authentication policies</strong> control which tiers can access the API.
        </Typography>
      </Box>

      {/* Search */}
      <Box sx={{ mb: 3 }}>
        <TextField
          fullWidth
          variant="outlined"
          placeholder="Search policies..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          InputProps={{
            startAdornment: <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} />,
          }}
        />
      </Box>

      {/* Policies Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell width="40px"></TableCell>
              <TableCell>Policy Name & Type</TableCell>
              <TableCell>Namespace</TableCell>
              <TableCell>Target</TableCell>
              <TableCell>Groups/Tiers</TableCell>
              <TableCell>Rules</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredPolicies.map((policy) => (
              <React.Fragment key={policy.id}>
                <TableRow 
                  hover 
                  sx={{ cursor: 'pointer' }} 
                  onClick={() => togglePolicyExpansion(policy.id)}
                >
                  <TableCell>
                    {expandedPolicies.has(policy.id) ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Chip
                        icon={getPolicyTypeIcon(policy.type)}
                        label={policy.type === 'auth' ? 'AUTH POLICY' : 'RATE LIMIT'}
                        color={getPolicyTypeColor(policy.type) as any}
                        size="small"
                        variant="filled"
                      />
                      <Box>
                        <Typography variant="subtitle2" fontWeight="bold">
                          {policy.name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {policy.id}
                        </Typography>
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip label={policy.namespace} size="small" variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">
                      {policy.targetRef?.kind || 'Gateway'}/{policy.targetRef?.name || 'inference-gateway'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                      {policy.items && policy.items.length > 0 ? (
                        policy.items.map((item: any) => (
                          <Chip 
                            key={item.id} 
                            label={item.value} 
                            size="small" 
                            variant="filled"
                            sx={{ 
                              backgroundColor: getTierColor(item.value), 
                              color: 'white',
                              fontSize: '0.7rem',
                              height: '20px'
                            }}
                          />
                        ))
                      ) : (
                        <Chip 
                          label="all" 
                          size="small" 
                          variant="outlined"
                          sx={{ 
                            fontSize: '0.7rem',
                            height: '20px',
                            color: '#666'
                          }}
                        />
                      )}
                    </Box>
                  </TableCell>
                  <TableCell>
                    {policy.type === 'auth' ? (
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                        <Chip 
                          label={`${policy.items?.length || 0} tiers`} 
                          size="small" 
                          color="primary" 
                          variant="outlined"
                        />
                        {policy.config?.auth?.type && (
                          <Chip 
                            label={policy.config.auth.type} 
                            size="small" 
                            color="success" 
                            variant="filled"
                          />
                        )}
                      </Box>
                    ) : (
                      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                        {policy.requestLimits && policy.requestLimits.tokenLimit && (
                          <Chip 
                            label={`${policy.requestLimits.tokenLimit!.toLocaleString()}/${policy.requestLimits.timePeriod}`}
                            size="small" 
                            color="warning" 
                            variant="filled"
                          />
                        )}
                        {policy.items && policy.items.length > 0 && (
                          <Chip 
                            label={`→ ${policy.items[0].value} tier`}
                            size="small" 
                            color="primary" 
                            variant="outlined"
                            sx={{ borderColor: getTierColor(policy.items[0].value), color: getTierColor(policy.items[0].value) }}
                          />
                        )}
                        <Chip 
                          label={`${(policy.config as any)?.limits?.length || 0} endpoints`} 
                          size="small" 
                          color="info" 
                          variant="outlined"
                        />
                      </Box>
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={policy.isActive ? 'Active' : 'Inactive'}
                      color={policy.isActive ? 'success' : 'default'}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      {new Date(policy.created).toLocaleDateString()}
                    </Typography>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell style={{ paddingBottom: 0, paddingTop: 0 }} colSpan={8}>
                    <Collapse in={expandedPolicies.has(policy.id)} timeout="auto" unmountOnExit>
                      {renderPolicyDetails(policy)}
                    </Collapse>
                  </TableCell>
                </TableRow>
              </React.Fragment>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Empty State */}
      {filteredPolicies.length === 0 && (
        <Box
          sx={{
            textAlign: 'center',
            py: 6,
            bgcolor: 'background.paper',
            borderRadius: 1,
            mt: 2,
          }}
        >
          <Typography variant="h6" color="text.secondary" gutterBottom>
            No policies found
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {searchTerm ? 'Try adjusting your search criteria' : 'No Kuadrant policies are currently configured in the cluster.'}
          </Typography>
        </Box>
      )}

    </Box>
  );
};

export default PolicyManager;