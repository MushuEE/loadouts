# Local Testing Guide

This guide explains how to verify the Polyglot Inventory System locally.

## Prerequisites
- **Go 1.25+**
- **curl** (for API testing)
- **git**

## 1. Unit Testing
The core logic (Deep Merging, Schema Validation, and Bloom Filter) is covered by Go unit tests.

```bash
cd loadouts/backend
go test ./internal/core/...
```

## 2. API Integration Testing (End-to-End)
We use a `seed.sh` script to verify the full lifecycle:
1. Registering a Schema.
2. Creating a Global Item.
3. Applying User Overrides.
4. Fetching the Merged Polyglot Result.

### Running with In-Memory Store (Fastest)
The backend defaults to an in-memory store if no `DATABASE_URL` is provided.

```bash
# In one terminal, start the server
cd loadouts/backend
go run cmd/server/main.go

# In another terminal, run the seed script
cd loadouts/backend
bash scripts/seed.sh
```

## 3. Manual API Exploration
You can interact with the API directly using `curl`.

### Check Schema
```bash
curl http://localhost:8080/api/v1/schemas/combat_stats
```

### Fetch Item with Overrides
Ensure you pass the `X-User-ID` header to see the merged result:
```bash
curl -H "X-User-ID: user_999" http://localhost:8080/api/v1/items/iron_sword
```

## 4. Key Behaviors to Verify
- **Schema Validation**: Try adding metadata that violates the schema (e.g., a string for a number field). The API should return a `400` or `500` error with validation details.
- **Bloom Filter**: The server logs will show `Populated Bloom Filter...` on startup. If you try to recreate an item ID, the Bloom filter will trigger a fast-path check.
- **Deep Merge**: Verify that fields not overridden in the `UserMetadata` are still preserved from the `BaseMetadata`.
