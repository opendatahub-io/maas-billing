import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, useLocation } from 'react-router-dom';
import {
  AppBar,
  Box,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Toolbar,
  Typography,
  Avatar,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Alert,
  Tooltip,
} from '@mui/material';
import {
  Policy as PolicyIcon,
  BarChart as MetricsIcon,
  PlayArrow as SimulatorIcon,
  Speed as QoSIcon,
  AccountCircle,
  Settings,
  Logout,
  LightMode,
  DarkMode,
  ContentCopy as CopyIcon,
  Science as ExperimentalIcon,
} from '@mui/icons-material';

import PolicyManager from './components/PolicyManager';
import MetricsDashboard from './components/MetricsDashboard';
import RequestSimulator from './components/RequestSimulator';
import AuthCallback from './components/AuthCallback';
import QoSMonitor from './components/QoSMonitor';
import { ThemeProvider, useTheme } from './contexts/ThemeContext';
import apiService from './services/api';

const drawerWidth = 240;

function MainApp() {
  const [selectedView, setSelectedView] = useState('policies');
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [experimentalMode, setExperimentalMode] = useState(false);
  const { mode, toggleTheme } = useTheme();
  const [clusterStatus, setClusterStatus] = useState<{
    connected: boolean;
    user: string | null;
    cluster: string | null;
    loginUrl: string;
  }>({
    connected: false,
    user: null,
    cluster: null,
    loginUrl: process.env.REACT_APP_CONSOLE_URL || 'https://console-openshift-console.your-cluster.example.com'
  });
  const [showLoginDialog, setShowLoginDialog] = useState(false);

  const handleMenu = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleLogout = () => {
    // Clear any stored auth state
    localStorage.removeItem('oauth_authenticated');
    
    // Redirect to OpenShift OAuth login for fresh CLI session
    const returnUrl = encodeURIComponent(window.location.origin);
    const oauthBaseUrl = process.env.REACT_APP_OAUTH_URL || 'https://oauth-openshift.your-cluster.example.com';
    const loginUrl = `${oauthBaseUrl}/oauth/token/request?then=${returnUrl}`;
    
    console.log('🔐 Logging out and redirecting to OpenShift OAuth login...');
    window.location.href = loginUrl;
  };

  const redirectToLogin = () => {
    setShowLoginDialog(true);
  };

  const handleCloseLoginDialog = () => {
    setShowLoginDialog(false);
  };

  const handleRefreshAfterLogin = () => {
    setShowLoginDialog(false);
    window.location.reload();
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const toggleExperimentalMode = () => {
    setExperimentalMode(!experimentalMode);
    // If turning off experimental mode and currently viewing QoS, switch to policies
    if (experimentalMode && selectedView === 'qos') {
      setSelectedView('policies');
    }
  };

  // Check authentication status on mount (but don't auto-redirect)
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const status = await apiService.getClusterStatus();
        setClusterStatus(status);
        
        // Check authentication and show login dialog if needed
        if (!status.connected || status.user === 'system:anonymous' || !status.user) {
          console.log('🔐 User not authenticated, showing login dialog...');
          setShowLoginDialog(true);
        } else {
          console.log(`✅ Authenticated as: ${status.user}`);
          setShowLoginDialog(false);
        }
      } catch (error) {
        console.warn('Could not check authentication status:', error);
        // Set default status and show login dialog
        setClusterStatus({
          connected: false,
          user: null,
          cluster: null,
          loginUrl: process.env.REACT_APP_CONSOLE_URL || 'https://console-openshift-console.your-cluster.example.com'
        });
        setShowLoginDialog(true);
      }
    };

    checkAuth();
  }, []);
  const renderContent = () => {
    switch (selectedView) {
      case 'policies':
        return <PolicyManager />;
      case 'metrics':
        return <MetricsDashboard />;
      case 'qos':
        return <QoSMonitor />;
      case 'simulator':
        return <RequestSimulator />;
      default:
        return <PolicyManager />;
    }
  };

  const menuItems = [
    { id: 'policies', label: 'Policy Manager', icon: <PolicyIcon /> },
    { id: 'metrics', label: 'Live Metrics', icon: <MetricsIcon /> },
    ...(experimentalMode ? [{ id: 'qos', label: 'QoS Monitor', icon: <QoSIcon /> }] : []),
    { id: 'simulator', label: 'Playground', icon: <SimulatorIcon /> },
  ];

  return (
    <>
      <Box sx={{ display: 'flex' }}>
          
          {/* App Bar */}
        <AppBar
          position="fixed"
          sx={{
            width: '100%',
            zIndex: (theme) => theme.zIndex.drawer + 1,
            backgroundColor: '#151515',
            borderBottom: '1px solid #333',
          }}
        >
          <Toolbar sx={{ minHeight: '64px !important' }}>
            {/* Logo and Title */}
            <Box
              component="img"
              src="/redhat-fedora-logo.png"
              alt="Red Hat"
              sx={{ height: 32, mr: 2 }}
            />
            <Typography variant="h6" component="div" sx={{ color: 'white', mr: 2 }}>
              |
            </Typography>
            <Typography variant="h6" component="div" sx={{ color: 'white', fontWeight: 600 }}>
              MaaS
            </Typography>
            <Typography variant="body2" component="div" sx={{ color: '#999', ml: 1 }}>
              Inference Model as a Service
            </Typography>
            
            <Box sx={{ flexGrow: 1 }} />
            
            {/* Authentication Status */}
            {!clusterStatus.connected && (
              <Box sx={{ display: 'flex', gap: 1, mr: 2 }}>
                <Chip
                  label="Not Logged In"
                  color="warning"
                  size="small"
                  onClick={redirectToLogin}
                  sx={{ cursor: 'pointer' }}
                />
                <Chip
                  label="Refresh"
                  color="info"
                  size="small"
                  onClick={() => window.location.reload()}
                  sx={{ cursor: 'pointer' }}
                />
              </Box>
            )}
            {clusterStatus.connected && clusterStatus.user && (
              <Chip
                label={`Logged in as: ${clusterStatus.user}`}
                color="success"
                size="small"
                sx={{ mr: 2 }}
              />
            )}
            
            {/* Experimental Mode Toggle */}
            <Tooltip title={experimentalMode ? 'Disable Experimental Features' : 'Enable Experimental Features'}>
              <IconButton
                color="inherit"
                onClick={toggleExperimentalMode}
                sx={{ 
                  mr: 2,
                  backgroundColor: experimentalMode ? 'rgba(238, 0, 0, 0.2)' : 'transparent',
                  '&:hover': {
                    backgroundColor: experimentalMode ? 'rgba(238, 0, 0, 0.3)' : 'rgba(255, 255, 255, 0.1)',
                  }
                }}
              >
                <ExperimentalIcon sx={{ color: experimentalMode ? '#ff6b6b' : 'white' }} />
              </IconButton>
            </Tooltip>
            
            {/* Theme Toggle */}
            <IconButton
              color="inherit"
              onClick={toggleTheme}
              sx={{ mr: 2 }}
            >
              {mode === 'dark' ? <LightMode /> : <DarkMode />}
            </IconButton>
            
            <div>
              <IconButton
                size="large"
                aria-label="account of current user"
                aria-controls="menu-appbar"
                aria-haspopup="true"
                onClick={handleMenu}
                color="inherit"
              >
                <Avatar sx={{ width: 32, height: 32, bgcolor: 'primary.main' }}>
                  U
                </Avatar>
              </IconButton>
              <Menu
                id="menu-appbar"
                anchorEl={anchorEl}
                anchorOrigin={{
                  vertical: 'top',
                  horizontal: 'right',
                }}
                keepMounted
                transformOrigin={{
                  vertical: 'top',
                  horizontal: 'right',
                }}
                open={Boolean(anchorEl)}
                onClose={handleClose}
              >
                <MenuItem onClick={handleClose}>
                  <ListItemIcon>
                    <AccountCircle fontSize="small" />
                  </ListItemIcon>
                  Profile
                </MenuItem>
                <MenuItem onClick={() => { toggleTheme(); handleClose(); }}>
                  <ListItemIcon>
                    {mode === 'dark' ? <LightMode fontSize="small" /> : <DarkMode fontSize="small" />}
                  </ListItemIcon>
                  Switch to {mode === 'dark' ? 'Light' : 'Dark'} Mode
                </MenuItem>
                <MenuItem onClick={handleClose}>
                  <ListItemIcon>
                    <Settings fontSize="small" />
                  </ListItemIcon>
                  Settings
                </MenuItem>
                <MenuItem onClick={handleLogout}>
                  <ListItemIcon>
                    <Logout fontSize="small" />
                  </ListItemIcon>
                  Logout
                </MenuItem>
              </Menu>
            </div>
          </Toolbar>
        </AppBar>

        {/* Sidebar Drawer */}
        <Drawer
          sx={{
            width: drawerWidth,
            flexShrink: 0,
            '& .MuiDrawer-paper': {
              width: drawerWidth,
              boxSizing: 'border-box',
              backgroundColor: '#1a1a1a',
              borderRight: '1px solid #333',
            },
          }}
          variant="permanent"
          anchor="left"
        >
          {/* Navigation List */}
          <List sx={{ mt: 8 }}>
            {menuItems.map((item) => (
              <ListItem key={item.id} disablePadding>
                <ListItemButton
                  selected={selectedView === item.id}
                  onClick={() => setSelectedView(item.id)}
                  sx={{
                    mx: 1,
                    mb: 0.5,
                    borderRadius: 1,
                    '&.Mui-selected': {
                      backgroundColor: '#ee0000',
                      '&:hover': {
                        backgroundColor: '#cc0000',
                      },
                    },
                    '&:hover': {
                      backgroundColor: '#333',
                    },
                  }}
                >
                  <ListItemIcon sx={{ color: selectedView === item.id ? 'white' : '#999' }}>
                    {item.icon}
                  </ListItemIcon>
                  <ListItemText 
                    primary={item.label}
                    sx={{ 
                      '& .MuiListItemText-primary': {
                        color: selectedView === item.id ? 'white' : '#999',
                        fontWeight: selectedView === item.id ? 600 : 400,
                      }
                    }}
                  />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Drawer>

        {/* Main content */}
        <Box
          component="main"
          sx={{
            flexGrow: 1,
            bgcolor: 'background.default',
            p: 3,
            width: `calc(100% - ${drawerWidth}px)`,
          }}
        >
          <Toolbar />
          {renderContent()}
        </Box>
      </Box>

      {/* Login Instructions Dialog */}
      <Dialog 
        open={showLoginDialog} 
        onClose={handleCloseLoginDialog}
        maxWidth="md"
        fullWidth
        disableEscapeKeyDown
      >
        <DialogTitle>
          🔐 OpenShift Cluster Authentication Required
        </DialogTitle>
        <DialogContent>
          <Alert severity="info" sx={{ mb: 3 }}>
            <Typography variant="body2">
              This application requires authentication with the OpenShift cluster to fetch policies and manage tokens.
            </Typography>
          </Alert>
          
          <Typography variant="h6" gutterBottom>
            Step 1: Login via CLI
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Copy and run this command in your terminal:
          </Typography>
          
          <TextField
            fullWidth
            value={`oc login --web --server=${process.env.REACT_APP_CLUSTER_API_URL || 'https://api.your-cluster.example.com:6443'}`}
            InputProps={{ 
              readOnly: true,
              endAdornment: (
                <Button 
                  size="small" 
                  startIcon={<CopyIcon />}
                  onClick={() => copyToClipboard(`oc login --web --server=${process.env.REACT_APP_CLUSTER_API_URL || 'https://api.your-cluster.example.com:6443'}`)}
                >
                  Copy
                </Button>
              )
            }}
            sx={{ fontFamily: 'monospace', mb: 3 }}
          />
          
          <Typography variant="h6" gutterBottom>
            Step 2: Complete Web Authentication
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            The command will open your browser for authentication. Complete the login process.
          </Typography>
          
          <Typography variant="h6" gutterBottom>
            Step 3: Return and Refresh
          </Typography>
          <Typography variant="body2" color="text.secondary">
            After successful authentication, click the "Refresh Application" button below.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button 
            variant="contained" 
            color="primary"
            onClick={handleRefreshAfterLogin}
            size="large"
          >
            Refresh Application
          </Button>
          <Button onClick={handleCloseLoginDialog}>
            Cancel
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

function AppContent() {
  const location = useLocation();
  
  // Handle OAuth callback route
  if (location.pathname === '/auth/callback') {
    return <AuthCallback />;
  }
  
  // Main app content
  return <MainApp />;
}

function App() {
  return (
    <ThemeProvider>
      <Router>
        <Routes>
          <Route path="/auth/callback" element={<AuthCallback />} />
          <Route path="/*" element={<AppContent />} />
        </Routes>
      </Router>
    </ThemeProvider>
  );
}

export default App;