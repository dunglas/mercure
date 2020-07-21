from sseclient import SSEClient
import jwt
import os

token = jwt.encode(
    {'mercure': {'subscribe': ['*']}},
    os.environ.get('JWT_KEY', '!ChangeMe!'),
    algorithm='HS256',
)

updates = SSEClient(
    os.environ.get('HUB_URL', 'http://localhost:3001/.well-known/mercure'),
    params={'topic': ['*']},
    headers={'Authorization': b'Bearer '+token, 'Last-Event-ID': 'earliest'},
)
for update in updates:
    print("Update received: ", update)
