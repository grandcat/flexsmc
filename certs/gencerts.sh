#!/bin/bash
openssl req -x509 -newkey rsa:2048 -keyout key_$1.pem -out cert_$1.pem -days 365 -nodes -extensions xauth -config /etc/ssl/openssl.cnf
