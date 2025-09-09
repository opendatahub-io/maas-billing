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
} from '@mui/material';
import {
  Policy as PolicyIcon,
  BarChart as MetricsIcon,
  PlayArrow as SimulatorIcon,
  Key as TokenIcon,
  AccountCircle,
  Settings,
  Logout,
  LightMode,
  DarkMode,
} from '@mui/icons-material';

import PolicyManager from './components/PolicyManager';
import MetricsDashboard from './components/MetricsDashboard';
import RequestSimulator from './components/RequestSimulator';
import TokenManagement from './components/TokenManagement';
import AuthCallback from './components/AuthCallback';
import { ThemeProvider, useTheme } from './contexts/ThemeContext';
import apiService from './services/api';

const drawerWidth = 240;

function MainApp() {
  const [selectedView, setSelectedView] = useState('policies');
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
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
    loginUrl: 'https://console-openshift-console.apps.summit-gpu.octo-emerging.redhataicoe.com'
  });

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
    const loginUrl = `https://oauth-openshift.apps.summit-gpu.octo-emerging.redhataicoe.com/oauth/token/request?then=${returnUrl}`;
    
    console.log('🔐 Logging out and redirecting to OpenShift OAuth login...');
    window.location.href = loginUrl;
  };

  const redirectToLogin = () => {
    // For now, provide instructions for CLI login since OAuth integration is complex
    const instructions = `To use this application, you need to login to the OpenShift cluster via CLI:

1. Open a terminal
2. Run: oc login --web --server=https://api.summit-gpu.octo-emerging.redhataicoe.com:6443
3. Complete the web authentication
4. Return here and refresh the page

The backend needs an authenticated oc CLI session to fetch policies and tokens from the cluster.`;

    alert(instructions);
    
    // Optionally still redirect to web console for convenience
    const returnUrl = encodeURIComponent(window.location.href);
    const loginUrl = `https://console-openshift-console.apps.summit-gpu.octo-emerging.redhataicoe.com?then=${returnUrl}`;
    
    if (confirm('Would you like to open the OpenShift console in a new tab?')) {
      window.open(loginUrl, '_blank');
    }
  };

  // Check authentication status on mount (but don't auto-redirect)
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const status = await apiService.getClusterStatus();
        setClusterStatus(status);
        
        // Just log the status, don't auto-redirect
        if (!status.connected || status.user === 'system:anonymous' || !status.user) {
          console.warn('🔐 User not authenticated. Policies may not load. Use logout button to login.');
        } else {
          console.log(`✅ Authenticated as: ${status.user}`);
        }
      } catch (error) {
        console.warn('Could not check authentication status:', error);
        // Set default status
        setClusterStatus({
          connected: false,
          user: null,
          cluster: null,
          loginUrl: 'https://console-openshift-console.apps.summit-gpu.octo-emerging.redhataicoe.com'
        });
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
      case 'simulator':
        return <RequestSimulator />;
      case 'tokens':
        return <TokenManagement />;
      default:
        return <PolicyManager />;
    }
  };

  const menuItems = [
    { id: 'policies', label: 'Policy Manager', icon: <PolicyIcon /> },
    { id: 'metrics', label: 'Live Metrics', icon: <MetricsIcon /> },
    { id: 'simulator', label: 'Request Simulator', icon: <SimulatorIcon /> },
    { id: 'tokens', label: 'API Tokens', icon: <TokenIcon /> },
  ];

  return (
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