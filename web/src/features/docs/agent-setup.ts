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

import type { PricingModel } from '../pricing/types'

export type AgentKind = 'codex' | 'claude' | 'omp' | 'zcode' | 'vscode'
type ModelMetadata = Pick<
  PricingModel,
  | 'context_length'
  | 'max_output_tokens'
  | 'capabilities'
  | 'input_modalities'
  | 'supported_endpoint_types'
>
export type AgentSetupSnippet = {
  id: string
  title: string
  language: string
  body: string
  filename: string
  download?: boolean
}
export type AgentSetup = {
  kind: AgentKind
  title: string
  summary: string
  target: string
  endpoint: string
  steps: string[]
  snippets: AgentSetupSnippet[]
  script: string
  filename: string
  runCommand: string
  notes: string[]
}

export const AGENTS: {
  kind: AgentKind
  title: string
  summary: string
  sourceUrl: string
}[] = [
  {
    kind: 'codex',
    title: 'Codex',
    summary: 'OpenAI Responses API with a dedicated dgai launcher.',
    sourceUrl: 'https://developers.openai.com/codex/config-reference',
  },
  {
    kind: 'claude',
    title: 'Claude Code',
    summary: 'Anthropic Messages API with your selected model.',
    sourceUrl: 'https://code.claude.com/docs/en/settings',
  },
  {
    kind: 'omp',
    title: 'omp',
    summary: 'A dedicated dgai profile with your selected model.',
    sourceUrl: 'https://omp.sh/docs/custom-models',
  },
  {
    kind: 'zcode',
    title: 'ZCode',
    summary: 'An OpenAI-compatible provider in ZCode Model Settings.',
    sourceUrl: 'https://vercel.com/docs/ai-gateway/coding-agents/zcode',
  },
  {
    kind: 'vscode',
    title: 'VS Code',
    summary:
      'Native Custom Endpoint with secure API key enrollment in VS Code.',
    sourceUrl:
      'https://code.visualstudio.com/docs/agent-customization/language-models',
  },
]

export function filterAgentModels<
  T extends Pick<PricingModel, 'model_name' | 'supported_endpoint_types'>,
>(kind: AgentKind, models: T[]): T[] {
  const endpointsByKind: Record<AgentKind, string[]> = {
    codex: ['openai-response'],
    claude: ['anthropic'],
    omp: ['openai', 'openai-response', 'anthropic'],
    zcode: ['openai'],
    vscode: ['openai', 'openai-response', 'anthropic'],
  }
  const endpoints = endpointsByKind[kind]
  return models.filter((model) => {
    const supported = model.supported_endpoint_types ?? []
    if (supported.some((endpoint) => endpoints.includes(endpoint))) {
      return true
    }
    if (!supported.includes('openai')) {
      return false
    }
    if (kind === 'claude') {
      return /^(?:[^/]+\/)*claude[-.]/i.test(model.model_name)
    }
    if (kind === 'codex') {
      return /^(?:[^/]+\/)*(?:gpt-|cx\/|o\d(?:-|$))/i.test(model.model_name)
    }
    return false
  })
}

