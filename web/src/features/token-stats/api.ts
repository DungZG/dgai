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

export type UserTokenStat = {
  key: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  top_model: string
  top_model_tokens: number
  top_model_share: number
  model_count: number
}

export type UserTokenStatsReport = {
  start_timestamp: number
  end_timestamp: number
  rows: UserTokenStat[]
  totals: UserTokenStat
}

export async function getUserTokenStats(
  startTimestamp: number,
  endTimestamp: number
): Promise<UserTokenStatsReport> {
  const res = await api.get<{ data: UserTokenStatsReport }>(
    '/api/revenue/token-stats',
    {
      params: {
        start_timestamp: startTimestamp,
        end_timestamp: endTimestamp,
      },
    }
  )
  return res.data.data
}

export function useUserTokenStats(
  startTimestamp: number,
  endTimestamp: number
) {
  return useQuery({
    queryKey: ['user-token-stats', startTimestamp, endTimestamp],
    queryFn: () => getUserTokenStats(startTimestamp, endTimestamp),
  })
}
