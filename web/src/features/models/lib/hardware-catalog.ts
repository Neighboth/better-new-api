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

export interface ProviderTierOption {
  id: string
  label: string
  description: string
  allowedHardwareIds: string[] | null // null = all
}

export const PROVIDER_TIERS: Record<DeploymentProviderId, ProviderTierOption[]> =
  {
    modal: [
      {
        id: 'free_no_card',
        label: 'Free (No Credit Card)',
        description: 'CPU only. Add a payment method on Modal to unlock GPUs.',
        allowedHardwareIds: ['CPU'],
      },
      {
        id: 'starter_paid',
        label: 'Starter / Paid (Card Added)',
        description: 'All Modal GPUs (T4 to B300) + CPU',
        allowedHardwareIds: null,
      },
    ],
    colab: [
      {
        id: 'free',
        label: 'Free',
        description: 'CPU, Tesla T4 GPU, and TPU v5e-1 (subject to availability)',
        allowedHardwareIds: ['CPU', 'NVIDIA_TESLA_T4', 'TPU_V5E_1'],
      },
      {
        id: 'google_ai_pro',
        label: 'Google AI Pro',
        description: 'CPU, Tesla T4 GPU, and TPUs (v2-8, v5e-1)',
        allowedHardwareIds: ['CPU', 'NVIDIA_TESLA_T4', 'TPU_V2_8', 'TPU_V5E_1'],
      },
      {
        id: 'colab_pro',
        label: 'Colab Pro / Pay-As-You-Go',
        description: 'CPU, T4, L4, A100 (40GB), TPU v2-8 & v5e-1',
        allowedHardwareIds: [
          'CPU',
          'NVIDIA_TESLA_T4',
          'NVIDIA_L4',
          'NVIDIA_TESLA_A100',
          'TPU_V2_8',
          'TPU_V5E_1',
        ],
      },
      {
        id: 'google_ai_ultra',
        label: 'Colab Pro+ / Google AI Ultra',
        description: 'All GPUs (incl. H100 / A100 80GB / RTX PRO 6000) & TPUs',
        allowedHardwareIds: null,
      },
    ],
    kaggle: [
      {
        id: 'unverified',
        label: 'Unverified (CPU Only)',
        description: 'Phone verification required on Kaggle for GPUs & TPUs',
        allowedHardwareIds: ['CPU'],
      },
      {
        id: 'verified_free',
        label: 'Verified Free (30h GPU / 20h TPU)',
        description: 'CPU, T4 x2 & TPU v5e-8',
        allowedHardwareIds: [
          'CPU',
          'NvidiaTeslaT4',
          'TpuV5E8',
        ],
      },
      {
        id: 'cloud_linked',
        label: 'Extended / GCP Linked',
        description: 'All Kaggle GPUs (incl. L4) & TPUs (incl. v6e-8)',
        allowedHardwareIds: null,
      },
    ],
    huggingface: [
      {
        id: 'standard',
        label: 'Standard',
        description: 'All Hugging Face Inference Endpoint hardware',
        allowedHardwareIds: null,
      },
    ],
    runpod: [
      {
        id: 'standard',
        label: 'Standard',
        description: 'All RunPod GPUs & CPU',
        allowedHardwareIds: null,
      },
    ],
    hostinger: [
      {
        id: 'standard',
        label: 'Standard',
        description: 'All Hostinger GPU & KVM plans',
        allowedHardwareIds: null,
      },
    ],
    ionet: [
      {
        id: 'standard',
        label: 'Standard',
        description: 'All io.net GPUs & CPU clusters',
        allowedHardwareIds: null,
      },
    ],
  }

export function getHardwareForTier(
  provider: DeploymentProviderId,
  tier?: string,
  _eligibleGpus?: string[]
): HardwareOption[] {
  const all = PROVIDER_HARDWARE_CATALOG[provider] ?? []
  const tiers = PROVIDER_TIERS[provider] ?? []
  const matched =
    tiers.find((t) => t.id === tier) ?? tiers[0] ?? null
  if (!matched || matched.allowedHardwareIds === null) {
    return all
  }
  const allowed = new Set(matched.allowedHardwareIds)
  return all.filter((hw) => allowed.has(hw.id))
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
    { id: 'NvidiaL4', label: 'GPU: NVIDIA L4', kind: 'gpu' },
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
