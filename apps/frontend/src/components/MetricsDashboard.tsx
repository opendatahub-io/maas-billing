import React, { useState } from 'react';
import {
  Box,
  Card,
  CardContent,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Paper,
  Grid,
  CircularProgress,
  Alert,
  Toolbar,
  Collapse,
  IconButton,
  Tooltip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  List,
  ListItem,
  ListItemText,
  Divider,
} from '@mui/material';
import {
  CheckCircle as AcceptIcon,
  Cancel as RejectIcon,
  Security as PolicyIcon,
  Speed as RateLimitIcon,
  ExpandMore as ExpandMoreIcon,
  KeyboardArrowDown,
  KeyboardArrowUp,
  Info as InfoIcon,
  Timer as TimerIcon,
  Token as TokenIcon,
  AttachMoney as CostIcon,
  SmartToy as ModelIcon,
  Group as TeamIcon,
  Http as EndpointIcon,
  Search as SearchIcon,
} from '@mui/icons-material';

import { useLiveRequests, useDashboardStats } from '../hooks/useApi';
import { Request } from '../types';

// Helper function to format duration in ms with k notation for large values
const formatDuration = (milliseconds: number): string => {
  if (milliseconds >= 1000) {
    const k = (milliseconds / 1000).toFixed(1);
    return k.endsWith('.0') ? `${Math.floor(milliseconds / 1000)}kms` : `${k}kms`;
  }
  
  return `${milliseconds}ms`;
};

