"""Exercise the deployed, disposable sample without printing credentials."""
import json
import os
import sys
import time
import urllib.error
import urllib.request

base = os.environ.get('API_URL', 'http://127.0.0.1:8080')
admin = os.environ['GONEX_ADMIN_TOKEN']
reader = os.environ['GONEX_READER_TOKEN']

def request(method, path, token='', body=None, expected=200):
    data = None if body is None else json.dumps(body).encode()
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
    try:
        response = urllib.request.urlopen(req, timeout=4)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        raw = response.read()
        assert response.status == expected, (method, path, response.status, expected)
        assert admin.encode() not in raw and reader.encode() not in raw
        assert response.headers.get('X-Request-ID')
        return json.loads(raw) if raw else None

for attempt in range(60):
    try:
        request('GET', '/readyz')
        break
    except (OSError, AssertionError):
        if attempt == 59:
            raise
        time.sleep(1)
if '--check-recovery' in sys.argv:
    existing = request('GET', '/api/orders?limit=50&status=pending', reader)['items']
    assert any(item['item'] == 'Recovery probe' for item in existing), 'persisted order missing after recovery'
request('GET', '/api/orders', expected=401)
request('POST', '/api/orders', reader, {'item': 'Notebook', 'quantity': 2}, 403)
request('POST', '/api/orders', admin, {'item': 'A', 'quantity': 0}, 422)
created = request('POST', '/api/orders', admin, {'item': 'Notebook', 'quantity': 2}, 201)
order_id = created['id']
request('GET', '/api/orders/' + order_id, reader)
items = request('GET', '/api/orders?limit=1&status=pending', reader)['items']
assert len(items) == 1
request('PUT', '/api/orders/' + order_id, admin, {'item': 'Notebook', 'quantity': 3, 'status': 'completed'})
request('DELETE', '/api/orders/' + order_id, admin, expected=204)
request('GET', '/api/orders/' + order_id, reader, expected=404)
request('GET', '/openapi.json')
# A retained row proves persistence across the separate outage/restore drill.
request('POST', '/api/orders', admin, {'item': 'Recovery probe', 'quantity': 1}, 201)
print('PASS: deployed authentication, authorization, validation, CRUD, filtering, pagination and OpenAPI')