// JSON is embedded as a Python string, never as executable model or URL text.
const INSTALLER = String.raw`
import csv
import getpass
import io
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import tomllib
import uuid
import warnings

def object_pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("Duplicate configuration key: " + key)
        result[key] = value
    return result

def jsonc(text):
    # Strip only comments and trailing commas outside JSON strings.
    output, i = [], 0
    while i < len(text):
        if text[i] == '"':
            start = i
            i += 1
            while i < len(text):
                if text[i] == '\\':
                    i += 2
                elif text[i] == '"':
                    i += 1
                    break
                else:
                    i += 1
            output.append(text[start:i])
        elif text.startswith('//', i):
            end = text.find('\n', i)
            i = len(text) if end < 0 else end
            output.append(' ')
        elif text.startswith('/*', i):
            end = text.find('*/', i + 2)
            if end < 0:
                raise ValueError("Unterminated JSON comment")
            output.append(' ')
            i = end + 2
        else:
            output.append(text[i])
            i += 1
    text = ''.join(output)
    output, i = [], 0
    decoder = json.JSONDecoder()
    while i < len(text):
        if text[i] == '"':
            _, end = decoder.raw_decode(text, i)
            output.append(text[i:end])
            i = end
        elif text[i] == ',' and ''.join(output).rstrip()[-1:] not in ('', '[', '{', ',', ':') and text[i + 1:].lstrip().startswith(('}', ']')):
            i += 1
        else:
            output.append(text[i])
            i += 1
    return ''.join(output)

def read_json(path, expected, comments=False):
    if path.is_symlink():
        raise ValueError("Refusing symbolic-link configuration: " + str(path))
    if not path.exists():
        return expected()
    text = path.read_text(encoding='utf-8-sig')
    value = json.loads(jsonc(text) if comments else text, object_pairs_hook=object_pairs)
    if not isinstance(value, expected):
        raise ValueError("Unexpected configuration structure: " + str(path))
    return value

def mapping(parent, key):
    value = parent.setdefault(key, {})
    if not isinstance(value, dict):
        raise ValueError("Expected an object for " + key)
    return value

def private_file(path):
    if os.name == 'nt':
        result = subprocess.run(['whoami', '/user', '/fo', 'csv', '/nh'], check=True, capture_output=True, text=True)
        sid = next(csv.reader(io.StringIO(result.stdout)))[1]
        subprocess.run(['icacls', str(path), '/inheritance:r', '/grant:r', '*' + sid + ':F'], check=True, capture_output=True)
    else:
        os.chmod(path, 0o600)

def stage(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, name = tempfile.mkstemp(prefix='.dgai-', dir=path.parent)
    temporary = Path(name)
    try:
        with os.fdopen(descriptor, 'wb') as stream:
            private_file(temporary)
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        return temporary
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise

def save_all(writes):
    staged = []
    try:
        for path, content in writes:
            if path.is_symlink():
                raise ValueError("Refusing symbolic-link configuration: " + str(path))
            old = path.read_bytes() if path.exists() else None
            staged.append((path, stage(path, content.encode('utf-8')), old))
        for path, _, old in staged:
            if old is not None:
                backup = path.with_name(path.name + '.bak.' + uuid.uuid4().hex)
                os.replace(stage(backup, old), backup)
                print("Backup: " + str(backup))
        for path, temporary, old in staged:
            current = path.read_bytes() if path.exists() else None
            if current != old:
                raise ValueError("Configuration changed during setup: " + str(path))
            os.replace(temporary, path)
            print("Configured: " + str(path))
    finally:
        for _, temporary, _ in staged:
            temporary.unlink(missing_ok=True)

def encoded(value):
    return json.dumps(value, ensure_ascii=True, indent=2) + '\n'

def positive(value, prompt):
    if type(value) is int and value > 0:
        return value
    value = input(prompt + ' (from the model documentation): ').strip()
    if not value.isdecimal() or int(value) <= 0:
        raise ValueError("A documented positive token limit is required")
    return int(value)

def capability(value, prompt):
    if value is not None:
        return value
    answer = input(prompt + ' [y/n]: ').strip().lower()
    if answer not in ('y', 'n'):
        raise ValueError("Answer y or n using the model documentation")
    return answer == 'y'

def main():
    if sys.version_info < (3, 11):
        raise ValueError("Python 3.11 or later is required")
    home = Path.home()
    kind, model, base = SETUP['kind'], SETUP['model'], SETUP['base']
    metadata = SETUP['metadata']
    app = {'codex': 'codex', 'claude': 'claude', 'omp': 'omp'}.get(kind)
    if app and not shutil.which(app):
        raise ValueError("Install " + app + " and add it to PATH before running setup")
    if kind in ('codex', 'omp') and os.name == 'nt' and Path(shutil.which(app)).suffix.lower() in ('.cmd', '.bat'):
        raise ValueError("Install the native agent executable on PATH; batch wrappers cannot safely receive model arguments")
    if kind == 'omp' and (os.environ.get('PI_CONFIG_DIR') or os.environ.get('OMP_CONFIG_DIR')):
        raise ValueError("Use the standard omp configuration root for the dgai profile")
    print("Close the selected app before continuing. No app or dependency will be installed.")
    input("Press Enter when the app is closed: ")
    writes = []
    launch = None
    if kind == 'claude':
        path = Path(os.environ.get('CLAUDE_CONFIG_DIR', str(home / '.claude'))) / 'settings.json'
        config = read_json(path, dict)
        env = mapping(config, 'env')
        env.update(ANTHROPIC_BASE_URL=base, ANTHROPIC_MODEL=model)
    elif kind == 'codex':
        existing = Path(os.environ.get('CODEX_HOME', str(home / '.codex'))) / 'config.toml'
        if existing.exists():
            tomllib.loads(existing.read_text(encoding='utf-8'))
        path = home / '.dgai' / 'codex.json'
        config = read_json(path, dict)
        config.update(model=model, base_url=base + '/v1')
    elif kind == 'omp':
        # JSON is valid YAML; the dedicated profile leaves all existing YAML untouched.
        root = home / '.omp' / 'profiles' / 'dgai' / 'agent'
        path = root / 'models.yml'
        if (root / 'models.yaml').exists() or (root / 'models.json').exists():
            raise ValueError("The dgai profile already has another model file; choose a different profile before setup")
        config = read_json(path, dict)
        provider = mapping(mapping(config, 'providers'), 'dgai')
        models = provider.setdefault('models', [])
        if not isinstance(models, list) or any(not isinstance(item, dict) for item in models):
            raise ValueError("Invalid omp model list")
        entry = next((item for item in models if item.get('id') == model), None)
        if entry is None:
            entry = {'id': model}
            models.append(entry)
        entry.update(contextWindow=positive(metadata.get('context_length'), 'Context window'), maxTokens=positive(metadata.get('max_output_tokens'), 'Maximum output tokens'), api=SETUP['ompApi'], baseUrl=base if SETUP['ompApi'] == 'anthropic-messages' else base + '/v1')
        provider.update(baseUrl=base + '/v1', apiKey='DGAI_API_KEY')
        if metadata.get('input_modalities'):
            entry['input'] = [item for item in metadata['input_modalities'] if item in ('text', 'image')]
        writes.append((path, encoded(config)))
        path = home / '.dgai' / 'omp.json'
        config = read_json(path, dict)
        config['model'] = model
    elif kind == 'zcode':
        path = home / '.zcode' / 'v2' / 'config.json'
        config = read_json(path, dict)
        provider = mapping(mapping(config, 'provider'), 'dgai')
        provider.update(name='dgai', kind='openai-compatible', source='custom')
        options = mapping(provider, 'options')
        options.update(baseURL=base + '/v1', apiKeyRequired=True)
        entry = mapping(mapping(provider, 'models'), model)
        mapping(entry, 'limit').update(context=positive(metadata.get('context_length'), 'Context window'), output=positive(metadata.get('max_output_tokens'), 'Maximum output tokens'))
    else:
        if sys.platform == 'win32':
            default = Path(os.environ['APPDATA']) / 'Code' / 'User' / 'chatLanguageModels.json'
        elif sys.platform == 'darwin':
            default = home / 'Library' / 'Application Support' / 'Code' / 'User' / 'chatLanguageModels.json'
        else:
            default = Path(os.environ.get('XDG_CONFIG_HOME', str(home / '.config'))) / 'Code' / 'User' / 'chatLanguageModels.json'
        chosen = input('VS Code chatLanguageModels.json path [' + str(default) + ']: ').strip()
        path = Path(chosen).expanduser() if chosen else default
        config = read_json(path, list, comments=True)
        if any(not isinstance(item, dict) for item in config):
            raise ValueError("Invalid VS Code provider list")
        provider = next((item for item in config if item.get('name') == 'dgai' and item.get('vendor') == 'customendpoint'), None)
        if provider is None:
            provider = {'name': 'dgai', 'vendor': 'customendpoint'}
            config.append(provider)
        models = provider.setdefault('models', [])
        if not isinstance(models, list) or any(not isinstance(item, dict) for item in models):
            raise ValueError("Invalid VS Code model list")
        entry = next((item for item in models if item.get('id') == model), None)
        if entry is None:
            entry = {'id': model}
            models.append(entry)
        context = positive(metadata.get('context_length'), 'Context window (input plus output)')
        output = positive(metadata.get('max_output_tokens'), 'Maximum output tokens')
        if output >= context:
            raise ValueError("Maximum output tokens must be smaller than the context window")
        capabilities = metadata.get('capabilities')
        tools = None if capabilities is None else any(item in capabilities for item in ('tools', 'function_calling'))
        vision = None if metadata.get('input_modalities') is None else 'image' in metadata['input_modalities']
        entry.update(name=model, url=base + SETUP['route'], apiType=SETUP['vsApi'], maxInputTokens=context - output, maxOutputTokens=output, toolCalling=capability(tools, 'Does this model support tool calling?'), vision=capability(vision, 'Does this model support image input?'))
        if SETUP['vsApi'] == 'responses':
            entry['statefulResponses'] = False
        # SecretStorage enrollment belongs to VS Code; preserve an existing input reference.
        if 'apiKey' in provider and not (isinstance(provider['apiKey'], str) and provider['apiKey'].startswith('$' + '{input:') and provider['apiKey'].endswith('}')):
            raise ValueError("Enroll the existing dgai API key through VS Code before running setup")
        writes.append((path, encoded(config)))
        save_all(writes)
        print("Open VS Code: Chat: Manage Language Models > dgai gear > enter your API key. VS Code stores it securely and creates its input reference. Select the dgai model afterwards.")
        print("Configuration saved; authentication and a real model response are not verified.")
        return
    warnings.simplefilter('error', getpass.GetPassWarning)
    key = getpass.getpass('dgai API key (hidden): ').strip()
    if not key or any(ord(char) < 32 or ord(char) == 127 for char in key):
        raise ValueError("A nonempty API key without control characters is required")
    if kind == 'claude':
        env['ANTHROPIC_AUTH_TOKEN'] = key
    elif kind == 'zcode':
        options['apiKey'] = key
    else:
        config['api_key'] = key
        launch = home / '.dgai' / ('launch-' + kind + '.py')
        launcher = "import json, os, pathlib, shutil, subprocess, sys\n"
        launcher += "config = json.loads(pathlib.Path(__file__).with_name(" + repr(kind + '.json') + ").read_text(encoding='utf-8'))\n"
        launcher += "env = dict(os.environ, DGAI_API_KEY=config['api_key'])\n"
        if kind == 'codex':
            launcher += "args = ['codex', '-c', 'model=' + json.dumps(config['model'], ensure_ascii=False), '-c', 'model_provider=\"dgai\"', '-c', 'model_providers.dgai=' + '{name=\"dgai\",base_url=' + json.dumps(config['base_url'], ensure_ascii=False) + ',wire_api=\"responses\",env_key=\"DGAI_API_KEY\"}']\n"
        else:
            launcher += "args = ['omp', '--profile', 'dgai', '--model', 'dgai/' + config['model']]\n"
        launcher += "executable = shutil.which(args[0])\n"
        launcher += "if not executable or (os.name == 'nt' and pathlib.Path(executable).suffix.lower() in ('.cmd', '.bat')): raise SystemExit('Install the native agent executable on PATH; batch wrappers cannot safely receive model arguments.')\n"
        launcher += "args[0] = executable\n"
        launcher += "raise SystemExit(subprocess.call(args + sys.argv[1:], env=env))\n"
        writes.append((launch, launcher))
    writes.append((path, encoded(config)))
    save_all(writes)
    if launch:
        command = [sys.executable, str(launch)]
        print("Start your agent from your project directory with:")
        print(subprocess.list2cmdline(command) if os.name == 'nt' else shlex.join(command))
    else:
        print("Restart the app and select your dgai model.")
    print("Configuration saved; no API request was sent and model access is not verified.")

if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print('Setup stopped: ' + str(error), file=sys.stderr)
        sys.exit(1)
`

