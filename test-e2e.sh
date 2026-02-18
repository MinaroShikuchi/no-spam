#!/bin/bash
set -e

# E2E Test Script for no-spam
# Tests admin creating publisher, topic, and publishing

BASE_URL="https://localhost:8443"
CURL="curl -k -s"  # -k to ignore self-signed cert, -s for silent

echo "=== E2E Test: Admin Creates Publisher and Topic ==="
echo ""

# Step 1: Login as admin
echo "Step 1: Login as admin..."
ADMIN_RESPONSE=$($CURL -X POST "$BASE_URL/admin/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "REPLACE_WITH_ADMIN_PASSWORD"}')

ADMIN_TOKEN=$(echo $ADMIN_RESPONSE | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$ADMIN_TOKEN" ]; then
  echo "❌ Failed to login as admin"
  echo "Response: $ADMIN_RESPONSE"
  exit 1
fi
echo "✅ Admin logged in successfully"
echo ""

# Step 2: Create a publisher user
echo "Step 2: Create publisher user 'test-publisher'..."
CREATE_USER_RESPONSE=$($CURL -X POST "$BASE_URL/admin/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"username": "test-publisher", "password": "test123", "role": "publisher"}')

if echo "$CREATE_USER_RESPONSE" | grep -q "User created"; then
  echo "✅ Publisher user created"
elif echo "$CREATE_USER_RESPONSE" | grep -q "already exists"; then
  echo "⚠️  Publisher user already exists (using existing)"
else
  echo "❌ Failed to create publisher user"
  echo "Response: $CREATE_USER_RESPONSE"
  exit 1
fi
echo ""

# Step 3: Get token for the publisher
echo "Step 3: Get token for publisher..."
PUBLISHER_TOKEN_RESPONSE=$($CURL -X GET "$BASE_URL/admin/token?username=test-publisher" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

PUBLISHER_TOKEN=$(echo $PUBLISHER_TOKEN_RESPONSE | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$PUBLISHER_TOKEN" ]; then
  echo "❌ Failed to get publisher token"
  echo "Response: $PUBLISHER_TOKEN_RESPONSE"
  exit 1
fi
echo "✅ Publisher token retrieved"
echo ""

# Step 4: Try to publish to non-existent topic (should fail)
echo "Step 4: Try to publish to 'test-topic' (should fail)..."
PUBLISH_FAIL_RESPONSE=$($CURL -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/send" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PUBLISHER_TOKEN" \
  -d '{"topic": "test-topic", "payload": {"message": "Hello World"}}')

HTTP_STATUS=$(echo "$PUBLISH_FAIL_RESPONSE" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

if [ "$HTTP_STATUS" = "404" ]; then
  echo "✅ Publish correctly failed with 404 (topic not found)"
else
  echo "❌ Publish should have failed with 404, got: $HTTP_STATUS"
  echo "Response: $PUBLISH_FAIL_RESPONSE"
  exit 1
fi
echo ""

# Step 5: Create the topic
echo "Step 5: Create topic 'test-topic'..."
CREATE_TOPIC_RESPONSE=$($CURL -X POST "$BASE_URL/admin/topics" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"name": "test-topic"}')

if echo "$CREATE_TOPIC_RESPONSE" | grep -q "Topic created"; then
  echo "✅ Topic created"
elif echo "$CREATE_TOPIC_RESPONSE" | grep -q "already exists"; then
  echo "⚠️  Topic already exists (using existing)"
else
  echo "❌ Failed to create topic"
  echo "Response: $CREATE_TOPIC_RESPONSE"
  exit 1
fi
echo ""

# Step 6: Publish to the topic (should succeed)
echo "Step 6: Publish to 'test-topic' (should succeed)..."
PUBLISH_SUCCESS_RESPONSE=$($CURL -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/send" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $PUBLISHER_TOKEN" \
  -d '{"topic": "test-topic", "payload": {"message": "Hello World"}}')

HTTP_STATUS=$(echo "$PUBLISH_SUCCESS_RESPONSE" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

if [ "$HTTP_STATUS" = "200" ]; then
  echo "✅ Publish succeeded"
else
  echo "❌ Publish should have succeeded, got: $HTTP_STATUS"
  echo "Response: $PUBLISH_SUCCESS_RESPONSE"
  exit 1
fi
echo ""

# Step 7: Verify message in topic
echo "Step 7: Verify message in topic..."
MESSAGES_RESPONSE=$($CURL -X GET "$BASE_URL/admin/topics/test-topic/messages" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

if echo "$MESSAGES_RESPONSE" | grep -q "Hello World"; then
  echo "✅ Message found in topic"
else
  echo "❌ Message not found in topic"
  echo "Response: $MESSAGES_RESPONSE"
  exit 1
fi
echo ""

echo "=== ✅ All E2E Tests Passed ==="
