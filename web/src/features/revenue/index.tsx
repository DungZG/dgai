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
import { formatCurrencyUSD, formatNumber } from '@/lib/format'

import {
  revenueReportToCsv,
  useRevenueReport,
  type RevenueGroupBy,
} from './api'

const GROUPS: RevenueGroupBy[] = ['day', 'user', 'model']

function toDateInput(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/** Local midnight of the picked day; the server buckets on the same boundary. */
function dayStart(value: string): number {
  return Math.floor(new Date(`${value}T00:00:00`).getTime() / 1000)
}

function dayEnd(value: string): number {
  return Math.floor(new Date(`${value}T23:59:59`).getTime() / 1000)
}

export function Revenue() {
  const { t } = useTranslation()
  const today = new Date()
  const [groupBy, setGroupBy] = useState<RevenueGroupBy>('day')
  const [startDate, setStartDate] = useState(() => {
    const from = new Date(today.getTime() - 29 * 86400_000)
    return toDateInput(Math.floor(from.getTime() / 1000))
  })
  const [endDate, setEndDate] = useState(() => toDateInput(Math.floor(today.getTime() / 1000)))

  const startTimestamp = dayStart(startDate)
  const endTimestamp = dayEnd(endDate)
  const query = useRevenueReport(groupBy, startTimestamp, endTimestamp)
  const report = query.data

  function downloadCsv() {
    if (!report) return
    const blob = new Blob([revenueReportToCsv(report)], {
      type: 'text/csv;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `revenue-${groupBy}-${startDate}-${endDate}.csv`
    link.click()
    URL.revokeObjectURL(url)
  }

  const totals = report?.totals
  const groupLabel: Record<RevenueGroupBy, string> = {
    day: t('Date'),
    user: t('User'),
    model: t('Model'),
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Revenue')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={downloadCsv} disabled={!report}>
          {t('Export CSV')}
        </Button>
      </SectionPageLayout.Actions>
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
            <div className='flex items-center gap-1'>
              {GROUPS.map((group) => (
                <Button
                  key={group}
                  size='sm'
                  variant={groupBy === group ? 'default' : 'outline'}
                  onClick={() => setGroupBy(group)}
                >
                  {groupLabel[group]}
                </Button>
              ))}
            </div>
          </div>

          <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
            <SummaryCell
              label={t('Revenue')}
              value={formatCurrencyUSD(totals?.revenue ?? 0)}
            />
            <SummaryCell
              label={t('Cost')}
              value={formatCurrencyUSD(totals?.cost ?? 0)}
            />
            <SummaryCell
              label={t('Margin')}
              value={
                totals?.cost_known
                  ? `${formatCurrencyUSD(totals.margin)} (${totals.margin_percent.toFixed(1)}%)`
                  : '—'
              }
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
                  <TableHead>{groupLabel[groupBy]}</TableHead>
                  <TableHead className='text-right'>{t('Requests')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Prompt tokens')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Completion tokens')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Revenue')}</TableHead>
                  <TableHead className='text-right'>{t('Cost')}</TableHead>
                  <TableHead className='text-right'>{t('Margin')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(report?.rows ?? []).map((row) => (
                  <TableRow key={row.key}>
                    <TableCell>{row.key}</TableCell>
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
                      {formatCurrencyUSD(row.revenue)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {row.cost_known ? (
                        formatCurrencyUSD(row.cost)
                      ) : (
                        <Badge variant='outline'>{t('No cost recorded')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      {row.cost_known
                        ? `${formatCurrencyUSD(row.margin)} (${row.margin_percent.toFixed(1)}%)`
                        : '—'}
                    </TableCell>
                  </TableRow>
                ))}
                {report && report.rows.length === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={7}
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
      <div className='text-muted-foreground text-xs'>{label}</div>
      <div className='text-lg font-semibold tabular-nums'>{value}</div>
    </div>
  )
}