export function createAgentSetup(
  kind: AgentKind,
  serverAddress: string,
  model: string,
  metadata: ModelMetadata = {}
): AgentSetup {
  const url = new URL(serverAddress)
  const localHttpHosts: Record<string, boolean> = {
    localhost: true,
    '127.0.0.1': true,
    '[::1]': true,
  }
  if (
    !['https:', 'http:'].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new Error(
      'Use an HTTP(S) server URL without credentials, query, or fragment.'
    )
  }
  if (url.protocol === 'http:' && !localHttpHosts[url.hostname]) {
    throw new Error(
      'Use HTTPS for remote servers; HTTP is allowed only on localhost.'
    )
  }
  const hasControlCharacter = [...model].some((character) => {
    const code = character.codePointAt(0) ?? 0
    return code === 0 || (code >= 1 && code <= 31) || code === 127
  })
  if (!model.trim() || hasControlCharacter) {
    throw new Error('Select a valid model.')
  }
  const base = url.href.replace(/\/+$/, '').replace(/\/v1$/, '')
  const endpoints = metadata.supported_endpoint_types ?? []
  let api: 'chat-completions' | 'responses' | 'messages' = 'chat-completions'
  if (kind === 'codex') {
    api = 'responses'
  }
  if (kind === 'claude') {
    api = 'messages'
  }
  if (
    (kind === 'omp' || kind === 'vscode') &&
    !endpoints.includes('openai') &&
    endpoints.includes('openai-response')
  ) {
    api = 'responses'
  }
  if (
    (kind === 'omp' || kind === 'vscode') &&
    !endpoints.includes('openai') &&
    !endpoints.includes('openai-response') &&
    endpoints.includes('anthropic')
  ) {
    api = 'messages'
  }
  let route = '/v1/chat/completions'
  if (api === 'responses') {
    route = '/v1/responses'
  }
  if (api === 'messages') {
    route = '/v1/messages'
  }
  let ompApi: 'openai-completions' | 'openai-responses' | 'anthropic-messages' =
    'openai-completions'
  if (api === 'responses') {
    ompApi = 'openai-responses'
  }
  if (api === 'messages') {
    ompApi = 'anthropic-messages'
  }
  const payload = { kind, model, base, metadata, route, vsApi: api, ompApi }
  const script =
    '#!/usr/bin/env python3\n# Requires Python 3.11+. Review this file before running it.\nimport json\nSETUP = json.loads(' +
    JSON.stringify(JSON.stringify(payload)) +
    ')\n' +
    INSTALLER
  const agent = AGENTS.find((item) => item.kind === kind)
  if (!agent) throw new Error('Unknown agent.')
  const filename = `setup-dgai-${kind}.py`
  const target = {
    codex: '~/.dgai/codex.json',
    claude: '~/.claude/settings.json',
    omp: '~/.omp/profiles/dgai/agent/models.yml',
    zcode: '~/.zcode/v2/config.json',
    vscode: 'VS Code User/chatLanguageModels.json',
  }[kind]
  const steps = [
    'Install the selected app and Python 3.11 or later before setup.',
    'Download and review the script, close the app, then run the command from your download folder.',
  ]
  if (kind === 'vscode') {
    steps.push(
      'Open Chat: Manage Language Models, choose the dgai gear, and enter your API key in VS Code.'
    )
  } else {
    steps.push(
      'Enter your API key in the hidden local prompt; it is never included in the download.'
    )
  }
  if (kind === 'codex' || kind === 'omp') {
    steps.push(
      'Use the launcher command printed by the script from your project directory.'
    )
  } else {
    steps.push(
      'Restart the app, select the dgai model, and send a short test prompt.'
    )
  }
  const notes = [
    'Existing files are backed up before atomic replacement. Invalid configuration stops setup without overwriting it.',
    'The script saves configuration only; it does not install apps or verify model access.',
    'On macOS or Linux, use python3 if python is not available.',
  ]
  if (kind === 'omp' || kind === 'zcode' || kind === 'vscode') {
    notes.push(
      'If catalog limits are unavailable, the script asks for documented context and output limits. It never guesses them.'
    )
  }
  if (kind === 'omp') {
    notes.push(
      'On Windows, install the native omp release with https://omp.sh/install.ps1 in PowerShell; npm .cmd wrappers are not supported by this launcher.'
    )
  }
  if (kind === 'codex') {
    notes.push(
      'On Windows, the launcher requires a native Codex executable on PATH; wrapper commands are not supported.'
    )
  }
  if (kind === 'codex') {
    notes.push(
      'The dedicated launcher supplies provider overrides without changing your existing Codex TOML settings.'
    )
  }
  if (kind === 'omp') {
    notes.push(
      'The dgai profile is isolated from your normal omp profile. Its model file uses JSON-compatible YAML; hand-written YAML in this profile is left untouched and stops setup.'
    )
  }
  if (kind === 'zcode') {
    notes.push(
      'Quit ZCode completely before setup; a running ZCode can overwrite external configuration changes.'
    )
  }
  if (kind === 'vscode') {
    notes.push(
      'VS Code must support Native Custom Endpoint. API key enrollment remains a one-time step inside VS Code secure storage.',
      'For a portable install or named VS Code profile, enter its chatLanguageModels.json path when prompted.',
      'Tool calling and vision are enabled only from catalog metadata or your answers based on model documentation.'
    )
  } else {
    notes.push(
      'Local credential files and backups are restricted to your user account. Keep them private.'
    )
  }
  const previewLimits: Record<string, number> = {}
  if (metadata.context_length !== undefined) {
    previewLimits.context = metadata.context_length
  }
  if (metadata.max_output_tokens !== undefined) {
    previewLimits.output = metadata.max_output_tokens
  }
  let preview: unknown
  if (kind === 'claude') {
    preview = {
      env: {
        ANTHROPIC_BASE_URL: base,
        ANTHROPIC_MODEL: model,
        ANTHROPIC_AUTH_TOKEN: '<enter locally>',
      },
    }
  } else if (kind === 'codex') {
    preview = {
      model,
      model_provider: 'dgai',
      model_providers: {
        dgai: {
          name: 'dgai',
          base_url: base + '/v1',
          wire_api: 'responses',
          env_key: 'DGAI_API_KEY',
        },
      },
    }
  } else if (kind === 'omp') {
    const ompModel: Record<string, unknown> = { id: model }
    if (metadata.context_length !== undefined) {
      ompModel.contextWindow = metadata.context_length
    }
    if (metadata.max_output_tokens !== undefined) {
      ompModel.maxTokens = metadata.max_output_tokens
    }
    ompModel.api = ompApi
    let ompBaseUrl = base + '/v1'
    if (api === 'messages') ompBaseUrl = base
    preview = {
      providers: {
        dgai: {
          baseUrl: ompBaseUrl,
          api: ompApi,
          apiKey: 'DGAI_API_KEY',
          models: [ompModel],
        },
      },
    }
  } else if (kind === 'zcode') {
    preview = {
      provider: {
        dgai: {
          name: 'dgai',
          kind: 'openai-compatible',
          source: 'custom',
          options: {
            baseURL: base + '/v1',
            apiKey: '<enter locally>',
            apiKeyRequired: true,
          },
          models: { [model]: { limit: previewLimits } },
        },
      },
    }
  } else {
    const vscodeModel: Record<string, unknown> = {
      id: model,
      name: model,
      url: base + route,
      apiType: api,
    }
    if (
      metadata.context_length !== undefined &&
      metadata.max_output_tokens !== undefined
    ) {
      vscodeModel.maxInputTokens =
        metadata.context_length - metadata.max_output_tokens
      vscodeModel.maxOutputTokens = metadata.max_output_tokens
    } else if (metadata.max_output_tokens !== undefined) {
      vscodeModel.maxOutputTokens = metadata.max_output_tokens
    }
    if (metadata.capabilities !== undefined) {
      vscodeModel.toolCalling =
        metadata.capabilities.includes('tools') ||
        metadata.capabilities.includes('function_calling')
    }
    if (metadata.input_modalities !== undefined) {
      vscodeModel.vision = metadata.input_modalities.includes('image')
    }
    if (api === 'responses') vscodeModel.statefulResponses = false
    preview = [
      { name: 'dgai', vendor: 'customendpoint', models: [vscodeModel] },
    ]
  }
  notes.push(
    'The configuration preview contains no credentials. Missing limits and capabilities must be completed from the model documentation; preserve existing settings when merging manually.'
  )
  let previewBody = JSON.stringify(preview, null, 2)
  let previewLanguage = 'json'
  let previewFilename = `dgai-${kind}-preview.json`
  if (kind === 'codex') {
    previewLanguage = 'toml'
    previewFilename = 'dgai-codex-preview.toml'
    previewBody = [
      `model = ${JSON.stringify(model)}`,
      'model_provider = "dgai"',
      '',
      '[model_providers.dgai]',
      'name = "dgai"',
      `base_url = ${JSON.stringify(base + '/v1')}`,
      'wire_api = "responses"',
      'env_key = "DGAI_API_KEY"',
    ].join('\n')
  }
  const snippets: AgentSetupSnippet[] = [
    {
      id: 'setup-script',
      title: 'Downloadable setup script',
      language: 'python',
      body: script,
      filename,
      download: true,
    },
    {
      id: 'configuration-preview',
      title: 'Configuration preview',
      language: previewLanguage,
      body: previewBody,
      filename: previewFilename,
    },
  ]
  return {
    kind,
    title: agent.title,
    summary: agent.summary,
    target,
    endpoint: base + route,
    steps,
    notes,
    script,
    filename,
    runCommand: `python ${filename}`,
    snippets,
  }
}
