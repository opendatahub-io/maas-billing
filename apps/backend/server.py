#!/usr/bin/env python3
"""
MaaS Backend - Python HTTP Server
Provides API endpoints for Policy Manager, Metrics Dashboard, Request Simulator, and Token Management
"""

import http.server
import socketserver
import json
import urllib.request
import urllib.parse
import re
import os
import subprocess
from urllib.parse import urlparse, parse_qs
from datetime import datetime

# Mock tier configurations for localhost
TIER_CONFIGS = {
    'free': {
        'name': 'free',
        'usage': 1234,
        'limit': 10000,
        'models': ['vllm-simulator'],
        'namespace': 'inference-gateway-tier-free'
    },
    'premium': {
        'name': 'premium',
        'usage': 5678,
        'limit': 50000,
        'models': ['vllm-simulator', 'qwen3-0.6b-instruct'],
        'namespace': 'inference-gateway-tier-premium'
    },
    'enterprise': {
        'name': 'enterprise',
        'usage': 12345,
        'limit': 1000000,
        'models': ['vllm-simulator', 'qwen3-0.6b-instruct', 'llama2-7b'],
        'namespace': 'inference-gateway-tier-enterprise'
    }
}

# Mock user data for localhost
MOCK_USER = {
    'id': 'user-123',
    'email': 'user@company.com',
    'tier': 'premium',
    'namespace': 'inference-gateway-tier-premium'
}

# Configuration from environment variables
CLUSTER_DOMAIN = os.getenv('CLUSTER_DOMAIN', 'apps.summit-gpu.octo-emerging.redhataicoe.com')
KEY_MANAGER_BASE_URL = os.getenv('KEY_MANAGER_BASE_URL', f'https://key-manager-route-platform-services.{CLUSTER_DOMAIN}')
KEY_MANAGER_ADMIN_KEY = os.getenv('KEY_MANAGER_ADMIN_KEY', 'admin-key-placeholder')
OAUTH_BASE_URL = os.getenv('OAUTH_BASE_URL', f'https://oauth-openshift.{CLUSTER_DOMAIN}')
CONSOLE_BASE_URL = os.getenv('CONSOLE_BASE_URL', f'https://console-openshift-console.{CLUSTER_DOMAIN}')

# SSL Configuration
SSL_VERIFY = os.getenv('SSL_VERIFY', 'false').lower() == 'true'  # Default to false for development

# Timeout Configuration
DEFAULT_TIMEOUT = int(os.getenv('DEFAULT_TIMEOUT', '30'))
CLUSTER_TIMEOUT = int(os.getenv('CLUSTER_TIMEOUT', '10'))
SUBPROCESS_TIMEOUT = int(os.getenv('SUBPROCESS_TIMEOUT', '5'))

# Default team ID for single-user mode (can be overridden by environment variables)
DEFAULT_TEAM_ID = os.getenv('DEFAULT_TEAM_ID', 'default')
DEFAULT_USER_ID = os.getenv('DEFAULT_USER_ID', 'noyitz')

# Mock tokens for localhost
MOCK_TOKENS = [
    {
        'name': 'my-project-token',
        'created': '2024-01-15T10:30:00Z',
        'lastUsed': '2024-01-20T14:22:00Z',
        'usage': 456,
        'status': 'active'
    },
    {
        'name': 'legacy-token',
        'created': '2023-12-01T09:15:00Z',
        'lastUsed': '',
        'usage': 0,
        'status': 'unused'
    }
]

# Track simulator requests for metrics
SIMULATOR_METRICS = {
    'total_requests': 0,
    'successful_requests': 0,
    'failed_requests': 0,
    'auth_failures': 0,
    'rate_limits': 0
}

# OAuth token storage 
# WARNING: This is for development only. In production, use:
# - Redis for session storage
# - Encrypted cookies with proper session management
# - Database-backed user sessions with expiration
# - Proper OAuth state validation
OAUTH_TOKENS = {}

def create_ssl_context():
    """Create SSL context based on configuration"""
    import ssl
    ctx = ssl.create_default_context()
    
    if not SSL_VERIFY:
        # For development environments with self-signed certificates
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
        print("⚠️  SSL verification disabled (development mode)")
    else:
        print("🔒 SSL verification enabled (production mode)")
    
    return ctx

def exchange_oauth_code_for_token(code, redirect_uri):
    """Exchange OAuth authorization code for access token"""
    try:
        import urllib.request
        import urllib.parse
        import ssl
        
        # OpenShift OAuth token endpoint
        token_url = f'{OAUTH_BASE_URL}/oauth/token'
        
        # OAuth client credentials (these would need to be registered in OpenShift)
        client_id = 'maas-billing-app'
        # Note: In production, you'd need a client_secret registered with OpenShift
        
        # Prepare the token exchange request
        data = {
            'grant_type': 'authorization_code',
            'code': code,
            'redirect_uri': redirect_uri,
            'client_id': client_id
        }
        
        # Encode the data
        encoded_data = urllib.parse.urlencode(data).encode('utf-8')
        
        # Create the request
        headers = {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json'
        }
        
        req = urllib.request.Request(token_url, data=encoded_data, headers=headers, method='POST')
        
        # Create SSL context
        ctx = create_ssl_context()
        
        # Make the request
        with urllib.request.urlopen(req, context=ctx, timeout=DEFAULT_TIMEOUT) as response:
            response_data = json.loads(response.read().decode('utf-8'))
            
            if 'access_token' in response_data:
                print(f"✅ OAuth token exchange successful")
                return response_data['access_token']
            else:
                print(f"❌ OAuth token exchange failed: {response_data}")
                return None
                
    except Exception as e:
        print(f"❌ OAuth token exchange error: {e}")
        return None

def call_key_manager_api(endpoint, method='GET', data=None):
    """Call the key-manager API with proper authentication"""
    import urllib.request
    import urllib.parse
    import ssl
    
    url = f"{KEY_MANAGER_BASE_URL}{endpoint}"
    headers = {
        'Authorization': f'Bearer {KEY_MANAGER_ADMIN_KEY}',
        'Content-Type': 'application/json'
    }
    
    # Create SSL context
    ctx = create_ssl_context()
    
    try:
        if method == 'GET':
            req = urllib.request.Request(url, headers=headers)
        elif method == 'POST':
            req_data = json.dumps(data).encode('utf-8') if data else None
            req = urllib.request.Request(url, data=req_data, headers=headers, method='POST')
        elif method == 'DELETE':
            req = urllib.request.Request(url, headers=headers, method='DELETE')
        else:
            raise Exception(f"Unsupported method: {method}")
        
        with urllib.request.urlopen(req, context=ctx, timeout=DEFAULT_TIMEOUT) as response:
            response_data = json.loads(response.read().decode('utf-8'))
            return response_data
    except urllib.error.HTTPError as e:
        error_body = e.read().decode('utf-8') if e.fp else str(e)
        print(f"❌ Key manager API error: {e.code} {e.reason} - {error_body}")
        raise Exception(f"Key manager API error: {e.code} {e.reason}")
    except Exception as e:
        print(f"❌ Key manager API connection error: {e}")
        raise Exception(f"Key manager API connection error: {str(e)}")

