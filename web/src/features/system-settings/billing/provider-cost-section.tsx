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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'

const EMPTY_FORM = {
  channel_id: '',
  model_name: '',
  input_price: '',
  output_price: '',
}

export type ProviderCost = {
  id: number
  channel_id: number
  model_name: string
  input_price: number
  output_price: number
}

export type ModelMargin = {
  channel_id: number
  model_name: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  revenue: number
  cost: number
  margin: number
  margin_percent: number
  cost_known: boolean
}

type MarginsResponse = {
  items: ModelMargin[]
}

function usd(value: number): string {
  // Token prices are fractions of a cent, so small values need more digits.
  return `$${value.toFixed(Math.abs(value) < 0.01 ? 6 : 4)}`
}

export function ProviderCostSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const costsQuery = useQuery({
    queryKey: ['provider-costs'],
    queryFn: async () => {
      const res = await api.get<{ data: ProviderCost[] }>('/api/provider_cost/')
      return res.data.data ?? []
    },
  })

  const marginsQuery = useQuery({
    queryKey: ['provider-cost-margins'],
    queryFn: async () => {
      const res = await api.get<{ data: MarginsResponse }>(
        '/api/provider_cost/margins'
      )
      return res.data.data?.items ?? []
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await api.delete(`/api/provider_cost/${id}`)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['provider-costs'] })
      await queryClient.invalidateQueries({
        queryKey: ['provider-cost-margins'],
      })
      toast.success(t('Cost entry deleted'))
    },
    onError: () => toast.error(t('Failed to delete cost entry')),
  })

  const [form, setForm] = useState(EMPTY_FORM)

  const saveMutation = useMutation({
    mutationFn: async () => {
      await api.put('/api/provider_cost/', {
        channel_id: Number(form.channel_id),
        model_name: form.model_name.trim(),
        input_price: Number(form.input_price),
        output_price: Number(form.output_price),
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['provider-costs'] })
      await queryClient.invalidateQueries({
        queryKey: ['provider-cost-margins'],
      })
      setForm(EMPTY_FORM)
      toast.success(t('Cost entry saved'))
    },
    onError: () => toast.error(t('Failed to save cost entry')),
  })

  const canSave =
    Number(form.channel_id) > 0 &&
    form.model_name.trim() !== '' &&
    form.input_price !== '' &&
    form.output_price !== ''

  const costs = costsQuery.data ?? []
  const margins = marginsQuery.data ?? []

  return (
    <SettingsSection title={t('Provider Cost & Margin')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Upstream cost per million tokens. Margin is computed from consume logs, so it never affects billing.'
        )}
      </p>

      <div className='flex flex-col gap-2'>
        <h4 className='text-sm font-semibold'>{t('Recorded costs')}</h4>
        <div className='flex flex-wrap items-end gap-2'>
          <Input
            className='w-24'
            placeholder={t('Channel ID')}
            value={form.channel_id}
            onChange={(e) => setForm({ ...form, channel_id: e.target.value })}
          />
          <Input
            className='w-56'
            placeholder={t('Model name')}
            value={form.model_name}
            onChange={(e) => setForm({ ...form, model_name: e.target.value })}
          />
          <Input
            className='w-40'
            placeholder={t('Input price (USD / 1M)')}
            value={form.input_price}
            onChange={(e) => setForm({ ...form, input_price: e.target.value })}
          />
          <Input
            className='w-40'
            placeholder={t('Output price (USD / 1M)')}
            value={form.output_price}
            onChange={(e) => setForm({ ...form, output_price: e.target.value })}
          />
          <Button
            disabled={!canSave || saveMutation.isPending}
            onClick={() => saveMutation.mutate()}
          >
            {t('Save')}
          </Button>
        </div>
        {costs.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No cost entries yet.')}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Channel')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Input price (USD / 1M)')}</TableHead>
                <TableHead>{t('Output price (USD / 1M)')}</TableHead>
                <TableHead>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {costs.map((cost) => (
                <TableRow key={cost.id}>
                  <TableCell>{cost.channel_id}</TableCell>
                  <TableCell>{cost.model_name}</TableCell>
                  <TableCell>{usd(cost.input_price)}</TableCell>
                  <TableCell>{usd(cost.output_price)}</TableCell>
                  <TableCell>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => deleteMutation.mutate(cost.id)}
                    >
                      {t('Delete')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <div className='flex flex-col gap-2'>
        <h4 className='text-sm font-semibold'>
          {t('Margin over the last 30 days')}
        </h4>
        {margins.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No consume logs in this period.')}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Channel')}</TableHead>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Requests')}</TableHead>
                <TableHead>{t('Revenue')}</TableHead>
                <TableHead>{t('Cost')}</TableHead>
                <TableHead>{t('Margin')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {margins.map((row) => (
                <TableRow key={`${row.channel_id}-${row.model_name}`}>
                  <TableCell>{row.channel_id}</TableCell>
                  <TableCell>{row.model_name}</TableCell>
                  <TableCell>{row.requests}</TableCell>
                  <TableCell>{usd(row.revenue)}</TableCell>
                  <TableCell>
                    {row.cost_known ? (
                      usd(row.cost)
                    ) : (
                      <Badge variant='outline'>{t('No cost recorded')}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    {row.cost_known
                      ? `${usd(row.margin)} (${row.margin_percent.toFixed(1)}%)`
                      : '—'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </SettingsSection>
  )
}
