#!/bin/bash

# EIR Service HTTP/2 API Test Cases
# This script demonstrates how to test the EIR service using curl

# Configuration
HTTP_HOST="localhost"
HTTP_PORT="8082"
BASE_URL="http://${HTTP_HOST}:${HTTP_PORT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper function to print section headers
print_section() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}$1${NC}"
    echo -e "${YELLOW}========================================${NC}\n"
}

# Helper function to print test results
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ PASSED${NC}\n"
    else
        echo -e "${RED}✗ FAILED${NC}\n"
    fi
}

# 1. Health Check
print_section "1. Health Check"
echo "GET ${BASE_URL}/health"
curl -v "${BASE_URL}/health" 2>&1 | grep -E "(< HTTP|\"status\")"
print_result $?

# 2. Check IMEI - Valid IMEI (should return white/grey/black list status)
print_section "2. Check IMEI - Valid IMEI"
IMEI="123456789012345"
echo "GET ${BASE_URL}/api/v1/check-imei/${IMEI}"
curl "${BASE_URL}/api/v1/check-imei/${IMEI}" | jq '.'
print_result $?

# 3. Check IMEI - Invalid IMEI (too short)
print_section "3. Check IMEI - Invalid IMEI (too short)"
echo "GET ${BASE_URL}/api/v1/check-imei/12345"
curl "${BASE_URL}/api/v1/check-imei/12345" | jq '.'
print_result $?

# 4. Insert TAC Range - Blacklist
print_section "4. Insert TAC Range - Blacklist"
echo "POST ${BASE_URL}/api/v1/insert-tac"
echo "Request Body:"
cat <<EOF | tee /dev/tty | curl -X POST "${BASE_URL}/api/v1/insert-tac" \
    -H "Content-Type: application/json" \
    -d @- | jq '.'
{
    "startTac": "35000000",
    "endTac": "35999999",
    "color": "black"
}
EOF
print_result $?

# 5. Insert TAC Range - Whitelist
print_section "5. Insert TAC Range - Whitelist"
echo "POST ${BASE_URL}/api/v1/insert-tac"
echo "Request Body:"
cat <<EOF | tee /dev/tty | curl -X POST "${BASE_URL}/api/v1/insert-tac" \
    -H "Content-Type: application/json" \
    -d @- | jq '.'
{
    "startTac": "86000000",
    "endTac": "86999999",
    "color": "white"
}
EOF
print_result $?

# 6. Insert TAC Range - Greylist
print_section "6. Insert TAC Range - Greylist"
echo "POST ${BASE_URL}/api/v1/insert-tac"
echo "Request Body:"
cat <<EOF | tee /dev/tty | curl -X POST "${BASE_URL}/api/v1/insert-tac" \
    -H "Content-Type: application/json" \
    -d @- | jq '.'
{
    "startTac": "52000000",
    "endTac": "52999999",
    "color": "grey"
}
EOF
print_result $?

# 7. Check TAC - Blacklisted
print_section "7. Check TAC - Blacklisted TAC"
echo "GET ${BASE_URL}/api/v1/check-tac/35123456"
curl "${BASE_URL}/api/v1/check-tac/35123456" | jq '.'
print_result $?

# 8. Check TAC - Whitelisted
print_section "8. Check TAC - Whitelisted TAC"
echo "GET ${BASE_URL}/api/v1/check-tac/86123456"
curl "${BASE_URL}/api/v1/check-tac/86123456" | jq '.'
print_result $?

# 9. Insert IMEI - Specific Device
print_section "9. Insert IMEI - Specific Device"
echo "POST ${BASE_URL}/api/v1/insert-imei"
echo "Request Body:"
cat <<EOF | tee /dev/tty | curl -X POST "${BASE_URL}/api/v1/insert-imei" \
    -H "Content-Type: application/json" \
    -d @- | jq '.'
{
    "imei": "123456789012345",
    "color": "grey"
}
EOF
print_result $?

# 10. Check Previously Inserted IMEI
print_section "10. Check Previously Inserted IMEI"
echo "GET ${BASE_URL}/api/v1/check-imei/123456789012345"
curl "${BASE_URL}/api/v1/check-imei/123456789012345" | jq '.'
print_result $?

# 11. List All Equipment
print_section "11. List All Equipment"
echo "GET ${BASE_URL}/api/v1/equipment"
curl "${BASE_URL}/api/v1/equipment" | jq '.'
print_result $?

# 12. Get Equipment by IMEI
print_section "12. Get Equipment by IMEI"
echo "GET ${BASE_URL}/api/v1/equipment/123456789012345"
curl "${BASE_URL}/api/v1/equipment/123456789012345" | jq '.'
print_result $?

# 13. Delete Equipment by IMEI
print_section "13. Delete Equipment by IMEI"
echo "DELETE ${BASE_URL}/api/v1/equipment/123456789012345"
curl -X DELETE "${BASE_URL}/api/v1/equipment/123456789012345" | jq '.'
print_result $?

# 14. Equipment Not Found
print_section "14. Equipment Not Found"
echo "GET ${BASE_URL}/api/v1/equipment/999999999999999"
curl "${BASE_URL}/api/v1/equipment/999999999999999" | jq '.'
print_result $?

# 15. Test HTTP/2 Connection (requires curl with HTTP/2 support)
print_section "15. Test HTTP/2 Connection"
echo "Testing HTTP/2 support..."
curl -I --http2 "${BASE_URL}/health" 2>&1 | grep -E "(HTTP/2|< HTTP)"
print_result $?

echo -e "\n${GREEN}All test cases completed!${NC}\n"
