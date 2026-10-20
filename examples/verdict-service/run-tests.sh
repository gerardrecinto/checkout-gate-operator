#!/usr/bin/env bash
# Runs the tests in a JDK 21 container, so no local JDK 21 is needed.
set -euo pipefail
cd "$(dirname "$0")"
docker run --rm -v "$PWD":/app -v "$HOME/.m2":/root/.m2 -w /app maven:3.9-eclipse-temurin-21 mvn -q -B test
