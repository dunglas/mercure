import os

import jwt
from sseclient import SSEClient

HUB_URL = os.environ.get("HUB_URL", "https://localhost/.well-known/mercure")

token = jwt.encode(
    {
        "iss": os.environ.get("ISSUER", "https://localhost"),
        "aud": HUB_URL,
        "exp": 4102444800,
        "authorization_details": [
            {
                "type": "https://mercure.rocks/authorization-detail",
                "actions": ["subscribe"],
                "topics": [{"match": "*"}],
            }
        ],
    },
    os.environ.get("JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!"),
    algorithm="HS256",
    headers={"typ": "at+jwt"},
)

updates = SSEClient(
    HUB_URL,
    params={"match": ["*"]},
    headers={"Authorization": b"Bearer " + token},
)
for update in updates:
    print("Update received: ", update)
