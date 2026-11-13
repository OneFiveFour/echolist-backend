#!/usr/bin/env python3
"""List/revoke EchoList device sessions without persisting login credentials."""
import argparse
import base64
import getpass
import json
import sys
from datetime import datetime, timezone
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # Never forward credentials to a different URL.


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True, help="Backend URL, e.g. https://notes.example.org")
    parser.add_argument("--username", default="admin")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("list", help="List active device sessions")
    revoke = commands.add_parser("revoke", help="Revoke one device session immediately")
    revoke.add_argument("session_id")
    args = parser.parse_args()
    url = urlsplit(args.url)
    if url.scheme not in ("http", "https") or not url.netloc or url.username or url.password or url.query or url.fragment:
        parser.error("--url must be an HTTP(S) backend URL without credentials, query or fragment")
    opener = build_opener(NoRedirect())
    token = None

    def call(method, payload):
        headers = {"Content-Type": "application/json", "Connect-Protocol-Version": "1"}
        if token:
            headers["Authorization"] = "Bearer " + token
        req = Request(args.url.rstrip("/") + "/auth.v1.AuthService/" + method,
                      data=json.dumps(payload).encode(), headers=headers, method="POST")
        with opener.open(req, timeout=30) as response:
            return json.load(response)

    own_id = None
    try:
        login = call("Login", {"username": args.username, "password": getpass.getpass("Password: "),
                               "deviceName": "Session management (temporary)"})
        token = login["accessToken"]
        # Only extract the ID of our own temporary session for cleanup. The
        # server authenticates every request; this decoded claim grants no trust.
        payload = token.split(".")[1]
        own_id = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))["sid"]
        if args.command == "revoke":
            call("RevokeSession", {"sessionId": args.session_id})
            print("Session revoked (if it belongs to this account).")
        else:
            sessions = call("ListSessions", {}).get("sessions", [])
            print("ID                                    DEVICE                         LAST REFRESH (UTC)      EXPIRES (UTC)")
            for session in sessions:
                if session["id"] == own_id:
                    continue
                def date(key):
                    return datetime.fromtimestamp(int(session[key]) / 1000, timezone.utc).strftime("%Y-%m-%d %H:%M")
                name = json.dumps(session.get("deviceName", ""), ensure_ascii=True)
                print(f'{session["id"]}  {name:<30} {date("lastUsedAt"):<23} {date("expiresAt")}')
    except (HTTPError, URLError, TimeoutError, ValueError, KeyError) as error:
        # Do not print request bodies or bearer tokens.
        print(f"Session management failed: {error}", file=sys.stderr)
        return 1
    finally:
        if own_id:
            try:
                call("RevokeSession", {"sessionId": own_id})
            except (HTTPError, URLError, TimeoutError, ValueError):
                print("Could not revoke the temporary management session; retry when the backend is available.", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
