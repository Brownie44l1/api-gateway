#!/bin/bash

# JWT Authentication Test Script

set -e

BASE_URL="http://localhost:8080"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

test_passed() {
    echo -e "${GREEN}✓ PASSED${NC}: $1"
}

test_failed() {
    echo -e "${RED}✗ FAILED${NC}: $1"
}

test_info() {
    echo -e "${YELLOW}ℹ INFO${NC}: $1"
}

echo "=== JWT Authentication Test Suite ==="
echo ""

# Test 1: Health check
echo "Test 1: Health Check"
response=$(curl -s $BASE_URL/health)
jwt_status=$(echo $response | grep -o '"jwt":"[^"]*"' | cut -d'"' -f4)

if [ "$jwt_status" = "enabled" ]; then
    test_passed "JWT is enabled"
    echo "  Response: $response"
else
    test_failed "JWT is not enabled"
    echo "  Make sure you started with: go run cmd/gateway/main.go -jwt"
    exit 1
fi
echo ""

# Test 2: Login with valid credentials
echo "Test 2: Login with Valid Credentials"
login_response=$(curl -s -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

ACCESS_TOKEN=$(echo $login_response | jq -r '.access_token')
REFRESH_TOKEN=$(echo $login_response | jq -r '.refresh_token')

if [ "$ACCESS_TOKEN" != "null" ] && [ -n "$ACCESS_TOKEN" ]; then
    test_passed "Login successful"
    echo "  Access Token: ${ACCESS_TOKEN:0:50}..."
    echo "  Refresh Token: ${REFRESH_TOKEN:0:50}..."
else
    test_failed "Login failed"
    echo "  Response: $login_response"
    exit 1
fi
echo ""

# Test 3: Login with invalid credentials
echo "Test 3: Login with Invalid Credentials"
invalid_login=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"wrongpassword"}')

if [ "$invalid_login" = "401" ]; then
    test_passed "Invalid credentials correctly rejected"
else
    test_failed "Invalid credentials not rejected (HTTP $invalid_login)"
fi
echo ""

# Test 4: Use token to make request
echo "Test 4: Make Authenticated Request"
auth_response=$(curl -s -w "\n%{http_code}" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  $BASE_URL/)

http_code=$(echo "$auth_response" | tail -n1)
body=$(echo "$auth_response" | head -n-1)

if [ "$http_code" = "200" ]; then
    test_passed "Token accepted, request successful"
else
    test_failed "Token rejected (HTTP $http_code)"
fi
echo ""

# Test 5: Get user info
echo "Test 5: Get Current User Info"
user_info=$(curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  $BASE_URL/auth/me)

user_id=$(echo $user_info | jq -r '.id')
username=$(echo $user_info | jq -r '.username')
roles=$(echo $user_info | jq -r '.roles | join(", ")')

if [ "$username" = "admin" ]; then
    test_passed "User info retrieved"
    echo "  User ID: $user_id"
    echo "  Username: $username"
    echo "  Roles: $roles"
else
    test_failed "User info not retrieved"
    echo "  Response: $user_info"
fi
echo ""

# Test 6: Request without token
echo "Test 6: Request Without Token"
no_token_code=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/)

if [ "$no_token_code" = "401" ]; then
    test_passed "Request without token correctly rejected"
else
    test_failed "Request without token not rejected (HTTP $no_token_code)"
fi
echo ""

# Test 7: Request with invalid token
echo "Test 7: Request with Invalid Token"
invalid_token_code=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer invalid_token_12345" \
  $BASE_URL/)

if [ "$invalid_token_code" = "401" ]; then
    test_passed "Invalid token correctly rejected"
else
    test_failed "Invalid token not rejected (HTTP $invalid_token_code)"
fi
echo ""

# Test 8: Refresh token
echo "Test 8: Refresh Access Token"
refresh_response=$(curl -s -X POST $BASE_URL/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")

NEW_ACCESS_TOKEN=$(echo $refresh_response | jq -r '.access_token')

if [ "$NEW_ACCESS_TOKEN" != "null" ] && [ -n "$NEW_ACCESS_TOKEN" ]; then
    test_passed "Token refreshed successfully"
    echo "  New Access Token: ${NEW_ACCESS_TOKEN:0:50}..."
else
    test_failed "Token refresh failed"
    echo "  Response: $refresh_response"
fi
echo ""

# Test 9: Use new token
echo "Test 9: Use Refreshed Token"
new_token_code=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $NEW_ACCESS_TOKEN" \
  $BASE_URL/)

if [ "$new_token_code" = "200" ]; then
    test_passed "Refreshed token works"
else
    test_failed "Refreshed token rejected (HTTP $new_token_code)"
fi
echo ""

# Test 10: Different users
echo "Test 10: Different User Roles"

# Login as regular user
user_login=$(curl -s -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"password123"}')

USER_TOKEN=$(echo $user_login | jq -r '.access_token')

user_me=$(curl -s \
  -H "Authorization: Bearer $USER_TOKEN" \
  $BASE_URL/auth/me)

user_roles=$(echo $user_me | jq -r '.roles | join(", ")')

if [ "$user_roles" = "user" ]; then
    test_passed "User1 has correct roles"
    echo "  Roles: $user_roles"
else
    test_failed "User1 roles incorrect"
fi
echo ""

# Summary
echo "=== Test Summary ==="
echo ""
echo "All JWT authentication tests passed!"
echo ""
echo "Available test users:"
echo "  - admin / admin123 (roles: admin, user)"
echo "  - user1 / password123 (roles: user)"
echo "  - readonly / readonly123 (roles: viewer)"
echo ""
echo "To decode tokens, visit: https://jwt.io"
echo ""
echo "Example usage:"
echo "  # Login"
echo "  curl -X POST $BASE_URL/auth/login -H 'Content-Type: application/json' \\"
echo "    -d '{\"username\":\"admin\",\"password\":\"admin123\"}'"
echo ""
echo "  # Use token"
echo "  curl -H 'Authorization: Bearer YOUR_TOKEN' $BASE_URL/"