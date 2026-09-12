#!/bin/bash
# End-to-end smoke test of the retailer item import flow against a running server.
#
# This intentionally does NOT depend on reaching a real store: the interesting behaviour
# (URL canonicalization, dedupe, provenance, affiliate linking, loadout integration) all
# works whether or not the scrape succeeds. That is the point of the design.
set -euo pipefail
API=${API:-http://localhost:8080/api/v1}

jq() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }

GH=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='gearhead'][0]")
echo "gearhead=$GH"

echo
echo "=== 1. Supported retailers ==="
curl -s "$API/imports/suppliers" | jq "[s['name'] for s in d['suppliers']]"

echo
echo "=== 2. Preview an Amazon URL (ASIN extracted from a noisy share link) ==="
curl -s -X POST "$API/imports/preview" -H 'Content-Type: application/json' \
  -d '{"url":"https://www.amazon.com/Therm-Rest-UberLite/dp/B07P8ZQZ8Z/ref=sr_1_3?keywords=x&qid=170"}' \
  | jq "json.dumps({'status':d['status'],'supplier':d['supplier_name'],'product_id':d['draft']['target']['product_id'],'affiliate_url':d['affiliate_url'],'warning':d.get('warning','')[:60]},indent=1)"

echo
echo "=== 3. Non-product URLs are rejected with a useful message ==="
echo -n "search page: "; curl -s -o /tmp/imp_err.json -w "%{http_code} " -X POST "$API/imports/preview" \
  -H 'Content-Type: application/json' -d '{"url":"https://www.rei.com/search?q=tent"}'
python3 -c "import json;print(json.load(open('/tmp/imp_err.json'))['error']['message'])"
echo -n "not a url:   "; curl -s -o /dev/null -w "%{http_code}\n" -X POST "$API/imports/preview" \
  -H 'Content-Type: application/json' -d '{"url":"javascript:alert(1)"}'

echo
echo "=== 4. SSRF guard: internal addresses are refused outright ==="
for target in "http://169.254.169.254/latest/meta-data/" "http://localhost:8080/healthz" "http://10.0.0.1/admin" "http://vault.internal/secret"; do
  echo -n "  $target -> HTTP "
  curl -s -o /tmp/ssrf.json -w "%{http_code} " -X POST "$API/imports/preview" \
    -H 'Content-Type: application/json' -d "{\"url\":\"$target\"}"
  python3 -c "import json;print(json.load(open('/tmp/ssrf.json'))['error']['message'][:60])"
done

echo
echo "=== 5. Commit an import (manual-entry path) ==="
curl -s -X POST "$API/imports/commit" -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"url":"https://www.amazon.com/dp/B07P8ZQZ8Z","name":"Therm-a-Rest NeoAir UberLite","category":"sleep","brand":"Therm-a-Rest","weight_g":250,"cost_cents":18000}' \
  | jq "json.dumps({'created':d['created'],'id':d['item']['id'],'origin':d['item']['origin'],'verified':d['item']['verified'],'core':d['item']['base_metadata']['core'],'import':d['item']['base_metadata']['import']},indent=1)"

echo
echo "=== 6. Re-import the same product from a different URL shape: no duplicate ==="
echo -n "commit again -> HTTP "; curl -s -o /tmp/imp2.json -w "%{http_code}\n" -X POST "$API/imports/commit" \
  -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"url":"https://www.amazon.com/gp/product/B07P8ZQZ8Z?psc=1","name":"Some Other Name","category":"sleep"}'
python3 -c "import json;d=json.load(open('/tmp/imp2.json'));print('  created =',d['created'],'| id =',d['item']['id'])"
echo -n "preview now reports: "; curl -s -X POST "$API/imports/preview" -H 'Content-Type: application/json' \
  -d '{"url":"https://www.amazon.com/dp/B07P8ZQZ8Z"}' | jq "d['status'] + ' -> ' + d['existing_item']['id']"

echo
echo "=== 7. Imported item resolves through the layer system, with its affiliate source ==="
curl -s -H "X-Profile-ID: $GH" "$API/items/therm-a-rest-neoair-uberlite" \
  | jq "json.dumps({'name':d['name'],'origin':d.get('origin'),'verified':d['verified'],'weight_g':d['metadata']['core']['weight_g'],'sources':d['sources']},indent=1)"

echo
echo "=== 8. Importing requires an acting profile ==="
echo -n "anonymous commit -> HTTP "; curl -s -o /dev/null -w "%{http_code}\n" -X POST "$API/imports/commit" \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.rei.com/product/999/x","name":"Nope","category":"pack"}'

echo
echo "=== 9. The imported item works in a loadout and moves the stats ==="
LDT=$(curl -s -X POST "$API/loadouts" -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"name":"Import smoke test"}' | jq "d['loadout']['id']")
curl -s -X PUT "$API/loadouts/$LDT/entries" -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"entries":[{"item_id":"therm-a-rest-neoair-uberlite","quantity":2}]}' \
  | jq "json.dumps({'stats':d['stats'],'issues':d['issues']},indent=1)"
curl -s -X DELETE "$API/loadouts/$LDT" -H "X-Profile-ID: $GH" -o /dev/null

echo
echo "=== 10. An unknown store still imports (generic supplier, plain link) ==="
curl -s -X POST "$API/imports/commit" -H 'Content-Type: application/json' -H "X-Profile-ID: $GH" \
  -d '{"url":"https://shop.example.com/products/cottage-widget","name":"Cottage Widget","category":"pack","weight_g":80,"cost_cents":4200}' \
  | jq "json.dumps({'created':d['created'],'id':d['item']['id'],'supplier':d['item']['base_metadata']['import']['supplier']},indent=1)"
curl -s "$API/items/cottage-widget" | jq "json.dumps(d['sources'],indent=1)"

echo
echo "All import checks completed."
