import os

import jwt
from sseclient import SSEClient

token = jwt.encode(
    {"mercure": {"subscribe": ["*"]}},
    os.environ.get("JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!"),
    algorithm="HS256",
)

updates = SSEClient(
    os.environ.get("HUB_URL", "https://localhost/.well-known/mercure"),
    params={"topic": ["*"]},
    headers={"Authorization": b"Bearer " + token},
)
for update in updates:
    print("Update received: ", update)