// Expandable row component
const RequestRow: React.FC<{ request: Request }> = ({ request }) => {
  const [open, setOpen] = useState(false);
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);

  const getPolicyChipProps = (policyType?: string) => {
    switch (policyType) {
      case 'AuthPolicy':
        return { color: 'primary' as const, icon: <PolicyIcon /> };
      case 'RateLimitPolicy':
        return { color: 'warning' as const, icon: <RateLimitIcon /> };
      case 'None':
        return { color: 'success' as const, icon: <AcceptIcon /> };
      default:
        return { color: 'default' as const, icon: undefined };
    }
  };

  const getDecisionChipProps = (decision: string) => {
    return decision === 'accept' 
      ? { color: 'success' as const, icon: <AcceptIcon /> }
      : { color: 'error' as const, icon: <RejectIcon /> };
  };

  const policyProps = getPolicyChipProps(request.policyType);
  const decisionProps = getDecisionChipProps(request.decision);

  return (
    <>
      <TableRow hover sx={{ '& > *': { borderBottom: 'unset' } }}>
        <TableCell>
          <IconButton
            aria-label="expand row"
            size="small"
            onClick={() => setOpen(!open)}
          >
            {open ? <KeyboardArrowUp /> : <KeyboardArrowDown />}
          </IconButton>
        </TableCell>
        <TableCell>
          <Typography variant="body2">
            {new Date(request.timestamp).toLocaleTimeString()}
          </Typography>
        </TableCell>
        <TableCell>
          <Chip 
            label={request.team}
            size="small"
            variant="outlined"
            icon={<TeamIcon />}
          />
        </TableCell>
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <ModelIcon fontSize="small" />
            <Typography variant="body2">
              {request.model}
            </Typography>
          </Box>
        </TableCell>
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <EndpointIcon fontSize="small" />
            <Typography variant="body2">
              {request.endpoint || 'N/A'}
            </Typography>
          </Box>
        </TableCell>
        <TableCell>
          <Chip
            label={request.decision}
            color={decisionProps.color}
            size="small"
            icon={decisionProps.icon}
          />
        </TableCell>
        <TableCell>
          <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
            {request.policyDecisions?.map((policy, index) => (
              <Chip
                key={index}
                label={policy.policyType}
                color={policy.decision === 'allow' ? 'success' : 'error'}
                size="small"
                variant="outlined"
              />
            )) || (
              <Chip
                label={request.policyType || 'Unknown'}
                color={policyProps.color}
                size="small"
                icon={policyProps.icon}
              />
            )}
          </Box>
        </TableCell>
        <TableCell align="right">
          <Typography variant="body2">
            {request.totalResponseTime ? formatDuration(request.totalResponseTime) : '-'}
          </Typography>
        </TableCell>
        <TableCell align="right">
          {request.modelInference ? (
            <Tooltip title={`Token breakdown: ${request.modelInference.inputTokens} prompt tokens + ${request.modelInference.outputTokens} completion tokens = ${request.modelInference.totalTokens} total tokens`}>
              <Box sx={{ textAlign: 'right', cursor: 'help' }}>
                <Typography variant="body2" component="span">
                  {request.modelInference.inputTokens} + {request.modelInference.outputTokens} = 
                </Typography>
                <Typography variant="body2" component="span" fontWeight="bold" sx={{ ml: 0.5 }}>
                  {request.modelInference.totalTokens}
                </Typography>
              </Box>
            </Tooltip>
          ) : (
            <Typography variant="body2" color="text.secondary">
              -
            </Typography>
          )}
        </TableCell>
        <TableCell>
          <Tooltip title="View detailed information">
            <IconButton size="small" onClick={() => setDetailDialogOpen(true)}>
              <InfoIcon />
            </IconButton>
          </Tooltip>
        </TableCell>
      </TableRow>
      <TableRow>
        <TableCell style={{ paddingBottom: 0, paddingTop: 0 }} colSpan={10}>
          <Collapse in={open} timeout="auto" unmountOnExit>
            <Box sx={{ margin: 1 }}>
              <Typography variant="h6" gutterBottom component="div">
                Request Details
              </Typography>
              <Grid container spacing={2}>
                {/* Authentication Details */}
                {request.authentication && (
                  <Grid item xs={12} md={6}>
                    <Card variant="outlined">
                      <CardContent>
                        <Typography variant="subtitle2" gutterBottom>
                          Authentication
                        </Typography>
                        <List dense>
                          <ListItem>
                            <ListItemText 
                              primary="Method" 
                              secondary={request.authentication.method}
                            />
                          </ListItem>
                          {request.authentication.principal && (
                            <ListItem>
                              <ListItemText 
                                primary="Principal" 
                                secondary={request.authentication.principal}
                              />
                            </ListItem>
                          )}
                          {request.authentication.groups && (
                            <ListItem>
                              <ListItemText 
                                primary="Groups" 
                                secondary={request.authentication.groups.join(', ')}
                              />
                            </ListItem>
                          )}
                          <ListItem>
                            <ListItemText 
                              primary="Valid" 
                              secondary={
                                <Chip 
                                  label={request.authentication.isValid ? 'Yes' : 'No'}
                                  color={request.authentication.isValid ? 'success' : 'error'}
                                  size="small"
                                />
                              }
                            />
                          </ListItem>
                        </List>
                      </CardContent>
                    </Card>
                  </Grid>
                )}

                {/* Model Inference Details */}
                {request.modelInference && (
                  <Grid item xs={12} md={6}>
                    <Card variant="outlined">
                      <CardContent>
                        <Typography variant="subtitle2" gutterBottom>
                          Model Inference
                        </Typography>
                        <List dense>
                          <ListItem>
                            <ListItemText 
                              primary="Input Tokens" 
                              secondary={request.modelInference.inputTokens}
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary="Output Tokens" 
                              secondary={request.modelInference.outputTokens}
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary="Response Time" 
                              secondary={`${request.modelInference.responseTime}ms`}
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary="Finish Reason" 
                              secondary={request.modelInference.finishReason}
                            />
                          </ListItem>
                        </List>
                      </CardContent>
                    </Card>
                  </Grid>
                )}

                {/* Policy Decisions */}
                <Grid item xs={12} md={6}>
                  <Card variant="outlined">
                    <CardContent>
                      <Typography variant="subtitle2" gutterBottom>
                        Policy Decisions
                      </Typography>
                      {request.policyDecisions?.map((policy, index) => (
                        <Box key={index} sx={{ mb: 2 }}>
                          <Typography variant="body2" fontWeight="bold">
                            {policy.policyName} ({policy.enforcementPoint})
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            Decision: <Chip 
                              label={policy.decision} 
                              color={policy.decision === 'allow' ? 'success' : 'error'} 
                              size="small" 
                            />
                          </Typography>
                          <Typography variant="body2" color="text.secondary">
                            Reason: {policy.reason}
                          </Typography>
                          {policy.processingTime && (
                            <Typography variant="body2" color="text.secondary">
                              Processing Time: {policy.processingTime}ms
                            </Typography>
                          )}
                          {index < (request.policyDecisions?.length || 0) - 1 && <Divider sx={{ mt: 1 }} />}
                        </Box>
                      ))}
                    </CardContent>
                  </Card>
                </Grid>

                {/* Raw Log Data */}
                {request.rawLogData && (
                  <Grid item xs={12} md={6}>
                    <Card variant="outlined">
                      <CardContent>
                        <Typography variant="subtitle2" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <InfoIcon fontSize="small" />
                          Raw Envoy Log Data
                        </Typography>
                        <List dense>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="primary.main">
                                  Response Code
                                </Typography>
                              }
                              secondary={
                                <Chip 
                                  label={request.rawLogData.responseCode}
                                  color={request.rawLogData.responseCode === 200 ? 'success' : 'error'}
                                  size="small"
                                />
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="info.main">
                                  Flags
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', backgroundColor: 'grey.100', px: 1, borderRadius: 1 }}>
                                  {request.rawLogData.flags || '-'}
                                </Typography>
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="secondary.main">
                                  Route
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', backgroundColor: 'grey.100', px: 1, borderRadius: 1 }}>
                                  {request.rawLogData.route || '-'}
                                </Typography>
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="warning.main">
                                  Bytes Received
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2">
                                  {request.rawLogData.bytesReceived} bytes
                                </Typography>
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="warning.main">
                                  Bytes Sent
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2">
                                  {request.rawLogData.bytesSent} bytes
                                </Typography>
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="success.main">
                                  Host
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', backgroundColor: 'success.50', px: 1, borderRadius: 1 }}>
                                  {request.rawLogData.host || '-'}
                                </Typography>
                              }
                            />
                          </ListItem>
                          <ListItem>
                            <ListItemText 
                              primary={
                                <Typography variant="body2" fontWeight="medium" color="success.main">
                                  Upstream Host
                                </Typography>
                              }
                              secondary={
                                <Typography variant="body2" sx={{ fontFamily: 'monospace', backgroundColor: 'success.50', px: 1, borderRadius: 1 }}>
                                  {request.rawLogData.upstreamHost || '-'}
                                </Typography>
                              }
                            />
                          </ListItem>
                        </List>
                      </CardContent>
                    </Card>
                  </Grid>
                )}
              </Grid>
            </Box>
          </Collapse>
        </TableCell>
      </TableRow>

      {/* Detail Dialog */}
      <Dialog open={detailDialogOpen} onClose={() => setDetailDialogOpen(false)} maxWidth="md" fullWidth>
        <DialogTitle>
          Request Details - {request.id}
        </DialogTitle>
        <DialogContent>
          <Accordion defaultExpanded>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="h6">Request Information</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <Grid container spacing={2}>
                <Grid item xs={12} sm={6}>
                  <TextField
                    label="Request ID"
                    value={request.id}
                    fullWidth
                    margin="dense"
                    variant="outlined"
                    InputProps={{ readOnly: true }}
                  />
                </Grid>
                <Grid item xs={12} sm={6}>
                  <TextField
                    label="Timestamp"
                    value={new Date(request.timestamp).toLocaleString()}
                    fullWidth
                    margin="dense"
                    variant="outlined"
                    InputProps={{ readOnly: true }}
                  />
                </Grid>
                <Grid item xs={12} sm={6}>
                  <TextField
                    label="Team"
                    value={request.team}
                    fullWidth
                    margin="dense"
                    variant="outlined"
                    InputProps={{ readOnly: true }}
                  />
                </Grid>
                <Grid item xs={12} sm={6}>
                  <TextField
                    label="Model"
                    value={request.model}
                    fullWidth
                    margin="dense"
                    variant="outlined"
                    InputProps={{ readOnly: true }}
                  />
                </Grid>
                {request.endpoint && (
                  <Grid item xs={12} sm={6}>
                    <TextField
                      label="Endpoint"
                      value={request.endpoint}
                      fullWidth
                      margin="dense"
                      variant="outlined"
                      InputProps={{ readOnly: true }}
                    />
                  </Grid>
                )}
                {request.httpMethod && (
                  <Grid item xs={12} sm={6}>
                    <TextField
                      label="HTTP Method"
                      value={request.httpMethod}
                      fullWidth
                      margin="dense"
                      variant="outlined"
                      InputProps={{ readOnly: true }}
                    />
                  </Grid>
                )}
                {request.clientIp && (
                  <Grid item xs={12} sm={6}>
                    <TextField
                      label="Client IP"
                      value={request.clientIp}
                      fullWidth
                      margin="dense"
                      variant="outlined"
                      InputProps={{ readOnly: true }}
                    />
                  </Grid>
                )}
                {request.traceId && (
                  <Grid item xs={12} sm={6}>
                    <TextField
                      label="Trace ID"
                      value={request.traceId}
                      fullWidth
                      margin="dense"
                      variant="outlined"
                      InputProps={{ readOnly: true }}
                    />
                  </Grid>
                )}
              </Grid>
            </AccordionDetails>
          </Accordion>
          
          {request.queryText && (
            <Accordion>
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography variant="h6">Query Text</Typography>
              </AccordionSummary>
              <AccordionDetails>
                <TextField
                  value={request.queryText}
                  fullWidth
                  multiline
                  rows={4}
                  variant="outlined"
                  InputProps={{ readOnly: true }}
                />
              </AccordionDetails>
            </Accordion>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </>
  );
};