def fetch_user_tokens():
    """Fetch user tokens from the key-manager and get actual API key values from Kubernetes secrets"""
    try:
        print(f"🔍 Fetching tokens for user: {DEFAULT_USER_ID}")
        response = call_key_manager_api(f'/users/{DEFAULT_USER_ID}/keys')
        
        # Transform the response to match frontend expectations
        tokens = []
        if 'keys' in response:
            for key_info in response['keys']:
                secret_name = key_info.get('secret_name', 'unknown')
                
                # Try to get the actual API key value from the Kubernetes secret
                print(f"🔍 About to call get_api_key_from_secret for: {secret_name}")
                actual_api_key = get_api_key_from_secret(secret_name)
                print(f"🔍 get_api_key_from_secret returned: {actual_api_key[:10] + '...' if actual_api_key else 'None'}")
                
                token_data = {
                    'name': secret_name,  # Keep secret name as identifier
                    'displayName': key_info.get('alias', secret_name),  # Use alias for display
                    'created': key_info.get('created_at', ''),
                    'lastUsed': '',  # Key manager doesn't track last used
                    'usage': 0,     # Usage would come from metrics
                    'status': key_info.get('status', 'active'),
                    'team_id': key_info.get('team_id', ''),
                    'team_name': key_info.get('team_name', ''),
                    'policy': key_info.get('policy', ''),
                    'alias': key_info.get('alias', ''),
                    'actualApiKey': actual_api_key  # Include the actual API key value
                }
                
                print(f"🔍 Token data for {secret_name}: actualApiKey = {actual_api_key[:10] + '...' if actual_api_key else 'None'}")
                
                tokens.append(token_data)
        
        print(f"✅ Retrieved {len(tokens)} tokens from key-manager with API key values")
        return tokens
    except Exception as e:
        print(f"❌ Failed to fetch user tokens: {e}")
        # Return empty list on error to prevent UI crashes
        return []

def get_api_key_from_secret(secret_name):
    """Get the actual API key value from a Kubernetes secret"""
    try:
        import subprocess
        import base64
        
        print(f"🔍 Attempting to get API key for secret: {secret_name}")
        
        # Try to get API keys from environment variables first (for development)
        env_key = os.getenv(f'API_KEY_{secret_name.replace("-", "_").upper()}')
        if env_key:
            print(f"🔑 Using API key from environment for {secret_name}: {env_key[:10]}...")
            return env_key
        
        # Try the oc command approach
        result = subprocess.run([
            'oc', 'get', 'secret', secret_name, '-n', 'llm', 
            '-o', 'jsonpath={.data.api_key}'
        ], capture_output=True, text=True, timeout=SUBPROCESS_TIMEOUT)
        
        print(f"🔍 oc command result: returncode={result.returncode}")
        print(f"🔍 stdout length: {len(result.stdout) if result.stdout else 0}")
        print(f"🔍 stderr: '{result.stderr}'")
        
        if result.returncode == 0 and result.stdout:
            # Decode the base64 encoded API key
            encoded_key = result.stdout.strip()
            if encoded_key:
                try:
                    decoded_key = base64.b64decode(encoded_key).decode('utf-8')
                    print(f"🔑 Successfully retrieved API key for secret {secret_name}: {decoded_key[:10]}...")
                    return decoded_key
                except Exception as decode_error:
                    print(f"❌ Failed to decode base64 for {secret_name}: {decode_error}")
                    return None
            else:
                print(f"⚠️ Empty api_key field for secret {secret_name}")
                return None
        else:
            print(f"⚠️ oc command failed for secret {secret_name}")
            print(f"   Return code: {result.returncode}")
            print(f"   Stderr: {result.stderr}")
            return None
            
    except Exception as e:
        print(f"❌ Exception getting API key from secret {secret_name}: {e}")
        import traceback
        traceback.print_exc()
        return None

def get_user_tier_from_team():
    """Get user tier information from team configuration"""
    try:
        print(f"🔍 Fetching team info for: {DEFAULT_TEAM_ID}")
        response = call_key_manager_api(f'/teams/{DEFAULT_TEAM_ID}')
        
        # Map team policy to tier information
        policy = response.get('policy', 'unlimited-policy')
        
        # Create tier info based on team policy
        tier_info = {
            'name': policy,
            'usage': 0,  # Would come from metrics
            'limit': 100000,  # Default limit
            'models': ['vllm-simulator', 'qwen3-0.6b-instruct'],  # Default models
            'team_id': response.get('team_id', ''),
            'team_name': response.get('team_name', ''),
            'policy': policy
        }
        
        print(f"✅ Retrieved tier info: {policy}")
        return tier_info
    except Exception as e:
        print(f"❌ Failed to fetch tier info: {e}")
        # Return default tier on error
        return {
            'name': 'default',
            'usage': 0,
            'limit': 10000,
            'models': ['vllm-simulator'],
            'team_id': DEFAULT_TEAM_ID,
            'team_name': 'Default Team',
            'policy': 'unlimited-policy'
        }

def fetch_kuadrant_policies():
    """Fetch policies from Kuadrant - no mock data, real data only"""
    print("🔍 fetch_kuadrant_policies() called")
    
    # Check if running in cluster
    if is_running_in_cluster():
        # Use Kubernetes API directly from within cluster
        print("🔍 Running in cluster - attempting to fetch policies via Kubernetes API")
        cluster_policies = fetch_policies_from_k8s_api()
        if cluster_policies:
            print(f"✅ Retrieved {len(cluster_policies)} real policies from cluster API")
            return cluster_policies
        else:
            print("❌ No policies returned from cluster API")
            return []
    else:
        # For localhost, try external cluster API access
        print("🔍 Running locally - attempting to fetch policies via external cluster API")
        try:
            # Try to get cluster token
            token_result = subprocess.run(['oc', 'whoami', '-t'], capture_output=True, text=True, timeout=SUBPROCESS_TIMEOUT)
            if token_result.returncode == 0:
                token = token_result.stdout.strip()
                print("🔍 Using oc token for external cluster API access")
                
                # Use external cluster API endpoints
                cluster_policies = fetch_policies_from_external_k8s_api(token)
                if cluster_policies:
                    print(f"✅ Retrieved {len(cluster_policies)} real policies from external API")
                    return cluster_policies
                else:
                    print("❌ No policies returned from external cluster API")
                    return []
            else:
                print("❌ No oc token available - cannot access cluster from localhost")
                return []
        except Exception as e:
            print(f"❌ External cluster access failed: {e}")
            return []

def is_running_in_cluster():
    """Detect if running inside a Kubernetes cluster"""
    import os
    return os.path.exists('/var/run/secrets/kubernetes.io/serviceaccount/token')

