# Request for Kuadrant Policy Access

**Subject: Request for Kuadrant Policy Access - Need to Read Real Policies**

Hi [Admin Name],

Thanks to [Colleague Name], I now understand exactly what permissions I need. The real Kuadrant policies exist in the `llm` namespace and use `tokenratelimitpolicies`.

**What I found:**
- AuthPolicies exist: `oc get authpolicies -A` shows `gateway-auth-policy` in `llm` namespace
- TokenRateLimitPolicies exist: `oc get tokenratelimitpolicy gateway-token-rate-limits -n llm`
- My current permissions don't allow access to read these policies

**What I need:**
Read access to view the actual Kuadrant policies for dashboard integration.

**Specific commands needed:**
```bash
# Grant read access to llm namespace for Kuadrant policies
oc policy add-role-to-user view noyitz -n llm

# Grant read access to platform-services and kuadrant-system if policies exist there too
oc policy add-role-to-user view noyitz -n platform-services  
oc policy add-role-to-user view noyitz -n kuadrant-system
```

**Alternative (more specific permissions):**
If you prefer more restrictive access, you can create a custom role:
```bash
# Create a role that only allows reading Kuadrant policies
oc create role kuadrant-policy-reader \
  --verb=get,list,watch \
  --resource=authpolicies,tokenratelimitpolicies,ratelimitpolicies \
  -n llm

# Bind the role to my user
oc create rolebinding noyitz-kuadrant-reader \
  --role=kuadrant-policy-reader \
  --user=noyitz \
  -n llm
```

**Context:**
- My username: `noyitz`
- I need read-only access for monitoring/dashboard purposes
- The backend tries to fetch real policies but falls back to mock data due to permissions
- Authentication and rate limiting work perfectly - I just need policy visibility

Once this access is granted, my dashboard will automatically display the real policy configurations instead of mock data.

Thanks!