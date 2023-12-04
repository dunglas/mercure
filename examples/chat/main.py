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

import jwt
from flask import Flask, abort, make_response, render_template, request
from uritemplate import expand

HUB_URL = os.environ.get("HUB_URL", "https://localhost/.well-known/mercure")
JWT_KEY = os.environ.get("JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")
MESSAGE_URI_TEMPLATE = os.environ.get(
    "MESSAGE_URI_TEMPLATE", "https://chat.example.com/messages/{id}"
)

SUBSCRIPTIONS_TEMPLATE = "/.well-known/mercure/subscriptions/{topic}{/subscriber}"
SUBSCRIPTIONS_TOPIC = expand(SUBSCRIPTIONS_TEMPLATE, topic=MESSAGE_URI_TEMPLATE)

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
            "mercure": {
                "subscribe": [MESSAGE_URI_TEMPLATE, SUBSCRIPTIONS_TEMPLATE],
                "publish": [MESSAGE_URI_TEMPLATE],
                "payload": {"username": username},
            }
        },
        JWT_KEY,
        algorithm="HS256",
    )

    resp = make_response(
        render_template(
            "chat.html",
            config={
                "hubURL": HUB_URL,
                "messageURITemplate": MESSAGE_URI_TEMPLATE,
                "subscriptionsTopic": SUBSCRIPTIONS_TOPIC,
                "username": username,
            },
        )
    )
    resp.set_cookie(
        "mercureAuthorization",
        token,
        httponly=True,
        path="/.well-known/mercure",
        samesite="strict",
        domain=os.environ.get("COOKIE_DOMAIN", None),
        secure=request.is_secure,
    )  # Force secure to True for real apps

    return resp
