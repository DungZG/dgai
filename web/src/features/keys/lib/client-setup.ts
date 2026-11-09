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

/**
 * Snippets that wire a dgai token into Claude Code and Codex.
 *
 * Claude Code reads ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN and speaks the
 * Anthropic Messages API, which the gateway serves at /v1/messages.
 * Codex reads a TOML provider block and speaks the Responses API at /v1.
 */

export type ClientKind = 'claude-code' | 'codex'

export type ClientSnippet = {
  id: string
  title: string
  /** Language for the code block label. */
  language: 'bash' | 'powershell' | 'toml' | 'json'
  body: string
}

export function normalizeKey(raw: string): string {
  const trimmed = raw.trim()
  return trimmed.startsWith('sk-') ? trimmed : `sk-${trimmed}`
}

/** Claude Code wants the origin only; it appends /v1/messages itself. */
export function claudeBaseUrl(serverAddress: string): string {
  return serverAddress.replace(/\/+$/, '')
}

/** Codex posts to base_url + /responses, so the /v1 suffix is required. */
export function codexBaseUrl(serverAddress: string): string {
  return `${claudeBaseUrl(serverAddress)}/v1`
}

export function claudeSnippets(
  serverAddress: string,
  apiKey: string,
  models: { sonnet?: string; opus?: string; haiku?: string }
): ClientSnippet[] {
  const base = claudeBaseUrl(serverAddress)
  const key = normalizeKey(apiKey)

  const envLines = [
    `export ANTHROPIC_BASE_URL="${base}"`,
    `export ANTHROPIC_AUTH_TOKEN="${key}"`,
  ]
  if (models.sonnet) envLines.push(`export ANTHROPIC_MODEL="${models.sonnet}"`)
  if (models.opus)
    envLines.push(`export ANTHROPIC_DEFAULT_OPUS_MODEL="${models.opus}"`)
  if (models.haiku)
    envLines.push(`export ANTHROPIC_DEFAULT_HAIKU_MODEL="${models.haiku}"`)

  const psLines = [
    `$env:ANTHROPIC_BASE_URL = "${base}"`,
    `$env:ANTHROPIC_AUTH_TOKEN = "${key}"`,
  ]
  if (models.sonnet) psLines.push(`$env:ANTHROPIC_MODEL = "${models.sonnet}"`)
  if (models.opus)
    psLines.push(`$env:ANTHROPIC_DEFAULT_OPUS_MODEL = "${models.opus}"`)
  if (models.haiku)
    psLines.push(`$env:ANTHROPIC_DEFAULT_HAIKU_MODEL = "${models.haiku}"`)

  return [
    {
      id: 'claude-bash',
      title: 'Shell (bash / zsh)',
      language: 'bash',
      body: envLines.join('\n'),
    },
    {
      id: 'claude-powershell',
      title: 'PowerShell (Windows)',
      language: 'powershell',
      body: psLines.join('\n'),
    },
    {
      id: 'claude-settings',
      title: '~/.claude/settings.json',
      language: 'json',
      body: JSON.stringify(
        {
          env: {
            ANTHROPIC_BASE_URL: base,
            ANTHROPIC_AUTH_TOKEN: key,
            ...(models.sonnet ? { ANTHROPIC_MODEL: models.sonnet } : {}),
            ...(models.opus
              ? { ANTHROPIC_DEFAULT_OPUS_MODEL: models.opus }
              : {}),
            ...(models.haiku
              ? { ANTHROPIC_DEFAULT_HAIKU_MODEL: models.haiku }
              : {}),
          },
        },
        null,
        2
      ),
    },
  ]
}

export function codexSnippets(
  serverAddress: string,
  apiKey: string,
  model: string
): ClientSnippet[] {
  const base = codexBaseUrl(serverAddress)
  const key = normalizeKey(apiKey)
  const provider = 'dgai'

  const toml = [
    `model = "${model}"`,
    `model_provider = "${provider}"`,
    '',
    `[model_providers.${provider}]`,
    'name = "dgai"',
    `base_url = "${base}"`,
    'wire_api = "responses"',
    'env_key = "DGAI_API_KEY"',
  ].join('\n')

  return [
    {
      id: 'codex-config',
      title: '~/.codex/config.toml',
      language: 'toml',
      body: toml,
    },
    {
      id: 'codex-bash',
      title: 'Shell (bash / zsh)',
      language: 'bash',
      body: [`export DGAI_API_KEY="${key}"`].join('\n'),
    },
    {
      id: 'codex-powershell',
      title: 'PowerShell (Windows)',
      language: 'powershell',
      body: [`$env:DGAI_API_KEY = "${key}"`].join('\n'),
    },
  ]
}
