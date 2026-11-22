export type DeploymentProviderId =
  | 'modal'
  | 'colab'
  | 'kaggle'
  | 'huggingface'
  | 'runpod'
  | 'hostinger'
  | 'ionet'

export interface HardwareOption {
  id: string
  label: string
  kind: 'cpu' | 'gpu' | 'tpu'
}

export const PROVIDER_LABELS: Record<DeploymentProviderId, string> = {
  modal: 'Modal',
  colab: 'Google Colab',
  kaggle: 'Kaggle',
  huggingface: 'Hugging Face Spaces',
  runpod: 'RunPod',
  hostinger: 'Hostinger GPU',
  ionet: 'io.net',
}

export const PROVIDER_HARDWARE_CATALOG: Record<
  DeploymentProviderId,
  HardwareOption[]
> = {
  modal: [
    { id: 'B300', label: 'GPU: NVIDIA B300 (262GB)', kind: 'gpu' },
    { id: 'B200', label: 'GPU: NVIDIA B200 (180GB)', kind: 'gpu' },
    { id: 'H200', label: 'GPU: NVIDIA H200 SXM (141GB)', kind: 'gpu' },
    { id: 'H100', label: 'GPU: NVIDIA H100 (80GB)', kind: 'gpu' },
    { id: 'RTX-PRO-6000', label: 'GPU: NVIDIA RTX PRO 6000 (96GB)', kind: 'gpu' },
    { id: 'A100-80GB', label: 'GPU: NVIDIA A100 (80GB)', kind: 'gpu' },
    { id: 'A100-40GB', label: 'GPU: NVIDIA A100 (40GB)', kind: 'gpu' },
    { id: 'L40S', label: 'GPU: NVIDIA L40S (48GB)', kind: 'gpu' },
    { id: 'A10G', label: 'GPU: NVIDIA A10G (24GB)', kind: 'gpu' },
    { id: 'L4', label: 'GPU: NVIDIA L4 (24GB)', kind: 'gpu' },
    { id: 'T4', label: 'GPU: NVIDIA T4 (16GB)', kind: 'gpu' },
    { id: 'CPU', label: 'CPU (Standard)', kind: 'cpu' },
  ],
  colab: [
    { id: 'NVIDIA_TESLA_T4', label: 'GPU: NVIDIA Tesla T4 (16GB)', kind: 'gpu' },
    { id: 'NVIDIA_L4', label: 'GPU: NVIDIA L4 (24GB)', kind: 'gpu' },
    { id: 'NVIDIA_TESLA_A100', label: 'GPU: NVIDIA A100 (40GB)', kind: 'gpu' },
    { id: 'NVIDIA_TESLA_A100_80GB', label: 'GPU: NVIDIA A100 (80GB)', kind: 'gpu' },
    { id: 'NVIDIA_RTX_PRO_6000', label: 'GPU: NVIDIA RTX PRO 6000', kind: 'gpu' },
    { id: 'NVIDIA_H100_80GB', label: 'GPU: NVIDIA H100 (80GB)', kind: 'gpu' },
    { id: 'TPU_V2_8', label: 'TPU: v2-8', kind: 'tpu' },
    { id: 'TPU_V5E_1', label: 'TPU: v5e-1', kind: 'tpu' },
    { id: 'TPU_V6E_1', label: 'TPU: v6e-1', kind: 'tpu' },
    { id: 'CPU', label: 'CPU (Standard)', kind: 'cpu' },
  ],
  kaggle: [
    { id: 'NvidiaTeslaT4', label: 'GPU: NVIDIA Tesla T4 x2', kind: 'gpu' },
    { id: 'NvidiaP100', label: 'GPU: NVIDIA Tesla P100', kind: 'gpu' },
    { id: 'NvidiaL4', label: 'GPU: NVIDIA L4', kind: 'gpu' },
    { id: 'TpuV38', label: 'TPU: v3-8', kind: 'tpu' },
    { id: 'TpuV5E8', label: 'TPU: v5e-8', kind: 'tpu' },
    { id: 'TpuV6E8', label: 'TPU: v6e-8', kind: 'tpu' },
    { id: 'CPU', label: 'CPU (4 cores, 30 GB RAM)', kind: 'cpu' },
  ],
  huggingface: [
    { id: 't4-small', label: 'GPU: NVIDIA T4 Small (16GB)', kind: 'gpu' },
    { id: 't4-medium', label: 'GPU: NVIDIA T4 Medium', kind: 'gpu' },
    { id: 'l4x1', label: 'GPU: NVIDIA L4 x1 (24GB)', kind: 'gpu' },
    { id: 'l4x4', label: 'GPU: NVIDIA L4 x4 (96GB)', kind: 'gpu' },
    { id: 'a10g-small', label: 'GPU: NVIDIA A10G Small (24GB)', kind: 'gpu' },
    { id: 'a10g-large', label: 'GPU: NVIDIA A10G Large', kind: 'gpu' },
    { id: 'l40sx1', label: 'GPU: NVIDIA L40S x1 (48GB)', kind: 'gpu' },
    { id: 'l40sx4', label: 'GPU: NVIDIA L40S x4 (192GB)', kind: 'gpu' },
    { id: 'a100-large', label: 'GPU: NVIDIA A100 x1 (80GB)', kind: 'gpu' },
    { id: 'a100x2', label: 'GPU: NVIDIA A100 x2 (160GB)', kind: 'gpu' },
    { id: 'a100x4', label: 'GPU: NVIDIA A100 x4 (320GB)', kind: 'gpu' },
    { id: 'a100x8', label: 'GPU: NVIDIA A100 x8 (640GB)', kind: 'gpu' },
    { id: 'h100x1', label: 'GPU: NVIDIA H100 x1 (80GB)', kind: 'gpu' },
    { id: 'h100x4', label: 'GPU: NVIDIA H100 x4 (320GB)', kind: 'gpu' },
    { id: 'h100x8', label: 'GPU: NVIDIA H100 x8 (640GB)', kind: 'gpu' },
    { id: 'h200x1', label: 'GPU: NVIDIA H200 x1 (141GB)', kind: 'gpu' },
    { id: 'h200x8', label: 'GPU: NVIDIA H200 x8', kind: 'gpu' },
    { id: 'cpu-basic', label: 'CPU: Basic (x2)', kind: 'cpu' },
    { id: 'cpu-upgrade', label: 'CPU: Upgrade (x4)', kind: 'cpu' },
  ],
  runpod: [
    { id: 'B200', label: 'GPU: NVIDIA B200 (180GB)', kind: 'gpu' },
    { id: 'H200', label: 'GPU: NVIDIA H200 (141GB)', kind: 'gpu' },
    { id: 'H100-SXM', label: 'GPU: NVIDIA H100 SXM (80GB)', kind: 'gpu' },
    { id: 'A100-80GB', label: 'GPU: NVIDIA A100 (80GB)', kind: 'gpu' },
    { id: 'L40S', label: 'GPU: NVIDIA L40S (48GB)', kind: 'gpu' },
    { id: 'RTX-4090', label: 'GPU: NVIDIA RTX 4090 (24GB)', kind: 'gpu' },
    { id: 'RTX-3090', label: 'GPU: NVIDIA RTX 3090 (24GB)', kind: 'gpu' },
    { id: 'CPU', label: 'CPU (Standard)', kind: 'cpu' },
  ],
  hostinger: [
    { id: 'H100', label: 'GPU: NVIDIA H100 (80GB)', kind: 'gpu' },
    { id: 'A100', label: 'GPU: NVIDIA A100 (80GB)', kind: 'gpu' },
    { id: 'L40S', label: 'GPU: NVIDIA L40S (48GB)', kind: 'gpu' },
    { id: 'RTX-4090', label: 'GPU: NVIDIA RTX 4090 (24GB)', kind: 'gpu' },
    { id: 'CPU', label: 'CPU (KVM VPS)', kind: 'cpu' },
  ],
  ionet: [
    { id: 'H100-80GB', label: 'GPU: NVIDIA H100 (80GB)', kind: 'gpu' },
    { id: 'A100-80GB', label: 'GPU: NVIDIA A100 (80GB)', kind: 'gpu' },
    { id: 'L40S', label: 'GPU: NVIDIA L40S (48GB)', kind: 'gpu' },
    { id: 'RTX-4090', label: 'GPU: NVIDIA RTX 4090 (24GB)', kind: 'gpu' },
    { id: 'RTX-3090', label: 'GPU: NVIDIA RTX 3090 (24GB)', kind: 'gpu' },
    { id: 'CPU', label: 'CPU (Cluster)', kind: 'cpu' },
  ],
}
