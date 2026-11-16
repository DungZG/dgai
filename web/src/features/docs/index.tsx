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

import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

import { AgentWorkbench } from './agent-workbench'
import { DOC_SECTIONS, DOCS_BASE_URL } from './content'

export function Docs() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [activeId, setActiveId] = useState(DOC_SECTIONS[0].id)
  const active =
    DOC_SECTIONS.find((section) => section.id === activeId) ?? DOC_SECTIONS[0]
  const serverAddress =
    typeof status?.server_address === 'string' &&
    status.server_address.trim() !== ''
      ? status.server_address.trim().replace(/\/+$/, '')
      : DOCS_BASE_URL
  const body = active.body.replaceAll(DOCS_BASE_URL, serverAddress)

  return (
    <PublicLayout>
      <div className='mx-auto flex max-w-6xl flex-col gap-10 px-4 py-6 sm:px-6 lg:px-8'>
        <AgentWorkbench serverAddress={serverAddress} />
        <div className='flex min-w-0 flex-col gap-8 lg:flex-row'>
          <nav
            aria-label={t('Documentation sections')}
            className='shrink-0 lg:w-56'
          >
            <h2 className='mb-4 text-xl font-semibold'>{t('Reference')}</h2>
            <ul className='flex flex-wrap gap-2 lg:flex-col lg:gap-1'>
              {DOC_SECTIONS.map((section) => (
                <li key={section.id}>
                  <Button
                    type='button'
                    variant='ghost'
                    aria-current={section.id === active.id ? 'page' : undefined}
                    onClick={() => setActiveId(section.id)}
                    className={cn(
                      'w-full rounded-md px-3 py-2 text-left text-sm transition-colors',
                      section.id === active.id
                        ? 'bg-muted text-foreground font-medium'
                        : 'text-muted-foreground hover:bg-muted/50'
                    )}
                  >
                    {t(section.title)}
                  </Button>
                </li>
              ))}
            </ul>
          </nav>
          <article className='min-w-0 flex-1'>
            <Markdown className='max-w-3xl'>{body}</Markdown>
          </article>
        </div>
      </div>
    </PublicLayout>
  )
}
