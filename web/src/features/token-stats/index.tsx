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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatNumber } from '@/lib/format'

import { useUserTokenStats } from './api'

function toDateInput(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/** Local midnight of the picked day; matches the day picker of the revenue page. */
function dayStart(value: string): number {
  return Math.floor(new Date(`${value}T00:00:00`).getTime() / 1000)
}

function dayEnd(value: string): number {
  return Math.floor(new Date(`${value}T23:59:59`).getTime() / 1000)
}

export function UserTokenStats() {
  const { t } = useTranslation()
  const today = new Date()
  const [startDate, setStartDate] = useState(() =>
    toDateInput(Math.floor((today.getTime() - 29 * 86400_000) / 1000))
  )
  const [endDate, setEndDate] = useState(() =>
    toDateInput(Math.floor(today.getTime() / 1000))
  )

  const query = useUserTokenStats(dayStart(startDate), dayEnd(endDate))
  const report = query.data
  const totals = report?.totals

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('User Token Usage')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <div className='flex flex-wrap items-end gap-2'>
            <label className='flex flex-col gap-1 text-xs'>
              {t('From')}
              <Input
                type='date'
                value={startDate}
                max={endDate}
                onChange={(event) => setStartDate(event.target.value)}
              />
            </label>
            <label className='flex flex-col gap-1 text-xs'>
              {t('To')}
              <Input
                type='date'
                value={endDate}
                min={startDate}
                onChange={(event) => setEndDate(event.target.value)}
              />
            </label>
            <p className='text-muted-foreground text-xs'>
              {t('Token usage by user')}
            </p>
          </div>

          <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
            <SummaryCell
              label={t('Active users')}
              value={formatNumber(report?.rows.length ?? 0)}
            />
            <SummaryCell
              label={t('Total tokens')}
              value={formatNumber(totals?.total_tokens ?? 0)}
            />
            <SummaryCell
              label={t('Requests')}
              value={formatNumber(totals?.requests ?? 0)}
            />
          </div>

          <div className='min-h-0 flex-1 overflow-auto rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead className='text-right'>{t('Requests')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Prompt tokens')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Completion tokens')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Total tokens')}
                  </TableHead>
                  <TableHead>{t('Top model')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Top model tokens')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Share of total tokens')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Models used')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(report?.rows ?? []).map((row) => (
                  <TableRow key={row.key}>
                    <TableCell className='font-medium'>{row.key}</TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.requests)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.prompt_tokens)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.completion_tokens)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.total_tokens)}
                    </TableCell>
                    <TableCell>{row.top_model || '—'}</TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.top_model_tokens)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {row.top_model_share.toFixed(1)}%
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.model_count)}
                    </TableCell>
                  </TableRow>
                ))}
                {report && report.rows.length === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={9}
                      className='text-muted-foreground text-center'
                    >
                      {t('No consume logs in this period.')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function SummaryCell({ label, value }: { label: string; value: string }) {
  return (
    <div className='rounded-lg border p-3'>
      <p className='text-muted-foreground text-xs'>{label}</p>
      <p className='text-xl font-semibold'>{value}</p>
    </div>
  )
}
