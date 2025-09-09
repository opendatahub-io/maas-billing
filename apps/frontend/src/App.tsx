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
    // Redirect to OpenShift OAuth login with proper OAuth flow
    const returnUrl = encodeURIComponent(window.location.origin);
    const clientId = 'openshift-cli-client';
    const oauthUrl = `https://oauth-openshift.apps.summit-gpu.octo-emerging.redhataicoe.com/oauth/authorize?client_id=${clientId}&response_type=code&redirect_uri=${returnUrl}/auth/callback&scope=user:info`;
    window.location.href = oauthUrl;
  };

  const redirectToLogin = () => {
    // Redirect to OpenShift OAuth login
    const returnUrl = encodeURIComponent(window.location.origin);
    const clientId = 'openshift-cli-client';
    const oauthUrl = `https://oauth-openshift.apps.summit-gpu.octo-emerging.redhataicoe.com/oauth/authorize?client_id=${clientId}&response_type=code&redirect_uri=${returnUrl}/auth/callback&scope=user:info`;
    
    console.log('🔐 Redirecting to OpenShift OAuth login...');
    console.log('Return URL:', `${window.location.origin}/auth/callback`);
    
    window.location.href = oauthUrl;
  };

  // Check authentication status on mount
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const status = await apiService.getClusterStatus();
        setClusterStatus(status);
        
        // Check if user is authenticated
        if (!status.connected || status.user === 'system:anonymous' || !status.user) {
          console.log('🔐 User not authenticated (system:anonymous), redirecting to OpenShift login...');
          redirectToLogin();
          return;
        } else {
          console.log(`✅ Authenticated as: ${status.user}`);
        }
      } catch (error) {
        console.warn('Could not check authentication status:', error);
        // Set default status, don't redirect automatically
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