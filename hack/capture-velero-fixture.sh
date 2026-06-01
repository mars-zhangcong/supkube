#!/usr/bin/env bash
# capture-velero-fixture.sh — Phase 0 verify-before-architect for PRD-007.
#
# What
# ----
# 抓真集群的 Velero CR + BSL 对象结构 + 子 CR (DataUpload/DataDownload/PVB)
# 到 engineer-testing/fixtures/velero-real-<date>/ 作为 fixture.
#
# Why (PRD-007 P1)
# ----------------
# 评审 P1: PRD-007 §4.3 Layer 4 Backup Copy 写的 "逐对象复制" 是错的 — Velero 卷
# 数据在共享 Kopia 仓库 bucket/kopia/<ns>, 按 ns 不按备份. 必须抓真 fixture
# 来验证: (a) BSL 对象结构真实长什么样 (b) 快照型备份 vs data-mover 备份 BSL
# 对象差异 (c) 复制粒度真的应该是 kopia/<ns> 而不是 backups/<name>/
#
# 输出
# ----
# engineer-testing/fixtures/velero-real-<date>/
# ├── README.md                        # 抓 fixture 时的集群/版本/BSL 状态摘要
# ├── bsl-listing.txt                  # `s3 ls --recursive` / `az storage blob list` 完整输出
# ├── bsl-tree.txt                     # 用 tree 格式呈现 bucket 内对象层级
# ├── backups/
# │   ├── backup-<name>-1.yaml         # 完整 Backup CR (含 .status, kubectl get -o yaml)
# │   └── backup-<name>-2.yaml
# ├── dataupload/                      # 关联 DataUpload CR (snapshotMoveData=true 备份)
# │   └── dataupload-<...>.yaml
# ├── datadownload/                    # 关联 DataDownload (restore 时)
# │   └── ...
# ├── pvb/                             # PodVolumeBackup (fs-backup 用)
# │   └── ...
# └── volumesnapshotcontents/          # CSI snapshot 类型用 VSC
#     └── ...
#
# Usage
# -----
#   ./hack/capture-velero-fixture.sh
#   ./hack/capture-velero-fixture.sh --context=aks-jumborca-dev
#   ./hack/capture-velero-fixture.sh --output=fixtures/foo
#   ./hack/capture-velero-fixture.sh --namespace=test-app-ns
#   ./hack/capture-velero-fixture.sh --dry-run
#
# Flags
# -----
#   --context=<name>     kubectl context (默认: 当前 active context)
#   --output=<path>      输出目录 (默认: engineer-testing/fixtures/velero-real-<UTC-ts>/)
#   --namespace=<ns>     只抓某 ns 下的 Backup (减小 fixture; 默认全抓)
#   --dry-run            只 print 命令, 不执行
#   -h | --help          show this help
#
# Caveats
# -------
# - BSL 对象结构抓取假设 provider 是 aws (含 MinIO-compatible) 或 azure. 其他
#   provider (gcp/swift/etc.) 当前只 dump CR yaml + 在 README 标 "BSL_LIST_SKIPPED".
# - 不写入 BSL secret 内容; access key 在 fixture 里用 <REDACTED> 占位.
# - 不删 fixture 目录; 用户可保留也可 rm -rf.

set -euo pipefail

# ─── Defaults ───────────────────────────────────────────────────────────────
CONTEXT=""
OUTPUT=""
NS_FILTER=""
DRY_RUN=0
VELERO_NS="velero"

usage() {
  sed -n '2,50p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
}

# ─── Parse flags ────────────────────────────────────────────────────────────
for arg in "$@"; do
  case "$arg" in
    --context=*)   CONTEXT="${arg#*=}" ;;
    --output=*)    OUTPUT="${arg#*=}" ;;
    --namespace=*) NS_FILTER="${arg#*=}" ;;
    --dry-run)     DRY_RUN=1 ;;
    -h|--help)     usage ;;
    *)
      echo "ERROR: unknown flag: $arg" >&2
      echo "Run with -h for usage." >&2
      exit 2
      ;;
  esac
