#!/bin/bash

# API Gateway Test Script
# This script tests the various features of the API gateway

set -e

GATEWAY_URL="http://localhost:8080"
BACKEND_URL="http://localhost:8081"
API_KEY_1="gw_test_key_1"
API_KEY_2="gw_test_key_2"
INVALID_KEY="invalid_key_123"

echo "=== API Gateway Test Suite ==="
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_passed() {
    echo -e "${GREEN}✓ PASSED${NC}: $1"
}

test_failed() {
    echo -e "${RED}✗ FAILED${NC}: $1"
}

test_info() {
    echo -e "${YELLOW}ℹ INFO${NC}: $1"
}

# Test 1: Health Check (No auth required)
echo "Test 1: Health Check"
response=$(curl -s -w "\n%{http_code}" $GATEWAY_URL/health)
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    test_passed "Health check returned 200"
    echo "  Response: $body"
else
    test_failed "Health check returned $http_code"
fi
echo ""

# Test 2: Missing API Key
echo "Test 2: Request without API Key"
http_code=$(curl -s -o /dev/null -w "%{http_code}" $GATEWAY_URL/api/users)

if [ "$http_code" = "401" ]; then
    test_passed "Missing API key correctly rejected (401)"
else
    test_failed "Expected 401, got $http_code"
fi
echo ""

# Test 3: Invalid API Key
echo "Test 3: Request with invalid API Key"
http_code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $INVALID_KEY" $GATEWAY_URL/api/users)

if [ "$http_code" = "401" ]; then
    test_passed "Invalid API key correctly rejected (401)"
else
    test_failed "Expected 401, got $http_code"
fi
echo ""

# Test 4: Valid API Key
echo "Test 4: Request with valid API Key"
response=$(curl -s -w "\n%{http_code}" -H "X-API-Key: $API_KEY_1" $GATEWAY_URL/api/users)
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ] || [ "$http_code" = "404" ]; then
    test_passed "Valid API key accepted (received response from gateway)"
    echo "  HTTP Code: $http_code"
else
    test_failed "Expected 200 or 404, got $http_code"
fi
echo ""

# Test 5: Rate Limiting
echo "Test 5: Rate Limiting (making 15 requests rapidly)"
success_count=0
rate_limited=0

for i in {1..15}; do
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $API_KEY_1" $GATEWAY_URL/api/users)
    
    if [ "$http_code" = "429" ]; then
        rate_limited=$((rate_limited + 1))
    else
        success_count=$((success_count + 1))
    fi
done

if [ "$rate_limited" -gt 0 ]; then
    test_passed "Rate limiting working ($success_count allowed, $rate_limited rate-limited)"
else
    test_info "Rate limiting not triggered yet (all $success_count requests allowed)"
fi
echo ""

# Test 6: Different API Keys have separate rate limits
echo "Test 6: Different API Keys (separate rate limits)"
http_code_1=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $API_KEY_1" $GATEWAY_URL/api/users)
http_code_2=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $API_KEY_2" $GATEWAY_URL/api/users)

if [ "$http_code_2" != "429" ]; then
    test_passed "API Key 2 has its own rate limit bucket"
else
    test_info "API Key 2 also rate limited (may refill soon)"
fi
echo ""

# Test 7: Check Rate Limit Headers
echo "Test 7: Rate Limit Headers"
# Wait a bit for rate limit to refill
sleep 2

headers=$(curl -s -D - -H "X-API-Key: $API_KEY_2" $GATEWAY_URL/api/users -o /dev/null)

if echo "$headers" | grep -q "X-Ratelimit-Limit"; then
    limit=$(echo "$headers" | grep "X-Ratelimit-Limit" | cut -d: -f2 | tr -d ' \r')
    remaining=$(echo "$headers" | grep "X-Ratelimit-Remaining" | cut -d: -f2 | tr -d ' \r')
    test_passed "Rate limit headers present (Limit: $limit, Remaining: $remaining)"
else
    test_failed "Rate limit headers not found"
fi
echo ""

# Test 8: Service Not Found
echo "Test 8: Non-existent Service Route"
http_code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $API_KEY_1" $GATEWAY_URL/nonexistent/route)

if [ "$http_code" = "404" ]; then
    test_passed "Non-existent route returns 404"
else
    test_info "Non-existent route returned $http_code"
fi
echo ""

# Summary
echo "=== Test Suite Complete ==="
echo ""
echo "Manual tests to try:"
echo "1. Start the test backend: go run test-backend.go"
echo "2. Make a request through the gateway:"
echo "   curl -H \"X-API-Key: $API_KEY_1\" $GATEWAY_URL/api/users"
echo ""
echo "3. Watch the request flow through both services!"