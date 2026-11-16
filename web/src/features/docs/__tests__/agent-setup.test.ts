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
import { spawnSync } from 'node:child_process'
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'

import { afterEach, describe, expect, it } from 'vitest'

import {
  AGENTS,
  createAgentSetup,
  filterAgentModels,
  type AgentKind,
} from '../agent-setup'

const homes: string[] = []
const metadata = {
  context_length: 120000,
  max_output_tokens: 12000,
  supported_endpoint_types: ['openai'],
  capabilities: ['tools'] as const,
  input_modalities: ['text'] as const,
}
const fixtureMetadata = {
  ...metadata,
  capabilities: [...metadata.capabilities],
  input_modalities: [...metadata.input_modalities],
}
const key = 'sk-test-";$()\\keep-secret'

function fixture() {
  const home = mkdtempSync(join(tmpdir(), 'dgai-agent-'))
  homes.push(home)
  return home
}

function put(home: string, path: string, text: string) {
  const file = join(home, path)
  mkdirSync(dirname(file), { recursive: true })
  writeFileSync(file, text)
  return file
}

function execute(
  home: string,
  kind: AgentKind,
  model = 'test-model',
  answers: string[] = [],
  options: { missingApp?: boolean } = {},
  metadata: Parameters<typeof createAgentSetup>[3] = fixtureMetadata
) {
  const setup = createAgentSetup(
    kind,
    'https://gateway.example/v1/',
    model,
    metadata
  )
  const path = put(home, setup.filename, setup.script)
  const bootstrap = String.raw`
import builtins, getpass, json, os, pathlib, runpy, shutil, sys
home, script, key, answers, missing = sys.argv[1:]
pathlib.Path.home = classmethod(lambda cls: cls(home))
os.environ['APPDATA'] = str(pathlib.Path(home) / 'AppData' / 'Roaming')
os.environ['XDG_CONFIG_HOME'] = str(pathlib.Path(home) / '.config')
for name in ('CLAUDE_CONFIG_DIR', 'CODEX_HOME', 'PI_CONFIG_DIR', 'OMP_CONFIG_DIR'):
    os.environ.pop(name, None)
getpass.getpass = lambda prompt: key
replies = iter(json.loads(answers))
builtins.input = lambda prompt: next(replies, '')
real_which = shutil.which
shutil.which = lambda name: (None if missing == 'yes' else '/installed/' + name) if name in ('codex', 'claude', 'omp') else real_which(name)
runpy.run_path(script, run_name='__main__')
`
  return spawnSync(
    process.env.PYTHON ?? (process.platform === 'win32' ? 'python' : 'python3'),
    [
      '-c',
      bootstrap,
      home,
      path,
      key,
      JSON.stringify(answers),
      options.missingApp ? 'yes' : 'no',
    ],
    { encoding: 'utf8' }
  )
}

function json(home: string, path: string) {
  return JSON.parse(readFileSync(join(home, path), 'utf8'))
}

function vsPath() {
  if (process.platform === 'win32') {
    return 'AppData/Roaming/Code/User/chatLanguageModels.json'
  }
  if (process.platform === 'darwin') {
    return 'Library/Application Support/Code/User/chatLanguageModels.json'
  }
  return '.config/Code/User/chatLanguageModels.json'
}

afterEach(() => {
  for (const home of homes.splice(0)) {
    rmSync(home, { recursive: true, force: true })
  }
})

