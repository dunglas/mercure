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
    HUB_URL: the URL of the Mercure hub (default: http://localhost:3000/.well-known/mercure)
    TOPIC: the topic to use (default: http://example.com/chat)
    TARGET: the target to use (default: chan)
    COOKIE_DOMAIN: the cookie domain (default: None)
"""

from flask import Flask, make_response, request, render_template
import jwt
import os
app = Flask(__name__)


@app.route("/")
def chat():
    targets = [os.environ.get('TARGET', 'chan')]
    token = jwt.encode(
        {'mercure': {'subscribe': targets, 'publish': targets}},
        os.environ.get('JWT_KEY', '!ChangeMe!'),
        algorithm='HS256'
    )

    hub_url = os.environ.get('HUB_URL', 'http://localhost:3000/.well-known/mercure')
    topic = os.environ.get('TOPIC', 'http://example.com/chat')

    resp = make_response(render_template('chat.html', config={
                         'hubURL': hub_url, 'topic': topic}))
    resp.set_cookie('mercureAuthorization', token, httponly=True, path='/.well-known/mercure',
                    samesite="strict", domain=os.environ.get('COOKIE_DOMAIN', None), secure=request.is_secure)  # Force secure to True for real apps

    return resp
