#!/bin/bash
# openssl req -x509 -newkey rsa:2048 -keyout key_$1.pem -out cert_$1.pem -days 365 -nodes -extensions xauth -config /etc/ssl/openssl.cnf
openssl req \
    -new \
    -newkey rsa:4096 \
    -days 365 \
    -nodes \
    -x509 \
	-extensions xauth \
	-config /etc/ssl/openssl.cnf \
    -subj "/C=DE/ST=BY/L=Munich/O=TUM/CN=n$1.flexsmc.local" \
    -keyout key_$1.pem \
    -out cert_$1.pem
