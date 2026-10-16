import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const MODAL_GPUS = [
  'T4',
  'A10G',
  'A100-40GB',
  'A100-80GB',
  'H100',
] as const

function buildModalScript(appName: string, modelId: string, gpu: string): string {
  const safeApp = (appName || 'llm-server').replaceAll(/[^a-zA-Z0-9-_]/g, '-')
  return `import modal

# 1) pip install modal
# 2) modal setup  (paste the token from modal.com -> Settings -> API tokens)
# 3) modal deploy modal_server.py
#
# The server exposes an OpenAI-compatible API. Add its URL as a new
# OpenAI-compatible channel pointing at the deployed /v1 endpoint.

app = modal.App("${safeApp}")

image = modal.Image.debian_slim().pip_install(
    "vllm",
    "huggingface_hub",
)

volume = modal.Volume.from_name("model-weights", create_if_missing=True)

MODEL_ID = "${modelId || 'meta-llama/Llama-3-8B'}"


@app.function(
    image=image,
    gpu="${gpu}",
    volumes={"/models": volume},
    timeout=3600,
    scaledown_window=300,
)
def serve():
    import subprocess

    subprocess.run(
        [
            "python", "-m", "vllm.entrypoints.openai.api_server",
            "--model", MODEL_ID,
            "--download-dir", "/models",
            "--host", "0.0.0.0",
            "--port", "8000",
        ],
        check=True,
    )
`
}

export function ModalDeployHelper() {
  const { t } = useTranslation()
  const [appName, setAppName] = useState('llm-server')
  const [modelId, setModelId] = useState('')
  const [gpu, setGpu] = useState<string>('A10G')
  const [copied, setCopied] = useState(false)

  const script = useMemo(
    () => buildModalScript(appName.trim(), modelId.trim(), gpu),
    [appName, modelId, gpu]
  )

  const copyScript = async () => {
    try {
      await navigator.clipboard.writeText(script)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className='rounded-lg border p-4 space-y-4'>
      <h4 className='font-semibold text-sm'>
        {t('Modal deploy script generator')}
      </h4>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Modal deploys code, not containers, so there is no one-click create here. Fill the fields, copy the script to modal_server.py, run modal deploy, then add the served URL as an OpenAI-compatible channel.'
        )}
      </p>
      <div className='grid gap-4 md:grid-cols-3'>
        <div className='space-y-1'>
          <label className='text-sm font-medium'>{t('App name')}</label>
          <Input
            placeholder='llm-server'
            value={appName}
            onChange={(e) => setAppName(e.target.value)}
          />
        </div>
        <div className='space-y-1'>
          <label className='text-sm font-medium'>{t('Model repository')}</label>
          <Input
            placeholder='meta-llama/Llama-3-8B'
            value={modelId}
            onChange={(e) => setModelId(e.target.value)}
          />
        </div>
        <div className='space-y-1'>
          <label className='text-sm font-medium'>{t('GPU')}</label>
          <Select value={gpu} onValueChange={(v) => setGpu(v ?? 'A10G')}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {MODAL_GPUS.map((g) => (
                <SelectItem key={g} value={g}>
                  {g}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <pre className='max-h-64 overflow-auto rounded-md bg-muted p-3 font-mono text-xs'>
        {script}
      </pre>
      <Button variant='outline' size='sm' onClick={copyScript}>
        {copied ? t('Copied') : t('Copy script')}
      </Button>
    </div>
  )
}
