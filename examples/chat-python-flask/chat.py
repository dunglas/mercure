# A minimalist chat system, using the Flask microframework to handle cookie authentication
# Install:
# pip install Flask PyJWT
#
# Run (in debug mode):
# FLASK_APP=chat.py FLASK_DEBUG=1 flask run
#
# Available environment variables:
# * JWT_KEY: the JWT key to use (must be shared with the Mercure hub)
# * HUB_URL: the URL of the Mercure hub (default: http://localhost:3000/hub)
# * TOPIC: the topic to use (default: http://example.com/chat)
# * TARGET: the target to use (default: chan)

from flask import Flask, make_response, request, render_template
import jwt
import os
app = Flask(__name__)


@app.route("/")
def chat():
    targets = [os.environ.get('TARGET', 'chan')]
    token = jwt.encode(
        {'mercure': {'subscribe': targets, 'publish': targets}},
        os.environ.get('JWT_KEY', '!UnsecureChangeMe!'),
        algorithm='HS256'
    )

    hub_url = os.environ.get('HUB_URL', 'http://localhost:3000/hub')
    topic = os.environ.get('HUB_URL', 'http://example.com/chat')

    resp = make_response(render_template('chat.html', config={
                         'hubURL': hub_url, 'topic': topic}))
    resp.set_cookie('mercureAuthorization', token, httponly=True, path='/hub',
                    samesite="strict", secure=request.is_secure)  # Force secure to true for real apps

    return resp
