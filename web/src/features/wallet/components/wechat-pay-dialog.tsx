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
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

type WeChatPayDialogProps = {
  codeUrl: string
  onClose: () => void
}

export function WeChatPayDialog(props: WeChatPayDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={Boolean(props.codeUrl)} onOpenChange={props.onClose}>
      <DialogContent className='sm:max-w-sm'>
        <DialogHeader>
          <DialogTitle>{t('Scan with WeChat to pay')}</DialogTitle>
          <DialogDescription>
            {t(
              'After payment succeeds, close this dialog and refresh the wallet balance.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='flex justify-center rounded-xl bg-white p-5'>
          {props.codeUrl ? (
            <QRCodeSVG value={props.codeUrl} size={240} level='M' />
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}
