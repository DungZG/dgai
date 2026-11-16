import { useTranslation } from 'react-i18next'

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
import { CodeBlock } from '@/components/ai-elements/code-block'

import type { AgentSetup } from './agent-setup'

export function SetupInstructions(props: { setup: AgentSetup }) {
  const { t } = useTranslation()
  const previews = props.setup.snippets.filter(
    (snippet) => snippet.id !== 'setup-script'
  )

  return (
    <div className='space-y-5'>
      <ol className='border-border/70 space-y-3 border-l pl-5 text-sm leading-6'>
        {props.setup.steps.map((step, index) => (
          <li key={`${props.setup.kind}-${step}`} className='pl-1'>
            <span className='font-medium'>{index + 1}. </span>
            {t(step)}
          </li>
        ))}
      </ol>
      {previews.length > 0 && (
        <div className='space-y-3'>
          <h3 className='text-sm font-medium'>
            {t('Manual configuration preview')}
          </h3>
          {previews.map((snippet) => (
            <CodeBlock
              key={snippet.id}
              code={snippet.body}
              filename={snippet.filename}
              language={snippet.language}
              showToolbar
              title={t(snippet.title)}
            />
          ))}
        </div>
      )}
      <ul className='text-muted-foreground space-y-1 text-xs leading-5'>
        {props.setup.notes.map((note) => (
          <li key={note}>• {t(note)}</li>
        ))}
      </ul>
    </div>
  )
}
