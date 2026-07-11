// i18n fragment · vsl (Volume Snapshot Locations view).
// 登记处去中心化样板：本视图文案自成一片段，加/改本视图文案只碰此文件，绝不动 en.js
// 单体 → 并行 WI 在 locale 上永不撞。装配见 src/i18n.js 的 import.meta.glob。
export default {
  vsl: {
    title: 'Snapshot Locations',
    desc: 'Volume Snapshot Locations tell Velero how to take volume snapshots — CSI for in-cluster CSI drivers, or cloud-native (AWS EBS / GCP PD / Azure Disk) for managed storage.',
    create: 'Create Snapshot Location',
    config: 'Config',
    noConfig: 'no config (uses Velero defaults)',
    emptyHint: 'No Snapshot Locations yet. Create one to enable CSI / cloud-native volume snapshots.'
  }
}