done

# ─── Helpers ────────────────────────────────────────────────────────────────
TS="$(date -u +%Y-%m-%d-%H%M%S)"
if [[ -z "$OUTPUT" ]]; then
  OUTPUT="engineer-testing/fixtures/velero-real-${TS}"
fi

KUBECTL_BASE=(kubectl)
if [[ -n "$CONTEXT" ]]; then
  KUBECTL_BASE+=(--context "$CONTEXT")
fi

log()    { printf '\033[36m[capture]\033[0m %s\n' "$*" >&2; }
warn()   { printf '\033[33m[warn]\033[0m %s\n'    "$*" >&2; }
error()  { printf '\033[31m[error]\033[0m %s\n'   "$*" >&2; }
fatal()  { error "$*"; exit 1; }

run() {
  # Run or print depending on DRY_RUN.
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '\033[90m[dry-run]\033[0m %s\n' "$*" >&2
  else
    eval "$@"
  fi
}

kc()    { "${KUBECTL_BASE[@]}" "$@"; }
kc_v()  { "${KUBECTL_BASE[@]}" -n "$VELERO_NS" "$@"; }

mkdir_p() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '\033[90m[dry-run]\033[0m mkdir -p %s\n' "$1" >&2
  else
    mkdir -p "$1"
  fi
}

write_to() {
  # write_to <path> <content>
  local path="$1"; shift
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '\033[90m[dry-run]\033[0m write %d bytes -> %s\n' "${#1}" "$path" >&2
  else
    printf '%s\n' "$1" > "$path"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Phase 0a — Pre-flight
# ─────────────────────────────────────────────────────────────────────────────
phase_0a_preflight() {
  log "Phase 0a: pre-flight checks"

  # Check kubectl exists.
  command -v kubectl >/dev/null 2>&1 || fatal "kubectl not found in PATH"

  # Check context.
  local current_ctx
  current_ctx="$(kc config current-context 2>/dev/null || true)"
  local effective_ctx="${CONTEXT:-$current_ctx}"
  log "kubectl context: ${effective_ctx:-<none>}"

  case "$effective_ctx" in
    docker-desktop|aks-jumborca-dev|aks-jumborca-test) : ;;
    *) warn "context '$effective_ctx' is NOT one of the expected dev contexts (docker-desktop / aks-jumborca-dev / aks-jumborca-test). Continuing anyway." ;;
  esac

  # Check velero ns exists.
  if ! kc get ns "$VELERO_NS" >/dev/null 2>&1; then
    fatal "namespace '$VELERO_NS' not found — is Velero installed?"
  fi

  # Check BSL count.
  local bsl_count
  bsl_count="$(kc_v get backupstoragelocation --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$bsl_count" -lt 1 ]]; then
    fatal "no BackupStorageLocation found in ns '$VELERO_NS' — fixture meaningless"
  fi
  log "found $bsl_count BackupStorageLocation(s)"

  # Check Backup count (no label-based ns filter — Velero doesn't label Backup
  # CRs by source ns; ns filtering happens post-hoc in phase_0c by reading
  # .spec.includedNamespaces).
  local bk_count
  bk_count="$(kc_v get backup --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$bk_count" -lt 1 ]]; then
    fatal "no Backup CR found in ns '$VELERO_NS' — fixture meaningless"
  fi
  log "found $bk_count Backup CR(s) (cluster-wide; ns filter applied in phase 0c)"
}

