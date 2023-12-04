FROM tiangolo/meinheld-gunicorn:python3.9

WORKDIR /app

COPY ./requirements.txt /app
RUN pip install --no-cache-dir -r requirements.txt

COPY ./main.py .
COPY ./static static
COPY ./templates templates
