#!/usr/bin/env bash

set -e

GOFUMPT_SPLIT_LONG_LINES=on go tool gofumpt -w -l -extra .