def fetch_policies_from_k8s_api():
    """Fetch policies using Kubernetes API from within cluster"""
    import urllib.request
    import urllib.parse
    import ssl
    
    try:
        # Read service account token
        with open('/var/run/secrets/kubernetes.io/serviceaccount/token', 'r') as f:
            token = f.read().strip()
        
        # Kubernetes API server endpoint
        k8s_host = 'kubernetes.default.svc'
        k8s_port = '443'
        
        # Create SSL context
        ctx = create_ssl_context()
        
        policies = []
        
        # First, check if Kuadrant APIs are available
        try:
            api_check_url = f'https://{k8s_host}:{k8s_port}/apis/kuadrant.io'
            api_check_request = urllib.request.Request(api_check_url)
            api_check_request.add_header('Authorization', f'Bearer {token}')
            api_check_request.add_header('Accept', 'application/json')
            
            with urllib.request.urlopen(api_check_request, context=ctx, timeout=CLUSTER_TIMEOUT) as response:
                api_data = json.loads(response.read().decode())
                print(f"✅ Kuadrant API is available. Versions: {[v['version'] for v in api_data.get('versions', [])]}")
        except Exception as api_check_error:
            print(f"❌ Kuadrant API not available: {api_check_error}")
            return []
        
        # Fetch AuthPolicies
        try:
            auth_url = f'https://{k8s_host}:{k8s_port}/apis/kuadrant.io/v1/authpolicies'
            auth_request = urllib.request.Request(auth_url)
            auth_request.add_header('Authorization', f'Bearer {token}')
            auth_request.add_header('Accept', 'application/json')
            
            with urllib.request.urlopen(auth_request, context=ctx, timeout=CLUSTER_TIMEOUT) as response:
                auth_data = json.loads(response.read().decode())
                print(f"🔍 Fetched {len(auth_data.get('items', []))} AuthPolicies")
            
                for policy in auth_data.get('items', []):
                    auth_items = []
                    rules = policy.get('spec', {}).get('rules', {})
                    
                    # Process authentication rules
                    for auth_name, auth_config in rules.get('authentication', {}).items():
                        auth_items.append({
                            "id": auth_name,
                            "type": "authentication",
                            "config": auth_config,
                            "description": f"API Key authentication with {auth_config.get('credentials', {}).get('authorizationHeader', {}).get('prefix', 'unknown')} prefix"
                        })
                    
                    # Process authorization rules
                    for auth_name, auth_config in rules.get('authorization', {}).items():
                        rego = auth_config.get('opa', {}).get('rego', '')
                        allowed_groups = []
                        if 'groups[_] ==' in rego:
                            import re
                            groups = re.findall(r'groups\[_\] == "([^"]+)"', rego)
                            allowed_groups = groups
                        
                        auth_items.append({
                            "id": auth_name,
                            "type": "authorization", 
                            "config": auth_config,
                            "description": f"OPA policy allowing groups: {', '.join(allowed_groups)}",
                            "allowedGroups": allowed_groups
                        })
                    
                    # Process response rules
                    for resp_name, resp_config in rules.get('response', {}).items():
                        auth_items.append({
                            "id": resp_name,
                            "type": "response",
                            "config": resp_config,
                            "description": f"{resp_name.title()} response filter"
                        })
                    
                    policies.append({
                        "id": f"{policy.get('metadata', {}).get('namespace', 'default')}/{policy.get('metadata', {}).get('name', 'unknown')}",
                        "name": policy.get('metadata', {}).get('name', 'unknown'),
                        "description": f"AuthPolicy for {policy.get('spec', {}).get('targetRef', {}).get('name', 'unknown')} with API key authentication and group-based authorization",
                        "type": "auth",
                        "namespace": policy.get('metadata', {}).get('namespace', 'default'),
                        "targetRef": policy.get('spec', {}).get('targetRef', {}),
                        "created": policy.get('metadata', {}).get('creationTimestamp', ''),
                        "modified": policy.get('metadata', {}).get('resourceVersion', ''),
                        "isActive": True,
                        "items": auth_items,
                        "status": policy.get('status', {}),
                        "fullSpec": policy.get('spec', {})
                    })
                    
        except Exception as auth_error:
            print(f"⚠️ Failed to fetch AuthPolicies: {auth_error}")
        
        # Fetch TokenRateLimitPolicies
        try:
            rate_url = f'https://{k8s_host}:{k8s_port}/apis/kuadrant.io/v1alpha1/tokenratelimitpolicies'
            rate_request = urllib.request.Request(rate_url)
            rate_request.add_header('Authorization', f'Bearer {token}')
            rate_request.add_header('Accept', 'application/json')
            
            with urllib.request.urlopen(rate_request, context=ctx, timeout=CLUSTER_TIMEOUT) as response:
                rate_data = json.loads(response.read().decode())
                print(f"🔍 Fetched {len(rate_data.get('items', []))} TokenRateLimitPolicies")
                
                for policy in rate_data.get('items', []):
                    rate_items = []
                    limits = policy.get('spec', {}).get('limits', {})
                    
                    for limit_name, limit_config in limits.items():
                        rates = limit_config.get('rates', [])
                        conditions = limit_config.get('when', [])
                        counters = limit_config.get('counters', [])
                        
                        rate_description = f"Rate limit: {limit_name}"
                        if rates:
                            rate_parts = []
                            for rate in rates:
                                rate_parts.append(f"{rate.get('limit', 'unknown')} tokens per {rate.get('window', 'unknown')}")
                            rate_description += f" - {', '.join(rate_parts)}"
                        
                        if conditions:
                            condition_desc = ", ".join([cond.get('predicate', '') for cond in conditions])
                            rate_description += f" (when: {condition_desc})"
                        
                        rate_items.append({
                            "id": limit_name,
                            "type": "rate-limit",
                            "config": limit_config,
                            "description": rate_description,
                            "rates": rates,
                            "counters": counters,
                            "conditions": conditions
                        })
                    
                    policies.append({
                        "id": f"{policy.get('metadata', {}).get('namespace', 'default')}/{policy.get('metadata', {}).get('name', 'unknown')}",
                        "name": policy.get('metadata', {}).get('name', 'unknown'),
                        "description": f"Token-based rate limiting policy with per-user limits",
                        "type": "rate-limit",
                        "namespace": policy.get('metadata', {}).get('namespace', 'default'),
                        "targetRef": policy.get('spec', {}).get('targetRef', {}),
                        "created": policy.get('metadata', {}).get('creationTimestamp', ''),
                        "modified": policy.get('metadata', {}).get('resourceVersion', ''),
                        "isActive": True,
                        "items": rate_items,
                        "status": policy.get('status', {}),
                        "fullSpec": policy.get('spec', {})
                    })
                    
        except Exception as rate_error:
            print(f"⚠️ Failed to fetch TokenRateLimitPolicies: {rate_error}")
        
        print(f"📋 Successfully fetched {len(policies)} policies via Kubernetes API")
        if policies:
            return policies
        else:
            print("⚠️ No policies found in cluster, this might be normal if no policies are configured")
            return policies  # Return empty list if no policies exist
        
    except Exception as e:
        print(f"⚠️ Failed to fetch policies via Kubernetes API: {e}")
        import traceback
        traceback.print_exc()
        return []

