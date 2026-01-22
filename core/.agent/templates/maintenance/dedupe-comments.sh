#!/bin/bash
# Ricochet Dedupe Helper
# Usage: ./dedupe-comments.sh --base 123 --dupe 456

set -e

BASE=""
DUPE=""

while [[ "$#" -gt 0 ]]; do
    case $1 in
        --base) BASE="$2"; shift ;;
        --dupe) DUPE="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

if [[ -z "$BASE" || -z "$DUPE" ]]; then
    echo "Usage: $0 --base <id> --dupe <id>"
    exit 1
fi

echo "Marking #$BASE as duplicate of #$DUPE"

BODY="🚨 **Potential Duplicate Detected**

This issue seems similar to #$DUPE.
Please verify if they cover the same topic."

gh issue comment "$BASE" --body "$BODY"
