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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { CompanyHeader } from '@/components/company-header'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useAuthStore } from '@/stores/auth-store'

import {
  CONTACT_DEFAULT_VALUES,
  contactSchema,
  type ContactValues,
} from './lib/schema'

const INPUT_CLASS =
  'h-12 w-full rounded-lg border-violet-200 bg-white px-4 text-base data-[size=default]:h-12 dark:border-slate-700 dark:bg-slate-900'

export function Contact() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const form = useForm<ContactValues>({
    resolver: zodResolver(contactSchema),
    defaultValues: CONTACT_DEFAULT_VALUES,
    mode: 'onTouched',
  })
  const message = form.watch('message')
  const textFields = [
    {
      name: 'name',
      label: t('How should we address you?'),
      required: true,
      maxLength: 100,
      autoComplete: 'name',
    },
    {
      name: 'email',
      label: t('Contact email'),
      required: true,
      maxLength: 254,
      autoComplete: 'email',
      type: 'email',
    },
    {
      name: 'company',
      label: t('Company name'),
      maxLength: 200,
      autoComplete: 'organization',
    },
  ] as const
  const selectFields = [
    {
      name: 'companySize',
      label: t('Company size'),
      options: [
        ['1-10', t('1-10 employees')],
        ['11-50', t('11-50 employees')],
        ['51-200', t('51-200 employees')],
        ['201-1000', t('201-1000 employees')],
        ['1000+', t('1000+ employees')],
      ],
    },
    {
      name: 'industry',
      label: t('Industry'),
      options: [
        ['technology', t('Technology')],
        ['finance', t('Finance')],
        ['education', t('Education')],
        ['healthcare', t('Healthcare')],
        ['retail', t('Retail')],
        ['other', t('Other')],
      ],
    },
    {
      name: 'country',
      label: t('Country / region'),
      options: [
        ['china', t('China')],
        ['united-states', t('United States')],
        ['japan', t('Japan')],
        ['singapore', t('Singapore')],
        ['other', t('Other')],
      ],
    },
    {
      name: 'stage',
      label: t('Purchase evaluation stage'),
      required: true,
      wide: true,
      options: [
        ['exploring', t('Exploring')],
        ['evaluating', t('Evaluating solutions')],
        ['ready', t('Ready to purchase')],
        ['using', t('Already using the service')],
      ],
    },
    {
      name: 'monthlyTokens',
      label: t('Estimated monthly token usage'),
      required: true,
      wide: true,
      options: [
        ['under-1m', t('Under 1M tokens')],
        ['1m-10m', t('1M-10M tokens')],
        ['10m-100m', t('10M-100M tokens')],
        ['100m-1b', t('100M-1B tokens')],
        ['over-1b', t('Over 1B tokens')],
        ['unsure', t('Not sure yet')],
      ],
    },
  ] as const
  const sourceOptions = [
    ['search', t('Search engine')],
    ['referral', t('Referral')],
    ['social', t('Social media')],
    ['event', t('Event')],
    ['other', t('Other')],
  ]

  return (
    <div className='min-h-screen bg-[#faf8ff] text-slate-900 dark:bg-slate-950 dark:text-white'>
      <CompanyHeader user={user} />
      <main className='mx-auto max-w-[1180px] px-6 py-10 md:px-12'>
        <h1 className='text-3xl font-semibold'>{t('Contact us')}</h1>
        <p className='mt-3 text-sm text-slate-600 dark:text-slate-300'>
          {t('Tell us about your needs and our team will get back to you.')}
        </p>
        <Alert className='my-8 border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950'>
          <AlertDescription id='contact-availability'>
            {t('Online submissions are not available yet. Please email us.')}{' '}
            <a
              href='mailto:brook-cc@outlook.com'
              className='font-medium underline underline-offset-4'
            >
              brook-cc@outlook.com
            </a>
          </AlertDescription>
        </Alert>
        <Form {...form}>
          <form
            className='grid grid-cols-1 gap-x-8 gap-y-6 md:grid-cols-2 lg:grid-cols-3'
            noValidate
            onSubmit={(event) => event.preventDefault()}
          >
            {textFields.map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem
                    className={
                      item.name === 'company'
                        ? 'lg:col-start-1 lg:row-start-2'
                        : undefined
                    }
                  >
                    <FormLabel className='text-base'>
                      {item.label}
                      {'required' in item && (
                        <span className='text-red-600'> *</span>
                      )}
                    </FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        autoComplete={item.autoComplete}
                        maxLength={item.maxLength}
                        type={'type' in item ? item.type : 'text'}
                        placeholder={t('Please enter')}
                        className={INPUT_CLASS}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
            <div className='flex min-w-0 gap-2 lg:col-start-3 lg:row-start-1'>
              <FormField
                control={form.control}
                name='phoneCode'
                render={({ field }) => (
                  <FormItem className='w-24 shrink-0'>
                    <FormLabel>{t('Calling code')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className={INPUT_CLASS}>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {['+86', '+1', '+44', '+81', '+65', '+852'].map(
                            (code) => (
                              <SelectItem key={code} value={code}>
                                {code}
                              </SelectItem>
                            )
                          )}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='phone'
                render={({ field }) => (
                  <FormItem className='min-w-0 flex-1'>
                    <FormLabel className='text-base'>
                      {t('Contact phone')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='tel'
                        autoComplete='tel-national'
                        maxLength={40}
                        placeholder={t('Please enter')}
                        className={INPUT_CLASS}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            {selectFields.map((item) => (
              <FormField
                key={item.name}
                control={form.control}
                name={item.name}
                render={({ field }) => (
                  <FormItem
                    className={
                      'wide' in item ? 'md:col-span-2 lg:col-span-3' : undefined
                    }
                  >
                    <FormLabel className='text-base'>
                      {item.label}
                      {'required' in item && (
                        <span className='text-red-600'> *</span>
                      )}
                    </FormLabel>
                    <Select
                      items={item.options.map(([value, label]) => ({
                        value,
                        label,
                      }))}
                      value={field.value || null}
                      onValueChange={(value) => field.onChange(value ?? '')}
                      onOpenChange={(open) => {
                        if (!open) field.onBlur()
                      }}
                    >
                      <FormControl>
                        <SelectTrigger className={INPUT_CLASS}>
                          <SelectValue placeholder={t('Please select')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {item.options.map(([value, label]) => (
                            <SelectItem key={value} value={value}>
                              {label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
            <FormField
              control={form.control}
              name='message'
              render={({ field }) => (
                <FormItem className='md:col-span-2 lg:col-span-3'>
                  <FormLabel className='text-base'>
                    {t('Tell us about your needs')}
                    <span className='text-red-600'> *</span>
                  </FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      maxLength={500}
                      placeholder={t(
                        'Describe your use case, model needs, or other questions'
                      )}
                      className='min-h-48 rounded-lg border-violet-200 bg-white p-4 text-base dark:border-slate-700 dark:bg-slate-900'
                    />
                  </FormControl>
                  <p className='text-right text-sm text-slate-500'>
                    {message.length} / 500
                  </p>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='source'
              render={({ field }) => (
                <FormItem className='md:col-span-2 lg:col-span-3'>
                  <FormLabel className='text-base'>
                    {t('How did you hear about us?')}
                  </FormLabel>
                  <Select
                    items={sourceOptions.map(([value, label]) => ({
                      value,
                      label,
                    }))}
                    value={field.value || null}
                    onValueChange={(value) => field.onChange(value ?? '')}
                  >
                    <FormControl>
                      <SelectTrigger className={INPUT_CLASS}>
                        <SelectValue placeholder={t('Please select')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectGroup>
                        {sourceOptions.map(([value, label]) => (
                          <SelectItem key={value} value={value}>
                            {label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </FormItem>
              )}
            />
            <div className='flex gap-4 md:col-span-2 lg:col-span-3'>
              <Button
                type='submit'
                disabled
                aria-describedby='contact-availability'
                size='lg'
                className='min-w-32'
              >
                {t('Submit')}
              </Button>
              <Button
                type='button'
                size='lg'
                variant='outline'
                onClick={() => form.reset()}
              >
                {t('Cancel')}
              </Button>
            </div>
          </form>
        </Form>
      </main>
    </div>
  )
}
