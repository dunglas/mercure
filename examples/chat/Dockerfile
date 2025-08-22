# hadolint ignore=DL3006
FROM tiangolo/meinheld-gunicorn

WORKDIR /app

COPY ./requirements.txt /app
RUN pip install --no-cache-dir -r requirements.txt

COPY ./main.py .
COPY ./static static
COPY ./templates templates
