#!/usr/bin/env python3
"""
Named Token Smoke Test (Python Version)

Tests the named token functionality including:
- Creating a named token
- Validating it works for API calls
- Negative test for 401 with invalid token
- Verifying Kubernetes secret was created with correct metadata
"""

import os
import sys
import json
import time
import base64
import subprocess
from typing import Optional, Tuple, Dict, Any
from datetime import datetime

try:
    import requests
except ImportError:
    print("Error: requests library not found. Install with: pip install requests")
    sys.exit(1)


class Colors:
    """ANSI color codes for terminal output"""
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'  # No Color


class NamedTokenTest:
    """Named token smoke test class"""
    
    def __init__(self):
        self.base_url = os.getenv('MAAS_API_BASE_URL', '').rstrip('/')
        self.oc_token = os.getenv('OC_TOKEN', '')
        self.namespace_prefix = os.getenv('NAMESPACE_PREFIX', 'maas')
        self.keep_secrets = os.getenv('KEEP_SECRETS', 'false').lower() == 'true'
        self.token_name = f"smoke-test-{int(time.time())}"
        self.maas_token: Optional[str] = None
        self.expiration_timestamp: Optional[int] = None
        self.secret_namespace: Optional[str] = None
        self.secret_name: Optional[str] = None
        self.test_failed = False
        
    def log_info(self, msg: str):
        """Log info message"""
        print(f"{Colors.BLUE}[INFO]{Colors.NC} {msg}")
    
    def log_success(self, msg: str):
        """Log success message"""
        print(f"{Colors.GREEN}[✓]{Colors.NC} {msg}")
    
    def log_error(self, msg: str):
        """Log error message"""
        print(f"{Colors.RED}[✗]{Colors.NC} {msg}")
        self.test_failed = True
    
    def log_warning(self, msg: str):
        """Log warning message"""
        print(f"{Colors.YELLOW}[!]{Colors.NC} {msg}")
    
    def check_prerequisites(self) -> bool:
        """Check that all prerequisites are met"""
        self.log_info("Checking prerequisites...")
        
        if not self.base_url:
            self.log_error("MAAS_API_BASE_URL environment variable is not set")
            return False
        
        if not self.oc_token:
            self.log_error("OC_TOKEN environment variable is not set")
            return False
        
        # Check for kubectl
        try:
            subprocess.run(['kubectl', 'version', '--client'],
                         capture_output=True, check=True)
        except (subprocess.CalledProcessError, FileNotFoundError):
            self.log_error("kubectl is not installed or not in PATH")
            return False
        
        self.log_success("All prerequisites met")
        return True
    
    def test_create_named_token(self) -> bool:
        """Test 1: Create a named token"""
        self.log_info(f"Test 1: Creating named token '{self.token_name}'...")
        
        try:
            response = requests.post(
                f"{self.base_url}/v1/tokens",
                headers={
                    "Authorization": f"Bearer {self.oc_token}",
                    "Content-Type": "application/json"
                },
                json={
                    "expiration": "1h",
                    "name": self.token_name
                },
                timeout=30
            )
            
            if response.status_code != 201:
                self.log_error(f"Failed to create named token. HTTP code: {response.status_code}")
                self.log_error(f"Response: {response.text}")
                return False
            
            data = response.json()
            self.maas_token = data.get('token')
            self.expiration_timestamp = data.get('expiresAt')
            
            if not self.maas_token:
                self.log_error(f"Token not found in response: {data}")
                return False
            
            self.log_success("Named token created successfully")
            self.log_info(f"Token: {self.maas_token[:20]}...")
            
            if self.expiration_timestamp:
                exp_date = datetime.fromtimestamp(self.expiration_timestamp)
                self.log_info(f"Expires at: {self.expiration_timestamp} ({exp_date})")
            
            return True
            
        except requests.RequestException as e:
            self.log_error(f"Request failed: {e}")
            return False
    
    def test_token_works(self) -> bool:
        """Test 2: Validate the token works"""
        self.log_info("Test 2: Validating token works for API calls...")
        
        if not self.maas_token:
            self.log_error("No token available for testing")
            return False
        
        try:
            response = requests.get(
                f"{self.base_url}/v1/models",
                headers={"Authorization": f"Bearer {self.maas_token}"},
                timeout=30
            )
            
            if response.status_code != 200:
                self.log_error(f"Token validation failed. HTTP code: {response.status_code}")
                self.log_error(f"Response: {response.text}")
                return False
            
            self.log_success("Token works correctly for authenticated requests")
            return True
            
        except requests.RequestException as e:
            self.log_error(f"Request failed: {e}")
            return False
    
    def test_invalid_token_401(self) -> bool:
        """Test 3: Invalid token should return 401"""
        self.log_info("Test 3: Testing invalid token returns 401...")
        
        invalid_token = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid.token"
        
        try:
            response = requests.get(
                f"{self.base_url}/v1/models",
                headers={"Authorization": f"Bearer {invalid_token}"},
                timeout=30
            )
            
            if response.status_code == 401:
                self.log_success("Invalid token correctly returned 401 Unauthorized")
                return True
            else:
                self.log_error(f"Expected 401 for invalid token, got: {response.status_code}")
                return False
            
        except requests.RequestException as e:
            self.log_error(f"Request failed: {e}")
            return False
    
    def find_secret(self) -> Tuple[Optional[str], Optional[str]]:
        """Find the Kubernetes secret for the token"""
        # Get tier namespaces
        try:
            result = subprocess.run(
                ['kubectl', 'get', 'namespaces',
                 '-l', 'maas.opendatahub.io/tier-namespace=true',
                 '-o', 'jsonpath={.items[*].metadata.name}'],
                capture_output=True,
                text=True,
                check=False
            )
            namespaces = result.stdout.strip().split()
        except subprocess.CalledProcessError:
            namespaces = []
        
        if not namespaces:
            self.log_warning("No tier namespaces found, checking default namespaces...")
            namespaces = [
                f"{self.namespace_prefix}-tier-free",
                f"{self.namespace_prefix}-tier-premium",
                f"{self.namespace_prefix}-tier-enterprise"
            ]
        
        for ns in namespaces:
            self.log_info(f"Checking namespace: {ns}")
            
            try:
                result = subprocess.run(
                    ['kubectl', 'get', 'secrets', '-n', ns,
                     '-l', 'maas.opendatahub.io/token-secret=true',
                     '-o', 'json'],
                    capture_output=True,
                    text=True,
                    check=False
                )
                
                if result.returncode != 0:
                    continue
                
                secrets = json.loads(result.stdout)
                
                for secret in secrets.get('items', []):
                    annotations = secret.get('metadata', {}).get('annotations', {})
                    if annotations.get('maas.opendatahub.io/token-name') == self.token_name:
                        secret_name = secret['metadata']['name']
                        return secret_name, ns
                        
            except (subprocess.CalledProcessError, json.JSONDecodeError):
                continue
        
        return None, None
    
    def test_secret_metadata(self) -> bool:
        """Test 4: Verify Kubernetes secret exists with correct metadata"""
        self.log_info("Test 4: Verifying Kubernetes secret was created with metadata...")
        
        self.log_info(f"Looking for token metadata secret for token: {self.token_name}")
        
        secret_name, secret_namespace = self.find_secret()
        
        if not secret_name or not secret_namespace:
            self.log_error(f"Token metadata secret not found for token: {self.token_name}")
            return False
        
        # Store for cleanup
        self.secret_name = secret_name
        self.secret_namespace = secret_namespace
        
        self.log_success(f"Found token metadata secret: {secret_name} in namespace: {secret_namespace}")
        
        # Get secret data
        try:
            result = subprocess.run(
                ['kubectl', 'get', 'secret', secret_name,
                 '-n', secret_namespace, '-o', 'json'],
                capture_output=True,
                text=True,
                check=True
            )
            secret_data = json.loads(result.stdout)
        except (subprocess.CalledProcessError, json.JSONDecodeError) as e:
            self.log_error(f"Failed to get secret data: {e}")
            return False
        
        # Validate required fields
        self.log_info("Validating secret metadata fields...")
        
        required_fields = ['username', 'creationDate', 'expirationDate', 'name', 'status']
        all_fields_present = True
        
        data = secret_data.get('data', {})
        
        for field in required_fields:
            if field not in data:
                self.log_error(f"Required field '{field}' not found in secret")
                all_fields_present = False
                continue
            
            # Decode base64 value
            try:
                value = base64.b64decode(data[field]).decode('utf-8')
                self.log_info(f"  {field}: {value}")
                
                # Validate specific fields
                if field == 'name' and value != self.token_name:
                    self.log_error(f"  Expected name '{self.token_name}', got '{value}'")
                    all_fields_present = False
                
                if field == 'status' and value != 'active':
                    self.log_warning(f"  Expected status 'active', got '{value}'")
                
                if field in ('creationDate', 'expirationDate'):
                    # Validate RFC3339 timestamp
                    try:
                        datetime.fromisoformat(value.replace('Z', '+00:00'))
                    except ValueError:
                        self.log_error(f"  Invalid timestamp format: {value}")
                        all_fields_present = False
                        
            except Exception as e:
                self.log_error(f"  Failed to decode field '{field}': {e}")
                all_fields_present = False
        
        if not all_fields_present:
            self.log_error("Some required fields are missing or invalid")
            return False
        
        self.log_success("All required metadata fields present and valid")
        
        # Verify the actual token value is NOT stored
        if 'token' in data:
            self.log_error("SECURITY ISSUE: The secret contains the actual token value! This should NOT happen.")
            return False
        else:
            self.log_success("Confirmed: Actual token value is NOT stored in the secret (as expected)")
        
        return True
    
    def cleanup(self):
        """Cleanup test resources"""
        cleanup_performed = False
        
        # Clean up token
        if os.getenv('CLEANUP_ON_EXIT', 'true').lower() == 'true' and self.maas_token:
            self.log_info("Cleaning up: Revoking test token...")
            try:
                requests.delete(
                    f"{self.base_url}/v1/tokens",
                    headers={"Authorization": f"Bearer {self.oc_token}"},
                    timeout=30
                )
                self.log_info("Token revoked successfully")
                cleanup_performed = True
            except Exception as e:
                self.log_warning(f"Token revocation failed (non-critical): {e}")
        
        # Clean up secret (default: yes, unless KEEP_SECRETS=true)
        if not self.keep_secrets and self.secret_name and self.secret_namespace:
            self.log_info("Cleaning up: Deleting test secret...")
            try:
                subprocess.run(
                    ['kubectl', 'delete', 'secret', self.secret_name,
                     '-n', self.secret_namespace],
                    capture_output=True,
                    check=True
                )
                self.log_info("Secret deleted successfully")
                cleanup_performed = True
            except subprocess.CalledProcessError as e:
                self.log_warning(f"Secret deletion failed (non-critical): {e}")
        elif self.keep_secrets and self.secret_name:
            self.log_info(f"Keeping secret {self.secret_name} for inspection (KEEP_SECRETS=true)")
        
        if cleanup_performed:
            self.log_info("Cleanup completed")
    
    def run(self) -> int:
        """Run all tests"""
        print("=" * 42)
        print("  Named Token Smoke Test (Python)")
        print("=" * 42)
        print()
        
        if not self.check_prerequisites():
            return 1
        print()
        
        # Run tests
        if not self.test_create_named_token():
            self.test_failed = True
        print()
        
        if not self.test_failed:
            if not self.test_token_works():
                self.test_failed = True
            print()
        
        if not self.test_invalid_token_401():
            self.test_failed = True
        print()
        
        if not self.test_failed:
            if not self.test_secret_metadata():
                self.test_failed = True
            print()
        
        # Cleanup
        self.cleanup()
        
        # Summary
        print("=" * 42)
        if not self.test_failed:
            self.log_success("All tests passed!")
            print("=" * 42)
            return 0
        else:
            self.log_error("Some tests failed!")
            print("=" * 42)
            return 1


if __name__ == '__main__':
    test = NamedTokenTest()
    try:
        sys.exit(test.run())
    except KeyboardInterrupt:
        print("\n\nTest interrupted by user")
        test.cleanup()
        sys.exit(1)
    except Exception as e:
        print(f"\n\nUnexpected error: {e}")
        test.cleanup()
        sys.exit(1)

