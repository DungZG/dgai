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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

import type { PricingModel } from '../../pricing/types'
import { AgentWorkbench } from '../agent-workbench'

const catalogue = vi.hoisted(() => ({
  models: [] as PricingModel[],
  isLoading: false,
  error: null as Error | null,
  refetch: vi.fn(),
}))

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => catalogue,
}))

const i18n = createInstance()
const models: PricingModel[] = [
  {
    id: 1,
    model_name: 'gpt-coding',
    supported_endpoint_types: ['openai-response'],
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
  },
  {
    id: 2,
    model_name: 'gpt-review',
    supported_endpoint_types: ['openai-response'],
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
  },
  {
    id: 3,
    model_name: 'claude-coding',
    supported_endpoint_types: ['anthropic'],
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
  },
]

beforeAll(async () => {
  await i18n
    .use(initReactI18next)
    .init({ lng: 'en', fallbackLng: 'en', resources: {} })
})

beforeEach(() => {
  catalogue.models = models
  catalogue.error = null
  catalogue.isLoading = false
  catalogue.refetch.mockReset()
})

function renderWorkbench() {
  return render(
    <I18nextProvider i18n={i18n}>
      <AgentWorkbench serverAddress='https://gateway.example' />
    </I18nextProvider>
  )
}

describe('agent setup workbench', () => {
  it('replaces an incompatible model when switching agents and keeps the selected agent accessible', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    const input = screen.getByRole('combobox', { name: 'Model' })
    expect(input).toHaveValue('gpt-coding')
    await user.click(input)
    await user.click(screen.getByRole('option', { name: 'gpt-review' }))
    await waitFor(() => expect(input).toHaveValue('gpt-review'))

    await user.click(screen.getByRole('button', { name: 'Claude Code' }))
    expect(input).toHaveValue('claude-coding')
    expect(screen.getByRole('button', { name: 'Claude Code' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: 'Codex' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    expect(
      screen.getByText('https://gateway.example/v1/messages')
    ).toBeVisible()
  })

  it('shows an empty catalogue and retries without inventing a model or download', async () => {
    catalogue.models = []
    renderWorkbench()
    expect(screen.getByText('No compatible models')).toBeVisible()
    expect(
      screen.queryByRole('combobox', { name: 'Model' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('Download setup script')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(catalogue.refetch).toHaveBeenCalledOnce()
  })

  it('hides stale setup on catalogue failure and offers retry', async () => {
    catalogue.error = new Error('Catalogue unavailable')
    renderWorkbench()
    expect(screen.getByText('Could not load models')).toBeVisible()
    expect(screen.queryByText('Download setup script')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(catalogue.refetch).toHaveBeenCalledOnce()
  })

  it('downloads the selected agent, model and real endpoint without web-entered credentials', () => {
    renderWorkbench()
    const download = screen.getByText('Download setup script').closest('a')
    expect(download).not.toBeNull()
    expect(download).toHaveAttribute('download', 'setup-dgai-codex.py')
    const href = download?.getAttribute('href')
    expect(href).toBeTruthy()
    const script = decodeURIComponent(href?.split(',').slice(1).join(',') ?? '')
    const payloadLine = script.match(/^SETUP = json\.loads\((.+)\)$/m)
    expect(payloadLine).not.toBeNull()
    const payload = JSON.parse(JSON.parse(payloadLine?.[1] ?? 'null'))
    expect(payload).toMatchObject({
      kind: 'codex',
      model: 'gpt-coding',
      base: 'https://gateway.example',
      route: '/v1/responses',
    })
    expect(payload).not.toHaveProperty('apiKey')
    expect(script).toContain('getpass')
    expect(screen.queryByLabelText(/api key/i)).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Copy run command' })
    ).toBeVisible()
  })

  it('explains the VS Code secure-storage enrollment step separately', async () => {
    renderWorkbench()
    await userEvent.click(screen.getByRole('button', { name: 'VS Code' }))
    expect(
      screen.getByText(
        'VS Code saves your API key through Manage Language Models after the script configures the endpoint.'
      )
    ).toBeVisible()
    expect(
      screen.getByText(
        'Open Chat: Manage Language Models, choose the dgai gear, and enter your API key in VS Code.'
      )
    ).toBeVisible()
    expect(screen.queryByLabelText(/api key/i)).not.toBeInTheDocument()
  })
})
