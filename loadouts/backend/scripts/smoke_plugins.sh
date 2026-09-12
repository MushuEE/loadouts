#!/bin/bash
# End-to-end smoke test of the plugin model against a running server.
#
# The through-line is that plugin authors are untrusted. Every section below is really a
# check that some specific thing an author might want to do is either mediated or refused:
# they cannot widen their own access, cannot read a loadout the viewer cannot, cannot get
# their secrets printed onto the page, and cannot execute anything on our origin.
set -euo pipefail
API=${API:-http://localhost:8080/api/v1}
BASE=${BASE:-http://localhost:8080}

jq() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }

GH=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='gearhead'][0]")
SP=$(curl -s "$API/profiles" | jq "[p['id'] for p in d if p['handle']=='trailsponsor'][0]")
LDT=$(curl -s "$API/loadouts" | jq "[l['loadout']['id'] for l in d if l['owner_handle']=='gearhead'][0]")
echo "gearhead=$GH sponsor=$SP loadout=$LDT"

echo
echo "=== 1. The seeded directory: two widget plugins and one embed ==="
curl -s "$API/plugins" | jq "'\n'.join('  %-18s v%d  owner=%-9s %s' % (p['plugin']['slug'],p['plugin']['latest_version'],p['plugin']['owner_type'],p['plugin']['description'][:44]) for p in d['plugins'])"

echo
echo "=== 2. A widget renders server-side: the frontend receives data, never an expression ==="
curl -s -H "X-Profile-ID: $GH" "$API/plugins/render?surface=loadout.panel&loadout_id=$LDT" \
  | jq "'\n'.join('\n'.join(
      ['  %s [%s]' % (v['title'],v['kind'])] +
      ['    %-13s %9s %5.1f%%' % (p['label'],p['display'],p['share']*100) for p in (v.get('widget') or {}).get('points',[])] +
      (['    ' + ' | '.join(c['label'] for c in (v.get('widget') or {}).get('columns',[]))] if (v.get('widget') or {}).get('columns') else []) +
      ['    ' + ' | '.join(c['display'] for c in r) for r in (v.get('widget') or {}).get('rows',[])]
    ) for v in d['views'])"

echo
echo "=== 3. A community plugin reads the community's own metadata layer ==="
echo "    (the host has no idea what an 'ul_score' is; the community defined it)"
curl -s -H "X-Profile-ID: $GH" "$API/plugins/render?surface=loadout.sidebar&loadout_id=$LDT" \
  | jq "'\n'.join('    %-16s %s' % (s['label']+':',s['display']) for v in d['views'] for s in (v.get('widget') or {}).get('stats',[]))"

echo
echo "=== 4. Publishing validates the manifest, so a broken plugin is a 400 not a broken page ==="
publish_bad() {
  echo -n "  $1 -> HTTP "
  curl -s -o /tmp/plg_err.json -w "%{http_code} " -X POST "$API/plugins" \
    -H "X-Profile-ID: $GH" -H 'Content-Type: application/json' -d "$2"
  python3 -c "import json;e=json.load(open('/tmp/plg_err.json')).get('error');print(e['message'][:72] if e else 'ACCEPTED')"
}
publish_bad "unparseable expression " '{"name":"Bad","manifest":{"api_version":1,"capabilities":{"read_loadout":true},"views":[{"id":"v","title":"V","surface":"loadout.panel","kind":"widget","widget":{"type":"stat_grid","source":"loadout.entries","stats":[{"label":"X","value":"sum(item.weight_g"}]}}]}}'
publish_bad "unknown function      " '{"name":"Bad","manifest":{"api_version":1,"capabilities":{"read_loadout":true},"views":[{"id":"v","title":"V","surface":"loadout.panel","kind":"widget","widget":{"type":"stat_grid","source":"loadout.entries","stats":[{"label":"X","value":"exfiltrate(item)"}]}}]}}'
publish_bad "nested aggregate      " '{"name":"Bad","manifest":{"api_version":1,"capabilities":{"read_loadout":true},"views":[{"id":"v","title":"V","surface":"loadout.panel","kind":"widget","widget":{"type":"stat_grid","source":"loadout.entries","stats":[{"label":"X","value":"sum(sum(item.weight_g))"}]}}]}}'
publish_bad "unknown surface       " '{"name":"Bad","manifest":{"api_version":1,"capabilities":{},"views":[{"id":"v","title":"V","surface":"admin.panel","kind":"widget","widget":{"type":"stat_grid","source":"loadout.entries","stats":[{"label":"X","value":"1"}]}}]}}'
publish_bad "embed with no html    " '{"name":"Bad","manifest":{"api_version":1,"capabilities":{},"views":[{"id":"v","title":"V","surface":"loadout.sidebar","kind":"embed"}]}}'

echo
echo "=== 5. Installs are a consent gate: a partial grant is refused ==="
PID=$(curl -s "$API/plugins" | jq "[p['plugin']['id'] for p in d['plugins'] if p['plugin']['slug']=='trip-route'][0]")
echo -n "  granting only loadout:read when storage+network are asked -> HTTP "
curl -s -o /tmp/plg_inst.json -w "%{http_code} " -X POST "$API/plugins/installs" \
  -H "X-Profile-ID: $SP" -H 'Content-Type: application/json' \
  -d "{\"plugin_id\":\"$PID\",\"scope_type\":\"profile\",\"scope_id\":\"$SP\",\"granted_caps\":[\"loadout:read\"],\"settings\":{\"api_key\":\"secret-key-12345\"}}"
