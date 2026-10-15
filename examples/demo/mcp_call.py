#!/usr/bin/env python3
"""Minimal MCP client over Streamable HTTP, standard library only.

Used by run-demo.sh to show what an AI agent sees. Not a general client.

  mcp_call.py --list                       list the tools the server exposes
  mcp_call.py uptime '{"server":"demo-web"}'   call one tool and print its JSON
"""
import json
import os
import sys
import urllib.request

URL = os.environ.get('MCP_URL', 'http://127.0.0.1:8080/mcp')
TOKEN = os.environ.get('MCP_HTTP_TOKEN', 'demo-token-not-a-secret')


def _post(body, session=None):
    headers = {
        'Content-Type': 'application/json',
        'Accept': 'application/json, text/event-stream',
        'Authorization': 'Bearer ' + TOKEN,
    }
    if session:
        headers['Mcp-Session-Id'] = session
    req = urllib.request.Request(URL, json.dumps(body).encode(), headers)
    with urllib.request.urlopen(req, timeout=30) as resp:
        raw = resp.read().decode()
        session_id = resp.headers.get('Mcp-Session-Id') or session
    return _parse(raw), session_id


def _parse(raw):
    """Return the JSON-RPC message from a plain JSON or an SSE response."""
    raw = raw.strip()
    if not raw:
        return None
    if raw.startswith('{'):
        return json.loads(raw)
    for line in raw.splitlines():
        if line.startswith('data:'):
            return json.loads(line[5:].strip())
    return None


def connect():
    init, session = _post({
        'jsonrpc': '2.0', 'id': 1, 'method': 'initialize',
        'params': {
            'protocolVersion': '2025-03-26',
            'capabilities': {},
            'clientInfo': {'name': 'demo-agent', 'version': '0'},
        },
    })
    if init is None or 'error' in init:
        sys.exit('initialize failed: %s' % (init,))
    _post({'jsonrpc': '2.0', 'method': 'notifications/initialized'}, session)
    return session


def main(argv):
    if len(argv) < 2:
        sys.exit(__doc__)
    session = connect()
    if argv[1] == '--list':
        msg, _ = _post({'jsonrpc': '2.0', 'id': 2, 'method': 'tools/list'}, session)
        for tool in sorted(msg['result']['tools'], key=lambda t: t['name']):
            print(tool['name'])
        return
    args = json.loads(argv[2]) if len(argv) > 2 else {}
    msg, _ = _post({
        'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call',
        'params': {'name': argv[1], 'arguments': args},
    }, session)
    if 'error' in msg:
        print(json.dumps(msg['error'], indent=2))  # protocol-level error
        sys.exit(1)
    result = msg['result']
    # MCP_COMPACT=1 prints one line per result (used for the README recording).
    indent = None if os.environ.get('MCP_COMPACT') else 2
    for item in result.get('content', []):
        if item.get('type') == 'text':
            try:
                print(json.dumps(json.loads(item['text']), indent=indent))
            except ValueError:
                print(item['text'])
    if result.get('isError'):
        sys.exit(1)


if __name__ == '__main__':
    main(sys.argv)
