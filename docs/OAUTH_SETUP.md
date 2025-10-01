# OAuth Setup for MaaS Billing

## Overview

The MaaS Billing application now supports OAuth2 authentication flow with OpenShift, allowing users to authenticate via GitHub (or other configured identity providers) without needing CLI access.

## How It Works

1. **User visits localhost:3000**
2. **If not authenticated** → Frontend automatically redirects to OpenShift OAuth (no button click needed)
3. **OpenShift OAuth** → Redirects to GitHub (or configured IdP) for authentication
4. **After successful login** → OpenShift redirects back to `localhost:3000/auth/callback`
5. **Frontend exchanges code** → Backend exchanges authorization code for access token
6. **Backend uses token** → Makes authenticated requests to Kubernetes API
7. **Automatic redirect** → User is redirected back to policies page
8. **Policies load** → User sees Kuadrant policies seamlessly

## Required OpenShift Configuration

To enable this OAuth flow, you need to register the application as an OAuth client in OpenShift:

### 1. Create OAuth Client

```bash
oc apply -f - <<EOF
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: maas-billing-app
redirectURIs:
  - "http://localhost:3000/auth/callback"
  - "https://your-production-domain.com/auth/callback"
grantMethod: auto
EOF
```

### 2. Update Client Configuration (if needed)

If you need to modify the client later:

```bash
oc edit oauthclient maas-billing-app
```

### 3. Verify OAuth Client

```bash
oc get oauthclient maas-billing-app -o yaml
```

## Current Implementation Status

✅ **Implemented:**
- Frontend OAuth redirect flow
- Backend OAuth code exchange endpoint
- React Router integration for `/auth/callback`
- Secure state parameter for CSRF protection
- Error handling for OAuth failures

⚠️ **Requires Setup:**
- OAuth client registration in OpenShift (see above)
- Production deployment configuration
- Proper session management (currently using simple in-memory storage)

## Testing the Flow

1. **Start the servers:**
   ```bash
   # Backend
   cd apps/backend && python3 server.py 3003
   
   # Frontend  
   cd apps/frontend && npm start
   ```

2. **Visit localhost:3000**
3. **Automatic redirect** → You'll be automatically redirected to OpenShift OAuth
4. **Complete authentication** → Login via GitHub or configured IdP
5. **Automatic return** → You'll be redirected back and see policies loaded seamlessly

**No manual steps required!** The entire authentication flow is automatic.

## Production Considerations

For production deployment, you'll need:

1. **Secure session management** (replace in-memory token storage)
2. **HTTPS endpoints** for OAuth redirects
3. **Proper client secret management**
4. **Token refresh handling**
5. **Session timeout and cleanup**

## Troubleshooting

### Common Issues:

1. **"Invalid client" error**
   - Ensure OAuth client is registered in OpenShift
   - Check client_id matches in frontend and backend

2. **"Invalid redirect_uri" error**
   - Ensure redirect URI is registered in OAuth client
   - Check for exact URL match (including protocol)

3. **CORS errors**
   - Backend already includes CORS headers
   - Ensure frontend and backend are on expected ports

4. **State parameter mismatch**
   - Clear localStorage and try again
   - Check browser console for specific error

## Files Modified

- `apps/frontend/src/components/PolicyManager.tsx` - OAuth redirect logic
- `apps/frontend/src/components/AuthCallback.tsx` - OAuth callback handler
- `apps/frontend/src/App.tsx` - React Router integration
- `apps/frontend/src/services/api.ts` - OAuth code exchange API
- `apps/backend/server.py` - OAuth token exchange endpoint

The implementation provides a seamless authentication experience where users authenticate via their existing GitHub credentials through OpenShift's OAuth server.
