#!/bin/bash
# End-to-end smoke test of the Day 0 object model against a running server.
set -euo pipefail
API=${API:-http://localhost:8080/api/v1}

jq() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }

GH=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='gearhead'][0]")
SP=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='trailsponsor'][0]")
echo "gearhead=$GH sponsor=$SP"

echo
echo "=== 1. Item, global layer only (anonymous) ==="
curl -s "$API/items/tent-copper-spur-ul2" | jq "json.dumps({'layers':d['applied_layers'],'metadata':d['metadata']},indent=1)"

echo
echo "=== 2. Same item in UL community context, as the owner (all 4 layers) ==="
curl -s -H "X-Profile-ID: $GH" "$API/items/tent-copper-spur-ul2?community=ul-backpacking" \
  | jq "json.dumps({'layers':d['applied_layers'],'metadata':d['metadata'],'provenance':d['provenance']},indent=1)"

echo
echo "=== 3. Same item viewed by ANOTHER profile: private notes must be absent ==="
curl -s -H "X-Profile-ID: $SP" "$API/items/tent-copper-spur-ul2?community=ul-backpacking&owner=$GH" \
  | jq "json.dumps({'layers':d['applied_layers'],'has_private_notes':'notes' in d['metadata']},indent=1)"

echo
echo "=== 4. Loadout detail: nested entries, stats, validation ==="
LDT=$(curl -s "$API/discover?q=PCT" | jq "d[0]['loadout']['id']")
curl -s -H "X-Profile-ID: $GH" "$API/loadouts/$LDT" | jq "json.dumps({
  'name': d['loadout']['name'],
  'template': d['template']['template']['name'] + ' v' + str(d['template']['version']['version']),
  'stats': d['stats'],
  'issues': d['issues'],
  'root_entries': [(e['entry']['slot_id'], e['item']['name'], len(e.get('children') or [])) for e in d['entries']],
}, indent=1)"

echo
echo "=== 5. Template versioning: publish v2, existing loadout stays on v1 ==="
TPL=$(curl -s -H "X-Profile-ID: $GH" "$API/loadouts/$LDT" | jq "d['template']['template']['id']")
curl -s -X POST -H "X-Profile-ID: $GH" -H 'Content-Type: application/json' \
  -d '{"changelog":"Add a bear canister slot","slots":[
        {"id":"shelter","name":"Shelter","accepted_categories":["shelter"],"required":true},
        {"id":"bear-can","name":"Bear Canister","accepted_categories":["organizer"],"required":true}]}' \
  "$API/templates/$TPL/versions" | jq "json.dumps({'latest':d['template']['latest_version'],'versions':d['versions']})"
curl -s -H "X-Profile-ID: $GH" "$API/loadouts/$LDT" | jq "'existing loadout still pinned to v' + str(d['loadout']['template_version']) + ', issues: ' + str(len(d['issues']))"

echo
echo "=== 6. Fork: sponsor forks the gearhead loadout ==="
FORK=$(curl -s -X POST -H "X-Profile-ID: $SP" "$API/loadouts/$LDT/fork")
echo "$FORK" | jq "json.dumps({'id':d['loadout']['id'],'name':d['loadout']['name'],'owner':d['owner']['handle'],'forked_from':d['loadout']['forked_from'],'entries':len(d['entries']),'stats':d['stats']},indent=1)"

echo
echo "=== 7. Private loadout is invisible to others ==="
FORK_ID=$(echo "$FORK" | jq "d['loadout']['id']")
echo -n "as owner: "; curl -s -o /dev/null -w "%{http_code}\n" -H "X-Profile-ID: $SP" "$API/loadouts/$FORK_ID"
echo -n "as other: "; curl -s -o /dev/null -w "%{http_code}\n" -H "X-Profile-ID: $GH" "$API/loadouts/$FORK_ID"
echo -n "anonymous: "; curl -s -o /dev/null -w "%{http_code}\n" "$API/loadouts/$FORK_ID"

echo
echo "=== 8. Community layer writes are admin-gated ==="
echo -n "non-admin write: "; curl -s -o /dev/null -w "%{http_code}\n" -X PUT -H "X-Profile-ID: $SP" \
  -H 'Content-Type: application/json' -d '{"metadata":{"ul_backpacking":{"ul_score":1}}}' \
  "$API/communities/ul-backpacking/items/tent-xmid-1/layer"
echo -n "admin write:     "; curl -s -o /dev/null -w "%{http_code}\n" -X PUT -H "X-Profile-ID: $GH" \
  -H 'Content-Type: application/json' -d '{"metadata":{"ul_backpacking":{"ul_score":9.2}}}' \
  "$API/communities/ul-backpacking/items/tent-xmid-1/layer"

echo
echo "=== 9. Communities and membership ==="
curl -s -H "X-Profile-ID: $GH" "$API/communities/ul-backpacking" | jq "json.dumps({'name':d['community']['name'],'members':d['member_count'],'my_role':d['viewer_role']})"
curl -s "$API/communities/ul-backpacking/members" | jq "[(m['profile']['handle'], m['membership']['role']) for m in d]"

echo
echo "All checks completed."