def fetch_policies_from_external_k8s_api(token):
    """Fetch policies using external Kubernetes API endpoints (for localhost)"""
    try:
        import urllib.request
        import urllib.parse
        import ssl
        
        # External cluster API endpoints
        k8s_host = 'api.summit-gpu.octo-emerging.redhataicoe.com'
        k8s_port = '6443'
        
        # Create SSL context
        ctx = create_ssl_context()
        
        policies = []
        
        # Set up headers
        headers = {
            'Authorization': f'Bearer {token}',
            'Accept': 'application/json'
        }
        
        # Fetch AuthPolicies
        try:
            auth_url = f'https://{k8s_host}:{k8s_port}/apis/kuadrant.io/v1/namespaces/llm/authpolicies'
            auth_request = urllib.request.Request(auth_url, headers=headers)
            
            with urllib.request.urlopen(auth_request, context=ctx, timeout=CLUSTER_TIMEOUT) as response:
                auth_data = json.loads(response.read().decode())
                print(f"🔍 Fetched {len(auth_data.get('items', []))} AuthPolicies from external API")
                
                # Process the policies using the same logic as the cluster version
                for policy in auth_data.get('items', []):
                    auth_items = []
                    rules = policy.get('spec', {}).get('rules', {})
                    
                    # Process authentication rules
                    for auth_name, auth_config in rules.get('authentication', {}).items():
                        auth_items.append({
                            "id": auth_name,
                            "type": "authentication",
                            "config": auth_config,
                            "description": f"API Key authentication with {auth_config.get('credentials', {}).get('authorizationHeader', {}).get('prefix', 'unknown')} prefix"
                        })
                    
                    # Process authorization rules
                    for auth_name, auth_config in rules.get('authorization', {}).items():
                        rego = auth_config.get('opa', {}).get('rego', '')
                        allowed_groups = []
                        if 'groups[_] ==' in rego:
                            import re
                            groups = re.findall(r'groups\[_\] == "([^"]+)"', rego)
                            allowed_groups = groups
                        
                        auth_items.append({
                            "id": auth_name,
                            "type": "authorization", 
                            "config": auth_config,
                            "description": f"OPA policy allowing groups: {', '.join(allowed_groups)}",
                            "allowedGroups": allowed_groups
                        })
                    
                    # Process response rules
                    for resp_name, resp_config in rules.get('response', {}).items():
                        auth_items.append({
                            "id": resp_name,
                            "type": "response",
                            "config": resp_config,
                            "description": f"{resp_name.title()} response filter"
                        })
                    
                    policies.append({
                        "id": f"{policy.get('metadata', {}).get('namespace', 'default')}/{policy.get('metadata', {}).get('name', 'unknown')}",
                        "name": policy.get('metadata', {}).get('name', 'unknown'),
                        "description": f"AuthPolicy for {policy.get('spec', {}).get('targetRef', {}).get('name', 'unknown')} with API key authentication and group-based authorization",
                        "type": "auth",
                        "namespace": policy.get('metadata', {}).get('namespace', 'default'),
                        "targetRef": policy.get('spec', {}).get('targetRef', {}),
                        "created": policy.get('metadata', {}).get('creationTimestamp', ''),
                        "modified": policy.get('metadata', {}).get('resourceVersion', ''),
                        "isActive": True,
                        "items": auth_items,
                        "status": policy.get('status', {}),
                        "fullSpec": policy.get('spec', {})
                    })
                    
        except Exception as auth_error:
            print(f"⚠️ Failed to fetch AuthPolicies from external API: {auth_error}")
        
        # Fetch TokenRateLimitPolicies
        try:
            rate_url = f'https://{k8s_host}:{k8s_port}/apis/kuadrant.io/v1alpha1/namespaces/llm/tokenratelimitpolicies'
            rate_request = urllib.request.Request(rate_url, headers=headers)
            
            with urllib.request.urlopen(rate_request, context=ctx, timeout=CLUSTER_TIMEOUT) as response:
                rate_data = json.loads(response.read().decode())
                print(f"🔍 Fetched {len(rate_data.get('items', []))} TokenRateLimitPolicies from external API")
                
                for policy in rate_data.get('items', []):
                    rate_items = []
                    limits = policy.get('spec', {}).get('limits', {})
                    
                    for limit_name, limit_config in limits.items():
                        rates = limit_config.get('rates', [])
                        conditions = limit_config.get('when', [])
                        counters = limit_config.get('counters', [])
                        
                        rate_description = f"Rate limit: {limit_name}"
                        if rates:
                            rate_parts = []
                            for rate in rates:
                                rate_parts.append(f"{rate.get('limit', 'unknown')} tokens per {rate.get('window', 'unknown')}")
                            rate_description += f" - {', '.join(rate_parts)}"
                        
                        if conditions:
                            condition_desc = ", ".join([cond.get('predicate', '') for cond in conditions])
                            rate_description += f" (when: {condition_desc})"
                        
                        rate_items.append({
                            "id": limit_name,
                            "type": "rate-limit",
                            "config": limit_config,
                            "description": rate_description,
                            "rates": rates,
                            "counters": counters,
                            "conditions": conditions
                        })
                    
                    policies.append({
                        "id": f"{policy.get('metadata', {}).get('namespace', 'default')}/{policy.get('metadata', {}).get('name', 'unknown')}",
                        "name": policy.get('metadata', {}).get('name', 'unknown'),
                        "description": f"Token-based rate limiting policy with per-user limits",
                        "type": "rate-limit",
                        "namespace": policy.get('metadata', {}).get('namespace', 'default'),
                        "targetRef": policy.get('spec', {}).get('targetRef', {}),
                        "created": policy.get('metadata', {}).get('creationTimestamp', ''),
                        "modified": policy.get('metadata', {}).get('resourceVersion', ''),
                        "isActive": True,
                        "items": rate_items,
                        "status": policy.get('status', {}),
                        "fullSpec": policy.get('spec', {})
                    })
                    
        except Exception as rate_error:
            print(f"⚠️ Failed to fetch TokenRateLimitPolicies from external API: {rate_error}")
        
        print(f"📋 Successfully fetched {len(policies)} policies via external Kubernetes API")
        return policies
        
    except Exception as e:
        print(f"⚠️ Failed to fetch policies via external Kubernetes API: {e}")
        return []

def fetch_cluster_metrics():
    """Fetch the same Envoy metrics from cluster Prometheus (works from both localhost and cluster)"""
    try:
        import urllib.request
        import urllib.parse
        import ssl
        
        # Get authentication token
        if is_running_in_cluster():
            # Read service account token
            with open('/var/run/secrets/kubernetes.io/serviceaccount/token', 'r') as f:
                token = f.read().strip()
            print("🔍 Using service account token for cluster Prometheus access")
        else:
            # For localhost, use oc token
            try:
                token_result = subprocess.run(['oc', 'whoami', '-t'], capture_output=True, text=True, timeout=SUBPROCESS_TIMEOUT)
                if token_result.returncode == 0:
                    token = token_result.stdout.strip()
                    print("🔍 Using oc token for cluster Prometheus access from localhost")
                else:
                    raise Exception("No oc token available")
            except Exception as e:
                print(f"❌ Cannot access cluster from localhost: {e}")
                raise Exception("Cluster access required - please login with 'oc login'")
        
        # Set up headers for K8s API
        headers = {
            'Authorization': f'Bearer {token}',
            'Accept': 'application/json'
        }
        
        # Use external cluster endpoints for both environments
        if is_running_in_cluster():
            prometheus_endpoints = [
                "https://prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9091",
                "https://prometheus-k8s.openshift-monitoring.svc.cluster.local:9091"
            ]
        else:
            # For localhost, use external cluster routes
            prometheus_endpoints = [
                "https://prometheus-user-workload-openshift-user-workload-monitoring.apps.summit-gpu.octo-emerging.redhataicoe.com",
                "https://prometheus-k8s-openshift-monitoring.apps.summit-gpu.octo-emerging.redhataicoe.com"
            ]
        
        # Use CONSISTENT metrics queries - only response-based metrics for LLM traffic
        metrics_queries = {
            'accepted_requests': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="2",namespace="llm"})',
            'rate_limited': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="4",namespace="llm"})',
            'auth_denied': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="4",namespace="llm"})',
            'server_errors': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="5",namespace="llm"})',
            'cluster_ingress_2xx_total': 'sum(haproxy_backend_http_responses_total{code="2xx"})',
            'cluster_ingress_4xx_total': 'sum(haproxy_backend_http_responses_total{code="4xx"})',
            'cluster_ingress_5xx_total': 'sum(haproxy_backend_http_responses_total{code="5xx"})',
            'cluster_4xx_1h': 'sum(increase(haproxy_backend_http_responses_total{code="4xx"}[1h]))',
            'cluster_4xx_recent': 'sum(increase(haproxy_backend_http_responses_total{code="4xx"}[10m]))',
            'limitador_status': 'sum(limitador_up{namespace="kuadrant-system"})',
            'http_requests': 'sum(rate(http_requests_total[5m]))'
        }
        
        raw_metrics = {}
        
        # Try to connect to Prometheus
        prometheus_connected = False
        for prometheus_base_url in prometheus_endpoints:
            try:
                # Create SSL context for cluster communication
                ctx = create_ssl_context()
                
                # Test connection
                test_url = f"{prometheus_base_url}/api/v1/query?query=up"
                req = urllib.request.Request(test_url, headers=headers)
                
                with urllib.request.urlopen(req, timeout=CLUSTER_TIMEOUT, context=ctx) as response:
                    test_data = json.loads(response.read().decode('utf-8'))
                    if test_data.get('status') == 'success':
                        print(f"✅ Connected to Prometheus: {prometheus_base_url}")
                        prometheus_connected = True
                        
                        # Query metrics
                        for metric_name, query in metrics_queries.items():
                            try:
                                query_url = f"{prometheus_base_url}/api/v1/query?query={urllib.parse.quote(query)}"
                                req = urllib.request.Request(query_url, headers=headers)
                                
                                with urllib.request.urlopen(req, timeout=CLUSTER_TIMEOUT, context=ctx) as response:
                                    data = json.loads(response.read().decode('utf-8'))
                                    
                                    if data.get('status') == 'success' and data.get('data', {}).get('result'):
                                        value = float(data['data']['result'][0]['value'][1])
                                        raw_metrics[metric_name] = value
                                        print(f"✅ {metric_name}: {value}")
                                    else:
                                        raw_metrics[metric_name] = 0.0
                                        print(f"⚠️ {metric_name}: No data (this is normal if no traffic)")
                                        
                            except Exception as e:
                                print(f"❌ Query failed for {metric_name}: {e}")
                                raw_metrics[metric_name] = 0.0
                        break
                        
            except Exception as e:
                print(f"❌ Failed to connect to {prometheus_base_url}: {e}")
                continue
        
        # If we couldn't connect to Prometheus, raise an error instead of estimating
        if not prometheus_connected:
            raise Exception("Unable to connect to any Prometheus endpoint. Cannot retrieve metrics.")
        
        # Return raw metrics (no estimation - same as localhost approach)
        print(f"🔍 Cluster mode: Retrieved {raw_metrics.get('total_requests', 0)} total requests from Envoy metrics")
        return raw_metrics
        
    except Exception as e:
        print(f"❌ Cluster metrics failed: {e}")
        raise

