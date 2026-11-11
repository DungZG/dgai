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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { SettingsSwitchField } from '../components/settings-form-layout'

export interface VietQRSettingsValues {
  VietQREnabled: boolean
  VietQRBankCode: string
  VietQRBankAccount: string
  VietQRAccountName: string
  VietQRUsername: string
  VietQRPassword: string
  VietQRSyncUsername: string
  VietQRSyncPassword: string
  VietQRSyncSecretKey: string
  VietQRSandbox: boolean
  VietQRVNDRate: number
  VietQRMinTopUp: number
  VietQRAPIBaseURL: string
}

interface Props {
  values: VietQRSettingsValues
  onValueChange: <K extends keyof VietQRSettingsValues>(
    key: K,
    value: VietQRSettingsValues[K]
  ) => void
}

export function VietQRSettingsSection({ values, onValueChange }: Props) {
  const { t } = useTranslation()

  return (
    <div className='space-y-4 pt-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('VietQR Bank Transfer')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Customers scan a VietQR code and transfer the exact amount. Without merchant credentials only the QR image is issued and an administrator must confirm each top-up.'
          )}
        </p>
      </div>
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Automatic reconciliation requires a VietQR merchant connection. VietQR currently links only MB Bank and BIDV accounts; other banks work in manual confirmation mode.'
          )}
        </AlertDescription>
      </Alert>

      <div className='grid gap-4 sm:grid-cols-2'>
        <SettingsSwitchField
          controlId='vietqr-enabled'
          checked={values.VietQREnabled}
          onCheckedChange={(v) => onValueChange('VietQREnabled', v)}
          label={t('Enable VietQR top-up')}
          className='py-0'
        />
        <SettingsSwitchField
          controlId='vietqr-sandbox'
          checked={values.VietQRSandbox}
          onCheckedChange={(v) => onValueChange('VietQRSandbox', v)}
          label={t('Sandbox mode')}
          description={t('Sandbox skips automatic settlement polling')}
          className='py-0'
        />
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-bank-code'>{t('Bank code')}</Label>
          <Input
            id='vietqr-bank-code'
            value={values.VietQRBankCode}
            placeholder='TPB'
            onChange={(event) =>
              onValueChange('VietQRBankCode', event.target.value.toUpperCase())
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-bank-account'>{t('Bank account number')}</Label>
          <Input
            id='vietqr-bank-account'
            value={values.VietQRBankAccount}
            onChange={(event) =>
              onValueChange('VietQRBankAccount', event.target.value.trim())
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-account-name'>{t('Account holder name')}</Label>
          <Input
            id='vietqr-account-name'
            value={values.VietQRAccountName}
            placeholder='NGUYEN VAN A'
            onChange={(event) =>
              onValueChange('VietQRAccountName', event.target.value)
            }
          />
        </div>
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-vnd-rate'>{t('VND per USD')}</Label>
          <Input
            id='vietqr-vnd-rate'
            type='number'
            min={0}
            step={100}
            value={values.VietQRVNDRate}
            onChange={(event) =>
              onValueChange(
                'VietQRVNDRate',
                event.target.value === '' ? 0 : event.target.valueAsNumber
              )
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-min-topup'>{t('Minimum top-up amount')}</Label>
          <Input
            id='vietqr-min-topup'
            type='number'
            min={1}
            value={values.VietQRMinTopUp}
            onChange={(event) =>
              onValueChange(
                'VietQRMinTopUp',
                event.target.value === '' ? 1 : event.target.valueAsNumber
              )
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-api-base-url'>{t('VietQR API base URL')}</Label>
          <Input
            id='vietqr-api-base-url'
            value={values.VietQRAPIBaseURL}
            placeholder='https://api.vietqr.org'
            onChange={(event) =>
              onValueChange('VietQRAPIBaseURL', event.target.value.trim())
            }
          />
        </div>
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-merchant-username'>{t('Merchant username')}</Label>
          <Input
            id='vietqr-merchant-username'
            value={values.VietQRUsername}
            onChange={(event) =>
              onValueChange('VietQRUsername', event.target.value.trim())
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-merchant-password'>{t('Merchant password')}</Label>
          <Input
            id='vietqr-merchant-password'
            type='password'
            value={values.VietQRPassword}
            onChange={(event) =>
              onValueChange('VietQRPassword', event.target.value)
            }
          />
        </div>
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-sync-username'>{t('Sync username')}</Label>
          <Input
            id='vietqr-sync-username'
            value={values.VietQRSyncUsername}
            onChange={(event) =>
              onValueChange('VietQRSyncUsername', event.target.value.trim())
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-sync-password'>{t('Sync password')}</Label>
          <Input
            id='vietqr-sync-password'
            type='password'
            value={values.VietQRSyncPassword}
            onChange={(event) =>
              onValueChange('VietQRSyncPassword', event.target.value)
            }
          />
        </div>
        <div className='grid gap-1.5'>
          <Label htmlFor='vietqr-sync-secret'>{t('Sync secret key')}</Label>
          <Input
            id='vietqr-sync-secret'
            type='password'
            value={values.VietQRSyncSecretKey}
            onChange={(event) =>
              onValueChange('VietQRSyncSecretKey', event.target.value)
            }
          />
        </div>
      </div>

      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'The sync credentials are the Basic auth pair you give VietQR so it can fetch a token and push transactions; they must differ from the merchant username and password.'
          )}
        </AlertDescription>
      </Alert>
    </div>
  )
}
