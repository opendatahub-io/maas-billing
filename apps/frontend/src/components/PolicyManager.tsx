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
  Tooltip,
  Collapse,
  Card,
  CardContent,
  Stack,
  Divider,
} from '@mui/material';
import {
  Search as SearchIcon,
  Schedule as ScheduleIcon,
  Security as AuthIcon,
  Speed as RateLimitIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
  Policy as PolicyIcon,
  Group as GroupIcon,
  Key as KeyIcon,
} from '@mui/icons-material';

import { Policy } from '../types';
import apiService from '../services/api';

const PolicyManager: React.FC = () => {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [filteredPolicies, setFilteredPolicies] = useState<Policy[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [expandedPolicies, setExpandedPolicies] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchPolicies = async () => {
      try {
        setLoading(true);
        const data = await apiService.getPolicies();
        // Ensure data is always an array
        const policies = Array.isArray(data) ? data : [];
        setPolicies(policies);
        setFilteredPolicies(policies);
      } catch (error) {
        console.error('Failed to fetch policies:', error);
        // Set empty arrays on error to prevent map errors
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

          {/* Rules Details */}
          {policy.items && policy.items.length > 0 && (
            <Card variant="outlined">
              <CardContent>
                <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  {policy.type === 'auth' ? <AuthIcon fontSize="small" /> : <RateLimitIcon fontSize="small" />}
                  {policy.type === 'auth' ? 'Authentication & Authorization Rules' : 'Rate Limiting Rules'}
                </Typography>
                <Stack spacing={2}>
                  {policy.items.map((item: any, index: number) => (
                    <Card key={index} variant="outlined" sx={{ bgcolor: 'grey.50' }}>
                      <CardContent>
                        <Typography variant="subtitle1" gutterBottom>
                          {item.id} ({item.type})
                        </Typography>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                          {item.description}
                        </Typography>
                        
                        {/* Authentication Rules */}
                        {item.type === 'authentication' && (
                          <Box>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <KeyIcon fontSize="small" />
                              API Key Authentication
                            </Typography>
                            <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                              Prefix: {item.config?.credentials?.authorizationHeader?.prefix || 'APIKEY'}
                            </Typography>
                          </Box>
                        )}
                        
                        {/* Authorization Rules */}
                        {item.type === 'authorization' && item.allowedGroups && (
                          <Box>
                            <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <GroupIcon fontSize="small" />
                              Allowed Groups
                            </Typography>
                            <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                              {item.allowedGroups.map((group: string) => (
                                <Chip key={group} label={group} size="small" color="primary" variant="outlined" />
                              ))}
                            </Box>
                          </Box>
                        )}
                        
                        {/* Rate Limit Rules */}
                        {item.type === 'rate-limit' && item.rates && (
                          <Box>
                            <Typography variant="subtitle2" gutterBottom>Rate Limits:</Typography>
                            {item.rates.map((rate: any, rateIndex: number) => (
                              <Chip 
                                key={rateIndex}
                                label={`${rate.limit} tokens per ${rate.window}`}
                                size="small"
                                color="warning"
                                sx={{ mr: 1, mb: 1 }}
                              />
                            ))}
                          </Box>
                        )}
                      </CardContent>
                    </Card>
                  ))}
                </Stack>
              </CardContent>
            </Card>
          )}
        </Stack>
      </Box>
    );
  };

  const getTierColor = (tierName: string) => {
    // Define distinct colors for each tier
    const tierColors: { [key: string]: string } = {
      'free': '#4caf50',        // Green
      'premium': '#ff9800',     // Orange
      'enterprise': '#9c27b0',  // Purple
      'basic': '#2196f3',       // Blue
      'pro': '#f44336',         // Red
      'ultimate': '#795548',    // Brown
    };
    return tierColors[tierName.toLowerCase()] || '#666';
  };

  const formatTimeRange = (policy: Policy) => {
    // Handle real Kuadrant policies that don't have timeRange
    if (!policy.timeRange) {
      return 'Always Active';
    }
    if (policy.timeRange.unlimited) {
      return 'Unlimited';
    }
    return `${policy.timeRange.startTime} - ${policy.timeRange.endTime}`;
  };

  const formatRequestLimits = (policy: Policy) => {
    // Handle real Kuadrant policies that don't have requestLimits
    if (!policy.requestLimits) {
      return null; // Don't show anything for auth policies
    }
    if (policy.requestLimits.tokenLimit === null || policy.requestLimits.tokenLimit === undefined) {
      return 'Unlimited';
    }
    return `${policy.requestLimits.tokenLimit.toLocaleString()} requests/${policy.requestLimits.timePeriod}`;
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
                    <Chip 
                      label={`${policy.items?.length || 0} configured`} 
                      size="small" 
                      color="info" 
                      variant="outlined"
                    />
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
                  <TableCell style={{ paddingBottom: 0, paddingTop: 0 }} colSpan={7}>
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
            {searchTerm ? 'Try adjusting your search criteria' : 'No policies found. Ensure you are authenticated with the cluster.'}
          </Typography>
        </Box>
      )}

    </Box>
  );
};

export default PolicyManager;