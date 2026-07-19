#!/bin/bash
# Back up the collected CSVs to the orphan 'data' branch of the origin repo.
# Runs from anywhere (uses absolute paths). The 'data' branch holds only CSVs,
# kept separate from main so code history stays clean. gym-config.env (the token)
# is never copied. Safe to run repeatedly — it only commits/pushes when CSVs changed.
set -e

RT="$HOME/Library/Application Support/ronimis"
BK="$RT/.backup-data"
REMOTE="ssh://git@github.com/gordon2/ronimis.git"

if [ ! -d "$BK/.git" ]; then
  rm -rf "$BK"
  mkdir -p "$BK"
  cd "$BK"
  git init -q
  git remote add origin "$REMOTE"
  git config user.name "gym-backup"
  git config user.email "gym-backup@ronimis.local"
  if git fetch -q origin data 2>/dev/null; then
    git checkout -q -b data origin/data
  else
    git checkout -q --orphan data
  fi
fi

cd "$BK"
cp "$RT"/gym-stats-*.csv "$BK"/ 2>/dev/null || true
git add -A
if git diff --cached --quiet; then
  echo "backup: no changes"
else
  git commit -q -m "Backup $(date '+%Y-%m-%d %H:%M')"
  git push -q -u origin data
  echo "backup: pushed $(ls "$BK"/gym-stats-*.csv 2>/dev/null | wc -l | tr -d ' ') files"
fi
