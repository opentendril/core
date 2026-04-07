FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends gcc g++ && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY src/ ./src/
COPY entrypoint.sh .

RUN chmod +x entrypoint.sh && \
    adduser --disabled-password --gecos '' tendril && \
    mkdir -p /app/data/dynamic_skills /app/logs /data /logs && \
    chown -R tendril:tendril /app /data /logs && \
    chmod -R 755 /app

USER tendril

ENTRYPOINT ["./entrypoint.sh"]