python3 -c "import json;e=json.load(open('/tmp/plg_inst.json')).get('error');print(e['message'][:72] if e else 'ACCEPTED')"

echo -n "  granting everything asked for                              -> HTTP "
curl -s -o /tmp/plg_inst.json -w "%{http_code}\n" -X POST "$API/plugins/installs" \
  -H "X-Profile-ID: $SP" -H 'Content-Type: application/json' \
  -d "{\"plugin_id\":\"$PID\",\"scope_type\":\"profile\",\"scope_id\":\"$SP\",\"granted_caps\":[\"loadout:read\",\"storage:write\",\"network:maps.googleapis.com\"],\"settings\":{\"api_key\":\"secret-key-12345\"}}"

echo
echo "=== 6. Secrets are redacted on the way out, but reach the frame ==="
echo -n "  install listing shows api_key as: "
curl -s -H "X-Profile-ID: $SP" "$API/plugins/installs?scope_type=profile" \
  | jq "[i['install']['settings'].get('api_key') for i in d['installs'] if i['plugin']['slug']=='trip-route'][0]"

echo
echo "=== 7. Install listings are scoped and authorized ==="
echo -n "  another profile reading gearhead's installs -> HTTP "
curl -s -o /dev/null -w "%{http_code}\n" -H "X-Profile-ID: $SP" "$API/plugins/installs?scope_type=profile&scope_id=$GH"
echo -n "  anonymous, unscoped                         -> HTTP "
curl -s -o /dev/null -w "%{http_code}\n" "$API/plugins/installs"

echo
echo "=== 8. The embed frame: sandboxed, CSP-locked, and framed only by the app ==="
SIID=$(curl -s -H "X-Profile-ID: $SP" "$API/plugins/installs?scope_type=profile" | jq "[i['install']['id'] for i in d['installs'] if i['plugin']['slug']=='trip-route'][0]")
curl -s -D /tmp/plg_hdr.txt -o /tmp/plg_frame.html \
  "$BASE/sandbox/plugins/$PID/versions/1/views/route/frame?install=$SIID"
grep -i -E "^(content-security-policy|cache-control|x-content-type-options|x-frame-options):" /tmp/plg_hdr.txt \
  | sed 's/^/  /' | cut -c1-200
echo -n "  author HTML served verbatim: "
grep -c "maps.googleapis.com" /tmp/plg_frame.html | sed 's/$/ reference(s)/'

echo
echo "=== 9. The frame document holds no data of its own ==="
echo -n "  context inlined into the frame HTML: "
grep -q "secret-key-12345\|ldt_" /tmp/plg_frame.html && echo "YES (leak!)" || echo "no - it arrives by postMessage from the parent"

echo
echo "=== 10. Capabilities are default-deny, and the asymmetry is deliberate ==="
SLDT=$(curl -s "$API/loadouts" | jq "[l['loadout']['id'] for l in d if l['owner_handle']=='trailsponsor'][0]")
curl -s -H "X-Profile-ID: $SP" "$API/plugins/render?surface=loadout.sidebar&loadout_id=$SLDT" | python3 -c "
import sys,json
for v in json.load(sys.stdin)['views']:
    c = v.get('embed_context')
    if not c: continue
    print('    granted:            ', ', '.join(c['capabilities']))
    print('    loadout in context: ', 'loadout' in c, '(loadout:read was granted)')
    print('    items in context:   ', 'items' in c, '(items:read was never asked for)')
    print('    api key in context: ', c['settings'].get('api_key') == 'secret-key-12345',
          '(real, because only the isolated frame sees it)')
"
echo -n "    api key in the install listing: "
curl -s -H "X-Profile-ID: $SP" "$API/plugins/installs?scope_type=profile" \
  | jq "[i['install']['settings'].get('api_key') for i in d['installs'] if i['plugin']['slug']=='trip-route'][0]" \
  | sed 's/$/ (redacted, because a widget would print it onto the page)/'

echo
echo "=== 11. Rendering respects loadout visibility, so a plugin cannot be a read primitive ==="
PRIV=$(curl -s -X POST "$API/loadouts" -H "X-Profile-ID: $GH" -H 'Content-Type: application/json' \
  -d '{"name":"Private Kit","visibility":"private"}' | jq "d['loadout']['id']")
echo -n "  owner renders their private loadout      -> HTTP "
curl -s -o /dev/null -w "%{http_code}\n" -H "X-Profile-ID: $GH" "$API/plugins/render?surface=loadout.panel&loadout_id=$PRIV"
echo -n "  another profile renders the same loadout -> HTTP "
curl -s -o /dev/null -w "%{http_code}\n" -H "X-Profile-ID: $SP" "$API/plugins/render?surface=loadout.panel&loadout_id=$PRIV"

echo
echo "Done."
