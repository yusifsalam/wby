FROM python:3.12-slim

WORKDIR /app

# pygrib's manylinux wheels bundle ecCodes, so no apt step is needed for the
# common case. If the wheel falls back to a source build on this platform,
# uncomment the following to provide the native library + build headers:
#   RUN apt-get update && apt-get install -y --no-install-recommends \
#       libeccodes-dev build-essential && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

ENV GRIB_DATA_DIR=/data \
    PORT=9090
EXPOSE 9090

CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port ${PORT}"]
