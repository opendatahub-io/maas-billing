import React, { useEffect, useState } from 'react';
import { Box, Typography, CircularProgress, Alert } from '@mui/material';
import apiService from '../services/api';

const AuthCallback: React.FC = () => {
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>('processing');
  const [message, setMessage] = useState('Processing authentication...');

  useEffect(() => {
    const handleCallback = async () => {
      try {
        const urlParams = new URLSearchParams(window.location.search);
        const code = urlParams.get('code');
        const state = urlParams.get('state');
        const error = urlParams.get('error');

        // Check for OAuth errors
        if (error) {
          setStatus('error');
          setMessage(`Authentication failed: ${error}`);
          return;
        }

        // Verify state parameter
        const savedState = localStorage.getItem('oauth_state');
        if (!state || state !== savedState) {
          setStatus('error');
          setMessage('Invalid state parameter. Possible CSRF attack.');
          return;
        }

        // Clear saved state
        localStorage.removeItem('oauth_state');

        if (!code) {
          setStatus('error');
          setMessage('No authorization code received');
          return;
        }

        // Exchange code for token via backend
        setMessage('Exchanging authorization code for access token...');
        const tokenResponse = await apiService.exchangeOAuthCode(code);
        
        if (tokenResponse.success) {
          setStatus('success');
          setMessage('Authentication successful! Redirecting...');
          
          // Store token info (backend will handle the actual token)
          localStorage.setItem('oauth_authenticated', 'true');
          
          // Redirect to main app after a short delay
          setTimeout(() => {
            window.location.href = window.location.origin;
          }, 2000);
        } else {
          setStatus('error');
          setMessage('Failed to exchange authorization code for token');
        }

      } catch (error) {
        console.error('OAuth callback error:', error);
        setStatus('error');
        setMessage(error instanceof Error ? error.message : 'Authentication failed');
      }
    };

    handleCallback();
  }, []);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        p: 3,
      }}
    >
      {status === 'processing' && (
        <>
          <CircularProgress size={60} sx={{ mb: 3 }} />
          <Typography variant="h5" gutterBottom>
            Completing Authentication
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {message}
          </Typography>
        </>
      )}

      {status === 'success' && (
        <>
          <Alert severity="success" sx={{ mb: 3, minWidth: 400 }}>
            <Typography variant="h6">Authentication Successful!</Typography>
            <Typography variant="body2">{message}</Typography>
          </Alert>
        </>
      )}

      {status === 'error' && (
        <>
          <Alert severity="error" sx={{ mb: 3, minWidth: 400 }}>
            <Typography variant="h6">Authentication Failed</Typography>
            <Typography variant="body2">{message}</Typography>
          </Alert>
          <Typography variant="body2" color="text.secondary">
            You can close this window and try logging in again.
          </Typography>
        </>
      )}
    </Box>
  );
};

export default AuthCallback;
