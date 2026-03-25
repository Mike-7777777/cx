#!/bin/bash
# Wrapper: reads stdin from CC and pipes to cx.exe
exec "$(dirname "$0")/cx.exe" statusline
