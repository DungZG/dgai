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
import { describe, expect, it } from 'vitest'

import { revenueReportToCsv, type RevenueReport } from '../api'

function report(overrides: Partial<RevenueReport> = {}): RevenueReport {
  return {
    group_by: 'user',
    start_timestamp: 0,
    end_timestamp: 0,
    quota_per_unit: 500000,
    rows: [
      {
        key: 'alice',
        requests: 2,
        prompt_tokens: 1000,
        completion_tokens: 500,
        quota: 500000,
        revenue: 1,
        cost: 0.25,
        margin: 0.75,
        margin_percent: 75,
        cost_known: true,
      },
    ],
    totals: {
      key: 'total',
      requests: 2,
      prompt_tokens: 1000,
      completion_tokens: 500,
      quota: 500000,
      revenue: 1,
      cost: 0.25,
      margin: 0.75,
      margin_percent: 75,
      cost_known: true,
    },
    ...overrides,
  }
}

describe('revenueReportToCsv', () => {
  it('writes a header, the rows and the totals', () => {
    const lines = revenueReportToCsv(report()).split('\n')
    expect(lines[0]).toBe(
      'user,requests,prompt_tokens,completion_tokens,revenue_usd,cost_usd,margin_usd,margin_percent'
    )
    expect(lines[1]).toBe('alice,2,1000,500,1.000000,0.250000,0.750000,75.00')
    expect(lines[2]).toBe('total,2,1000,500,1.000000,0.250000,0.750000,75.00')
  })

  it('leaves cost columns empty when no cost is recorded', () => {
    const csv = revenueReportToCsv(
      report({
        rows: [
          {
            key: 'bob',
            requests: 1,
            prompt_tokens: 10,
            completion_tokens: 10,
            quota: 250000,
            revenue: 0.5,
            cost: 0,
            margin: 0.5,
            margin_percent: 100,
            cost_known: false,
          },
        ],
      })
    )
    expect(csv.split('\n')[1]).toBe('bob,1,10,10,0.500000,,,')
  })

  it('quotes commas and neutralises formula-like cells', () => {
    const csv = revenueReportToCsv(
      report({
        rows: [
          {
            key: '=cmd|calc',
            requests: 1,
            prompt_tokens: 0,
            completion_tokens: 0,
            quota: 0,
            revenue: 0,
            cost: 0,
            margin: 0,
            margin_percent: 0,
            cost_known: true,
          },
        ],
      })
    )
    // A leading = is prefixed with an apostrophe; the value has no comma so it
    // is not additionally quoted.
    expect(csv.split('\n')[1].startsWith(`'=cmd|calc,`)).toBe(true)
  })
})