# ─────────────────────────────────────────────────────────────────────────────
# Phase 0b — BSL object listing
# ─────────────────────────────────────────────────────────────────────────────
phase_0b_bsl() {
  log "Phase 0b: capture BSL object structure"

  mkdir_p "$OUTPUT"
  local listing="$OUTPUT/bsl-listing.txt"
  local tree="$OUTPUT/bsl-tree.txt"

  [[ "$DRY_RUN" -eq 1 ]] && { log "(dry-run) skipping BSL listing"; return; }

  : > "$listing"
  : > "$tree"

  local bsl_names
  bsl_names="$(kc_v get backupstoragelocation -o jsonpath='{.items[*].metadata.name}')"

  for bsl in $bsl_names; do
    log "  BSL: $bsl"
    local provider bucket prefix region endpoint
    provider="$(kc_v get bsl "$bsl" -o jsonpath='{.spec.provider}')"
    bucket="$(kc_v get bsl "$bsl" -o jsonpath='{.spec.objectStorage.bucket}')"
    prefix="$(kc_v get bsl "$bsl" -o jsonpath='{.spec.objectStorage.prefix}' 2>/dev/null || true)"
    region="$(kc_v get bsl "$bsl" -o jsonpath='{.spec.config.region}' 2>/dev/null || true)"
    endpoint="$(kc_v get bsl "$bsl" -o jsonpath='{.spec.config.s3Url}' 2>/dev/null || true)"

    {
      echo "=========================================================="
      echo "BSL: $bsl"
      echo "  provider: $provider"
      echo "  bucket  : $bucket"
      echo "  prefix  : ${prefix:-<none>}"
      echo "  region  : ${region:-<none>}"
      echo "  endpoint: ${endpoint:-<default>}"
      echo "=========================================================="
    } >> "$listing"

    case "$provider" in
      aws)
        if command -v aws >/dev/null 2>&1; then
          local s3_uri="s3://${bucket}${prefix:+/$prefix}"
          local ep_arg=()
          [[ -n "$endpoint" ]] && ep_arg=(--endpoint-url "$endpoint")
          if aws s3 ls "$s3_uri" --recursive --human-readable "${ep_arg[@]}" \
               >> "$listing" 2>>"$listing"; then
            log "    aws s3 ls OK ($s3_uri)"
          else
            warn "    aws s3 ls failed for $s3_uri (creds? net?) — see listing"
          fi
        else
          warn "    aws CLI not installed — BSL_LIST_SKIPPED"
          echo "  [BSL_LIST_SKIPPED: aws CLI not installed]" >> "$listing"
        fi
        ;;
      azure)
        if command -v az >/dev/null 2>&1; then
          if az storage blob list --container-name "$bucket" --auth-mode login \
               --query '[].{name:name,size:properties.contentLength,modified:properties.lastModified}' \
               -o tsv >> "$listing" 2>>"$listing"; then
            log "    az storage blob list OK (container=$bucket)"
          else
            warn "    az storage blob list failed (creds? container?) — see listing"
          fi
        else
          warn "    az CLI not installed — BSL_LIST_SKIPPED"
          echo "  [BSL_LIST_SKIPPED: az CLI not installed]" >> "$listing"
        fi
        ;;
      *)
        warn "    provider '$provider' not supported by this script — BSL_LIST_SKIPPED"
        echo "  [BSL_LIST_SKIPPED: provider=$provider not supported (only aws|azure)]" >> "$listing"
        ;;
    esac
    echo "" >> "$listing"
  done

  # Build tree view (best-effort, from listing).
  {
    echo "# bsl-tree.txt — derived from bsl-listing.txt, shows logical path hierarchy"
    echo "# (heuristic; for canonical bytes/timestamps see bsl-listing.txt)"
    echo ""
    awk '
      /^BSL: / { print "\n" $0; next }
      /^  bucket  :/ { bucket = $3; print $0; next }
      /^  / { next }
      /^==/ { next }
      /^\[BSL_LIST_SKIPPED/ { print $0; next }
      NF == 0 { next }
      {
        # Last token = key/path (for both aws s3 ls and az tsv).
        key = $NF
        n = split(key, parts, "/")
        indent = ""
        for (i = 1; i < n; i++) indent = indent "  "
        print indent parts[n]
      }
    ' "$listing" > "$tree"
  } 2>/dev/null || warn "tree generation had issues; check $tree manually"
}