describe('agent installer contracts', () => {
  it('uses endpoint capabilities with the gateway coding-family fallback', () => {
    const models = [
      {
        model_name: 'arbitrary-response-model',
        supported_endpoint_types: ['openai-response'],
      },
      { model_name: 'gpt-no-endpoint', supported_endpoint_types: [] },
      {
        model_name: 'arbitrary-messages-model',
        supported_endpoint_types: ['anthropic'],
      },
      { model_name: 'claude-chat-route', supported_endpoint_types: ['openai'] },
      { model_name: 'cx/gpt-gateway', supported_endpoint_types: ['openai'] },
    ]
    expect(filterAgentModels('codex', models)).toEqual([models[0], models[4]])
    expect(filterAgentModels('claude', models)).toEqual([models[2], models[3]])
    expect(filterAgentModels('zcode', models)).toEqual([models[3], models[4]])
    expect(filterAgentModels('omp', models)).toEqual([
      models[0],
      models[2],
      models[3],
      models[4],
    ])
  })

  it('offers a credential-free download for all five agents and normalizes v1 once', () => {
    const endpointByKind: Record<AgentKind, string> = {
      codex: 'https://gateway.example/v1/responses',
      claude: 'https://gateway.example/v1/messages',
      omp: 'https://gateway.example/v1/chat/completions',
      zcode: 'https://gateway.example/v1/chat/completions',
      vscode: 'https://gateway.example/v1/chat/completions',
    }
    for (const { kind } of AGENTS) {
      const result = createAgentSetup(
        kind,
        'https://gateway.example/v1/',
        'test-model',
        fixtureMetadata
      )
      expect(result.filename).toBe(`setup-dgai-${kind}.py`)
      expect(result.script).not.toContain(key)
      expect(result.endpoint).toBe(endpointByKind[kind])
      expect(result.runCommand).toBe(`python ${result.filename}`)
      expect(result.snippets[0].body).toBe(result.script)
    }
  })

  it('rejects credentials in URLs and control characters in model IDs', () => {
    expect(() =>
      createAgentSetup('codex', 'https://secret@example.com', 'model')
    ).toThrow()
    expect(() =>
      createAgentSetup('codex', 'https://example.com?key=secret', 'model')
    ).toThrow()
    expect(() =>
      createAgentSetup('codex', 'https://example.com', 'model\ncode')
    ).toThrow()
  })
  it('requires HTTPS for remote credential endpoints but allows local HTTP', () => {
    expect(() =>
      createAgentSetup('claude', 'http://gateway.example', 'model')
    ).toThrow(/HTTPS/)
    expect(
      createAgentSetup('claude', 'http://127.0.0.1:8080/v1', 'model').endpoint
    ).toBe('http://127.0.0.1:8080/v1/messages')
  })
})

