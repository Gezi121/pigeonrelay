FROM python:3.11-slim

WORKDIR /app

RUN pip install --no-cache-dir requests

COPY entrypoint.sh client.py ./
RUN chmod +x entrypoint.sh

ENV PYTHONUNBUFFERED=1 \
    PIGEONRELAY_URL=https://pigeonrelay.example.com \
    LATENCY_TOKEN=default-latency-token \
    TCP_WORKERS=50 \
    TLS_WORKERS=10 \
    TCP_TIMEOUT=2 \
    TLS_TIMEOUT=3 \
    TEST_PORT=443 \
    MEASURE_INTERVAL=600 \
    REPORT_INTERVAL=1200 \
    FAST_INTERVAL=120 \
    PHASE1_SHARDS=4 \
    SHARD_INTERVAL=45 \
    CLIENT_ISP=china-telecom \
    TLL_MIN_MS=40 \
    DATA_DIR=data

ENTRYPOINT ["./entrypoint.sh"]