# ─────────────────────────────────────────────────────────────────────────────
# Phase 0c — Velero CRs
# ─────────────────────────────────────────────────────────────────────────────
dump_cr_kind() {
  # dump_cr_kind <kind> <subdir> [extra-selector]
  local kind="$1" subdir="$2" sel="${3:-}"
  local dir="$OUTPUT/$subdir"
  mkdir_p "$dir"

  [[ "$DRY_RUN" -eq 1 ]] && { log "  (dry-run) would dump $kind -> $dir"; return; }

  if ! kc_v get "$kind" >/dev/null 2>&1; then
    warn "  kind '$kind' not found in cluster — skip"
    echo "# $kind: CRD not present" > "$dir/_NOT_PRESENT.txt"
    return
  fi

  local names
  if [[ -n "$sel" ]]; then
    names="$(kc_v get "$kind" -l "$sel" --no-headers 2>/dev/null | awk '{print $1}')"
  else
    names="$(kc_v get "$kind" --no-headers 2>/dev/null | awk '{print $1}')"
  fi

  if [[ -z "$names" ]]; then
    log "  no $kind found"
    echo "# no $kind in ns $VELERO_NS at $(date -u +%FT%TZ)" > "$dir/_EMPTY.txt"
    return
  fi

  local count=0
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    kc_v get "$kind" "$name" -o yaml > "$dir/${name}.yaml" 2>/dev/null && count=$((count+1)) || \
      warn "    failed to dump $kind/$name"
  done <<<"$names"
  log "  dumped $count $kind"
}

