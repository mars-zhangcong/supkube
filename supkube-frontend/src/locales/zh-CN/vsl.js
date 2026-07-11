// i18n 片段 · vsl（快照位置视图）。加/改本视图文案只碰此文件，绝不动 zh-CN.js 单体。
// 装配见 src/i18n.js 的 import.meta.glob。
export default {
  vsl: {
    title: '快照位置',
    desc: '快照位置告诉 Velero 如何拍摄卷快照——CSI 用于集群内 CSI 驱动，或云原生（AWS EBS / GCP PD / Azure Disk）用于托管存储。',
    create: '创建快照位置',
    config: '配置',
    noConfig: '无配置（使用 Velero 默认值）',
    emptyHint: '尚无快照位置。创建一个以启用 CSI / 云原生卷快照。'
  }
}
