#!/bin/sh
# Fails under the run check's sandbox: it needs the network.
curl -sSI --max-time 10 https://example.com >/dev/null || { echo "no network"; exit 1; }
echo "reached example.com"
