export interface DeploymentProviderStatus {
  provider: string
  enabled?: boolean
  configured?: boolean
  can_connect?: boolean
}

export interface ParsedDeploymentProviders {
  ionetEnabled: boolean
  modalEnabled: boolean
  hfEnabled: boolean
  anyEnabled: boolean
}

export function parseDeploymentProviders(
  providers: DeploymentProviderStatus[] | undefined | null
): ParsedDeploymentProviders {
  const list = providers ?? []
  const findProvider = (name: string) =>
    list.find((p) => p.provider?.toLowerCase() === name.toLowerCase())
  const ionet = findProvider('io.net')
  const modal = findProvider('modal')
  const hf = findProvider('huggingface') ?? findProvider('hugging face spaces')

  const ionetEnabled = Boolean(ionet?.enabled)
  const modalEnabled = Boolean(modal?.enabled)
  const hfEnabled = Boolean(hf?.enabled)
  const anyEnabled = Boolean(
    (ionetEnabled && (ionet?.configured ?? true)) ||
      (modalEnabled && (modal?.configured ?? true)) ||
      (hfEnabled && (hf?.configured ?? true)) ||
      ionetEnabled ||
      modalEnabled ||
      hfEnabled
  )
  return { ionetEnabled, modalEnabled, hfEnabled, anyEnabled }
}
