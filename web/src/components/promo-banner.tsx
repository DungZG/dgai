import { Gift, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

// Bump the id to bring a dismissed banner back for a new campaign.
const PROMO_BANNER_ID = 'dgai-hstx-2026'
const PROMO_BANNER_END = Date.parse('2026-12-31T23:59:59+07:00')
const PROMO_BANNER_STORAGE_KEY = 'dgai-promo-banner'

function isBannerDismissed() {
  if (Date.now() > PROMO_BANNER_END) return true
  try {
    return localStorage.getItem(PROMO_BANNER_STORAGE_KEY) === PROMO_BANNER_ID
  } catch {
    return false
  }
}

export function PromoBanner() {
  const { t } = useTranslation()
  const [dismissed, setDismissed] = useState(isBannerDismissed)

  if (dismissed) return null

  const dismiss = () => {
    setDismissed(true)
    try {
      localStorage.setItem(PROMO_BANNER_STORAGE_KEY, PROMO_BANNER_ID)
    } catch {
      // Storage can be blocked; hiding it for this view is enough.
    }
  }

  return (
    <div className='flex shrink-0 items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-3 py-1.5 text-xs text-amber-700 dark:text-amber-300'>
      <Gift className='size-3.5 shrink-0' />
      <p className='min-w-0 flex-1 truncate'>
        <span className='font-medium'>
          {t('Top up $10 and enter code DGAIHSTX to get $5 bonus')}
        </span>
        <span className='hidden text-amber-700/70 sm:inline dark:text-amber-300/70'>
          {' · '}
          {t('One use per account · Ends 31/12/2026')}
        </span>
      </p>
      <button
        type='button'
        onClick={dismiss}
        className='shrink-0 rounded p-0.5 transition-colors hover:bg-amber-500/20'
        aria-label={t('Dismiss')}
      >
        <X className='size-3.5' />
      </button>
    </div>
  )
}
