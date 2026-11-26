import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { handleServerError } from '@/lib/handle-server-error'

import { claimDailyCheckin, getDailyCheckin } from '../../api'
import { PanelWrapper } from '../ui/panel-wrapper'

const CHECKIN_QUERY_KEY = ['daily-checkin'] as const

export function CheckinCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const statusQuery = useQuery({
    queryKey: CHECKIN_QUERY_KEY,
    queryFn: async () => {
      const res = await getDailyCheckin()
      return res.data ?? null
    },
    staleTime: 60_000,
  })

  const claim = useMutation({
    mutationFn: claimDailyCheckin,
    onSuccess: (res) => {
      if (!res.success) {
        handleServerError(res, t('Check-in failed'))
        return
      }
      const credits = res.data?.credits ?? 0
      toast.success(
        t('You received {{credits}} credits', { credits: String(credits) })
      )
      void queryClient.invalidateQueries({ queryKey: CHECKIN_QUERY_KEY })
      void queryClient.invalidateQueries({ queryKey: ['user-self'] })
    },
    onError: (error) => handleServerError(error, t('Check-in failed')),
  })

  const status = statusQuery.data
  const claimed = status?.claimed_today ?? false

  return (
    <PanelWrapper
      title={
        <span className='flex items-center gap-2'>
          <IconBadge tone='success' size='sm'>
            <CalendarCheck />
          </IconBadge>
          {t('Daily check-in')}
        </span>
      }
      description={t('10 credits = $1 · 1–5 credits per day')}
      loading={statusQuery.isLoading}
    >
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-col gap-1'>
          {claimed ? (
            <>
              <span className='text-sm font-medium'>
                {t('You received {{credits}} credits today', {
                  credits: String(status?.credits_today ?? 0),
                })}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Come back after 00:00 to check in again')}
              </span>
            </>
          ) : (
            <span className='text-muted-foreground text-sm'>
              {t('Check in today to receive 1–5 credits')}
            </span>
          )}
          <span className='text-muted-foreground text-xs'>
            {t('Total credits: {{credits}}', {
              credits: String(status?.total_credits ?? 0),
            })}
          </span>
        </div>
        <Button
          onClick={() => claim.mutate()}
          disabled={claimed || claim.isPending || statusQuery.isLoading}
        >
          <CalendarCheck data-icon='inline-start' />
          {claimed ? t('Checked in') : t('Check in')}
        </Button>
      </div>
    </PanelWrapper>
  )
}
