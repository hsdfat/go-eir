#!/bin/bash
# Database Migration Script for EIR

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default database configuration
DATABASE_URL="${DATABASE_URL:-host=14.225.198.206 user=adong password=adong123 dbname=adongfoodv4 port=5432 sslmode=disable}"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${GREEN}EIR Database Migration Tool${NC}"
echo "=============================="
echo ""

# Build the migration tool
echo -e "${YELLOW}Building migration tool...${NC}"
cd "$PROJECT_ROOT"
mkdir -p bin
go build -o bin/migrate ./cmd/migrate

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build migration tool${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Migration tool built successfully${NC}"
echo ""

# Parse command line arguments
VERIFY=false
STATUS=false
CREATE_PARTITION=0

while [[ $# -gt 0 ]]; do
    case $1 in
        --verify)
            VERIFY=true
            shift
            ;;
        --status)
            STATUS=true
            shift
            ;;
        --create-partition)
            CREATE_PARTITION="$2"
            shift 2
            ;;
        --database-url)
            DATABASE_URL="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --verify              Verify schema after migration"
            echo "  --status              Show migration status"
            echo "  --create-partition YEAR   Create audit_log partitions for a specific year"
            echo "  --database-url URL    Database connection string"
            echo "  --help                Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  DATABASE_URL          Database connection string (default: configured in script)"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run migration
ARGS="-database-url=${DATABASE_URL}"

if [ "$STATUS" = true ]; then
    echo -e "${YELLOW}Getting migration status...${NC}"
    ARGS="$ARGS -status"
elif [ "$CREATE_PARTITION" -gt 0 ]; then
    echo -e "${YELLOW}Creating partitions for year $CREATE_PARTITION...${NC}"
    ARGS="$ARGS -create-partition=$CREATE_PARTITION"
else
    echo -e "${YELLOW}Running database migration...${NC}"
    if [ "$VERIFY" = true ]; then
        ARGS="$ARGS -verify"
    fi
fi

./bin/migrate $ARGS

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ Migration completed successfully!${NC}"
else
    echo ""
    echo -e "${RED}✗ Migration failed!${NC}"
    exit 1
fi
