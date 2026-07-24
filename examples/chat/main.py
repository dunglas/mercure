# -*- coding: utf-8 -*-
"""A minimalist chat system, using the Flask microframework to handle cookie authentication

Install:
    pip install -r requirements.txt

Run (prod):
    gunicorn chat:app

Run (dev):
    FLASK_APP=chat.py FLASK_DEBUG=1 flask run

Deploy on Heroku:
    heroku login
    heroku config:set HUB_URL=https://demo.mercure.rocks/.well-known/mercure
    heroku config:set COOKIE_DOMAIN=.mercure.rocks
    git subtree push --prefix examples/chat-python-flask heroku master

Environment variables:
    JWT_KEY: the JWT key to use (must be shared with the Mercure hub)
    HUB_URL: the URL of the Mercure hub (default: https://localhost/.well-known/mercure)
    COOKIE_DOMAIN: the cookie domain (default: None)
"""

import os
from urllib.parse import quote

import jwt
from flask import Flask, abort, make_response, render_template, request

HUB_URL = os.environ.get("HUB_URL", "https://localhost/.well-known/mercure")
# The issuer identifier of this app, bound to the signing key in the hub's
# issuer block (defaults to https://localhost in the Caddyfile).
ISSUER = os.environ.get("ISSUER", "https://localhost")
JWT_KEY = os.environ.get("JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")
MESSAGE_PATTERN = os.environ.get(
    "MESSAGE_PATTERN", "https://chat.example.com/messages/:id"
)

# The collection of subscription events for the message subscription, per the
# /.well-known/mercure/subscriptions/{match_type}/{match} scheme: the match is
# the message pattern, percent-encoded exactly once.
SUBSCRIPTIONS_TOPIC = "/.well-known/mercure/subscriptions/urlpattern/" + quote(
    MESSAGE_PATTERN, safe=""
)
# Per-subscriber presence events live one segment deeper.
PRESENCE_PATTERN = SUBSCRIPTIONS_TOPIC + "/:subscriber"

app = Flask(__name__)


@app.route("/", methods=["GET"])
def join():
    return render_template("join.html")


@app.route("/", methods=["POST"])
def chat():
    username = request.form["username"]
    if not username:
        abort(400)

    token = jwt.encode(
        {
            "iss": ISSUER,
            "aud": HUB_URL,
            "exp": 4102444800,
            "authorization_details": [
                {
                    "type": "https://mercure.rocks/authorization-detail",
                    "actions": ["publish"],
                    "topics": [{"match": MESSAGE_PATTERN, "match_type": "urlpattern"}],
                },
                {
                    "type": "https://mercure.rocks/authorization-detail",
                    "actions": ["subscribe"],
                    "topics": [
                        {"match": MESSAGE_PATTERN, "match_type": "urlpattern"},
                        {"match": SUBSCRIPTIONS_TOPIC, "match_type": "exact"},
                        {"match": PRESENCE_PATTERN, "match_type": "urlpattern"},
                    ],
                    "payload": {"username": username},
                },
            ],
        },
        JWT_KEY,
        algorithm="HS256",
        headers={"typ": "at+jwt"},
    )

    resp = make_response(
        render_template(
            "chat.html",
            config={
                "hubURL": HUB_URL,
                "messagePattern": MESSAGE_PATTERN,
                "subscriptionsTopic": SUBSCRIPTIONS_TOPIC,
                "presencePattern": PRESENCE_PATTERN,
                "username": username,
            },
        )
    )
    resp.set_cookie(
        "mercure_access_token",
        token,
        httponly=True,
        path="/.well-known/mercure",
        samesite="strict",
        domain=os.environ.get("COOKIE_DOMAIN", None),
        secure=request.is_secure,
    )  # Force secure to True for real apps

    return resp
