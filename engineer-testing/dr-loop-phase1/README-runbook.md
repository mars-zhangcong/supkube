# DR-Loop Phase-1 Runbook (cross-cloud forward + reverse)

Ready-to-apply artifact pack for SupKube cross-cloud disaster-recovery loop
validation. Forward direction = **dev (Longhorn) → test (AKS managed-csi)**;
reverse = **test → dev** (failback). Track 3 = files only; no cluster
mutation beyond read-only gets and `kubectl --dry-run=client`.

## Phase-0 facts baked into this pack
- **Backups MUST be fs-backup (Kopia file-level)**: use `snapshotMoveData` /
  `--default-volumes-to-fs-backup` (a.k.a. file-system-backup). **Never** a
  pure CSI snapshot. Backup names **MUST** be prefixed `test-`.
- **Namespace = `dr-test`** (the doc's `test-app` is renamed).
- **fingerprintMode = `enforce`** everywhere (`warn` is half-baked).
- **3 custom Transforms** (no builtin covers them): SC remap, image rewrite,
  LB→ClusterIP. See `transforms/`.

## Directory layout
```
manifests/   00-namespace.yaml 10-postgres.yaml 20-adminer.yaml 30-seed-5rows.yaml
transforms/  sc-remap-fwd.yaml sc-remap-rev.yaml image-rewrite.yaml lb-to-clusterip.yaml
importpolicy/ import-fwd-dev-to-test.yaml import-rev-test-to-dev.yaml
scripts/     verify-count.sh verify-pgsha.sh verify-fingerprint.sh verify-ns-not-terminating.sh
```

## Placeholders to fill before applying
- `transforms/image-rewrite.yaml` → `REPLACE_ME.azurecr.io` = real ACR.
- `importpolicy/*.yaml` → `sourceBSL: shared-azure-bsl` = real BSL name;
  `sourceClusterID` = source cluster's kube-system UID
  (`kubectl --context <ctx> get ns kube-system -o jsonpath='{.metadata.uid}'`).

---

## Exact apply order

### A. Seed the source workload (SOURCE cluster, e.g. dev)
```bash
SRC=aks-jumborca-dev
kubectl --context "$SRC" apply -f manifests/00-namespace.yaml
kubectl --context "$SRC" apply -f manifests/10-postgres.yaml
kubectl --context "$SRC" apply -f manifests/20-adminer.yaml
kubectl --context "$SRC" -n dr-test rollout status statefulset/postgres
kubectl --context "$SRC" apply -f manifests/30-seed-5rows.yaml
kubectl --context "$SRC" -n dr-test wait --for=condition=complete job/seed-5rows --timeout=180s
```

### B. Install the Transforms (in EACH cluster's `supkube` ns)
Order within a TransformSet matters (later-wins on same path); these touch
disjoint resources so order is free, but apply all four:
```bash
for CTX in aks-jumborca-dev aks-jumborca-test; do
  kubectl --context "$CTX" apply -f transforms/
done
```
A TransformSet (PRD-002 two-layer) wraps these refs; the forward restore
references `[sc-remap-fwd, image-rewrite, lb-to-clusterip]`, the reverse
references `[sc-remap-rev, image-rewrite, lb-to-clusterip]`. (TransformSet
wrapper CMs are out of scope for this files-only pack — compose at restore.)

### C. Install ImportPolicies
```bash
kubectl --context aks-jumborca-test apply -f importpolicy/import-fwd-dev-to-test.yaml
kubectl --context aks-jumborca-dev  apply -f importpolicy/import-rev-test-to-dev.yaml
```

---

## The 6-step DR loop

1. **Backup (source)** — fs-backup, name prefixed `test-`:
   `velero backup create test-dr-fwd-001 --include-namespaces dr-test --snapshot-move-data --default-volumes-to-fs-backup` (or the SupKube Backup CR equivalent).
2. **Replicate to BSL** — backup tarball + `.supkube-fingerprint.json` land
   in the shared Azure BSL.
3. **Import (target)** — ImportPolicy controller polls the BSL, validates the
   fingerprint (enforce), and creates the Velero Backup CR in `velero` ns.
4. **Restore (target)** — restore the imported backup with the forward
   TransformSet applied (`resourceModifierRef`).
5. **Verify** — run the 5 Gates below.
6. **Failback (reverse)** — repeat 1–5 with reverse SC remap, source/target
   swapped, backup name `test-dr-rev-001`.

---

## The 5 Gates

| Gate | What it proves | Command | Evidence |
|------|----------------|---------|----------|
| **G1 Import** | backup CR appeared in target `velero` ns, fingerprint passed | `kubectl --context $DST -n velero get backup test-dr-fwd-001`; `BACKEND=az AZ_ACCOUNT=… AZ_CONTAINER=velero ./scripts/verify-fingerprint.sh test-dr-fwd-001` | _paste tarballSHA256 + sourceClusterID_ |
| **G2 Restore done** | restore finished `Completed`, no PartiallyFailed | `kubectl --context $DST -n velero get restore <r> -o jsonpath='{.status.phase}'` | _phase_ |
| **G3 Row count** | exactly 5 rows survived | `CTX=$DST ./scripts/verify-count.sh 5` | _PASS/FAIL_ |
| **G4 Byte-identical** | data SHA matches source | `SRC_CTX=$SRC DST_CTX=$DST ./scripts/verify-pgsha.sh` | _SRC/DST sha256_ |
| **G5 No stuck ns** | LB→ClusterIP worked; cleanup not wedged | `CTX=$DST ./scripts/verify-ns-not-terminating.sh` | _PASS/FAIL_ |

Reverse loop reuses the same Gates with `SRC`/`DST` swapped and
`sc-remap-rev` in place of `sc-remap-fwd`.

---

## Cleanup (MANDATORY — 测试用例.md §0.4, prevents cloud bills)
```bash
for CTX in aks-jumborca-dev aks-jumborca-test; do
  kubectl --context "$CTX" delete ns dr-test 2>/dev/null || true
done
velero backup delete test-dr-fwd-001 test-dr-rev-001 --confirm
velero restore delete <restores> --confirm
# Orphan VolumeSnapshotContent (DeletionPolicy=Retain) + cloud disk snapshots:
kubectl get volumesnapshotcontent -o custom-columns=NAME:.metadata.name,POLICY:.spec.deletionPolicy
# Azure: az snapshot list -g MC_<rg>_<cluster>_<region> ...; az snapshot delete ...
```

## Validation status
All manifests, Transform CMs, and ImportPolicy CRs pass
`kubectl --dry-run=client -o yaml` (see the delivery report). The four
`scripts/*.sh` pass `bash -n` syntax checks.