def fetch_real_metrics():
    """Fetch real metrics from Prometheus - unified approach for both environments"""
    try:
        # Define CONSISTENT metrics queries - use downstream requests for everything
        metrics_queries = {
            'accepted_requests': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="2",namespace="llm"})',
            'rate_limited': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="4",namespace="llm"})',
            'auth_denied': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="4",namespace="llm"})',
            'server_errors': 'sum(envoy_http_downstream_rq_xx{envoy_response_code_class="5",namespace="llm"})',
            'cluster_ingress_2xx_total': 'sum(haproxy_backend_http_responses_total{code="2xx"})',
            'cluster_ingress_4xx_total': 'sum(haproxy_backend_http_responses_total{code="4xx"})',
            'cluster_ingress_5xx_total': 'sum(haproxy_backend_http_responses_total{code="5xx"})',
            'cluster_4xx_1h': 'sum(increase(haproxy_backend_http_responses_total{code="4xx"}[1h]))',
            'cluster_4xx_recent': 'sum(increase(haproxy_backend_http_responses_total{code="4xx"}[10m]))',
            'limitador_status': 'sum(limitador_up{namespace="kuadrant-system"})',
            'http_requests': 'sum(rate(http_requests_total[5m]))'
        }
        
        # Always try to fetch real cluster metrics first
        try:
            print("🔍 Attempting to fetch real cluster metrics...")
            raw_metrics = fetch_cluster_metrics()
            environment = "cluster" if is_running_in_cluster() else "localhost-with-cluster-access"
            print(f"✅ Successfully fetched real cluster metrics from {environment}")
        except Exception as cluster_error:
            print(f"⚠️ Failed to fetch cluster metrics: {cluster_error}")
            print("🔍 Falling back to localhost-only metrics")
            # For localhost fallback, start with clean slate - only count simulator requests
            raw_metrics = {
                'accepted_requests': 0,
                'rate_limited': 0,
                'auth_denied': 0,
                'server_errors': 0,
                'cluster_ingress_2xx_total': 0,
                'cluster_ingress_4xx_total': 0,
                'cluster_ingress_5xx_total': 0,
                'cluster_4xx_1h': 0,
                'cluster_4xx_recent': 0,
                'limitador_status': 1.0,  # Assume connected for localhost
                'http_requests': 0.0
            }
            environment = "localhost-fallback"
        
        # Build final metrics structure with CONSISTENT calculations
        accepted_requests = int(raw_metrics.get('accepted_requests', 0))
        rate_limited = int(raw_metrics.get('rate_limited', 0))
        auth_denied = int(raw_metrics.get('auth_denied', 0))
        server_errors = int(raw_metrics.get('server_errors', 0))
        
        # Add simulator metrics to the totals
        global SIMULATOR_METRICS
        if not is_running_in_cluster():
            print(f"🎯 Adding simulator metrics: {SIMULATOR_METRICS['total_requests']} total, {SIMULATOR_METRICS['successful_requests']} successful")
            accepted_requests += SIMULATOR_METRICS['successful_requests']
            rate_limited += SIMULATOR_METRICS['rate_limits']
            auth_denied += SIMULATOR_METRICS['auth_failures']
        
        # Calculate total from components (this ensures math is correct)
        # Only count LLM-related requests, not all Envoy traffic
        total_requests = accepted_requests + rate_limited + auth_denied + server_errors
        rejected_requests = rate_limited + auth_denied + server_errors
        
        print(f"🔍 Metrics breakdown: {accepted_requests} accepted + {rate_limited} rate_limited + {auth_denied} auth_denied + {server_errors} server_errors = {total_requests} total")
        
        # Get cluster-wide metrics for context
        cluster_2xx_total = int(raw_metrics.get('cluster_ingress_2xx_total', 0))
        cluster_4xx_total = int(raw_metrics.get('cluster_ingress_4xx_total', 0))
        cluster_5xx_total = int(raw_metrics.get('cluster_ingress_5xx_total', 0))
        cluster_4xx_recent = int(raw_metrics.get('cluster_4xx_recent', 0))
        cluster_4xx_1h = int(raw_metrics.get('cluster_4xx_1h', 0))
        
        # Build final metrics structure
        metrics = {
            "totalRequests": total_requests,
            "acceptedRequests": accepted_requests,
            "rejectedRequests": rejected_requests,
            "authFailedRequests": auth_denied,
            "rateLimitedRequests": rate_limited,
            "policyEnforcedRequests": rejected_requests,
            "source": "prometheus-envoy-metrics",
            "kuadrantStatus": {
                "istioConnected": True,  # Based on envoy metrics availability
                "authorinoConnected": True,  # Based on auth metrics availability  
                "limitadorConnected": raw_metrics.get('limitador_status', 0) > 0
            },
            "rawMetrics": {
                "total_requests_calculated": total_requests,  # Our calculated total
                "rate_limited": rate_limited,
                "accepted_requests": accepted_requests,
                "auth_denied": auth_denied,
                "server_errors": server_errors,
                "cluster_ingress_2xx_total": cluster_2xx_total,
                "cluster_ingress_4xx_total": cluster_4xx_total,
                "cluster_ingress_5xx_total": cluster_5xx_total,
                "cluster_4xx_1h": cluster_4xx_1h,
                "cluster_4xx_recent": cluster_4xx_recent,
                "limitador_status": raw_metrics.get('limitador_status', 0),
                "http_requests": raw_metrics.get('http_requests', 0),
                "simulator_metrics": SIMULATOR_METRICS if not is_running_in_cluster() else {},
                "prometheus_raw": raw_metrics  # Show what Prometheus actually returned
            }
        }
        
        print(f"✅ Retrieved metrics from Prometheus ({environment}): {total_requests} total requests")
        return metrics
        
    except Exception as e:
        print(f"❌ Failed to fetch real metrics: {e}")
        # No fallback - raise error if metrics cannot be retrieved
        raise Exception(f"Unable to fetch metrics from Prometheus: {str(e)}")

