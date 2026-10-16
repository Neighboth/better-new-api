import { describe, expect, it } from 'vitest'

import { parseDeploymentProviders } from '../lib/deployment-providers'

describe('parseDeploymentProviders', () => {
  it('reports disabled when no provider is enabled', () => {
    expect(parseDeploymentProviders([])).toEqual({
      ionetEnabled: false,
      modalEnabled: false,
      hfEnabled: false,
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
      anyEnabled: true,
    })
  })

  it('tolerates missing payloads', () => {
    expect(parseDeploymentProviders(undefined).anyEnabled).toBe(false)
    expect(parseDeploymentProviders(null).anyEnabled).toBe(false)
  })
})
