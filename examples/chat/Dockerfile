FROM tiangolo/meinheld-gunicorn:python3.8-alpine3.11

COPY ./requirements.txt /app
RUN pip install -r requirements.txt
COPY ./main.py .
COPY ./static static
COPY ./templates templates
