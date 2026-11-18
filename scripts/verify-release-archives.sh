#!/usr/bin/env bash
# Verify GoReleaser dist archives contain the poke executable (pokedex is an install-time alias).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${DIST_DIR:-$ROOT/dist}"

if [[ ! -d "$DIST" ]]; then
  echo "error: dist directory not found at $DIST" >&2
  echo "run: goreleaser release --snapshot --clean --skip=publish" >&2
  exit 1
fi

shopt -s nullglob
archives=("$DIST"/*.tar.gz "$DIST"/*.zip)
if ((${#archives[@]} == 0)); then
  echo "error: no .tar.gz or .zip archives under $DIST" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

fail=0
checked=0

for archive in "${archives[@]}"; do
  name="$(basename "$archive")"
  extract="$tmpdir/$checked"
  mkdir -p "$extract"

  case "$archive" in
    *.tar.gz)
      tar -xzf "$archive" -C "$extract"
      poke_names=("poke")
      ;;
    *.zip)
      unzip -q "$archive" -d "$extract"
      poke_names=("poke.exe")
      ;;
    *)
      continue
      ;;
  esac

  checked=$((checked + 1))
  echo "==> $name"

  poke_ok=0
  for n in "${poke_names[@]}"; do
    if find "$extract" \( -type f -o -type l \) -name "$n" | grep -q .; then
      echo "  found $n"
      poke_ok=1
      break
    fi
  done
  if ((poke_ok == 0)); then
    echo "  missing ${poke_names[*]}" >&2
    fail=1
  fi

  # Archives must not ship a duplicate pokedex binary anymore.
  if find "$extract" \( -type f -o -type l \) \( -name 'pokedex' -o -name 'pokedex.exe' \) | grep -q .; then
    echo "  unexpected pokedex binary in archive (should be install-time alias only)" >&2
    fail=1
  else
    echo "  ok: no duplicate pokedex binary"
  fi
done

if ((checked == 0)); then
  echo "error: no archives were checked under $DIST" >&2
  exit 1
fi

if ((fail != 0)); then
  echo "error: one or more archives failed poke verification" >&2
  exit 1
fi

echo "ok: verified $checked archive(s) contain poke only"