describe('generated Python installers', () => {
  it('merges Claude settings, preserves permissions and env, and creates a private exact backup', () => {
    const home = fixture()
    const original = JSON.stringify({
      permissions: { allow: ['Read'] },
      env: { OTHER_KEY: 'keep' },
      theme: 'dark',
    })
    const file = put(home, '.claude/settings.json', original)
    const result = execute(home, 'claude')
    expect(result.status, result.stderr).toBe(0)
    const config = json(home, '.claude/settings.json')
    expect(config.permissions).toEqual({ allow: ['Read'] })
    expect(config.env).toEqual({
      OTHER_KEY: 'keep',
      ANTHROPIC_BASE_URL: 'https://gateway.example',
      ANTHROPIC_AUTH_TOKEN: key,
      ANTHROPIC_MODEL: 'test-model',
    })
    const backup = readdirSync(dirname(file)).find((name) =>
      name.startsWith('settings.json.bak.')
    )
    expect(backup).toBeTruthy()
    expect(readFileSync(join(dirname(file), backup ?? ''), 'utf8')).toBe(
      original
    )
    if (process.platform !== 'win32') {
      expect(statSync(file).mode & 0o777).toBe(0o600)
      expect(statSync(join(dirname(file), backup ?? '')).mode & 0o777).toBe(
        0o600
      )
    }
    expect(result.stdout + result.stderr).not.toContain(key)
  })

  it.each(['claude', 'zcode'] as const)(
    'does not clobber invalid %s JSON',
    (kind) => {
      const home = fixture()
      const relative =
        kind === 'claude' ? '.claude/settings.json' : '.zcode/v2/config.json'
      const file = put(home, relative, '{invalid')
      const result = execute(home, kind)
      expect(result.status).not.toBe(0)
      expect(readFileSync(file, 'utf8')).toBe('{invalid')
      expect(readdirSync(dirname(file))).toEqual(
        ['config.json', 'settings.json'].filter((name) =>
          relative.endsWith(name)
        )
      )
    }
  )

  it('preserves Codex TOML byte-for-byte and serializes hostile model text as data', () => {
    const home = fixture()
    const original =
      '# keep formatting\nmodel = "old"\n[model_providers.other]\nname = "Other"\n'
    const file = put(home, '.codex/config.toml', original)
    const model = 'x";$(touch PWNED);\\unicode-模型'
    const result = execute(home, 'codex', model)
    expect(result.status, result.stderr).toBe(0)
    expect(readFileSync(file, 'utf8')).toBe(original)
    expect(json(home, '.dgai/codex.json')).toMatchObject({
      model,
      api_key: key,
      base_url: 'https://gateway.example/v1',
    })
    const launcher = readFileSync(join(home, '.dgai/launch-codex.py'), 'utf8')
    expect(launcher).not.toContain(model)
    expect(launcher).not.toContain(key)
    expect(launcher).toContain('env_key=')
    expect(launcher).not.toContain('shell=True')
    const probe = String.raw`
import json, runpy, shutil, subprocess, sys, tomllib
launcher = sys.argv[1]
sys.argv = [launcher]
shutil.which = lambda name: '/native/' + name + '.exe'
def capture(args, env):
    parsed = {}
    for index, argument in enumerate(args):
        if argument == '-c':
            parsed.update(tomllib.loads(args[index + 1]))
    print(json.dumps({'config': parsed, 'key': env['DGAI_API_KEY']}))
    return 0
subprocess.call = capture
runpy.run_path(launcher, run_name='__main__')
`
    const launched = spawnSync(
      process.env.PYTHON ??
        (process.platform === 'win32' ? 'python' : 'python3'),
      ['-c', probe, join(home, '.dgai/launch-codex.py')],
      { encoding: 'utf8' }
    )
    expect(launched.status, launched.stderr).toBe(0)
    expect(JSON.parse(launched.stdout)).toMatchObject({
      key,
      config: {
        model,
        model_provider: 'dgai',
        model_providers: {
          dgai: { env_key: 'DGAI_API_KEY', wire_api: 'responses' },
        },
      },
    })
  })

  it('aborts on invalid existing Codex TOML and on missing CLI prerequisites', () => {
    const home = fixture()
    const file = put(home, '.codex/config.toml', '[broken')
    expect(execute(home, 'codex').status).not.toBe(0)
    expect(readFileSync(file, 'utf8')).toBe('[broken')
    expect(
      execute(fixture(), 'claude', 'model', [], { missingApp: true }).stderr
    ).toContain('Install claude')
  })

  it('isolates omp from existing YAML and preserves other providers in its JSON-compatible profile', () => {
    const home = fixture()
    const original = 'providers:\n  old:\n    api: openai-completions\n'
    const file = put(home, '.omp/agent/models.yml', original)
    put(
      home,
      '.omp/profiles/dgai/agent/models.yml',
      JSON.stringify({
        providers: { other: { baseUrl: 'https://other.example' } },
      })
    )
    const result = execute(home, 'omp')
    expect(result.status, result.stderr).toBe(0)
    expect(readFileSync(file, 'utf8')).toBe(original)
    expect(json(home, '.omp/profiles/dgai/agent/models.yml')).toMatchObject({
      providers: {
        other: { baseUrl: 'https://other.example' },
        dgai: {
          apiKey: 'DGAI_API_KEY',
          models: [
            {
              id: 'test-model',
              api: 'openai-completions',
              contextWindow: 120000,
              maxTokens: 12000,
            },
          ],
        },
      },
    })
    expect(json(home, '.dgai/omp.json').api_key).toBe(key)
  })

  it('keeps ZCode providers and model metadata when adding the chosen model', () => {
    const home = fixture()
    put(
      home,
      '.zcode/v2/config.json',
      JSON.stringify({
        theme: 'dark',
        provider: {
          old: { name: 'keep' },
          dgai: {
            models: { previous: { custom: true } },
            options: { extra: true },
          },
        },
      })
    )
    const result = execute(home, 'zcode')
    expect(result.status, result.stderr).toBe(0)
    expect(json(home, '.zcode/v2/config.json')).toMatchObject({
      theme: 'dark',
      provider: {
        old: { name: 'keep' },
        dgai: {
          kind: 'openai-compatible',
          options: {
            extra: true,
            apiKey: key,
            baseURL: 'https://gateway.example/v1',
          },
          models: {
            previous: { custom: true },
            'test-model': { limit: { context: 120000, output: 12000 } },
          },
        },
      },
    })
  })

  it('merges VS Code JSONC while preserving real secret references and unrelated groups', () => {
    const home = fixture()
    const reference = '${input:existing-secret-id}'
    put(
      home,
      vsPath(),
      `// keep providers\n[ {"name":"other", "vendor":"openai", "apiKey":"${reference}"}, {"name":"dgai","vendor":"customendpoint","apiKey":"${reference}","models":[{"id":"old","url":"https://example.com//path"},],}, ]`
    )
    const result = execute(home, 'vscode')
    expect(result.status, result.stderr).toBe(0)
    const config = json(home, vsPath())
    expect(config[0].name).toBe('other')
    expect(config[1].apiKey).toBe(reference)
    expect(config[1].models).toContainEqual({
      id: 'old',
      url: 'https://example.com//path',
    })
    expect(config[1].models).toContainEqual({
      id: 'test-model',
      name: 'test-model',
      url: 'https://gateway.example/v1/chat/completions',
      apiType: 'chat-completions',
      maxInputTokens: 108000,
      maxOutputTokens: 12000,
      toolCalling: true,
      vision: false,
    })
    expect(readFileSync(join(home, vsPath()), 'utf8')).not.toContain(key)
    expect(result.stdout).toContain('dgai gear')
  })

  it('does not invent a VS Code secret reference on a fresh install', () => {
    const home = fixture()
    const result = execute(home, 'vscode')
    expect(result.status, result.stderr).toBe(0)
    expect(json(home, vsPath())[0]).not.toHaveProperty('apiKey')
  })
  it('asks for undocumented VS Code limits and capabilities instead of inventing them', () => {
    const home = fixture()
    const incompleteMetadata: Parameters<typeof createAgentSetup>[3] = {
      supported_endpoint_types: ['openai'],
    }
    const result = execute(
      home,
      'vscode',
      'model-without-catalog-metadata',
      ['', '', '120000', '12000', 'y', 'n'],
      {},
      incompleteMetadata
    )
    expect(result.status, result.stderr).toBe(0)
    const config = json(home, vsPath())
    expect(config[0].models[0]).toMatchObject({
      maxInputTokens: 108000,
      maxOutputTokens: 12000,
      toolCalling: true,
      vision: false,
    })
  })

  it('rejects duplicate JSON keys instead of silently losing settings', () => {
    const home = fixture()
    const original = '{"env":{},"env":{"KEEP":"value"}}'
    const file = put(home, '.claude/settings.json', original)
    expect(execute(home, 'claude').status).not.toBe(0)
    expect(readFileSync(file, 'utf8')).toBe(original)
  })
  it('leaves a hand-written omp profile unchanged rather than parsing YAML incorrectly', () => {
    const home = fixture()
    const original = 'providers:\n  keep:\n    api: openai-completions\n'
    const file = put(home, '.omp/profiles/dgai/agent/models.yml', original)
    expect(execute(home, 'omp').status).not.toBe(0)
    expect(readFileSync(file, 'utf8')).toBe(original)
  })

  it('does not normalize malformed VS Code JSONC into an empty valid configuration', () => {
    const home = fixture()
    const file = put(home, vsPath(), '[,]')
    expect(execute(home, 'vscode').status).not.toBe(0)
    expect(readFileSync(file, 'utf8')).toBe('[,]')
  })
})