class CORSRequestHandler(http.server.BaseHTTPRequestHandler):
    def do_OPTIONS(self):
        """Handle preflight CORS requests"""
        self.send_response(200)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.send_header('Access-Control-Max-Age', '86400')
        self.end_headers()
    
    def do_GET(self):
        """Handle GET requests"""
        parsed_path = urlparse(self.path)
        path = parsed_path.path
        
        print(f"📡 GET {path}")
        
        if path == '/health':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
            self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
            self.end_headers()
            response = {"status": "ok", "timestamp": datetime.now().isoformat()}
        elif path == '/api/v1/policies':
            print("📋 Fetching policies...")
            policies = fetch_kuadrant_policies()
            print(f"📋 Got {len(policies)} policies from fetch function")
            
            if policies:
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": policies,
                    "timestamp": datetime.now().isoformat()
                }
                print(f"📋 Returning {len(policies)} real policies")
            else:
                # Return 200 with error details so frontend can handle it properly
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                error_response = {
                    "success": False,
                    "error": "Unable to fetch policies from cluster. Please ensure you're logged into the cluster with 'oc login' or check cluster connectivity.",
                    "timestamp": datetime.now().isoformat(),
                    "data": []
                }
                self.wfile.write(json.dumps(error_response).encode('utf-8'))
                return
        elif path == '/api/v1/metrics/dashboard':
            print("📊 Fetching dashboard metrics...")
            
            try:
                metrics_data = fetch_real_metrics()
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": metrics_data,
                    "timestamp": datetime.now().isoformat()
                }
                environment = "cluster" if is_running_in_cluster() else "localhost"
                print(f"📊 Returning real metrics ({environment}): {metrics_data['totalRequests']} total requests")
            except Exception as e:
                print(f"❌ Failed to fetch metrics: {e}")
                # Return error instead of mock data
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                error_response = {
                    "success": False,
                    "error": f"Unable to fetch metrics from Prometheus: {str(e)}",
                    "timestamp": datetime.now().isoformat()
                }
                self.wfile.write(json.dumps(error_response).encode('utf-8'))
                return
        elif path == '/api/v1/metrics/live-requests':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
            self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
            self.end_headers()
            response = {
                "success": True,
                "data": [],
                "timestamp": datetime.now().isoformat()
            }
            print("📈 Returning live requests")
        elif path == '/api/v1/models':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
            self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
            self.end_headers()
            response = {
                "success": True,
                "data": [
                    {"name": "vllm-simulator", "description": "VLLM Simulator Model"},
                    {"name": "qwen3-0.6b-instruct", "description": "Qwen3 0.6B Instruct Model"}
                ],
                "timestamp": datetime.now().isoformat()
            }
        elif path == '/api/v1/tokens/user/tier':
            try:
                user_tier = get_user_tier_from_team()
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": user_tier,
                    "timestamp": datetime.now().isoformat()
                }
            except Exception as e:
                print(f"❌ Failed to fetch user tier: {e}")
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                error_response = {
                    "success": False,
                    "error": f"Unable to fetch user tier: {str(e)}",
                    "timestamp": datetime.now().isoformat()
                }
                self.wfile.write(json.dumps(error_response).encode('utf-8'))
                return
        elif path == '/api/v1/cluster/status':
            try:
                # Check cluster connectivity and get user info
                cluster_status = {
                    "connected": False,
                    "user": None,
                    "cluster": None,
                    "loginUrl": "https://console-openshift-console.apps.summit-gpu.octo-emerging.redhataicoe.com"
                }
                
                try:
                    # Try to get current user - but don't fail if oc command doesn't work
                    user_result = subprocess.run(['oc', 'whoami'], capture_output=True, text=True, timeout=SUBPROCESS_TIMEOUT)
                    if user_result.returncode == 0:
                        cluster_status["connected"] = True
                        cluster_status["user"] = user_result.stdout.strip()
                        
                        # Try to get cluster info
                        try:
                            cluster_result = subprocess.run(['oc', 'cluster-info'], capture_output=True, text=True, timeout=SUBPROCESS_TIMEOUT)
                            if cluster_result.returncode == 0:
                                # Extract cluster URL from cluster-info output
                                import re
                                cluster_match = re.search(r'https://[^\s]+', cluster_result.stdout)
                                if cluster_match:
                                    cluster_status["cluster"] = cluster_match.group(0)
                        except Exception as e:
                            print(f"⚠️ Could not get cluster info: {e}")
                except Exception as e:
                    print(f"⚠️ Could not check oc login status: {e}")
                    # Don't fail - just return disconnected status
                
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": cluster_status,
                    "timestamp": datetime.now().isoformat()
                }
            except Exception as e:
                print(f"❌ Failed to check cluster status: {e}")
                self.send_response(200)  # Still return 200 but with disconnected status
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": {
                        "connected": False,
                        "user": None,
                        "cluster": None,
                        "loginUrl": "https://console-openshift-console.apps.summit-gpu.octo-emerging.redhataicoe.com",
                        "error": str(e)
                    },
                    "timestamp": datetime.now().isoformat()
                }
        elif path == '/api/v1/auth/oauth/exchange':
            try:
                # Handle OAuth code exchange
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)
                request_data = json.loads(post_data.decode('utf-8'))
                
                code = request_data.get('code')
                redirect_uri = request_data.get('redirect_uri')
                
                if not code:
                    self.send_response(400)
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    error_response = {
                        "success": False,
                        "error": "Missing authorization code",
                        "timestamp": datetime.now().isoformat()
                    }
                    self.wfile.write(json.dumps(error_response).encode('utf-8'))
                    return
                
                # Exchange code for token with OpenShift OAuth server
                oauth_token = exchange_oauth_code_for_token(code, redirect_uri)
                
                if oauth_token:
                    # Store the token securely (in production, use proper session management)
                    # For now, we'll store it in a simple dict (this is not production-ready)
                    global OAUTH_TOKENS
                    session_id = f"session_{datetime.now().timestamp()}"
                    OAUTH_TOKENS[session_id] = oauth_token
                    
                    self.send_response(200)
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    response = {
                        "success": True,
                        "data": {
                            "session_id": session_id,
                            "message": "Authentication successful"
                        },
                        "timestamp": datetime.now().isoformat()
                    }
                    self.wfile.write(json.dumps(response).encode('utf-8'))
                    return
                else:
                    self.send_response(400)
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    error_response = {
                        "success": False,
                        "error": "Failed to exchange authorization code for token",
                        "timestamp": datetime.now().isoformat()
                    }
                    self.wfile.write(json.dumps(error_response).encode('utf-8'))
                    return
                    
            except Exception as e:
                print(f"❌ OAuth exchange error: {e}")
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                error_response = {
                    "success": False,
                    "error": f"OAuth exchange failed: {str(e)}",
                    "timestamp": datetime.now().isoformat()
                }
                self.wfile.write(json.dumps(error_response).encode('utf-8'))
                return
        elif path == '/api/v1/tokens':
            try:
                tokens = fetch_user_tokens()
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                response = {
                    "success": True,
                    "data": tokens,
                    "timestamp": datetime.now().isoformat()
                }
            except Exception as e:
                print(f"❌ Failed to fetch user tokens: {e}")
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                error_response = {
                    "success": False,
                    "error": f"Unable to fetch user tokens: {str(e)}",
                    "timestamp": datetime.now().isoformat()
                }
                self.wfile.write(json.dumps(error_response).encode('utf-8'))
                return
        else:
            self.send_response(404)
            self.send_header('Content-type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
            self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
            self.end_headers()
            response = {"error": "Not found", "path": path}
        
        self.wfile.write(json.dumps(response).encode('utf-8'))
    
    def do_POST(self):
        """Handle POST requests"""
        parsed_path = urlparse(self.path)
        path = parsed_path.path
        
        print(f"📤 POST {path}")
        
        # Initialize response variables
        response = {"error": "Not found", "path": path}
        status_code = 404
        
        if path == '/api/v1/auth/oauth/exchange':
            try:
                # Handle OAuth code exchange
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)
                request_data = json.loads(post_data.decode('utf-8'))
                
                code = request_data.get('code')
                redirect_uri = request_data.get('redirect_uri')
                
                if not code:
                    status_code = 400
                    response = {
                        "success": False,
                        "error": "Missing authorization code",
                        "timestamp": datetime.now().isoformat()
                    }
                else:
                    # Exchange code for token with OpenShift OAuth server
                    oauth_token = exchange_oauth_code_for_token(code, redirect_uri)
                    
                    if oauth_token:
                        # Store the token securely (in production, use proper session management)
                        global OAUTH_TOKENS
                        session_id = f"session_{datetime.now().timestamp()}"
                        OAUTH_TOKENS[session_id] = oauth_token
                        
                        status_code = 200
                        response = {
                            "success": True,
                            "data": {
                                "session_id": session_id,
                                "message": "Authentication successful"
                            },
                            "timestamp": datetime.now().isoformat()
                        }
                    else:
                        status_code = 400
                        response = {
                            "success": False,
                            "error": "Failed to exchange authorization code for token",
                            "timestamp": datetime.now().isoformat()
                        }
                        
            except Exception as e:
                print(f"❌ OAuth exchange error: {e}")
                status_code = 500
                response = {
                    "success": False,
                    "error": f"OAuth exchange failed: {str(e)}",
                    "timestamp": datetime.now().isoformat()
                }
        elif path == '/api/v1/simulator/chat/completions':
            try:
                # Declare global variables at the beginning
                global SIMULATOR_METRICS
                
                # Read request body
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)
                request_data = json.loads(post_data.decode('utf-8'))
                
                # Get authorization header
                auth_header = self.headers.get('Authorization', '')
                
                print(f"🎯 Simulator request: model={request_data.get('model')}, tier={request_data.get('tier')}")
                
                # Make REAL requests to the actual model endpoints through Kuadrant
                tier = request_data.get('tier', 'unknown')
                model = request_data.get('model', 'unknown')
                
                # All models should go through the same Kuadrant gateway endpoint
                # Kuadrant will handle routing to the appropriate backend based on the model name
                gateway_endpoint = os.getenv('MODEL_GATEWAY_URL', f'http://simulator-llm.{CLUSTER_DOMAIN}/v1/chat/completions')
                
                # For localhost development, try to use the same external endpoints
                # (requires proper API keys and network access)
                if is_running_in_cluster():
                    # In cluster, use the cluster-internal service endpoints
                    model_endpoints = {
                        'vllm-simulator': 'http://inference-gateway-istio.llm.svc.cluster.local/v1/chat/completions',
                        'qwen3-0-6b-instruct': 'http://inference-gateway-istio.llm.svc.cluster.local/v1/chat/completions'
                    }
                else:
                    # For localhost, try the external endpoints first
                    # If they fail, we'll provide helpful error messages
                    print(f"🔍 Localhost mode: Will attempt real request to external endpoint")
                
                endpoint_url = gateway_endpoint
                print(f"🔍 Using unified gateway endpoint: {endpoint_url}")
                print(f"🔍 Model '{model}' will be routed by Kuadrant to the appropriate backend")
                
                try:
                    # Prepare the real request to the model endpoint
                    real_request_data = {
                        "model": model,
                        "messages": request_data.get('messages', []),
                        "max_tokens": request_data.get('max_tokens', 100)
                    }
                    
                    # Prepare headers for the real request
                    real_headers = {
                        'Content-Type': 'application/json',
                        'Authorization': auth_header  # Pass through the auth header
                    }
                    
                    print(f"🌐 Making REAL request to {endpoint_url} with tier {tier}")
                    
                    # Make the real HTTP request to the model endpoint
                    import urllib.request
                    import urllib.parse
                    
                    req_data = json.dumps(real_request_data).encode('utf-8')
                    req = urllib.request.Request(endpoint_url, data=req_data, headers=real_headers, method='POST')
                    
                    try:
                            with urllib.request.urlopen(req, timeout=30) as real_response:
                                real_response_data = json.loads(real_response.read().decode('utf-8'))
                                
                                # Track successful request
                                SIMULATOR_METRICS['total_requests'] += 1
                                SIMULATOR_METRICS['successful_requests'] += 1
                                
                                status_code = 200
                                response = {
                                    "success": True,
                                    "data": real_response_data,
                                    "debug": {
                                        "tier": tier,
                                        "model": model,
                                        "endpoint": endpoint_url,
                                        "real_request": True,
                                        "localhost": not is_running_in_cluster(),
                                        "simulator_total": SIMULATOR_METRICS['total_requests']
                                    }
                                }
                                print(f"✅ Real model response received from {endpoint_url} (simulator total: {SIMULATOR_METRICS['total_requests']})")
                                
                        except urllib.error.HTTPError as e:
                            # Handle HTTP errors from the model endpoint (auth failures, rate limits, etc.)
                            error_body = e.read().decode('utf-8') if e.fp else str(e)
                            status_code = e.code
                            
                            # Track failed request
                            SIMULATOR_METRICS['total_requests'] += 1
                            SIMULATOR_METRICS['failed_requests'] += 1
                            
                            if e.code == 401:
                                SIMULATOR_METRICS['auth_failures'] += 1
                                error_msg = "Authentication failed - invalid API key or unauthorized tier"
                            elif e.code == 429:
                                SIMULATOR_METRICS['rate_limits'] += 1
                                error_msg = "Rate limit exceeded for this tier"
                            elif e.code == 403:
                                error_msg = "Forbidden - tier not allowed for this model"
                            else:
                                error_msg = f"Model endpoint error: {e.reason}"
                            
                            response = {
                                "success": False,
                                "error": error_msg,
                                "debug": {
                                    "tier": tier,
                                    "model": model,
                                    "endpoint": endpoint_url,
                                    "http_status": e.code,
                                    "error_body": error_body,
                                    "real_request": True,
                                    "simulator_total": SIMULATOR_METRICS['total_requests']
                                }
                            }
                            print(f"❌ Real model request failed: {e.code} {e.reason} (simulator total: {SIMULATOR_METRICS['total_requests']})")
                            
                        except Exception as e:
                            # Handle network errors, timeouts, etc.
                            # Track failed request
                            SIMULATOR_METRICS['total_requests'] += 1
                            SIMULATOR_METRICS['failed_requests'] += 1
                            
                            status_code = 500
                            response = {
                                "success": False,
                                "error": f"Network error connecting to model: {str(e)}",
                                "debug": {
                                    "tier": tier,
                                    "model": model,
                                    "endpoint": endpoint_url,
                                    "real_request": True,
                                    "network_error": True,
                                    "simulator_total": SIMULATOR_METRICS['total_requests']
                                }
                            }
                            print(f"❌ Network error connecting to {endpoint_url}: {e} (simulator total: {SIMULATOR_METRICS['total_requests']})")
                            
                    except Exception as e:
                        # Handle request preparation errors
                        status_code = 500
                        response = {
                            "success": False,
                            "error": f"Request preparation error: {str(e)}",
                            "debug": {"tier": tier, "model": model}
                        }
                
            except Exception as e:
                print(f"❌ Error in simulator: {e}")
                status_code = 500
                response = {"success": False, "error": str(e)}
        
        elif path == '/api/v1/tokens/create':
            try:
                # Create new token
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)
                request_data = json.loads(post_data.decode('utf-8'))
                
                token_name = request_data.get('name', '')
                token_description = request_data.get('description', '')
                
                # Generate a mock token
                import uuid
                new_token = f"maas_token_{uuid.uuid4().hex[:16]}"
                
                # Add to mock tokens
                MOCK_TOKENS.append({
                    'name': token_name,
                    'created': datetime.now().isoformat(),
                    'lastUsed': '',
                    'usage': 0,
                    'status': 'active'
                })
                
                self.send_response(200)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                
                response = {
                    "success": True,
                    "data": {
                        "token": new_token,
                        "name": token_name,
                        "description": token_description,
                        "created": datetime.now().isoformat()
                    }
                }
                self.wfile.write(json.dumps(response).encode('utf-8'))
                
            except Exception as e:
                print(f"❌ Error creating token: {e}")
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                response = {"success": False, "error": str(e)}
                self.wfile.write(json.dumps(response).encode('utf-8'))
        
        elif path == '/api/v1/tokens/test':
            try:
                # Test token functionality with real model endpoint
                content_length = int(self.headers['Content-Length'])
                post_data = self.rfile.read(content_length)
                request_data = json.loads(post_data.decode('utf-8'))
                
                token = request_data.get('token', '')
                model = request_data.get('model', 'vllm-simulator')
                message = request_data.get('message', 'Hello!')
                
                # Input validation
                if not token or not token.strip():
                    self.send_response(400)
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    response = {
                        "success": False,
                        "data": {"error": "Token is required"}
                    }
                    self.wfile.write(json.dumps(response).encode('utf-8'))
                    return
                
                if not message or not message.strip():
                    self.send_response(400)
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    response = {
                        "success": False,
                        "data": {"error": "Message is required"}
                    }
                    self.wfile.write(json.dumps(response).encode('utf-8'))
                    return
                
                print(f"🧪 Testing token with request:")
                print(f"   Token (first 10 chars): {token[:10]}...")
                print(f"   Model: {model}")
                print(f"   Message: {message}")
                
                # All models should go through the same Kuadrant gateway endpoint
                # Kuadrant will handle routing to the appropriate backend based on the model name
                gateway_endpoint = os.getenv('MODEL_GATEWAY_URL', f'http://simulator-llm.{CLUSTER_DOMAIN}/v1/chat/completions')
                endpoint_url = gateway_endpoint
                
                print(f"🔍 Using unified gateway endpoint for all models: {endpoint_url}")
                print(f"🔍 Model '{model}' will be routed by Kuadrant to the appropriate backend")
                
                # Prepare the real request
                real_request_data = {
                    "model": model,
                    "messages": [{"role": "user", "content": message}],
                    "max_tokens": 50
                }
                
                # Prepare headers - try both APIKEY and Bearer formats
                auth_format = 'APIKEY'  # Default to APIKEY
                if token.startswith('Bearer '):
                    auth_format = 'Bearer'
                    token = token[7:]  # Remove 'Bearer ' prefix
                elif token.startswith('APIKEY '):
                    token = token[7:]  # Remove 'APIKEY ' prefix
                
                real_headers = {
                    'Content-Type': 'application/json',
                    'Authorization': f'{auth_format} {token}'
                }
                
                print(f"🌐 Making REAL request to: {endpoint_url}")
                print(f"   Auth Format: {auth_format}")
                print(f"   Expected flow: Request → Kuadrant Auth → Kuadrant Rate Limiting → Model")
                
                # Make the real HTTP request
                import urllib.request
                import urllib.parse
                import ssl
                
                req_data = json.dumps(real_request_data).encode('utf-8')
                req = urllib.request.Request(endpoint_url, data=req_data, headers=real_headers, method='POST')
                
                # Create SSL context
                ctx = create_ssl_context()
                
                try:
                    with urllib.request.urlopen(req, timeout=30, context=ctx) as real_response:
                        response_headers = dict(real_response.headers)
                        response_body = real_response.read().decode('utf-8')
                        response_data = json.loads(response_body)
                        
                        print(f"✅ Real model response received:")
                        print(f"   Status: {real_response.status}")
                        print(f"   Headers: {response_headers}")
                        print(f"   Body: {response_body[:200]}...")
                        
                        self.send_response(200)
                        self.send_header('Content-type', 'application/json')
                        self.send_header('Access-Control-Allow-Origin', '*')
                        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                        self.end_headers()
                        
                        test_response = {
                            "success": True,
                            "data": {
                                "message": "Token test successful!",
                                "statusCode": real_response.status,
                                "request": {
                                    "url": endpoint_url,
                                    "method": "POST",
                                    "headers": real_headers,
                                    "body": real_request_data
                                },
                                "response": {
                                    "status": real_response.status,
                                    "headers": response_headers,
                                    "body": response_data
                                }
                            }
                        }
                        self.wfile.write(json.dumps(test_response).encode('utf-8'))
                        return
                        
                except urllib.error.HTTPError as e:
                    error_body = e.read().decode('utf-8') if e.fp else str(e)
                    error_headers = dict(e.headers) if hasattr(e, 'headers') else {}
                    
                    print(f"❌ Real model request failed:")
                    print(f"   Status: {e.code} {e.reason}")
                    print(f"   Headers: {error_headers}")
                    print(f"   Body: {error_body}")
                    
                    # Parse Kuadrant auth error details
                    auth_reason = error_headers.get('x-ext-auth-reason', '')
                    kuadrant_error = ""
                    if auth_reason:
                        try:
                            auth_details = json.loads(auth_reason)
                            kuadrant_error = f" | Kuadrant Auth Details: {auth_details}"
                        except:
                            kuadrant_error = f" | Kuadrant Auth Reason: {auth_reason}"
                    
                    self.send_response(200)  # Return 200 but with error details
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    
                    # Provide helpful error messages based on status code
                    if e.code == 401:
                        error_msg = f"Authentication Failed: API key '{token[:10]}...' is invalid or not found in Kuadrant{kuadrant_error}"
                    elif e.code == 403:
                        error_msg = f"Authorization Failed: API key is valid but not authorized for this model/tier{kuadrant_error}"
                    elif e.code == 429:
                        error_msg = f"Rate Limited: API key has exceeded rate limits{kuadrant_error}"
                    else:
                        error_msg = f"Request Failed: {e.code} {e.reason}{kuadrant_error}"
                    
                    test_response = {
                        "success": False,
                        "data": {
                            "error": error_msg,
                            "message": error_msg,
                            "statusCode": e.code,
                            "request": {
                                "url": endpoint_url,
                                "method": "POST", 
                                "headers": real_headers,
                                "body": real_request_data
                            },
                            "responseDetails": {
                                "status": e.code,
                                "headers": error_headers,
                                "body": error_body
                            }
                        }
                    }
                    self.wfile.write(json.dumps(test_response).encode('utf-8'))
                    return
                
                except Exception as network_error:
                    print(f"❌ Network error connecting to {endpoint_url}: {network_error}")
                    
                    self.send_response(200)  # Return 200 but with error details
                    self.send_header('Content-type', 'application/json')
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                    self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                    self.end_headers()
                    
                    test_response = {
                        "success": False,
                        "data": {
                            "error": f"Network Error: Unable to connect to {endpoint_url}. This could be due to network connectivity issues, DNS resolution problems, or the endpoint being unavailable.",
                            "statusCode": 503,
                            "request": {
                                "url": endpoint_url,
                                "method": "POST", 
                                "headers": real_headers,
                                "body": real_request_data
                            },
                            "responseDetails": {
                                "status": 503,
                                "headers": {},
                                "body": f"Network error: {str(network_error)}"
                            }
                        }
                    }
                    self.wfile.write(json.dumps(test_response).encode('utf-8'))
                    return
                
            except Exception as e:
                print(f"❌ Error testing token: {e}")
                import traceback
                traceback.print_exc()
                
                self.send_response(500)
                self.send_header('Content-type', 'application/json')
                self.send_header('Access-Control-Allow-Origin', '*')
                self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
                self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
                self.end_headers()
                
                response = {
                    "success": False, 
                    "data": {
                        "error": f"Internal server error: {str(e)}"
                    }
                }
                self.wfile.write(json.dumps(response).encode('utf-8'))
                return
        
        else:
            status_code = 404
            response = {"error": "Not found", "path": path}
        
        # Send the final response
        self.send_response(status_code)
        self.send_header('Content-type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.end_headers()
        self.wfile.write(json.dumps(response).encode('utf-8'))
    
    def do_DELETE(self):
        """Handle DELETE requests"""
        parsed_path = urlparse(self.path)
        path = parsed_path.path
        
        print(f"🗑️ DELETE {path}")
        
        # Set CORS headers
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS, PUT, DELETE')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.end_headers()
        
        if path.startswith('/api/v1/tokens/'):
            # Extract token name from path
            token_name = path.split('/')[-1]
            
            # Remove from mock tokens
            global MOCK_TOKENS
            MOCK_TOKENS = [token for token in MOCK_TOKENS if token['name'] != token_name]
            
            response = {
                "success": True,
                "data": {"message": f"Token '{token_name}' revoked successfully"}
            }
        else:
            response = {"error": "Not found", "path": path}
        
        self.wfile.write(json.dumps(response).encode('utf-8'))

def run_server(port=3002):
    """Run the HTTP server"""
    handler = CORSRequestHandler
    
    with socketserver.TCPServer(("", port), handler) as httpd:
        print(f"🚀 MaaS Python Backend running on port {port}")
        print(f"📊 Health check: http://localhost:{port}/health")
        print(f"📋 Policies API: http://localhost:{port}/api/v1/policies")
        print(f"🔑 Tokens API: http://localhost:{port}/api/v1/tokens")
        print(f"📈 Metrics API: http://localhost:{port}/api/v1/metrics/dashboard")
        print("")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n🛑 Server stopped")

if __name__ == "__main__":
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 3002
    run_server(port)