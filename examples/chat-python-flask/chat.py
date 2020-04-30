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
    COOKIE_DOMAIN: the cookie domain (default: None)
"""

from flask import Flask, make_response, request, render_template, abort
from sseclient import SSEClient
import jwt
import os
import threading
import json
import urllib.parse

HUB_URL = os.environ.get(
    'HUB_URL', 'http://localhost:3000/.well-known/mercure')
JWT_KEY = os.environ.get('JWT_KEY', '!ChangeMe!')
TOPIC = 'https://chat.example.com/messages/{id}'
SUBSCRIPTION_TOPIC = 'https://mercure.rocks/subscriptions/https%3A%2F%2Fchat.example.com%2Fmessages%2F%7Bid%7D/{subscriptionID}'

lock = threading.Lock()
last_event_id = None
connected_users = {}

app = Flask(__name__)


@app.route('/', methods=['GET'])
def join():
    return render_template('join.html')


@app.route('/', methods=['POST'])
def chat():
    username = request.form['username']
    if not username:
        abort(400)

    user_iri = 'https://chat.example.com/users/'+username
    targets = ['https://chat.example.com/user', user_iri,
               'https://mercure.rocks/targets/subscriptions/'+urllib.parse.quote(TOPIC, safe='')]
    token = jwt.encode(
        {'mercure': {'subscribe': targets, 'publish': targets}},
        JWT_KEY,
        algorithm='HS256',
    )

    lock.acquire()
    local_last_event_id = last_event_id
    cu = list(connected_users.keys())
    lock.release()

    resp = make_response(render_template('chat.html', config={
                         'hubURL': HUB_URL, 'userIRI': user_iri, 'connectedUsers': cu, 'lastEventID': local_last_event_id}))
    resp.set_cookie('mercureAuthorization', token, httponly=True, path='/.well-known/mercure',
                    samesite="strict", domain=os.environ.get('COOKIE_DOMAIN', None), secure=request.is_secure)  # Force secure to True for real apps

    return resp


@app.before_first_request
def start_sse_listener():
    t = threading.Thread(target=sse_listener)
    t.start()


def sse_listener():
    global connected_users
    global last_event_id

    token = jwt.encode(
        {'mercure': {'subscribe': ['https://chat.example.com/user',
                                   'https://mercure.rocks/targets/subscriptions/'+urllib.parse.quote(TOPIC, safe='')]}},
        JWT_KEY,
        algorithm='HS256',
    )

    updates = SSEClient(
        HUB_URL,
        params={'topic': [TOPIC, SUBSCRIPTION_TOPIC]},
        headers={'Authorization': b'Bearer '+token},
    )
    for update in updates:
        data = json.loads(update.data)

        if data['@type'] == 'https://chat.example.com/Message':
            # Store the chat history somewhere if you want to
            break

        if data['@type'] == 'https://mercure.rocks/Subscription':
            user = next((x for x in data['subscribe'] if x.startswith(
                'https://chat.example.com/users/')), None)

            if user is None:
                break

            # Instead of maintaining a local user list, you may want to use Redis or similar service
            lock.acquire()
            last_event_id = update.id
            if data['active']:
                connected_users[user] = True
            else:
                del connected_users[user]
            lock.release()
            print(connected_users)