phase_0c_crs() {
  log "Phase 0c: dump Velero CRs"

  # BSL itself (for fixture replay).
  dump_cr_kind backupstoragelocation backupstoragelocation

  # Backup: dump all then prune by .spec.includedNamespaces if NS_FILTER set.
  dump_cr_kind backup backups ""
  if [[ -n "$NS_FILTER" && "$DRY_RUN" -ne 1 ]]; then
    log "  applying ns filter: keep only backups with includedNamespaces containing '$NS_FILTER'"
    shopt -s nullglob
    for f in "$OUTPUT"/backups/*.yaml; do
      if ! grep -E "^  - $NS_FILTER\$|^  - '$NS_FILTER'\$" "$f" >/dev/null 2>&1 \
           && ! grep -E "includedNamespaces:.*\\b$NS_FILTER\\b" "$f" >/dev/null 2>&1; then
        rm -f "$f"
      fi
    done
    shopt -u nullglob
  fi

  # Restore (best-effort, may be empty).
  dump_cr_kind restore restores ""

  # Sub-CRs (no easy filter; we dump all and let phase 0d cross-reference).
  dump_cr_kind dataupload dataupload
  dump_cr_kind datadownload datadownload
  dump_cr_kind podvolumebackup pvb
  dump_cr_kind podvolumerestore pvr
  dump_cr_kind volumesnapshotcontent volumesnapshotcontents
}

# ─────────────────────────────────────────────────────────────────────────────
# Phase 0d — Cross-reference (which sub-CR belongs to which backup)
# ─────────────────────────────────────────────────────────────────────────────
phase_0d_xref() {
  log "Phase 0d: cross-reference backups <-> sub-CRs"

  [[ "$DRY_RUN" -eq 1 ]] && { log "(dry-run) skipping xref"; return; }

  local xref="$OUTPUT/_xref.tsv"
  {
    printf 'backup\ttype\tsnapshotMoveData\tdataupload_count\tpvb_count\tvsc_count\tphase\terrors\n'
  } > "$xref"

  shopt -s nullglob
  for bk_yaml in "$OUTPUT"/backups/*.yaml; do
    local name
    name="$(basename "$bk_yaml" .yaml)"

    local move_data phase errors
    move_data="$(kc_v get backup "$name" -o jsonpath='{.spec.snapshotMoveData}' 2>/dev/null || echo '')"
    phase="$(kc_v get backup "$name" -o jsonpath='{.status.phase}' 2>/dev/null || echo '')"
    errors="$(kc_v get backup "$name" -o jsonpath='{.status.errors}' 2>/dev/null || echo '0')"

    local du_count pvb_count vsc_count
    du_count="$(kc_v get dataupload -l "velero.io/backup-name=$name" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    pvb_count="$(kc_v get podvolumebackup -l "velero.io/backup-name=$name" --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    # VSC has no label by convention; reverse via spec.backupName isn't standard either.
    # Best-effort: count VSCs whose VolumeSnapshot annotation references this backup.
    vsc_count="$(kc get volumesnapshotcontent -o json 2>/dev/null \
                  | grep -c "\"velero.io/backup-name\": \"$name\"" || true)"

    # Infer type heuristically.
    local type="unknown"
    if [[ "$move_data" == "true" ]]; then
      type="data-mover"
    elif [[ "$pvb_count" -gt 0 ]]; then
      type="fs-backup"
    elif [[ "$vsc_count" -gt 0 ]]; then
      type="csi-snapshot"
    elif [[ "$du_count" -gt 0 ]]; then
      type="data-mover"
    else
      type="metadata-only?"
    fi

    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$name" "$type" "${move_data:-false}" "$du_count" "$pvb_count" "$vsc_count" "$phase" "$errors" \
      >> "$xref"
  done
  shopt -u nullglob

  log "  xref written -> $xref"
}

# ─────────────────────────────────────────────────────────────────────────────
# Phase 0e — README.md summary
# ─────────────────────────────────────────────────────────────────────────────
phase_0e_readme() {
  log "Phase 0e: write README.md"

  [[ "$DRY_RUN" -eq 1 ]] && { log "(dry-run) skipping README"; return; }

  local readme="$OUTPUT/README.md"
  local effective_ctx
  effective_ctx="${CONTEXT:-$(kc config current-context 2>/dev/null || echo '<unknown>')}"

  local k8s_ver velero_img bsl_summary backup_count_total
  k8s_ver="$(kc version -o json 2>/dev/null | grep -oE '"gitVersion":\s*"[^"]+"' | head -2 | tr '\n' ' ' || echo 'n/a')"
  velero_img="$(kc_v get deploy velero -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo 'n/a')"
  bsl_summary="$(kc_v get backupstoragelocation -o custom-columns=NAME:.metadata.name,PROVIDER:.spec.provider,BUCKET:.spec.objectStorage.bucket,PHASE:.status.phase --no-headers 2>/dev/null || echo 'n/a')"
  backup_count_total="$(kc_v get backup --no-headers 2>/dev/null | wc -l | tr -d ' ')"

  {
    cat <<EOF
# velero-real fixture — captured ${TS} UTC

## Why this fixture exists

PRD-007 §4.3 Layer 4 Backup Copy 评审 P1: 我之前写 "逐对象复制某个备份" 是错的.
Velero 的卷数据存在 BSL 的共享 Kopia 仓库 (\`bucket/<prefix>/kopia/<ns>/...\`),
**按 namespace 共享**, 不按单个 backup 隔离. 这个 fixture 抓真集群对象结构来:

1. 看 BSL 里 \`backups/<name>/\` (metadata) vs \`kopia/<ns>/\` (卷数据) 怎么分布
2. 对比 snapshotMoveData=false (CSI snapshot) vs snapshotMoveData=true (data mover) 的 BSL 差异
3. 验证 Layer 4 复制粒度应该是 \`kopia/<ns>/\` 而不是 \`backups/<name>/\`

## Cluster info

- context        : \`${effective_ctx}\`
- kubectl version: ${k8s_ver}
- velero image   : \`${velero_img}\`
- namespace      : \`${VELERO_NS}\`
- ns filter      : \`${NS_FILTER:-<none, all backups captured>}\`
- captured at    : ${TS} UTC

## BSL summary

\`\`\`
${bsl_summary}
\`\`\`

完整对象 listing 见 [bsl-listing.txt](./bsl-listing.txt)
路径树 (heuristic) 见 [bsl-tree.txt](./bsl-tree.txt)

## Backup summary

- backups total (cluster-wide): **${backup_count_total}**
- backups captured in this fixture: see \`backups/\` dir

### Cross-reference table (\_xref.tsv)

| backup | type | snapshotMoveData | DataUpload# | PVB# | VSC# | phase | errors |
| ------ | ---- | ---------------- | ----------- | ---- | ---- | ----- | ------ |
EOF
    if [[ -f "$OUTPUT/_xref.tsv" ]]; then
      tail -n +2 "$OUTPUT/_xref.tsv" \
        | awk -F'\t' '{printf "| %s | %s | %s | %s | %s | %s | %s | %s |\n",$1,$2,$3,$4,$5,$6,$7,$8}'
    else
      echo "| _(no xref data — dry-run or empty)_ |"
    fi

    cat <<'EOF'

## Layout

```
.
├── README.md                     # this file
├── _xref.tsv                     # backup <-> sub-CR cross reference
├── bsl-listing.txt               # raw `aws s3 ls --recursive` or `az storage blob list` output
├── bsl-tree.txt                  # derived path tree (heuristic)
├── backupstoragelocation/        # BSL CR yamls
├── backups/                      # Backup CR yamls (with .status)
├── restores/                     # Restore CR yamls
├── dataupload/                   # DataUpload (data-mover backup) yamls
├── datadownload/                 # DataDownload (data-mover restore) yamls
├── pvb/                          # PodVolumeBackup (fs-backup) yamls
├── pvr/                          # PodVolumeRestore yamls
└── volumesnapshotcontents/       # CSI VolumeSnapshotContent yamls
```

## Next steps — PRD-007 §4.3 verification

Once you have this fixture:

1. **Check BSL path layout in `bsl-tree.txt`**:
   - Expect to see `<prefix>/backups/<backup-name>/{velero-backup.json,...gz}` for metadata
   - Expect to see `<prefix>/kopia/<source-ns>/...` for data-mover volume data
   - Expect to see `<prefix>/restic/<source-ns>/...` for fs-backup volume data (legacy)

2. **For each backup type (see _xref.tsv), inspect what's actually in BSL**:
   - `csi-snapshot` (snapshotMoveData=false): only metadata in BSL; volume data is in
     cloud-provider region snapshots (NOT in BSL at all → Layer 4 won't copy this!).
   - `data-mover` (snapshotMoveData=true): metadata + kopia repo contents in BSL.
   - `fs-backup` (PodVolumeBackup): metadata + restic/kopia repo in BSL.

3. **Validate the right copy granularity**:
   - WRONG: `rclone copy s3://src/backups/foo s3://dst/backups/foo` (only metadata!)
   - RIGHT: `rclone sync s3://src/{backups/,kopia/<ns>/,restic/<ns>/,...} s3://dst/...`
     per source namespace.

4. **End-to-end test**: pick one data-mover backup, copy required BSL paths via
   rclone to a 2nd BSL, switch Velero to the 2nd BSL, run Restore, verify pod
   comes up with the right data.

## Caveats

- BSL credentials are NOT in this fixture (only paths/sizes). Replay requires the user
  to re-supply creds via env or `aws configure` / `az login`.
- VSC cross-reference uses a heuristic (grep for `velero.io/backup-name` annotation);
  if your Velero version doesn't set this annotation, VSC# will be 0 even when real.
- For GCP / Swift / etc. BSL providers, bsl-listing.txt will show `BSL_LIST_SKIPPED`;
  capture them manually with the appropriate CLI and append to bsl-listing.txt.
- Secrets values are NOT exfiltrated; only CR `.spec` / `.status` is dumped.
EOF
  } > "$readme"

  log "  README -> $readme"
}

# ─────────────────────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────────────────────
main() {
  log "capture-velero-fixture.sh starting (dry-run=$DRY_RUN)"
  log "output dir: $OUTPUT"

  phase_0a_preflight
  mkdir_p "$OUTPUT"
  phase_0b_bsl
  phase_0c_crs
  phase_0d_xref
  phase_0e_readme

  log "DONE. Fixture at: $OUTPUT"
  log "Next: review $OUTPUT/README.md and $OUTPUT/_xref.tsv to validate PRD-007 §4.3 P1."
}

main "$@"
