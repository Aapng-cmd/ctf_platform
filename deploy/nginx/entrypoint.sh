#!/bin/sh
set -eu

DOMAIN="${CERT_DOMAIN:-ctf.local}"
case "$DOMAIN" in
  *[!A-Za-z0-9.-]*)
    echo "CERT_DOMAIN must be a hostname" >&2
    exit 1
    ;;
esac

DIR=/etc/nginx/certs
mkdir -p "$DIR"
SANS="DNS:${DOMAIN},DNS:localhost,IP:127.0.0.1"
if [ -n "${CERT_SANS:-}" ]; then
  case "$CERT_SANS" in
    *[!A-Za-z0-9.:_,-]*)
      echo "CERT_SANS must look like IP:10.0.0.5,DNS:ctf.lan" >&2
      exit 1
      ;;
  esac
  SANS="${SANS},${CERT_SANS}"
fi

if [ ! -f "$DIR/fullchain.pem" ] || [ ! -f "$DIR/privkey.pem" ]; then
  openssl req -x509 -newkey rsa:2048 -sha256 -days 825 -nodes \
    -keyout "$DIR/privkey.pem" \
    -out "$DIR/fullchain.pem" \
    -subj "/CN=${DOMAIN}" \
    -addext "subjectAltName=${SANS}"
  cp "$DIR/fullchain.pem" "$DIR/ca.crt"
fi

exec /docker-entrypoint.sh "$@"
