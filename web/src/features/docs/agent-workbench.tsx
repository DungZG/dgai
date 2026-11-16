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
import { Download } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CodeBlock } from '@/components/ai-elements/code-block'
import { CopyButton } from '@/components/copy-button'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'

import { usePricingData } from '../pricing/hooks/use-pricing-data'
import {
  AGENTS,
  createAgentSetup,
  filterAgentModels,
  type AgentKind,
} from './agent-setup'
import { SetupInstructions } from './setup-instructions'

export function AgentWorkbench(props: { serverAddress: string }) {
  const { t } = useTranslation()
  const pricing = usePricingData()
  const [agentKind, setAgentKind] = useState<AgentKind>(AGENTS[0].kind)
  const [modelName, setModelName] = useState('')
  const agent = AGENTS.find((item) => item.kind === agentKind) ?? AGENTS[0]
  const availableModels = useMemo(
    () => filterAgentModels(agentKind, pricing.models),
    [agentKind, pricing.models]
  )
  const selectedModel =
    availableModels.find((model) => model.model_name === modelName) ??
    availableModels[0]
  const result = useMemo(() => {
    if (!selectedModel || pricing.error || pricing.isLoading) return null
    try {
      return {
        setup: createAgentSetup(
          agentKind,
          props.serverAddress,
          selectedModel.model_name,
          selectedModel
        ),
      }
    } catch {
      return { error: true }
    }
  }, [
    agentKind,
    selectedModel,
    props.serverAddress,
    pricing.error,
    pricing.isLoading,
  ])
  const setup = result?.setup

  let catalogue = (
    <Combobox
      id='docs-model'
      aria-label={t('Model')}
      options={availableModels.map((model) => ({
        value: model.model_name,
        label: model.model_name,
      }))}
      value={selectedModel?.model_name ?? ''}
      onValueChange={(value) => setModelName(value ?? '')}
      placeholder={t('Select a model')}
      emptyText={t('No compatible models')}
    />
  )
  if (pricing.isLoading) {
    catalogue = <LoadingState inline message={t('Loading models...')} />
  } else if (pricing.error) {
    catalogue = (
      <ErrorState
        className='min-h-0 p-4'
        title={t('Could not load models')}
        onRetry={() => void pricing.refetch()}
      />
    )
  } else if (availableModels.length === 0) {
    catalogue = (
      <EmptyState
        className='min-h-0 p-4'
        title={t('No compatible models')}
        description={t(
          'No model in the current catalogue supports this agent.'
        )}
        action={
          <Button
            variant='outline'
            size='sm'
            onClick={() => void pricing.refetch()}
          >
            {t('Retry')}
          </Button>
        }
      />
    )
  }

  return (
    <section aria-labelledby='docs-workbench-title' className='min-w-0'>
      <div className='mb-6 max-w-3xl'>
        <h1
          id='docs-workbench-title'
          className='text-3xl font-semibold tracking-tight'
        >
          {t('Connect your coding agent')}
        </h1>
        <p className='text-muted-foreground mt-2 text-sm leading-6'>
          {t(
            'Choose an agent and a model. Download the script, review it, then run it once to configure your app.'
          )}
        </p>
      </div>
      <div className='border-border/70 grid gap-8 border-y py-6 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]'>
        <div className='min-w-0 space-y-5'>
          <fieldset className='min-w-0'>
            <legend className='mb-2 text-sm font-medium'>{t('Agent')}</legend>
            <div className='flex flex-wrap gap-2'>
              {AGENTS.map((item) => (
                <Button
                  key={item.kind}
                  type='button'
                  variant={agentKind === item.kind ? 'secondary' : 'outline'}
                  aria-pressed={agentKind === item.kind}
                  onClick={() => {
                    if (item.kind === agentKind) return
                    setAgentKind(item.kind)
                    setModelName('')
                  }}
                >
                  {t(item.title)}
                </Button>
              ))}
            </div>
            <p className='text-muted-foreground mt-3 text-sm leading-6'>
              {t(agent.summary)}
            </p>
          </fieldset>
          <div className='space-y-2'>
            <label htmlFor='docs-model' className='text-sm font-medium'>
              {t('Model')}
            </label>
            {catalogue}
          </div>
          <dl className='min-w-0 space-y-4 text-sm'>
            <div>
              <dt className='text-muted-foreground mb-1'>{t('Endpoint')}</dt>
              <dd className='font-mono text-xs leading-5 break-all'>
                {setup?.endpoint ?? t('Select a model')}
              </dd>
            </div>
            {setup && (
              <div>
                <dt className='text-muted-foreground mb-1'>
                  {t('Configuration file')}
                </dt>
                <dd className='font-mono text-xs leading-5 break-all'>
                  {setup.target}
                </dd>
              </div>
            )}
          </dl>
          <a
            className='text-muted-foreground inline-flex text-xs underline underline-offset-4'
            href={agent.sourceUrl}
            target='_blank'
            rel='noreferrer'
          >
            {t('Official setup guide')}
          </a>
        </div>
        <div className='min-w-0 space-y-4' aria-live='polite'>
          {setup && (
            <>
              <Button
                size='lg'
                className='h-auto min-h-10 max-w-full py-2 whitespace-normal'
                render={
                  <a
                    href={`data:text/plain;charset=utf-8,${encodeURIComponent(setup.script)}`}
                    download={setup.filename}
                  />
                }
              >
                <Download aria-hidden='true' />
                {t('Download setup script')}
              </Button>
              <p className='text-muted-foreground text-sm leading-6'>
                {agentKind === 'vscode'
                  ? t(
                      'VS Code saves your API key through Manage Language Models after the script configures the endpoint.'
                    )
                  : t(
                      'Enter your API key only in the hidden prompt on your computer. The downloaded file contains no credentials.'
                    )}
              </p>
              <div className='flex flex-wrap items-center gap-2'>
                <code className='bg-muted max-w-full min-w-0 flex-1 overflow-x-auto rounded-md px-3 py-2 font-mono text-xs'>
                  {setup.runCommand}
                </code>
                <CopyButton
                  value={setup.runCommand}
                  variant='outline'
                  size='sm'
                  aria-label={t('Copy run command')}
                >
                  {t('Copy')}
                </CopyButton>
              </div>
              <SetupInstructions key={setup.kind} setup={setup} />
              <details className='text-sm'>
                <summary className='cursor-pointer py-2 font-medium'>
                  {t('Review setup script')}
                </summary>
                <CodeBlock
                  code={setup.script}
                  filename={setup.filename}
                  language='python'
                  showToolbar
                  title={setup.filename}
                  defaultCollapsed
                  collapsedLines={10}
                />
              </details>
            </>
          )}
          {!setup && !result?.error && (
            <p className='text-muted-foreground bg-muted/30 flex min-h-48 items-center justify-center rounded-lg p-6 text-center text-sm'>
              {t('Select a model to generate the setup script.')}
            </p>
          )}
          {result?.error && (
            <ErrorState
              className='min-h-0 p-4'
              title={t(
                'The server address or model is invalid. Contact the administrator.'
              )}
            />
          )}
        </div>
      </div>
    </section>
  )
}
