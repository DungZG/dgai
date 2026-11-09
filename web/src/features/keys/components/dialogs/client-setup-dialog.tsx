/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Download } from 'lucide-react'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Combobox } from '@/components/ui/combobox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getUserModels } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  claudeSnippets,
  codexSnippets,
  type ClientKind,
  type ClientSnippet,
} from '../../lib/client-setup'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  tokenKey: string
}

function getServerAddress(): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return status.server_address as string
    }
  } catch {
    /* empty */
  }
  return window.location.origin
}

export function ClientSetupDialog(props: Props) {
  const { t } = useTranslation()
  const [client, setClient] = useState<ClientKind>('claude-code')
  const [sonnet, setSonnet] = useState('')
  const [opus, setOpus] = useState('')
  const [haiku, setHaiku] = useState('')
  const [codexModel, setCodexModel] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['user-models-client-setup'],
    queryFn: async () => requireServerSuccess(await getUserModels()),
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })

  const models = useMemo(() => data?.data ?? [], [data?.data])
  const claudeOptions = useMemo(
    () =>
      models
        .filter((m) => /^claude-/i.test(m))
        .map((m) => ({ value: m, label: m })),
    [models]
  )
  const codexOptions = useMemo(
    () =>
      models
        .filter((m) => /^(gpt-|cx\/|o\d)/i.test(m))
        .map((m) => ({ value: m, label: m })),
    [models]
  )

  const serverAddress = getServerAddress()

  const snippets: ClientSnippet[] = useMemo(() => {
    if (client === 'claude-code') {
      return claudeSnippets(serverAddress, props.tokenKey, {
        sonnet,
        opus,
        haiku,
      })
    }
    return codexSnippets(serverAddress, props.tokenKey, codexModel)
  }, [client, codexModel, haiku, opus, props.tokenKey, serverAddress, sonnet])

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Connect a CLI')}
      description={t(
        'Pick a client, choose the models it should use, then copy the configuration.'
      )}
      contentClassName='sm:max-w-2xl'
      bodyClassName='space-y-4'
      footer={
        <Button variant='outline' onClick={() => props.onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      <Tabs
        value={client}
        onValueChange={(v) => setClient(v as ClientKind)}
        className='gap-4'
      >
        <TabsList className='grid w-full grid-cols-2'>
          <TabsTrigger value='claude-code'>Claude Code</TabsTrigger>
          <TabsTrigger value='codex'>Codex</TabsTrigger>
        </TabsList>
      </Tabs>

      {isLoading ? (
        <Skeleton className='h-32 w-full' />
      ) : client === 'claude-code' ? (
        <div className='space-y-3'>
          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='space-y-1.5'>
              <Label>{t('Sonnet model')}</Label>
              <Combobox
                options={claudeOptions}
                value={sonnet}
                onValueChange={(v) => setSonnet(v ?? '')}
                placeholder={t('Select model')}
                emptyText={t('No models found')}
              />
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Opus model')}</Label>
              <Combobox
                options={claudeOptions}
                value={opus}
                onValueChange={(v) => setOpus(v ?? '')}
                placeholder={t('Select model')}
                emptyText={t('No models found')}
              />
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Haiku model')}</Label>
              <Combobox
                options={claudeOptions}
                value={haiku}
                onValueChange={(v) => setHaiku(v ?? '')}
                placeholder={t('Optional')}
                emptyText={t('No models found')}
              />
            </div>
          </div>
        </div>
      ) : (
        <div className='space-y-1.5'>
          <Label>{t('Model')}</Label>
          <Combobox
            options={codexOptions}
            value={codexModel}
            onValueChange={(v) => setCodexModel(v ?? '')}
            placeholder={t('Select model')}
            emptyText={t('No models found')}
          />
        </div>
      )}

      {client === 'codex' && !codexModel ? (
        <p className='text-muted-foreground text-sm'>
          {t('Select a model to generate the configuration.')}
        </p>
      ) : (
        <div className='space-y-3'>
          {snippets.map((snippet) => (
            <SnippetBlock key={snippet.id} snippet={snippet} />
          ))}
        </div>
      )}
    </Dialog>
  )
}

function SnippetBlock(props: { snippet: ClientSnippet }) {
  const { t } = useTranslation()
  return (
    <div className='border-border/60 overflow-hidden rounded-lg border'>
      <div className='bg-muted/40 flex items-center justify-between gap-2 px-3 py-2'>
        <span className='text-muted-foreground text-xs font-medium'>
          {props.snippet.title}
        </span>
        <div className='flex items-center gap-1'>
          {props.snippet.filename && (
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant='ghost'
                    size='icon'
                    className='size-7'
                    aria-label={t('Download config file')}
                    onClick={() => downloadText(props.snippet)}
                  >
                    <Download className='size-3.5' />
                  </Button>
                }
              />
              <TooltipContent>{t('Download config file')}</TooltipContent>
            </Tooltip>
          )}
          <CopyButton value={props.snippet.body} variant='ghost' size='icon' />
        </div>
      </div>
      <pre className='overflow-x-auto px-3 py-2 text-xs leading-relaxed'>
        <code>{props.snippet.body}</code>
      </pre>
      {props.snippet.language === 'toml' && (
        <p className='text-muted-foreground border-border/60 border-t px-3 py-2 text-[11px]'>
          {t('Codex reads this file at ~/.codex/config.toml')}
        </p>
      )}
    </div>
  )
}

/** Saves a config snippet as a file so the user can drop it into place. */
function downloadText(snippet: ClientSnippet) {
  const blob = new Blob([snippet.body], {
    type: 'text/plain;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = snippet.filename ?? `${snippet.id}.txt`
  anchor.click()
  URL.revokeObjectURL(url)
}
