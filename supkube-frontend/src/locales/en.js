// English message catalog. Keys are organized by view; nav.* for sidebar,
// common.* for cross-view words (Cancel, Save, status badges, etc.).
// Keep entries in sync with locales/zh-CN.js.

export default {
  app: {
    title: 'SupKube — Kubernetes Data Protection',
    // v0.8.11: split into productName (reactive from branding store) +
    // titleTail (i18n-translated). Old `title` kept for compatibility.
    titleTail: '— Kubernetes Data Protection'
  },
  nav: {
    // v0.9.1.2 menu restructure (Mars 2026-05-26 review):
    //   Sidebar 10 → 7 entries; Observability hub (Advisor + Activity +
    //   Audit Log + Log Viewer) replaces 3 separate entries; Storage +
    //   Snapshot Locations move into Settings → Cluster Management.
    dashboard: 'Dashboard',
    drScores: 'DR Health',
    assistant: 'Assistant',
    observability: 'Observability',           // v0.9.1.2 hub (was 'Backup Advisor')
    applications: 'Applications',
    restorePoints: 'App Restore',             // v0.9.1.2 renamed from 'Restore Points'
    policies: 'Policies',
    transforms: 'Transforms',
    transformSets: 'Transform Sets',
    settings: 'Settings',

    // Retired sidebar entries — kept here because they still appear in
    // breadcrumbs, drawer titles, and deep-link bookmarks. The routes are
    // alive; only the sidebar surface was removed.
    activity: 'Activity',
    restores: 'Restores',                     // legacy — pre-v0.8.0 page
    storage: 'Storage',                       // moved to Settings → Cluster Management
    snapshotLocations: 'Snapshot Locations',  // moved to Settings → Cluster Management
    advisor: 'Backup Advisor'                 // moved to Observability → Advisor tab
  },
  observability: {
    subtitle: 'See what\'s happening across your backup estate — activity, advisor recommendations, audit trail, and logs in one place.',
    tabs: {
      activity: 'Activity',
      advisor: 'Backup Advisor',
      audit: 'Audit Log',
      logs: 'Log Viewer'
    },
    logViewer: {
      comingTitle: 'Log Viewer — coming in v0.8.14',
      comingBody: 'Stream backend / frontend / Velero / Dex logs with faceted filters and one-click "Download Logs" as a debug tarball. Replaces the carpet-bombing of kubectl logs commands.',
      bullet1: 'Faceted filtering: component, severity, pod, time range',
      bullet2: 'Live tail with pause / resume',
      bullet3: 'Per-Action log views (jump from Backup/Restore drawer)',
      bullet4: 'Runbook hints for known error patterns'
    }
  },
  common: {
    cancel: 'Cancel',
    close: 'Close',
    save: 'Save',
    create: 'Create',
    delete: 'Delete',
    edit: 'Edit',
    view: 'View',
    restore: 'Restore',
    validate: 'Validate',
    verify: 'Verify',
    details: 'Details',
    actions: 'Actions',
    name: 'Name',
    status: 'Status',
    type: 'Type',
    namespace: 'Namespace',
    created: 'Created',
    expires: 'Expires',
    yes: 'Yes',
    no: 'No',
    none: 'None',
    loading: 'Loading…',
    refresh: 'Refresh',
    selectAll: 'Select All',
    deleteSelected: 'Delete Selected',
    filterByName: 'Filter by name',
    confirm: 'Confirm',
    success: 'Success',
    error: 'Error',
    warning: 'Warning',
    export: 'Export',
    comingSoon: 'Coming in v0.8',
    filters: 'Filters',
    clearFilters: 'Clear Filters',
    noPermission: 'Your role does not allow this action',
    // v0.9.0.1 — used by reusable text-block modal
    copy: 'Copy',
    copied: 'Copied to clipboard',
    copyFailed: 'Could not copy. Select the text and copy manually.'
  },
  auth: {
    logout: 'Sign out'
  },
  login: {
    loading: 'Loading sign-in options…',
    loadFailed: 'Could not reach authentication service',
    redirecting: 'Redirecting to {name}…',
    exchanging: 'Completing sign-in…',
    success: 'Signed in successfully',
    callbackFailed: 'Sign-in callback failed',
    expired: 'Your session has expired. Please sign in again.',
    demoMode: 'Authentication is disabled (demo mode). Continue without signing in.',
    continue: 'Enter SupKube',
    noProviders: 'No authentication providers configured.'
  },
  phase: {
    Completed: 'Completed',
    Failed: 'Failed',
    InProgress: 'In Progress',
    PartiallyFailed: 'Partial',
    New: 'New',
    Unknown: 'Unknown'
  },
  compliance: {
    Compliant: 'Compliant',
    Unmanaged: 'Unmanaged',
    NonCompliant: 'Non-Compliant',
    InProgress: 'Backup In Progress',
    Empty: 'Empty'
  },
  copilot: {
    title: 'SupKube Copilot',
    welcome: 'Hi — ask me about your cluster DR posture. I can see backup health scores, which apps are at risk, and how to fix them.',
    thinking: 'Thinking…',
    placeholder: 'Ask about backups, DR scores, recommendations…',
    chip1: 'Which apps are most at risk?',
    chip2: 'Why is this app unprotected?',
    chip3: 'How do I raise the overall DR score?',
    errDisabled: 'AI not enabled: backend needs SUPKUBE_ALLOW_AI=1 + Azure OpenAI config.',
    errTimeout: 'Model call failed or timed out — please retry.',
    errAuth: 'Please log in first.',
    errGeneric: 'Request failed: {e}',
    confirm: 'Confirm',
    cancel: 'Cancel',
    actionDone: '✅ Backup created for {ns}: {name}',
    actionFail: '❌ Action failed: {e}',
    actionCancelled: 'Cancelled.'
  },
  drScores: {
    title: 'DR Health Scores',
    desc: 'Per-application backup posture — health score + actionable backup advice across the cluster.',
    avgScore: 'Avg Health',
    ofScored: '{n} of {total} scored',
    atRisk: 'At Risk',
    atRiskSub: 'fragile / critical / unbacked',
    unbacked: 'Unbacked Data',
    unbackedSub: 'stateful but no backup',
    drills: 'Drills Passed',
    drillsSub: 'recovery drill results',
    application: 'Application',
    healthScore: 'Health Score',
    data: 'Data',
    advice: 'Advice',
    ok: 'OK',
    unprotected: 'No Backup',
    protected: 'Protected',
    backupNow: 'Back up now',
    recommendations: 'Recommendations',
    noRecs: 'No issues — healthy.',
    dimensions: 'Score Breakdown',
    footnote: 'Rules {v} · scored at {at}',
    status: {
      noPolicy: 'No Policy',
      neverRan: 'Never Backed Up'
    },
    level: {
      high_resilience: 'High Resilience',
      compliant_low_risk: 'Compliant',
      fragile: 'Fragile',
      critical: 'Critical'
    },
    dim: {
      coverage: 'Coverage',
      resilience: 'Resilience',
      security: 'Security',
      reliability: 'Reliability'
    }
  },
  dashboard: {
    nodes: 'Nodes',
    namespaces: 'Namespaces',
    protected: 'Protected',
    totalBackups: 'Total Backups',
    successful: 'Successful',
    failed: 'Failed',
    recentBackups: 'Recent Backups',
    recentRestores: 'Recent Restores',
    viewAll: 'View All',
    importedTitle: 'Imported Restore Points',
    importedBody: '{count} restore points were imported from {sources} other clusters via shared Storage Profiles.',
    importedBody_one: '{count} restore point was imported from {sources} other cluster via shared Storage Profiles.',
    complianceTitle: 'Protection Compliance',
    complianceWarn: '{count} policies are configured as snapshot-only — these produce restore points that are not durable backups.',
    complianceOk: 'All {count} policies produce durable backups (Snapshot + Export).',
    review: 'Review',
    backupTrend: 'Backup Success Trend',
    storageUsage: 'Restore Points per Storage Profile',
    protectionCoverage: 'Application Protection Coverage',
    noActivity: 'No backup activity in the selected window.',
    noBackups: 'No backups yet.',
    noApps: 'No applications detected.'
  },
  applications: {
    title: 'Applications',
    desc: 'View details or perform actions on applications.',
    workloads: 'Workloads',         // legacy key (detail drawer still uses it)
    components: 'Components',       // v0.8.5: list column header
    restorePoints: 'Restore Points',// v0.8.5: new column
    labels: 'Labels',
    lastBackup: 'Last Backup',
    noRestorePoint: 'No restore point',
    statusFilterAll: 'All',
    showMore: 'Show {count} more labels …',
    showFewer: 'Show fewer labels',
    snapshot: 'Snapshot',
    backup: 'Backup',
    createPolicy: 'Create a Policy',
    // v0.8.9.2: one-click Snapshot dialog
    snapshotDialogTitle: 'Take an instant snapshot of "{ns}"',
    snapshotDialogBody: 'A cluster-local CSI volume snapshot will be created right now as a 24-hour rollback point. No object-store upload, independent of any schedule.',
    snapshotCommentLabel: 'Comment (optional — for later audit)',
    snapshotCommentPlaceholder: 'e.g. before v1.2.3 rollout',
    snapshotConfirm: 'Snapshot now',
    snapshotStarted: 'Snapshot of "{ns}" started — backup name: {name}',
    snapshotFailed: 'Snapshot failed: {msg}',
    snapshotProtectedNs: 'Snapshot of protected namespace is not allowed (kube-system / velero / supkube).'
  },
  backups: {
    phaseFilter: 'Filter by status',
    allPhases: 'All statuses',
    stats: {
      title: 'Backup Statistics',
      total: 'Total',
      success: 'Success',
      failed: 'Failed'
    }
  },
  restorePoints: {
    title: 'Restore Points',
    desc: 'View and manage all Restore Points created in this cluster',
    create: 'Create Restore Point',
    applicationType: 'Application type',
    namespace: 'Namespace',
    virtualMachine: 'Virtual Machine',
    allTypes: 'All Types',
    snapshotManual: 'Snapshot (manual)',
    scheduled: 'Scheduled',
    allSources: 'All Sources',
    local: 'Local',
    imported: 'Imported',
    filterPlaceholder: 'Filter by namespace or name',
    viewing: 'Viewing {filtered} out of {total} Restore Points',
    policy: 'Policy',
    profile: 'Profile',
    createdAt: 'Created At',
    expiresAt: 'Expires At',
    manual: '(manual)',
    // v0.8.9.2: one-click App-Snapshot badge
    manualSnapshot: 'Instant snapshot',
    manualSnapshotBy: 'Taken from the Apps page by {user}',
    deleteTitle: 'Delete Restore Point',
    deleteConfirmBody: 'Delete restore point <code>{name}</code>? Velero will cascade-delete the following:',
    deleteBullet1: 'Backup tarball + metadata in the linked Storage Profile (object storage)',
    deleteBullet2: 'CSI VolumeSnapshot + VolumeSnapshotContent (if CSI mode)',
    deleteBullet3: 'PodVolumeBackups for Filesystem mode (Restic/Kopia)',
    deleteBullet4: 'The Backup CR itself',
    deleteIrreversible: 'This action is irreversible. Data cannot be recovered after deletion.',
    deleteStarted: 'Cascade delete of "{name}" started — Velero is processing the DeleteBackupRequest. The row will disappear once cleanup completes (usually within 30s).',
    intentRestoreTitle: 'Pick a Restore Point to restore "{ns}" from',
    intentRestoreDesc: 'Below are the Restore Points captured for this application. Click ⋮ → Restore on any row to open the restore drawer.',
    // v0.8.9 Role pills (TYPE column on Restore Points)
    // v0.8.12 LBS2: renamed Snapshot/Exported → Local/Cloud.
    // Old keys remain valid via the i18n key (the displayed values change).
    roleSnapshot: 'Local',
    roleExported: 'Cloud',
    roleMetadata: 'Metadata',
    roleUnknown: 'Unknown',
    // v0.8.10 Type column tri-state. v0.8.12 LBS2: same rename.
    typeSnapshot: 'Local',
    typeExported: 'Cloud',
    typeImported: 'Imported',
    typeMetadata: 'Metadata',
    typeUnknown: 'Unknown',
    tooltipLocal: 'Local to this cluster',
    tooltipImported: 'Synced from another cluster',
    // v0.8.10 Policy column unified instant snapshot text
    instantSnapshot: 'Instant Snapshot',
    instantSnapshotGeneric: 'Manually created (no schedule)',
    // v0.8.10 G: Profile column tooltip for Snapshot RPs
    profileSnapshotTooltip: 'Snapshot lives in cluster-local CSI; no export Storage Profile. The metadata tarball lives on the default BSL.',
    // v0.8.6 Backup composition
    dataPath: 'Data Path',
    size: 'Size',
    volume: 'volume',
    volumes: 'volumes',
    dataPathLabel: {
      'csi-snapshot':  'CSI Snapshot',
      'data-mover':    'Data Mover',
      'filesystem':    'Filesystem',
      'metadata-only': 'Metadata Only',
      'unknown':       'Unknown'
    },
    dataPathHelp: {
      'csi-snapshot':  'CSI volume snapshots live on the cluster (CoW). Cross-cluster restore needs Storage Profile sync or Data Mover enabled.',
      'data-mover':    'CSI snapshots moved via Kopia to object storage; cross-cluster restore works; size reflects post-dedup bytes.',
      'filesystem':    'Restic/Kopia walked the filesystem; repository-level dedup; size is real bytes processed.',
      'metadata-only': 'Only K8s YAML was backed up (no PV data); apps must repopulate data after restore.',
      'unknown':       "Couldn't determine data path"
    },
    // v0.8.11 #24: stable Application Items count column
    applicationItems: 'App Items',
    // v0.8.11.2: shorter label used inside the Namespace cell chip
    applicationItemsChip: 'items',
    applicationItemsTooltip: 'Number of real application resources in this backup. Filters out Kubernetes Events, snapshot.storage.k8s.io CRs, and Helm-internal secrets that Velero captures but customers don\'t care about. Stable across runs.',
    sizeTooltip: {
      // v0.8.10.4 actual / reserved format
      actual:        'Actual moved: {size}',
      actualUnavailable: 'unavailable (CSI snapshot deleted by Velero v1.18)',
      reserved:      'Reserved (sum of source PVC capacities): {size}',
      // legacy keys kept for any other surface still using them
      volume:        'Volume data: {size} ({count} volumes, declared PVC capacity)',
      tarball:       'Resource tarball: {size}',
      tarballError:  'Resource tarball size unavailable: {reason}',
      unknown:       'No size data available for this restore point'
    }
  },
  // v0.8.6 Backup detail composition panel
  backupDetail: {
    composition: {
      title: 'Backup Composition',
      hint: 'Data path, volume bytes, and resource manifest tarball for this restore point',
      volumeData: 'Volume Data',
      volumeBreakdown: '{count} volume(s), declared PVC capacity',
      volumeEmpty: 'No PV data was captured in this backup',
      csiCaveat: 'CSI snapshot size reflects declared PVC capacity, NOT actual usage. For deduplicated/real bytes, enable Data Mover or Filesystem backup.',
      tarball: 'Resource Manifest (tar.gz)',
      tarballHelp: 'K8s YAML archive stored in the BSL bucket'
    }
  },
  restores: {
    title: 'Restores',
    fromBackup: 'From Backup',
    create: 'Create Restore',
    phaseFilter: 'Filter by status',
    allPhases: 'All statuses',
    pageSize: 'Items per page',
    pageSizeAll: 'All'
  },
  // PRD-002 v1.3 atomic Transforms — Velero ResourceModifier rule
  // bundles. The rule editor that used to live in TransformSets.vue
  // (single-layer model) now lives here.
  transforms: {
    title: 'Transforms',
    desc: 'A Transform is an atomic Velero ResourceModifier rule bundle. Compose Transforms inside a Transform Set to apply them at restore time. Built-in templates ship with the install; clone to customize.',
    create: 'New Transform',
    createTitle: 'New Transform',
    editTitle: 'Edit Transform: {name}',
    viewTitle: 'View Transform: {name}',
    filterPlaceholder: 'Filter by name or description',
    count: '{n} of {total} Transforms',
    empty: 'No Transforms yet. Built-in templates should appear shortly — refresh if they don\'t.',
    builtin: 'BUILT-IN',
    autoGenerated: 'AUTO',
    autoGeneratedHint: 'Created by Pre-flight "Apply Suggested Fix". Add it to a Transform Set before triggering Restore.',
    clone: 'Clone',
    noDesc: '(no description)',
    description: 'Description',
    descriptionPlaceholder: 'What does this Transform do, and when should it be used?',
    ruleCount: '{n} rule(s)',
    patches: 'patches',
    rules: 'Rules',
    addRule: 'Add Rule',
    addPatch: 'Add Patch',
    groupResource: 'Group / Resource',
    namespaces: 'Namespaces',
    allNamespaces: 'All namespaces (leave empty)',
    resourceNameRegex: 'Resource name regex',
    resourceNameRegexHint: 'Optional. e.g. ^my-app-.*$',
    pathPlaceholder: '/spec/ports/0/nodePort',
    valuePlaceholder: 'Value (string / number / JSON)',
    yamlPreview: 'View as YAML',
    savedToast: 'Saved Transform "{name}"',
    createdToast: 'Created Transform "{name}"',
    deletedToast: 'Deleted Transform "{name}"',
    deleteTitle: 'Delete Transform',
    deleteConfirm: 'Delete Transform "{name}"? It will be removed from the catalog. Transform Sets referencing it block the delete.',
    // 409 response from DELETE /transforms/:name
    deleteBlockedTitle: 'Transform is still referenced',
    deleteBlockedBody: 'Transform "{name}" is referenced by {n} Transform Set(s): {sets}. Remove the references first, then retry the delete.',
    fromConflictTitle: 'Authoring from Pre-flight conflict: {kind}',
    fromConflictHint: 'The rule editor was pre-loaded with the conflict context. Save this Transform and add it to a Transform Set, then re-open the Restore drawer.'
  },
  // PRD-002 v1.3 Transform Sets — now CONTAINERS that reference atomic
  // Transforms. No inline rule editor here; pick Transforms from the
  // catalog (Transforms page) and optionally set defaults for ${VAR}
  // placeholders.
  transformSets: {
    title: 'Transform Sets',
    desc: 'A Transform Set composes one or more atomic Transforms and supplies defaults for $VAR placeholders (compile.go substitution syntax). Velero applies the compiled rules to a Restore. Built-in sets ship with the install; clone to customize.',
    create: 'New Transform Set',
    createTitle: 'New Transform Set',
    editTitle: 'Edit Transform Set: {name}',
    viewTitle: 'View Transform Set: {name}',
    filterPlaceholder: 'Filter by name or description',
    count: '{n} of {total} Transform Sets',
    empty: 'No Transform Sets yet. Built-in templates should appear shortly — refresh if they don\'t.',
    builtin: 'BUILT-IN',
    clone: 'Clone',
    noDesc: '(no description)',
    description: 'Description',
    descriptionPlaceholder: 'What does this Transform Set do, and when should it be used?',
    refCount: '{n} transform ref(s)',
    refs: 'Transforms',
    refsLabel: 'Referenced Transforms',
    refsPlaceholder: 'Pick one or more Transforms…',
    refsEmpty: 'Pick at least one Transform from the catalog.',
    manageTransforms: 'manage Transforms…',
    defaults: 'Defaults',
    defaultsHint: 'Values supplied for $VAR placeholders inside the referenced Transforms. Per-ref overrides not yet honored — set them here.',
    defaultsKey: 'VAR',
    defaultsValue: 'value',
    addDefault: 'Add default',
    savedToast: 'Saved Transform Set "{name}"',
    createdToast: 'Created Transform Set "{name}"',
    deletedToast: 'Deleted Transform Set "{name}"',
    deleteTitle: 'Delete Transform Set',
    deleteConfirm: 'Delete Transform Set "{name}"? Any Restores currently referencing it will fail validation until they\'re re-pointed at another Transform Set.'
  },
  // v0.8.0 Activity page — unified Action stream
  activity: {
    title: 'Activity',
    desc: 'All actions performed in this cluster — Backups, Restores, and (v0.9) Exports. Click any row for full phase-by-phase progress.',
    actionDurationsTitle: 'Action Durations',
    legendRunning: 'running',
    legendCompleted: 'completed',
    legendFailed: 'failed',
    actionsHeader: 'Actions',
    allTypes: 'All Types',
    allStatuses: 'All Statuses',
    filterPlaceholder: 'Filter by name, namespace, policy…',
    empty: 'No actions yet. Trigger a Backup or Restore from another page to see it here.',
    phases: 'Phases',
    errors: 'errors',
    warnings: 'warnings',
    kpi: {
      total: 'total actions',
      completed: 'completed actions',
      failed: 'failed actions',
      skipped: 'skipped actions',
      avgDuration: 'avg duration',
      liveArtifacts: 'live artifacts',
      retiredArtifacts: 'retired artifacts'
    },
    status: {
      running: 'Running',
      completed: 'Completed',
      failed: 'Failed',
      partial: 'PartiallyFailed',
      skipped: 'Skipped',
      unknown: 'Unknown'
    },
    detail: {
      title: 'Action Details',
      // v0.8.10.2 per-entity H1 titles — parent passes entityTitleKey
      // to the drawer so the context shows correctly (e.g. opening from
      // Restore Points should say "Restore Point Details", not "Action
      // Details").
      titleRestorePoint: 'Restore Point Details',
      titleApplication: 'Application Details',
      titlePolicy: 'Policy Details',
      detailsSection: 'Details',
      empty: 'No action selected.',
      type: 'Type',
      status: 'Status',
      phases: 'Phases',
      protectedObject: 'Protected Object',
      policy: 'Policy',
      restorePoint: 'Restore Point',
      targetNamespace: 'Target Namespace',
      artifacts: 'Artifacts',
      artifactName: 'Artifact Name',
      artifactQty: 'Quantity',
      start: 'Start',
      end: 'End',
      duration: 'Duration',
      health: 'Health',
      viewYaml: 'View Action YAML',
      hideYaml: 'Hide YAML',
      errorsHeader: 'Errors ({n})',
      noBackupErrors: 'Backup CR reports errors but no matching DataUpload / PodVolumeBackup failure messages found. The detail likely lives in BSL <backup>-results.gz — v0.9 will fetch these via DownloadRequest.',
      warningsHeader: 'Warnings ({n})',
      loadingDetail: 'Loading detail from object storage…',
      fetchErrorPrefix: 'Could not fetch detail',
      // v0.9.1.10 (#103): when the per-resource detail fetch from object
      // storage fails, never leave the user stranded at "N errors". Explain
      // where the messages live and give the exact command to read them.
      detailFetchFallbackHint: 'Velero stores per-resource messages in object storage (BSL). SupKube could not retrieve them automatically (reason above). Read them directly with:',
      // v0.8.10 Plan-B paired-policy fields
      // v0.8.10.5: dropped "half" suffix — customers found it ambiguous
      // ("half of what?"). Plain "Snapshot" / "Export" is unambiguous
      // because the action card already shows backup type = "Backup".
      roleSnapshot: 'Snapshot',
      roleExport: 'Export',
      pairedWith: 'Paired with',
      pairedTooltip: 'This Backup is one half of a dual-policy run. The Local half writes to the in-cluster MinIO BSL (fast recovery); the Cloud half writes to your Cloud Storage Profile (off-site DR). They run serially — Local first, Cloud ~30s later. Click the linked Backup to see the other half.',
      policyRunAt: 'Policy Run At',
      policyRunAtTooltip: 'The logical timestamp both halves of this policy run share. Equals the Snapshot half’s creation time. Note: the Export half’s own data was captured ~30s LATER (Velero v1.18 limitation — see USER_MANUAL §6.x). A future upstream fix (preserveSnapshotsAfterUpload) will close this gap to zero.',
      // v0.8.10.1 grouped artifact breakdown
      applicationItems: 'Application Items',
      infrastructureItems: 'Infrastructure',
      veleroTotalItems: 'Velero total',
      veleroCountTooltip: 'Velero’s raw progress.totalItems counter. Includes Kubernetes Events generated during the snapshot operation and the snapshot.storage.k8s.io CRs themselves — these inflate the count without representing real application data. SupKube’s "Application Items" filters them out for stability across runs.',
      viewYamlItem: 'View YAML'
    }
  },
  restoreDrawer: {
    title: 'Restore From Backup',
    backToDetails: 'Back To Details',
    // v0.9.1.5: Imported RP badge + "restore as-is" mode banner.
    // Mars demo 2026-05-26: customers couldn't tell imported vs local
    // RPs, and the Restore button was disabled for imported RPs because
    // ListBackupArtifacts returns empty (source ns doesn't exist locally).
    // These strings surface both bits clearly.
    importedBadge: 'Imported',
    importedFrom: 'Synced from another cluster ({cluster})',
    importedUnknownSource: 'another cluster',
    specWholeTarball: 'Whole tarball',
    importedAsIsTitle: 'Whole-tarball restore (artifact-level selection unavailable for imported RPs)',
    importedAsIsBody: 'Velero will restore every resource in the backup tarball server-side. Client-side selection requires reading the BSL tarball metadata — coming in v0.9.1.7 (task #91). For now: Submit restores everything as-is.',
    // Tooltip strings shown on hover of a disabled Restore button so the
    // customer can self-diagnose. Each maps to one condition in cantSubmitReason.
    disabled: {
      noBackup:                 'No backup selected.',
      noTargetNs:               'Select a target namespace first.',
      noRestoreName:            'Restore name is empty.',
      confirmOverwrite:         'Tick "I understand — delete and recreate the namespace" first.',
      noArtifactsSelected:      'Select at least one artifact to restore.',
      // PRD-001 v2 (#104, finding #1): blockers are NEVER ignorable —
      // tell the user exactly how many they still need to resolve.
      mustResolveBlocker:       'Resolve {n} blocker(s) before Restore — use "Apply Fix" or "Go to Transform".',
      mustAcknowledgeWarning:   'Acknowledge {n} warning(s) — tick "Ignore this warning" on each row to proceed.',
      preflightRunning:         'Pre-flight check is running…',
      submitting:               'Restore is being submitted…'
    },
    // v0.9.0 MC3: cross-cluster restore section (only rendered when ≥2 clusters registered)
    targetClusterTitle: 'Target Cluster',
    targetClusterDesc: 'Where to restore. Pick "this-cluster" for an in-place restore (default); any other registered cluster receives a cross-cluster restore — the backup metadata must already be synced via a shared BSL.',
    crossClusterBanner: 'Cross-cluster restore selected. The target cluster\'s Velero will sync backup metadata from the shared BSL (~60s) before restore begins. The target cluster must have the same BSL configured.',
    appNameTitle: 'Application Name',
    appNameDesc: 'Select a namespace to restore into. The contents of the selected namespace will be overwritten with the restored application.',
    selectNs: 'Select a namespace',
    originalNs: 'Original namespace',
    createNewNs: 'Create A New Namespace',
    newNsLabel: 'New Namespace',
    newNsPlaceholder: 'my-restored-ns',
    newNsRequired: 'Please enter a namespace name',
    newNsCreated: 'Namespace "{name}" created',
    overwriteTitle: 'In-place restore will wipe the existing namespace',
    overwriteDesc: 'SupKube will delete namespace "{ns}" and all of its resources before restoring. This is required because Velero defaults to skipping resources that already exist, which would make the restore a no-op.',
    overwriteConfirm: 'I understand — delete and recreate the namespace',
    optionsTitle: 'Optional Restore Settings',
    restoreName: 'Restore Name',
    existingPolicy: 'Behavior when a resource already exists',
    policySkip: 'Skip existing resources',
    policySkipHint: 'Safe but the restore may be a no-op if the target namespace already contains the same objects.',
    policyUpdate: 'Update existing resources',
    policyUpdateHint: 'Overwrites mutable objects like ConfigMap/Secret. Note: PVC data is not updated because PVs are already bound.',
    specTitle: 'Spec',
    specRestoring: 'Restoring {selected} of {total} spec artifacts',
    selectAllArtifacts: 'Select All',
    deselectAllArtifacts: 'Deselect All',
    viewYaml: 'View YAML',
    noArtifacts: 'No restorable artifacts found in this namespace.',
    artifactsLoadError: 'Failed to load artifact list',
    submitted: 'Restore "{name}" submitted — Velero is processing',
    // v0.9.1.10 (#105): persistent, explicit submit feedback. The old 3s
    // toast vanished and left the user on the Restore Points list with no
    // sign anything was happening ("什么都没发生"). These messages back a
    // persistent notification + auto-routing to the live Activity stream.
    submittedTitle: 'Restore initiated',
    submittedBody: 'Velero is restoring "{name}" into namespace "{ns}". Opening the live Activity stream so you can watch its progress…',
    submittedXTitle: 'Cross-cluster restore initiated',
    submittedXBody: 'Restore "{name}" is now running on cluster "{cluster}". Switch to that cluster and open Activity to watch its progress.',
    // v0.8.2 Transform Set integration
    transformSet: 'Transform Set',
    transformSetNone: '(none — restore as-is)',
    transformSetManage: 'manage…',
    transformSetBuiltinBadge: 'BUILT-IN',
    fixApplied: 'Resolved by Transform "{name}"',
    // PRD-002 v1.3: Apply Fix now creates an atomic Transform (was a
    // Transform Set). The user must add it to a Transform Set before
    // triggering the Restore — we surface that via the toast + nav.
    fixCreatedToast: 'Transform "{name}" created. Review it and add to a Transform Set, then re-open the Restore drawer.',
    fixGoToTransform: 'Open Transform',
    // v0.7.12 Pre-flight
    preflightTitle: 'Pre-flight Check',
    preflightRerun: 'Re-run',
    preflightDeepCheck: 'Deep check (image registry DNS)',
    preflightDeepHint: 'adds ~8s, opt-in',
    preflightChecking: 'Scanning target cluster for conflicts…',
    preflightError: 'Pre-flight failed',
    preflightClean: 'No conflicts detected',
    preflightCleanDesc: 'The target namespace can accept this backup as-is.',
    preflightZeroConflicts: '0 conflicts',
    preflightBlockerCount: '{n} blocker(s)',
    preflightWarningCount: '{n} warning(s)',
    preflightMixed: '{blockers} blocker(s), {warnings} warning(s)',
    suggestedFix: 'Suggested fix (preview)',
    applyFix: 'Apply Fix',
    // PRD-001 v2 (#104, finding #1): kept for backwards-compat with any
    // older snapshots/screenshots referencing the key. Not rendered in
    // v0.10+ — the Checklist replaced the single global override.
    ignoreBlockers: 'I understand — ignore blockers and try anyway',
    // PRD-001 v2 (#104) — Checklist + Go-to-Transform
    severityBlocker: 'Blocker (must resolve)',
    severityWarning: 'Warning (may ignore)',
    mustResolveBlocker: 'Resolve {n} blocker(s) before Restore',
    mustAcknowledgeWarning: '{n} warning(s) need explicit acknowledgement',
    allConflictsResolved: 'All conflicts resolved — ready to restore.',
    allConflictsHandled: 'All conflicts handled ({n} warning(s) ignored).',
    matchingTransformSets: 'Matching TransformSets',
    ignoreThisWarning: 'Ignore this warning',
    ignoreBlockerDisabled: 'Blockers cannot be ignored — resolve via Transform',
    ignoreWarningTitle: 'Ignore this warning?',
    ignoreWarningConfirm: 'Ignoring "{kind}" means the restore may partially fail for affected artifacts. Continue?',
    ignoreWarningOk: 'Yes, ignore',
    goToTransform: 'Go to Transform',
    conflictKind: {
      NodePortCollision: 'NodePort already allocated',
      ClusterIPCollision: 'clusterIP already allocated',
      LoadBalancerIPCollision: 'LoadBalancer IP already claimed',
      IngressHostCollision: 'Ingress host already in use',
      PVBinding: 'PVC pins a specific PV',
      StorageClassMissing: 'StorageClass not on target cluster',
      ImageRegistryUnreachable: 'Image registry unreachable',
      ServiceAccountMissing: 'ServiceAccount missing',
      NodeSelectorMissing: 'NodeSelector labels missing',
      PriorityClassMissing: 'PriorityClass missing'
    }
  },
  policies: {
    title: 'Policies',
    desc: 'Policies are used to automate your data management workflows. They combine actions (e.g., Snapshot), a frequency or schedule, and a label-based selection criteria for the resources you want to manage.',
    create: 'Create New Policy',
    validation: 'Validation',
    valid: 'Valid',
    invalid: 'Invalid',
    resources: 'Resources',
    namespaceTooltip: 'Namespace: {name}',
    storageLocationCol: 'Storage Location',
    bslLocalTooltip: 'Local copy → {name}',
    bslCloudTooltip: 'Off-site copy → {name}',
    action: 'Action',
    // v0.9.1.12 PRD-009 (Mars 2026-06-01): align with Kasten K10 official
    // wording — drop L1/L2 vocabulary, restructure as "Snapshot (always)" +
    // "Enable Backups via Snapshot Exports" toggle. Internal i18n keys are
    // preserved for code stability; user-visible values follow Kasten 1:1
    // (note: "Backups" plural + "Exports" plural per K10 docs string).
    actionSnapshot: 'Snapshot',
    actionSnapshotExport: 'Snapshot + Export',
    frequency: 'Frequency',
    onDemand: 'On Demand',
    restorePoints: 'Restore Points',
    lastRunTime: 'Last Run Time',
    lastRunStatus: 'Last Run Status',
    paused: 'Paused',
    active: 'Active',
    scheduled: 'Scheduled',
    viewing: 'Viewing {filtered} out of {total} policies',
    allActions: 'All Actions',
    allFrequencies: 'All Frequencies',
    viewTitle: 'Policy: {name}',
    revalidate: 'Revalidate',
    revalidateOk: 'Policy spec is valid',
    revalidateFail: 'Policy spec looks malformed — check schedule and resource selector',
    editYaml: 'Edit YAML',
    editComingSoon: 'Full Edit form coming in v0.8. Showing YAML viewer for now.',
    editPolicy: 'Edit Policy: {name}',
    editFetchFailed: 'Could not fetch the latest spec. Editing the cached copy from the list.',
    nameImmutable: "Policy name can't change — Velero treats a rename as a new policy and the backup history doesn't migrate",
    yamlReadonlyHint: 'YAML viewer is read-only in this preview. Apply changes via kubectl or wait for v0.8.',
    runOnce: 'Run Once',
    runOnceConfirmTitle: 'Run Policy Once?',
    runOnceConfirmBody: 'Trigger an immediate backup using policy "{name}". A new Restore Point will be created with the same template as the scheduled runs.',
    runOnceStarted: 'Backup "{backup}" started — visible on Restore Points page.',
    newPolicy: 'New Policy',
    nameHelp: 'The display name for this policy',
    comments: 'Comments',
    commentsPlaceholder: 'Optional notes for teammates',
    actionHelp: 'The action that should be taken when this policy is executed',
    // v0.9.1.12 PRD-009: Kasten K10 model — Snapshot is always taken
    // (mandatory); user only chooses whether to also export to a Location
    // Profile (bucket). Wording is verbatim from Kasten docs:
    // "Enable Backups via Snapshot Exports" + "Export Location Profile".
    snapshotSectionTitle: 'Snapshot',
    snapshotSectionHelp: 'Local CSI Volume Snapshot of the namespace — always taken. Provides fast in-cluster rollback.',
    snapshotAlwaysOnPill: 'Snapshot (CSI)',
    enableExportLabel: 'Enable Backups via Snapshot Exports',
    enableExportHelp: 'When on, each snapshot is exported to a Location Profile (bucket) for off-site / cross-cluster recovery.',
    enableExportOnText: 'Export enabled',
    enableExportOffText: 'Snapshot only (no export)',
    exportLocationProfile: 'Export Location Profile',
    exportLocationProfileHelp: 'Where snapshot data is exported. Pick a Cloud Storage Profile (Local BSL can also serve as an export target).',
    backupFrequency: 'Backup Frequency',
    frequencyCustom: 'Custom',
    frequencyOnDemand: 'On Demand',
    onDemandNotice: 'Policy is paused — no automatic runs by cron. Only the Run Once action on the policy row will trigger a backup.',
    volumeMode: 'Volume Mode',
    volumeModeHelp: 'CSI requires the namespace PVCs to use a snapshot-capable StorageClass',
    // v0.8.7: 4-way Data Path
    dataPath: 'Data Path',
    dataPathHelp: 'How volume data is captured. Left column is for snapshot-only mode; right column unlocks when "Enable Backups via Snapshot Exports" is on.',
    dataPathSection: {
      snapshotOnly: 'Snapshot only',
      snapshotOnlySub: 'In-cluster CSI snapshot, no export',
      snapshotExport: 'Snapshot Export',
      snapshotExportSub: 'CSI snapshot + upload to Export Location Profile'
    },
    dataPathChoice: {
      csi: 'CSI Snapshot',
      mover: 'Data Mover',
      fs: 'Filesystem',
      meta: 'Metadata Only'
    },
    dataPathTip: {
      csi: 'Storage-layer CoW, cluster-local. Snapshots stay on this cluster. Cross-cluster restore needs Storage Profile sync or switch to Data Mover.',
      mover: 'CSI snapshot then Kopia upload to the BSL bucket. Cross-cluster restore works, content is deduplicated. Requires node-agent.',
      fs: 'Restic / Kopia walks the filesystem. Cross-cluster restore works, deduplicated. Requires node-agent. Use when CSI is not available.',
      meta: 'Only K8s YAML is captured (no PV data). Apps must repopulate data after restore.'
    },
    dataPathPrereq: 'Requires the node-agent DaemonSet (helm install velero --set deployNodeAgent=true). See USER_MANUAL §16.',
    resourcesHelp: 'Namespaces to include in the backup. Leave empty to include all.',
    namespacesPlaceholder: 'Select or type namespaces',
    csiCheck: 'CSI Compatibility',
    csiBlocker: 'Switch to Filesystem mode, or migrate the listed PVCs to a CSI-snapshot-capable StorageClass.',
    schedule: 'Schedule',
    // v0.8.12 LBS2: Snapshot/Export terminology replaced by Local/Cloud.
    // Keys retain old names for code stability; visible values updated.
    snapshotSchedule: 'Local Backup Schedule',
    snapshotRetention: 'Local Retention',
    exportSchedule: 'Cloud Backup Schedule',
    exportRetention: 'Cloud Retention',
    storageProfile: 'Cloud Storage Profile',
    storageProfilePlaceholder: 'Pick a Cloud Storage Profile',
    storageProfileAuto: 'Auto — use the default cloud location',
    storageProfileEmpty: 'No Cloud Storage Profiles configured. Add one on the Storage Locations page first.',
    includedNamespaces: 'Included Namespaces',
    snapshot: 'Local Backup',
    export: 'Cloud Backup',
    alwaysOn: 'Always on',
    enable: 'Enable',
    snapshotOnlyWarn: 'Without Snapshot Exports, this policy keeps only an in-cluster copy — vulnerable to cluster-wide failures. For DR you need to enable Backups via Snapshot Exports and pick an Export Location Profile.',
    // v0.8.12 LBS3: Object Lock warning + OK hint shown under the Cloud BSL selector
    noObjectLockWarn: 'This Cloud BSL has no Object Lock. Backups can be deleted before retention expires — ransom/insider threats unprotected. Enable bucket-level immutability (S3 Object Lock / Azure Immutable Blob) for 3-2-1-1-0 compliance.',
    objectLockOk: 'Object Lock active ({mode}). Backups become immutable for their retention period.',
    // v0.8.12 LBS4: 3-2-1-1-0 score preview in the Policy Wizard footer
    scorePreviewHint: 'How this policy will contribute to your 3-2-1-1-0 score.',
    scoreNotes: {
      notDual: 'Switch to L2 Local + Cloud to satisfy this rule.',
      localMissing: 'Local Backup Store not enabled. Settings → Storage Locations → Enable.',
      cloudMissing: 'Pick an Available Cloud BSL to satisfy this rule.',
      noCloudPicked: 'Select a Cloud Storage Profile.',
      cloudUnavailable: 'The selected Cloud BSL is not Available — check credentials.',
      enableLock: 'Enable Object Lock on the chosen Cloud BSL (S3 Object Lock / Azure Immutable Blob) for ransom protection.',
      pickNs: 'Pick at least one namespace to include.',
      pickName: 'Give the policy a name.'
    },
    disableExportTitle: 'Skip Cloud Backup?',
    disableExportBody: 'A Local-only policy will not survive cluster loss. Continue anyway?',
    disableExportConfirm: 'Yes, Local only',
    disableExportCancel: 'Keep Cloud enabled',
    protectionLevel: 'Protection Level',
    pause: 'Pause',
    resume: 'Resume',
    // ─── Agent D / Import Policy 2026-06-01 ─────────────────────────────
    // 在 Create Policy drawer 顶部新增 Action Type 选择：Snapshot vs Import。
    // Snapshot Policy 在源集群做备份；Import Policy 在目标集群从共享 BSL
    // 拉新 RP（持续 DR）。i18n key 严格按 SHARED CONTRACT。
    actionType: 'Action Type',
    actionTypeHelp: 'Snapshot Policy backs up in the source cluster; Import Policy pulls new RPs from a shared BSL in the target cluster — continuous DR.',
    actionTypeSnapshot: 'Snapshot Policy',
    actionTypeImport: 'Import Policy'
  },
  // Import Policy 表单 + 列表 + RP 行的 i18n key (SHARED CONTRACT, Agent D)
  importPolicy: {
    title: 'Import Policy',
    sourceBSL: 'Source Storage Location',
    sourceBSLHelp: 'The shared BSL this target cluster will scan for new Restore Points pushed by another (source) cluster.',
    mode: 'Import Mode',
    modeContinuous: 'Continuous',
    modeScheduled: 'Scheduled',
    continuousInterval: 'Poll Interval',
    continuousIntervalHelp: 'How often the target cluster scans the BSL for new RPs. Smaller = better RPO + more BSL API cost. 30s is the technical floor; beats Kasten\'s 5min.',
    cron: 'Cron Schedule',
    cronHelp: '5-field cron, on parity with Kasten.',
    sourceClusterID: 'Source Cluster Filter (optional)',
    sourceClusterIDHelp: 'When the BSL is written to by multiple clusters, restrict which source to accept; empty = accept all.',
    fingerprintMode: 'Fingerprint Verification',
    fingerprintEnforce: 'Enforce (recommended)',
    fingerprintWarn: 'Warn only',
    fingerprintDisabled: 'Disabled',
    fingerprintHelp: 'Enforce mode requires both clusters to share a fingerprint secret — cross-cluster default; missing signature is rejected. Warn accepts but tags the row "Unverified". Disabled skips verification entirely (only for same-source trusted BSLs).',
    rpoEstimate: '🛡 Worst-case RPO ≤ {min} min',
    rpoEstimateNote: '= source backup interval + import poll interval',
    importedChip: 'Imported',
    signatureValid: '✅ Signature valid · source: {cluster}',
    signatureMissing: '⚠ Unsigned · source unknown',
    signatureInvalid: '❌ Signature invalid · refusing import',
    actionRunOnce: 'Run Once',
    actionPause: 'Pause',
    actionResume: 'Resume',
    intervalTooShort: 'Interval must be ≥ 30s (BSL API cost guard)',
    cronInvalid: 'Invalid cron expression (5 fields)',
    typeChip: 'Import',
    typeChipTooltip: 'Import Policy — pulls Restore Points from a shared BSL into this cluster.',
    sourceBSLPlaceholder: 'Pick a Storage Profile to import from',
    sourceClusterIDPlaceholder: 'Empty = accept all source clusters',
    cronPlaceholder: '*/15 * * * *',
    intervalPlaceholder: '30s',
    pollPreset30s: '30s',
    pollPreset60s: '60s',
    pollPreset2m: '2min',
    pollPreset5m: '5min',
    schedulePreset5m: 'Every 5 min',
    schedulePreset15m: 'Every 15 min',
    schedulePreset1h: 'Hourly',
    schedulePresetDaily: 'Daily 02:00',
    schedulePresetWeekly: 'Weekly Sun 03:00',
    schedulePresetCustom: 'Custom cron',
    createdToast: 'Import Policy "{name}" created',
    paused: 'Import Policy "{name}" paused',
    resumed: 'Import Policy "{name}" resumed',
    runOnceStarted: 'Import Policy "{name}" — run-once dispatched'
  },
  // 后端返回的错误码 -> 前端 toast 文案（Agent D 完整一套）
  errors: {
    ERR_IMPORTPOLICY_BSL_NOTFOUND:       "Source BSL '{bsl}' not found",
    ERR_IMPORTPOLICY_CRON_INVALID:       'Invalid cron expression — must be 5 fields',
    ERR_IMPORTPOLICY_INTERVAL_TOO_SHORT: 'Poll interval too short — minimum is 30s',
    ERR_FINGERPRINT_REQUIRED:            'Fingerprint required but missing — enforce mode',
    ERR_FINGERPRINT_HMAC_INVALID:        'Fingerprint HMAC invalid — possible tampering or wrong shared secret'
  },
  // v0.9.0 Mode Switcher in sidebar
  clusterSwitcher: {
    mcm: 'Multi-Cluster Manager',
    clusters: 'Clusters',
    addCluster: '+ Add Cluster',
    manage: 'Manage clusters'
  },
  // v0.9.0 MC1: Cluster registry — Settings → Clusters tab + Add Cluster wizard
  clusters: {
    title: 'Clusters',
    subtitle: 'All Kubernetes clusters SupKube is managing — local + any remote clusters added for cross-cluster restore.',
    add: 'Add Cluster',
    primary: 'Primary',
    secondary: 'Secondary',
    nodes: 'nodes',
    lastChecked: 'checked',
    remove: 'Remove',
    removeTitle: 'Remove Cluster?',
    removeConfirm: 'Remove cluster "{name}"? Backups stored in this cluster\'s BSLs are NOT deleted, only the SupKube registry entry. The cluster can be re-added later.',
    removeFailed: 'Remove failed',
    removedToast: 'Cluster "{name}" removed',
    createdToast: 'Cluster "{name}" added',
    switchedTo: 'Switched to {name}',
    actions: {
      switch: 'Switch to this cluster',
      test: 'Test Connection',
      remove: 'Remove',
      viewKubeconfig: 'View Kubeconfig',
      installVelero: 'Install Velero (helm)',
      installSupkube: 'Install SupKube (helm)'
    },
    modal: {
      kubeconfigTitle: 'Kubeconfig — {name}',
      kubeconfigIntro: 'Where the kubeconfig for this cluster is stored, and the kubectl command to retrieve it from your workstation. The raw kubeconfig is never sent to the browser — copy the command below and run it locally.',
      installVeleroTitle: 'Install Velero on {name}',
      installVeleroIntro: 'Velero v1.18 (matches the version SupKube bundles) plus the CSI / Azure / AWS plugins. Run from your workstation with kubectl/helm configured against the target cluster.',
      installSupkubeTitle: 'Install SupKube on {name}',
      installSupkubeIntro: 'Deploy a SupKube instance on this cluster (e.g. for a true multi-site setup where each cluster has its own UI). Optional — for cross-cluster restore the central SupKube instance is sufficient.'
    },
    empty: {
      title: 'Add another cluster to unlock Multi-Cluster Manager',
      body: 'Once you add a second cluster, SupKube enables cross-cluster restore, aggregated DR topology, and (in v0.9.x+) shared policies. Cluster registration uploads a kubeconfig — store it in a K8s Secret in the supkube namespace.'
    },
    wizard: {
      title: 'Add Cluster',
      step1: 'Identify',
      step2: 'Connect',
      step3: 'Verify',
      name: 'Name',
      nameHint: 'Lowercase letters, digits, "-", ".". Becomes the Cluster CR\'s metadata.name.',
      nameInvalid: 'Must be DNS-1123 (lowercase, digits, dashes, dots).',
      nameReserved: 'Names "this-cluster" and "_mcm" are reserved.',
      displayName: 'Display Name',
      type: 'Type',
      typePrimary: 'Primary',
      typeSecondary: 'Secondary',
      typeHint: 'Primary = hosts SupKube itself. Secondary = remote cluster registered for cross-cluster operations.',
      description: 'Description',
      kubeconfig: 'Kubeconfig',
      chooseFile: 'Choose File',
      replaceFile: 'Replace File',
      kubeconfigHint: 'Upload the kubeconfig file. Stored as a K8s Secret in the supkube namespace — never sent to logs or telemetry.',
      fileTooLarge: 'Kubeconfig file too large (>256 KiB).',
      context: 'Context',
      contextPlaceholder: '(use current-context)',
      contextHint: 'Empty = use the kubeconfig\'s current-context.',
      testButton: 'Test Connection',
      k8sVersion: 'Kubernetes version',
      nodeCount: 'Node count',
      veleroMissing: 'Velero not detected. Install Velero v1.18+ on this cluster before using it for backup/restore.',
      back: 'Back',
      next: 'Next',
      add: 'Add Cluster',
      added: 'Cluster "{name}" added successfully.',
      addFailed: 'Failed to add cluster'
    }
  },
  // v0.8.13 HC4: Settings → Plugins tab
  plugins: {
    intro: 'Optional SupKube components — Velero (required), embedded Dex, in-cluster MinIO Local Backup Store. Toggle each by copying the helm upgrade command and running it from your workstation.',
    installed: 'Installed',
    notInstalled: 'Not Installed',
    required: 'Required',
    enableCmdLabel: 'Enable command',
    disableCmdLabel: 'Disable command',
    copy: 'Copy',
    copied: 'Command copied to clipboard',
    copyFailed: 'Could not copy. Select the command and copy manually.',
    cmdNote: 'Run from your workstation against the cluster where SupKube lives. `--reuse-values` preserves your other customisations.',
    empty: 'No plugins reported. The backend may be running an older version.'
  },
  // v0.9.0.2: Multi-Cluster Manager Dashboard (/multicluster route)
  mcm: {
    title: 'Multi-Cluster Manager',
    subtitle: 'Aggregated view of every cluster SupKube is managing. Click any row to drop into that cluster\'s dashboard.',
    clustersTitle: 'Clusters',
    clickHint: 'Click a row to switch context.',
    fetchFailed: 'Failed to load multi-cluster summary',
    totals: {
      clusters: 'Clusters',
      apps: 'Applications',
      policies: 'Policies',
      restorePoints: 'Restore Points',
      unhealthy: 'unhealthy',
      allHealthy: 'all healthy'
    },
    col: {
      name: 'Cluster',
      phase: 'Phase',
      k8s: 'K8s',
      nodes: 'Nodes',
      apps: 'Apps',
      policies: 'Pol',
      rps: 'RPs',
      lastBackup: 'Last Backup'
    }
  },
  // v0.8.12.5: DR Topology Dashboard hero card
  topology: {
    title: 'DR Topology',
    subtitle: 'Where each namespace is backed up — and how protected the cluster is overall.',
    clusters: 'clusters',
    policies: 'policies',
    policiesShort: 'pol',
    restorePoints: 'restore points',
    restorePointsShort: 'RPs',
    namespacesShort: 'ns',
    nsCovered: 'ns covered',
    nodes: 'nodes',
    current: 'Primary',
    local: 'Local',
    cloud: 'Cloud',
    allNamespaces: 'All Namespaces',
    more: 'more',
    lastBackup: 'last',
    neverBackedUp: 'no backup yet',
    minAgo: 'min ago',
    inactiveBSLs: 'inactive BSLs (no policy uses them)',
    scoreTitle: '3-2-1-1-0 Score',
    score: {
      three: '3 Copies',
      two: '2 Media',
      one: '1 Off-site',
      immutable: '1 Immutable',
      zero: '0 Errors'
    },
    // PRD-010 / ADR-040 D3 — Layer 1-5 badge tooltips (vocabulary aligned to PRD-009)
    layer: {
      l1: { short: 'Snapshot',     tooltip: 'L1 · Local Snapshot — CSI volumesnapshot, data stays in cluster' },
      l2: { short: 'Local BSL',    tooltip: 'L2 · Snapshot Export to local — in-cluster MinIO / NFS' },
      l3: { short: 'Cloud',        tooltip: 'L3 · Snapshot Export to cloud — Azure Blob / S3 / OSS' },
      l4: { short: 'Backup Copy',  tooltip: 'L4 · Backup Copy — cross-cloud rclone replica' },
      l5: { short: 'DR Drill',     tooltip: 'L5 · Recoverability verification — periodic sandbox restore' }
    },
    // PRD-010 / ADR-040 D4 — Layer 5 top verification badge (4 states)
    l5: {
      ok:    'Verified · last drill recent',
      warn:  'Overdue · last drill > 7d',
      error: 'Never verified',
      muted: 'DR Drill not enabled'
    },
    // PRD-010 / ADR-040 D2 — flow type labels (5 locked enum)
    flowType: {
      snapshot: 'Snapshot',
      export:   'Export',
      import:   'Import',
      copy:     'Backup Copy',
      restore:  'Restore'
    }
  },
  storage: {
    title: 'Storage Locations',
    create: 'Add Storage Location',
    provider: 'Provider',
    bucket: 'Bucket',
    region: 'Region',
    defaultBadge: 'Default',
    lastValidated: 'Last Validated',
    sync: 'Sync',
    lastSynced: 'Last Synced',
    syncSchedule: 'Sync Schedule',
    syncSchedule_value: 'Auto · every 60s (Velero default)',
    backupsFound: 'Backups Found',
    syncHint: 'Velero polls this object-storage profile every 60s.',
    nameImmutableHint: 'Name is immutable. To rename, delete and recreate.'
  },
  vsl: {
    title: 'Snapshot Locations',
    desc: 'Volume Snapshot Locations tell Velero how to take volume snapshots — CSI for in-cluster CSI drivers, or cloud-native (AWS EBS / GCP PD / Azure Disk) for managed storage.',
    create: 'Create Snapshot Location',
    config: 'Config',
    noConfig: 'no config (uses Velero defaults)',
    emptyHint: 'No Snapshot Locations yet. Create one to enable CSI / cloud-native volume snapshots.'
  },
  settings: {
    title: 'Settings',
    language: 'Language',
    theme: 'Theme',
    light: 'Light',
    dark: 'Dark',
    general: 'General',
    myAccess: 'My Access',
    auditLog: 'Audit Log',
    clusterHygiene: 'Cluster Hygiene',
    branding: 'Branding',
    plugins: 'Plugins',
    clusters: 'Clusters',
    // v0.8.8 Cluster Hygiene panel
    hygiene: {
      title: 'Orphan Resource Garbage Collection',
      statusOn: 'Enabled',
      statusOff: 'Disabled',
      explain1: "Velero v1.18's Data Mover path creates VolumeSnapshotContents with deletionPolicy=Retain to survive the upload phase, but never cleans them up after the parent Backup is deleted. Over time the cluster accumulates orphan VSC / VS / DataUpload / PodVolumeBackup — polluting the K8s API and continuing to incur object-storage cost.",
      explain2: 'SupKube periodically scans for these orphans (every 6 hours by default) and deletes them. Every cleanup run creates an Activity event you can review on the Activity page.',
      enabled: 'Automatic cleanup',
      enabledOn: 'Background scan is on — orphans will be deleted automatically.',
      enabledOff: 'Background scan is off — orphans only get removed when you click Run Now.',
      interval: 'Scan interval',
      intervalHint: 'How often the background scan runs',
      intervalOpt: {
        '1h': 'Every hour',
        '6h': 'Every 6 hours (recommended)',
        '12h': 'Every 12 hours',
        '24h': 'Every 24 hours'
      },
      manual: 'Manual trigger',
      runNow: 'Run Now',
      runNowHint: "Works whether automatic is on or off — appears as a SupKube task on the Activity page.",
      lastRun: 'Last Run',
      ran: 'Ran',
      noRunYet: "No run yet — enable automatic cleanup or click Run Now above.",
      saved: 'Settings saved',
      saveFailed: 'Save failed',
      loadFailed: 'Failed to load settings',
      runDone: 'Cleanup complete',
      runFailed: 'Cleanup failed'
    }
  },
  // v0.8.5 step 3.5: My Access panel
  myAccess: {
    currentUserTitle: 'Current User',
    email: 'Email',
    username: 'Username',
    groups: 'Groups',
    noGroups: '(none)',
    myRoleTitle: 'My Role',
    rbacDisabled: 'RBAC is disabled — every authenticated user is admin. Production deployments should enable auth.rbac.enabled in values.yaml.',
    allowedNamespaces: 'Allowed namespaces',
    editorNoNamespaces: 'You are bound as editor but have no namespace scope — you cannot perform any write operation. Ask your admin to add namespaces to your binding.',
    capabilitiesTitle: 'What this role can do',
    cap: {
      adminAll:        'Full cluster-wide access (Backups, Restores, Policies, Storage Profiles, Transform Sets, Audit Log, RBAC view)',
      editorBackup:    'Create / delete Backups (only in your namespaces)',
      editorRestore:   'Create / delete Restores (only in your namespaces)',
      editorPolicy:    'Create / edit / delete Policies (only in your namespaces)',
      editorPreflight: 'Run Pre-flight Conflict Check + Apply Suggested Fix',
      adminStorage:    'Manage Storage Profiles + Snapshot Profiles',
      adminTS:         'Manage Transform Sets',
      adminNs:         'Create / delete namespaces',
      adminAudit:      'Read audit log',
      viewerRead:      'View everything (read-only, cluster-wide)',
      unknownRole:     'No role assigned — contact your admin.'
    },
    allBindingsTitle: 'All Role Bindings',
    allBindingsSource: '(read-only, configured via Helm values)',
    noBindings: 'No bindings configured. Edit auth.rbac.bindings in values.yaml + helm upgrade --reset-values.',
    bindingType: 'Type',
    bindingSubject: 'Subject',
    bindingRole: 'Role',
    bindingNamespaces: 'Namespaces',
    editHint: 'To modify bindings, edit',
    editHintCont: 'in your values.yaml, then run helm upgrade --reset-values.'
  },
  // v0.8.5 step 4: Audit Log panel
  audit: {
    retentionTitle: 'Retention notice',
    retentionDesc: 'Audit records here come from K8s Events (default TTL 1 hour). For long-term retention, ship Events to Loki/EFK/Splunk — the backend already emits the same records to stdout for ingestion.',
    filterUser: 'Filter by user',
    filterResult: 'Result',
    filterResource: 'Resource',
    filterLimit: 'Limit',
    count: '{n} record(s)',
    empty: 'No audit records yet. Mutating operations (create / update / delete) will be logged here.',
    timestamp: 'Timestamp',
    user: 'User',
    result: 'Result',
    action: 'Action',
    resource: 'Resource',
    resourceName: 'Resource Name',
    namespace: 'Namespace',
    method: 'Method',
    statusCode: 'Status',
    sourceIP: 'Source IP'
  },
  advisor: {
    title: 'Backup Advisor',
    desc: 'Per-namespace recommendations: how aggressively each application should be backed up, based on workload type, persistent storage, and user-marked tier. Apply means open a prefilled Policy form — never auto-applied.',
    score: 'Score',
    tier: 'Tier',
    recommendedSchedule: 'Recommended Schedule',
    factors: 'Factors',
    moreFactors: '+{count} more factor(s)',
    apply: 'Apply Recommendation',
    skipAction: 'Not recommended',
    skipNoticeTitle: '{count} namespace(s) marked Skip Recommended',
    skipNoticeBody: 'These namespaces look stateless or empty. Skipping is fine for dev/staging, but verify before relying on it in production — Skip Recommended is a hint, not a guarantee.',
    tiers: {
      High: 'High Priority',
      Medium: 'Medium',
      Low: 'Low',
      Skip: 'Skip Recommended'
    },
    factors: {
      hasPVC: 'Has {count} PersistentVolumeClaim(s) — stateful data lives here',
      hasStatefulSet: 'Has {count} StatefulSet(s) — typically a database or queue',
      userTier: 'User-marked tier "{tier}"',
      defaultNs: 'Workloads in the default namespace are often ad-hoc — protect at least lightly',
      statelessNoService: 'Stateless workload with no Service — easy to rebuild from manifests',
      emptyNs: 'Namespace currently empty'
    },
    schedule: {
      hourly: 'Every hour',
      every6h: 'Every 6 hours',
      every12h: 'Every 12 hours',
      daily: 'Daily at midnight',
      weekly: 'Weekly (Sunday midnight)',
      monthly: 'Monthly (1st)',
      custom: 'Custom: {cron}',
      none: 'No backup'
    }
  },
  // v0.8.11: white-label branding panel
  branding: {
    intro: {
      title: 'White-label branding',
      body: 'Change the product name, sidebar logo, and browser tab favicon to match your organization. Stored cluster-wide — every user sees the new identity immediately, no page reload needed.'
    },
    productName: 'Product name',
    productNamePlaceholder: 'e.g. AcmeBackup',
    productNameHint: 'Shown in the sidebar header and the browser tab title.',
    logo: 'Sidebar logo',
    logoHint: 'Recommended source size: 220 × 230 px (sidebar renders it at 24 px height). SVG / PNG / JPEG / WebP, max 100 KB.',
    favicon: 'Browser tab favicon',
    faviconHint: 'Recommended size: 64 × 64 px (browser scales as needed). SVG / PNG / ICO, max 100 KB.',
    colorScheme: 'Color scheme',
    colorHint: 'Sets the accent colour used for links, primary buttons, and interactive highlights across the UI. Applies cluster-wide.',
    colorReset: 'Reset to default (Indigo)',
    preview: 'preview',
    chooseFile: 'Choose file…',
    reset: 'Reset',
    livePreview: 'Live preview',
    save: 'Save changes',
    discard: 'Discard',
    saveSuccess: 'Branding updated — all open sessions will refresh shortly.',
    errors: {
      tooLarge:    'File too large ({kb} KB) — limit is 100 KB',
      readFailed:  'Could not read file: {msg}',
      saveFailed:  'Save failed: {msg}',
      loadFailed:  'Could not load current branding: {msg}'
    }
  }
}
