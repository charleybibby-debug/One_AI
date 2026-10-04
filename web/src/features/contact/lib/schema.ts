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
import { z } from 'zod'

export const contactSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Please complete the required fields')
    .max(100),
  email: z.email('Invalid email address').max(254),
  phoneCode: z.enum(['+86', '+1', '+44', '+81', '+65', '+852']),
  phone: z.string().trim().max(40),
  company: z.string().trim().max(200),
  companySize: z.string().max(40),
  industry: z.string().max(40),
  country: z.string().max(40),
  stage: z.string().min(1, 'Please complete the required fields'),
  monthlyTokens: z.string().min(1, 'Please complete the required fields'),
  message: z
    .string()
    .trim()
    .min(1, 'Please complete the required fields')
    .max(500),
  source: z.string().max(40),
})
export type ContactValues = z.infer<typeof contactSchema>
export const CONTACT_DEFAULT_VALUES: ContactValues = {
  name: '',
  email: '',
  phoneCode: '+86',
  phone: '',
  company: '',
  companySize: '',
  industry: '',
  country: '',
  stage: '',
  monthlyTokens: '',
  message: '',
  source: '',
}
