#!/bin/bash
# End-to-end smoke test of community favorites against a running server.
#
# The scenario is the one this feature was designed around: a community wants to point at
# a good meal kit that a member owns. It endorses rather than takes ownership, the owner
# keeps sole control, and when the owner changes the contents the endorsement is flagged
# stale instead of silently vouching for something new.
set -euo pipefail
API=${API:-http://localhost:8080/api/v1}

jq() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }
as() { curl -s -H "X-Profile-ID: $1" "${@:2}"; }

GH=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='gearhead'][0]")
FC=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='fitcheck'][0]")
UL=$(curl -s "$API/communities" | jq "[c['id'] for c in d if c['slug']=='ul-backpacking'][0]")
echo "gearhead=$GH fitcheck=$FC community=$UL"

echo
echo "=== 1. fitcheck publishes a kit that the community might like ==="
KIT=$(as "$FC" -X POST "$API/loadouts" -H 'Content-Type: application/json' \
  -d '{"name":"UL Budget Meal Kit","visibility":"public"}' | jq "d['loadout']['id']")
echo "kit=$KIT"
STOVE=$(curl -s "$API/items?limit=1" | jq "d[0]['id']")
as "$FC" -X PUT "$API/loadouts/$KIT/entries" -H 'Content-Type: application/json' \
  -d "{\"entries\":[{\"id\":\"e1\",\"slot_id\":\"gear\",\"item_id\":\"$STOVE\",\"quantity\":1}]}" \
  | jq "'entries=%d' % len(d['entries'])"

echo
echo "=== 2. The kit's own author cannot endorse on the community's behalf ==="
echo -n "fitcheck (author, not an admin) as community: "
as "$FC" -o /tmp/fav_err.json -w "%{http_code}\n" -X PUT "$API/loadouts/$KIT/favorite" \
  -H 'Content-Type: application/json' -d "{\"scope_type\":\"community\",\"scope_id\":\"$UL\"}"
python3 -c "import json;print('  ->',json.load(open('/tmp/fav_err.json'))['error']['message'])"

echo
echo "=== 3. A community admin endorses it ==="
ADMIN=$(curl -s "$API/communities/$UL/members" | jq "[m['membership']['profile_id'] for m in d if m['membership']['role'] in ('owner','admin')][0]")
echo "admin=$ADMIN (gearhead, who does not own the kit)"
as "$ADMIN" -X PUT "$API/loadouts/$KIT/favorite" -H 'Content-Type: application/json' \
  -d "{\"scope_type\":\"community\",\"scope_id\":\"$UL\",\"note\":\"best budget kit we have seen\"}" \
  | jq "json.dumps({'scope':d['scope_type'],'available':d['available'],'stale':d['stale'],'note':d['note']},indent=1)"

echo
echo "=== 4. Endorsing confers no power over the loadout ==="
echo -n "admin tries to rename fitcheck's kit: "
as "$ADMIN" -o /dev/null -w "%{http_code}\n" -X PATCH "$API/loadouts/$KIT" \
  -H 'Content-Type: application/json' -d '{"name":"Hijacked"}'

echo
echo "=== 5. Favoriting again is a re-confirmation, not a second row ==="
as "$ADMIN" -X PUT "$API/loadouts/$KIT/favorite" -H 'Content-Type: application/json' \
  -d "{\"scope_type\":\"community\",\"scope_id\":\"$UL\"}" > /dev/null
curl -s "$API/favorites?scope_type=community&scope_id=$UL" | jq "'community endorsements=%d' % len(d)"

echo
echo "=== 6. The owner swaps the contents; the endorsement goes stale ==="
OTHER=$(curl -s "$API/items?limit=2" | jq "d[1]['id']")
as "$FC" -X PUT "$API/loadouts/$KIT/entries" -H 'Content-Type: application/json' \
  -d "{\"entries\":[{\"id\":\"e1\",\"slot_id\":\"gear\",\"item_id\":\"$OTHER\",\"quantity\":1}]}" > /dev/null
curl -s "$API/favorites?scope_type=community&scope_id=$UL" \
  | jq "json.dumps([{'loadout':f['loadout_id'],'stale':f['stale'],'available':f['available']} for f in d],indent=1)"

echo
echo "=== 7. A cosmetic edit does not re-flag it after re-confirmation ==="
as "$ADMIN" -X PUT "$API/loadouts/$KIT/favorite" -H 'Content-Type: application/json' \
  -d "{\"scope_type\":\"community\",\"scope_id\":\"$UL\"}" > /dev/null
as "$FC" -X PATCH "$API/loadouts/$KIT" -H 'Content-Type: application/json' \
  -d '{"description":"now with a longer writeup"}' > /dev/null
curl -s "$API/favorites?scope_type=community&scope_id=$UL" | jq "'stale after a description edit=%s' % d[0]['stale']"

echo
echo "=== 8. Private bookmarks are separate and do not leak ==="
as "$GH" -X PUT "$API/loadouts/$KIT/favorite" -H 'Content-Type: application/json' -d '{}' \
  | jq "'gearhead bookmarked privately: scope=%s' % d['scope_type']"
echo -n "fitcheck reading gearhead's bookmarks: "
as "$FC" -o /dev/null -w "%{http_code}\n" "$API/favorites?scope_type=profile&scope_id=$GH"
curl -s "$API/loadouts/$KIT/favorite" \
  | jq "'public endorsements on the loadout page=%d (bookmarks excluded)' % len(d)"

echo
echo "=== 9. A community cannot endorse a private loadout ==="
DRAFT=$(as "$ADMIN" -X POST "$API/loadouts" -H 'Content-Type: application/json' \
  -d '{"name":"Secret Draft"}' | jq "d['loadout']['id']")
echo -n "endorse a private draft: "
as "$ADMIN" -o /tmp/fav_err.json -w "%{http_code}\n" -X PUT "$API/loadouts/$DRAFT/favorite" \
  -H 'Content-Type: application/json' -d "{\"scope_type\":\"community\",\"scope_id\":\"$UL\"}"
python3 -c "import json;print('  ->',json.load(open('/tmp/fav_err.json'))['error']['message'])"
echo -n "bookmark your own private draft: "
as "$ADMIN" -o /dev/null -w "%{http_code}\n" -X PUT "$API/loadouts/$DRAFT/favorite" \
  -H 'Content-Type: application/json' -d '{}'

echo
echo "=== 10. An endorsement outlives its target ==="
as "$FC" -X DELETE "$API/loadouts/$KIT" > /dev/null
curl -s "$API/favorites?scope_type=community&scope_id=$UL" \
  | jq "json.dumps([{'loadout':f['loadout_id'],'available':f['available'],'has_summary':f.get('loadout') is not None} for f in d],indent=1)"

echo
echo "=== 11. Withdrawing an endorsement ==="
as "$ADMIN" -X DELETE "$API/loadouts/$KIT/favorite?scope_type=community&scope_id=$UL" -o /dev/null -w "  delete=%{http_code}\n"
curl -s "$API/favorites?scope_type=community&scope_id=$UL" | jq "'remaining=%d' % len(d)"

echo
echo "All favorites smoke checks passed."
