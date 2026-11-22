import { describe, expect, it } from 'vitest'

import { parseDeploymentProviders } from '../lib/deployment-providers'

describe('parseDeploymentProviders', () => {
  it('reports disabled when no provider is enabled', () => {
    expect(parseDeploymentProviders([])).toEqual({
      ionetEnabled: false,
      modalEnabled: false,
      hfEnabled: false,
      hostingerEnabled: false,
      runpodEnabled: false,
      colabEnabled: false,
      kaggleEnabled: false,
      anyEnabled: false,
    })
  })

  it('reports enabled when only Hugging Face is enabled', () => {
    expect(
      parseDeploymentProviders([
        { provider: 'io.net', enabled: false, configured: false },
        { provider: 'modal', enabled: false, configured: false },
        { provider: 'huggingface', enabled: true, configured: true },
      ])
    ).toEqual({
      ionetEnabled: false,
      modalEnabled: false,
      hfEnabled: true,
      hostingerEnabled: false,
      runpodEnabled: false,
      colabEnabled: false,
      kaggleEnabled: false,
      anyEnabled: true,
    })
  })

  it('reports enabled when colab or kaggle is enabled', () => {
    const res = parseDeploymentProviders([
      { provider: 'colab', enabled: true, configured: true },
      { provider: 'kaggle', enabled: true, configured: false },
    ])
    expect(res.colabEnabled).toBe(true)
    expect(res.kaggleEnabled).toBe(true)
    expect(res.anyEnabled).toBe(true)
  })

  it('tolerates missing payloads', () => {
    expect(parseDeploymentProviders(undefined).anyEnabled).toBe(false)
    expect(parseDeploymentProviders(null).anyEnabled).toBe(false)
  })
})
