#!/bin/bash

API_URL="http://localhost:8080/api/v1"
USER_ID="user_999"

echo "1. Registering Combat Schema..."
curl -X POST "$API_URL/schemas" \
     -H "Content-Type: application/json" \
     -d '{
       "id": "combat_stats",
       "version": "v1",
       "name": "Combat Stats",
       "author": "admin",
       "definition": {
         "type": "object",
         "properties": {
           "damage": {"type": "number", "minimum": 0},
           "speed": {"type": "number", "minimum": 0}
         },
         "required": ["damage"]
       }
     }'
echo -e "\n"

echo "2. Creating Global Item (Iron Sword)..."
curl -X POST "$API_URL/items" \
     -H "Content-Type: application/json" \
     -d '{
       "id": "iron_sword",
       "name": "Iron Sword",
       "base_metadata": {
         "combat_stats": {
           "damage": 10,
           "speed": 1.2
         }
       }
     }'
echo -e "\n"

echo "3. Adding User Overrides (Sharpened Sword + Personal Note)..."
curl -X POST "$API_URL/items/iron_sword/metadata" \
     -H "Content-Type: application/json" \
     -H "X-User-ID: $USER_ID" \
     -d '{
       "overrides": {
         "combat_stats": {
           "damage": 15
         }
       },
       "open_data": {
         "custom_name": "Dragon Slayer",
         "favorite": true
       }
     }'
echo -e "\n"

echo "4. Fetching Merged Result..."
curl -X GET "$API_URL/items/iron_sword" \
     -H "X-User-ID: $USER_ID"
echo -e "\n"

echo "5. Searching for Items..."
curl -X GET "$API_URL/items?q=sword"
echo -e "\n"
