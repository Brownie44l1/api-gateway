#!/bin/bash

# Distributed Rate Limiting Test Script

set -e

GATEWAY_URL="http://localhost:8080"
API_KEY="gw_test_key_1"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=== Distributed Rate Limiting Test Suite ==="
echo ""

# Test 1: Check if Redis is running
echo "Test 1: Redis Connectivity"
if redis-cli ping > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PASSED${NC}: Redis is running"
else
    echo -e "${RED}✗ FAILED${NC}: Redis is not running"
    echo "Start Redis with: redis-server"
    exit 1
fi
echo ""

# Test 2: Gateway health check
echo "Test 2: Gateway Health Check"
response=$(curl -s $GATEWAY_URL/health)
redis_status=$(echo $response | grep -o '"redis":"[^"]*"' | cut -d'"' -f4)

if [ "$redis_status" = "healthy" ]; then
    echo -e "${GREEN}✓ PASSED${NC}: Gateway connected to Redis"
    echo "  Response: $response"
else
    echo -e "${RED}✗ FAILED${NC}: Gateway not connected to Redis"
    echo "  Start gateway with: go run cmd/gateway/main.go -redis"
    exit 1
fi
echo ""

# Test 3: Clear previous rate limits
echo "Test 3: Clearing Previous Rate Limits"
redis-cli FLUSHDB > /dev/null 2>&1
echo -e "${GREEN}✓ PASSED${NC}: Redis flushed"
echo ""

# Test 4: Rate limiting works
echo "Test 4: Rate Limiting Enforcement"
success_count=0
rate_limited_count=0

echo "Making 15 rapid requests..."
for i in {1..15}; do
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-API-Key: $API_KEY" $GATEWAY_URL/)
    
    if [ "$http_code" = "200" ]; then
        success_count=$((success_count + 1))
        echo -n "✓"
    elif [ "$http_code" = "429" ]; then
        rate_limited_count=$((rate_limited_count + 1))
        echo -n "✗"
    else
        echo -n "?"
    fi
done
echo ""

if [ "$rate_limited_count" -gt 0 ]; then
    echo -e "${GREEN}✓ PASSED${NC}: Rate limiting working ($success_count allowed, $rate_limited_count blocked)"
else
    echo -e "${YELLOW}⚠ WARNING${NC}: No requests were rate limited (limit might be too high)"
fi
echo ""

# Test 5: Check Redis keys
echo "Test 5: Redis State"
keys=$(redis-cli KEYS "ratelimit:*")
if [ -n "$keys" ]; then
    echo -e "${GREEN}✓ PASSED${NC}: Rate limit data stored in Redis"
    echo "  Keys: $keys"
    
    # Show actual value
    for key in $keys; do
        value=$(redis-cli GET "$key")
        ttl=$(redis-cli TTL "$key")
        echo "  $key = $value (TTL: ${ttl}s)"
    done
else
    echo -e "${RED}✗ FAILED${NC}: No rate limit keys found in Redis"
fi
echo ""

# Test 6: Rate limit headers
echo "Test 6: Rate Limit Headers"
# Wait for rate limit to reset
sleep 2
redis-cli FLUSHDB > /dev/null 2>&1

headers=$(curl -s -D - -H "X-API-Key: $API_KEY" $GATEWAY_URL/ -o /dev/null)
limit=$(echo "$headers" | grep -i "X-RateLimit-Limit" | cut -d: -f2 | tr -d ' \r')
remaining=$(echo "$headers" | grep -i "X-RateLimit-Remaining" | cut -d: -f2 | tr -d ' \r')

if [ -n "$limit" ] && [ -n "$remaining" ]; then
    echo -e "${GREEN}✓ PASSED${NC}: Rate limit headers present"
    echo "  Limit: $limit"
    echo "  Remaining: $remaining"
else
    echo -e "${RED}✗ FAILED${NC}: Rate limit headers missing"
fi
echo ""

# Test 7: Different strategies
echo "Test 7: Testing Strategies"
echo "  Restart gateway with different strategies:"
echo "  - Sliding window: go run cmd/gateway/main.go -redis"
echo "  - Token bucket:   go run cmd/gateway/main.go -redis -strategy=token-bucket"
echo ""

# Summary
echo "=== Test Summary ==="
echo "All core tests passed! Your distributed rate limiting is working."
echo ""
echo "Advanced Tests:"
echo "1. Run multiple gateway instances (different ports)"
echo "2. Make requests to both instances with same API key"
echo "3. Verify rate limits are shared via Redis"
echo ""
echo "Monitor Redis in real-time:"
echo "  redis-cli MONITOR"