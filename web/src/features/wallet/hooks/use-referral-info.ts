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

import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import { getReferralInfo } from '../api'
import type { ReferralInfo } from '../types'

/**
 * Referral totals, commission rate and the recent reward events.
 *
 * Shared by the wallet card and the header bell so both read one cache entry.
 * It stays disabled for signed-out visitors: the endpoint requires a session.
 */
export function useReferralInfo() {
  const user = useAuthStore((s) => s.auth.user)
  return useQuery({
    queryKey: ['referral-info'],
    queryFn: async () => {
      const response = requireServerSuccess(await getReferralInfo())
      return (response.data ?? null) as ReferralInfo | null
    },
    enabled: Boolean(user),
    staleTime: 1000 * 60,
  })
}