const MetricsDashboard: React.FC = () => {
  const { requests, loading: requestsLoading, error: requestsError } = useLiveRequests(true);
  const { stats, loading: statsLoading, error: statsError } = useDashboardStats();
  const [filterByDecision, setFilterByDecision] = useState<'all' | 'accept' | 'reject'>('all');
  const [filterByPolicy, setFilterByPolicy] = useState<'all' | 'AuthPolicy' | 'RateLimitPolicy' | 'None'>('all');
  const [filterBySource, setFilterBySource] = useState<'all' | 'limitador' | 'authorino' | 'envoy' | 'kuadrant' | 'kserve'>('all');
  const [searchText, setSearchText] = useState('');

  const filteredRequests = requests.filter((request: Request) => {
    const decisionMatch = filterByDecision === 'all' || request.decision === filterByDecision;
    const policyMatch = filterByPolicy === 'all' || request.policyType === filterByPolicy;
    const sourceMatch = filterBySource === 'all' || request.source === filterBySource;
    const searchMatch = !searchText || 
      request.team.toLowerCase().includes(searchText.toLowerCase()) ||
      request.model.toLowerCase().includes(searchText.toLowerCase()) ||
      (request.queryText && request.queryText.toLowerCase().includes(searchText.toLowerCase())) ||
      (request.endpoint && request.endpoint.toLowerCase().includes(searchText.toLowerCase())) ||
      request.id.toLowerCase().includes(searchText.toLowerCase());
    
    return decisionMatch && policyMatch && sourceMatch && searchMatch;
  });

  if (requestsLoading || statsLoading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <CircularProgress />
        <Typography sx={{ ml: 2 }}>Loading live metrics...</Typography>
      </Box>
    );
  }

  if (requestsError || statsError) {
    return (
      <Alert severity="error" sx={{ mb: 2 }}>
        <Typography variant="h6">Error Loading Metrics</Typography>
        <Typography variant="body2">{requestsError || statsError}</Typography>
      </Alert>
    );
  }

  // Use real Prometheus metrics from dashboard API for top-level stats
  const {
    totalRequests = 0,
    acceptedRequests = 0, 
    rejectedRequests = 0,
    authFailedRequests = 0,
    rateLimitedRequests = 0,
    policyEnforcedRequests = 0,
    kuadrantStatus = {},
    authorinoStats = null,
    source = 'unknown'
  } = stats || {};

  // Extract real Authorino controller metrics (only what's available from Prometheus)
  const authConfigsManaged = authorinoStats?.authConfigs || 0;
  const authConfigReconciles = authorinoStats?.reconcileOperations || 0;
  
  // No more calculated metrics - only real Prometheus data

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h4" component="h1">
          Live Request Metrics
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" color="text.secondary">
            Data Source:
          </Typography>
          <Chip 
            label={source === 'prometheus-metrics' ? 'Real Prometheus' : 'Fallback'}
            color={source === 'prometheus-metrics' ? 'success' : 'warning'} 
            size="small"
          />
          {kuadrantStatus?.istioConnected && (
            <Chip label="Istio ✓" color="success" size="small" />
          )}
          {kuadrantStatus?.authorinoConnected && (
            <Chip label="Authorino ✓" color="success" size="small" />
          )}
        </Box>
      </Box>
      
      {/* Filters */}
      <Toolbar sx={{ px: 0, mb: 2, flexWrap: 'wrap', gap: 2 }}>
        <TextField
          size="small"
          label="Search"
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          sx={{ minWidth: 200 }}
          InputProps={{
            startAdornment: <SearchIcon sx={{ mr: 1 }} />
          }}
          placeholder="Search team, model, endpoint, or request ID..."
        />
        
        <FormControl size="small" sx={{ minWidth: 120 }}>
          <InputLabel>Decision</InputLabel>
          <Select
            value={filterByDecision}
            onChange={(e) => setFilterByDecision(e.target.value as any)}
            label="Decision"
          >
            <MenuItem value="all">All</MenuItem>
            <MenuItem value="accept">Accept</MenuItem>
            <MenuItem value="reject">Reject</MenuItem>
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>Policy Type</InputLabel>
          <Select
            value={filterByPolicy}
            onChange={(e) => setFilterByPolicy(e.target.value as any)}
            label="Policy Type"
          >
            <MenuItem value="all">All</MenuItem>
            <MenuItem value="AuthPolicy">Auth Policy</MenuItem>
            <MenuItem value="RateLimitPolicy">Rate Limit Policy</MenuItem>
            <MenuItem value="None">No Policy</MenuItem>
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>Source</InputLabel>
          <Select
            value={filterBySource}
            onChange={(e) => setFilterBySource(e.target.value as any)}
            label="Source"
          >
            <MenuItem value="all">All</MenuItem>
            <MenuItem value="limitador">Limitador</MenuItem>
            <MenuItem value="authorino">Authorino</MenuItem>
            <MenuItem value="envoy">Envoy</MenuItem>
            <MenuItem value="kuadrant">Kuadrant</MenuItem>
            <MenuItem value="kserve">KServe</MenuItem>
          </Select>
        </FormControl>
      </Toolbar>

      {/* First Row - Basic Stats */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <EndpointIcon sx={{ mr: 1 }} />
                <Typography color="text.secondary" gutterBottom>
                  Total Requests
                </Typography>
              </Box>
              <Typography variant="h4" component="div">
                {totalRequests}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <AcceptIcon sx={{ mr: 1, color: 'success.main' }} />
                <Typography color="text.secondary" gutterBottom>
                  Requests Approved
                </Typography>
              </Box>
              <Typography variant="h4" component="div" color="success.main">
                {acceptedRequests}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {totalRequests > 0 ? `${((acceptedRequests / totalRequests) * 100).toFixed(1)}%` : '0%'}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <RejectIcon sx={{ mr: 1, color: 'error.main' }} />
                <Typography color="text.secondary" gutterBottom>
                  Requests Rejected
                </Typography>
              </Box>
              <Typography variant="h4" component="div" color="error.main">
                {rejectedRequests}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {totalRequests > 0 ? `${((rejectedRequests / totalRequests) * 100).toFixed(1)}%` : '0%'}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <AcceptIcon sx={{ mr: 1, color: 'success.main' }} />
                <Typography color="text.secondary" gutterBottom>
                  Success Rate
                </Typography>
              </Box>
              <Typography variant="h4" component="div" color="success.main">
                {totalRequests > 0 ? `${((acceptedRequests / totalRequests) * 100).toFixed(1)}%` : 'N/A'}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Requests approved
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Second Row - Policy Breakdown */}
      <Grid container spacing={3} sx={{ mb: 4 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <PolicyIcon sx={{ mr: 1 }} />
                <Typography color="text.secondary" gutterBottom>
                  Authentication
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                <Box sx={{ textAlign: 'center' }}>
                  <Typography variant="h5" component="div" color="error.main">
                    {authFailedRequests}
                  </Typography>
                  <Typography variant="body2" color="error.main">
                    Blocked
                  </Typography>
                </Box>
                <Box sx={{ textAlign: 'center' }}>
                  <Typography variant="h5" component="div" color="success.main">
                    {totalRequests - authFailedRequests}
                  </Typography>
                  <Typography variant="body2" color="success.main">
                    Passed
                  </Typography>
                </Box>
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                Auth success: {totalRequests > 0 ? `${(((totalRequests - authFailedRequests) / totalRequests) * 100).toFixed(1)}%` : '0%'}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <RateLimitIcon sx={{ mr: 1 }} />
                <Typography color="text.secondary" gutterBottom>
                  Rate Limiting
                </Typography>
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                <Box sx={{ textAlign: 'center' }}>
                  <Typography variant="h5" component="div" color="error.main">
                    {rateLimitedRequests}
                  </Typography>
                  <Typography variant="body2" color="error.main">
                    Blocked
                  </Typography>
                </Box>
                <Box sx={{ textAlign: 'center' }}>
                  <Typography variant="h5" component="div" color="success.main">
                    {(totalRequests - authFailedRequests) - rateLimitedRequests}
                  </Typography>
                  <Typography variant="body2" color="success.main">
                    Passed
                  </Typography>
                </Box>
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                Rate success: {(totalRequests - authFailedRequests) > 0 ? `${((((totalRequests - authFailedRequests) - rateLimitedRequests) / (totalRequests - authFailedRequests)) * 100).toFixed(1)}%` : '0%'}
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        {/* Empty spaces for consistent layout */}
        <Grid item xs={12} sm={6} md={3}></Grid>
        <Grid item xs={12} sm={6} md={3}></Grid>
      </Grid>

      {/* Requests Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell />
              <TableCell>Timestamp</TableCell>
              <TableCell>
                <Typography variant="body2" fontWeight="medium">Tier</Typography>
              </TableCell>
              <TableCell>
                <Typography variant="body2" fontWeight="medium">Model</Typography>
              </TableCell>
              <TableCell>Endpoint</TableCell>
              <TableCell>Decision</TableCell>
              <TableCell>Policies</TableCell>
              <TableCell align="right">Duration (ms)</TableCell>
              <TableCell align="right">
                <Tooltip title="Token usage format: prompt + completion = total">
                  <Typography variant="body2" fontWeight="medium">Tokens</Typography>
                </Tooltip>
              </TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredRequests.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} align="center" sx={{ py: 4 }}>
                  <Typography color="text.secondary">
                    No requests found with current filters
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              filteredRequests.map((request: Request) => (
                <RequestRow key={request.id} request={request} />
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Real-time indicator */}
      <Box sx={{ mt: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                bgcolor: 'success.main',
                mr: 1,
                animation: 'pulse 2s infinite',
                '@keyframes pulse': {
                  '0%': { opacity: 1 },
                  '50%': { opacity: 0.5 },
                  '100%': { opacity: 1 },
                },
              }}
            />
            <Typography variant="body2" color="text.secondary">
              Live updates every 2s
            </Typography>
          </Box>
          <Typography variant="body2" color="text.secondary">
            • Dashboard: Real Prometheus metrics
          </Typography>
          <Typography variant="body2" color="text.secondary">
            • Table: Real Envoy access logs
          </Typography>
        </Box>
        <Typography variant="body2" color="text.secondary">
          Showing {filteredRequests.length} of {requests.length} total requests
        </Typography>
      </Box>
    </Box>
  );
};

export default MetricsDashboard;