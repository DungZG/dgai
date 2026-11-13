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

import { api } from '@/lib/api'

export type RevenueGroupBy = 'day' | 'user' | 'model'

export type RevenueRow = {
  key: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  quota: number
  revenue: number
  cost: number
  margin: number
  margin_percent: number
  cost_known: boolean
}

export type RevenueReport = {
  group_by: RevenueGroupBy
  start_timestamp: number
  end_timestamp: number
  quota_per_unit: number
  rows: RevenueRow[]
  totals: RevenueRow
}

export async function getRevenueReport(
  groupBy: RevenueGroupBy,
  startTimestamp: number,
  endTimestamp: number
): Promise<RevenueReport> {
  const res = await api.get<{ data: RevenueReport }>('/api/revenue/report', {
    params: {
      group_by: groupBy,
      start_timestamp: startTimestamp,
      end_timestamp: endTimestamp,
    },
  })
  return res.data.data
}

export function useRevenueReport(
  groupBy: RevenueGroupBy,
  startTimestamp: number,
  endTimestamp: number
) {
  return useQuery({
    queryKey: ['revenue-report', groupBy, startTimestamp, endTimestamp],
    queryFn: () => getRevenueReport(groupBy, startTimestamp, endTimestamp),
  })
}

function csvCell(value: string | number): string {
  const text = String(value)
  // A leading =, +, - or @ makes spreadsheet software treat the cell as a
  // formula, so prefix those with a quote.
  const safe = /^[=+\-@]/.test(text) ? `'${text}` : text
  return /[",\n]/.test(safe) ? `"${safe.replaceAll('"', '""')}"` : safe
}

/** Builds the CSV text for a report; the caller decides how to save it. */
export function revenueReportToCsv(report: RevenueReport): string {
  const header = [
    report.group_by,
    'requests',
    'prompt_tokens',
    'completion_tokens',
    'revenue_usd',
    'cost_usd',
    'margin_usd',
    'margin_percent',
  ]
  const lines = [header.join(',')]
  for (const row of [...report.rows, report.totals]) {
    lines.push(
      [
        csvCell(row.key),
        csvCell(row.requests),
        csvCell(row.prompt_tokens),
        csvCell(row.completion_tokens),
        csvCell(row.revenue.toFixed(6)),
        csvCell(row.cost_known ? row.cost.toFixed(6) : ''),
        csvCell(row.cost_known ? row.margin.toFixed(6) : ''),
        csvCell(row.cost_known ? row.margin_percent.toFixed(2) : ''),
      ].join(',')
    )
  }
  return lines.join('\n')
}
