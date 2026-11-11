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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'

import {
  VietQRSettingsSection,
  type VietQRSettingsValues,
} from '../vietqr-settings-section'

const initial: VietQRSettingsValues = {
  VietQREnabled: false,
  VietQRBankCode: 'TPB',
  VietQRBankAccount: '10006460824',
  VietQRAccountName: 'NGUYEN TRI DUNG',
  VietQRUsername: '',
  VietQRPassword: '',
  VietQRSyncUsername: '',
  VietQRSyncPassword: '',
  VietQRSyncSecretKey: '',
  VietQRSandbox: true,
  VietQRVNDRate: 26500,
  VietQRMinTopUp: 1,
  VietQRAPIBaseURL: 'https://dev.vietqr.org',
}

function Fixture(props: { onChange: (values: VietQRSettingsValues) => void }) {
  const [values, setValues] = useState(initial)
  return (
    <VietQRSettingsSection
      values={values}
      onValueChange={(key, value) => {
        setValues((previous) => {
          const next = { ...previous, [key]: value }
          props.onChange(next)
          return next
        })
      }}
    />
  )
}

describe('VietQR settings section', () => {
  it('enables the payment method from the switch', async () => {
    const changes: VietQRSettingsValues[] = []
    const user = userEvent.setup()
    render(<Fixture onChange={(v) => changes.push(v)} />)

    await user.click(
      screen.getByRole('switch', { name: 'Enable VietQR top-up' })
    )

    expect(changes.at(-1)?.VietQREnabled).toBe(true)
  })

  it('upper-cases the bank code so the QR URL stays valid', async () => {
    const changes: VietQRSettingsValues[] = []
    const user = userEvent.setup()
    render(<Fixture onChange={(v) => changes.push(v)} />)

    const bankCode = screen.getByRole('textbox', { name: 'Bank code' })
    await user.clear(bankCode)
    await user.type(bankCode, 'bidv')

    expect(changes.at(-1)?.VietQRBankCode).toBe('BIDV')
  })

  it('keeps every credential input masked', () => {
    render(<Fixture onChange={() => undefined} />)

    for (const name of [
      'Merchant password',
      'Sync password',
      'Sync secret key',
    ]) {
      expect(screen.getByLabelText(name)).toHaveAttribute('type', 'password')
    }
  })
})
