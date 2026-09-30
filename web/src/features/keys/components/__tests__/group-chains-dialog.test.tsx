/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  createGroupChain: vi.fn(),
  deleteGroupChain: vi.fn(),
  getGroupChains: vi.fn(),
  getUserGroups: vi.fn(),
  updateGroupChain: vi.fn(),
}))

vi.mock('../../api', () => ({
  createGroupChain: mocks.createGroupChain,
  deleteGroupChain: mocks.deleteGroupChain,
  getGroupChains: mocks.getGroupChains,
  updateGroupChain: mocks.updateGroupChain,
}))

vi.mock('@/lib/api', () => ({
  getUserGroups: mocks.getUserGroups,
}))

const { GroupChainsDialog } = await import('../group-chains-dialog')

const translations = {
  'A group chain can contain at most {{max}} groups':
    '分组链最多包含 {{max}} 个分组',
  'API keys': 'API 密钥',
  'Add a group': '添加分组',
  'Add at least one group, then arrange the fallback order.':
    '至少添加一个分组，然后排列回退顺序。',
  'Chain name': '分组链名称',
  'Create a chain to reuse the same fallback order across API keys.':
    '创建分组链，在多个 API 密钥之间复用相同的回退顺序。',
  'Create reusable fallback chains. The first group is tried first, then the remaining groups are tried in order.':
    '创建可复用的回退链。系统先尝试第一个分组，再按顺序尝试其余分组。',
  'Group chains': '分组链',
  'New chain': '新建分组链',
  '{{count}} / {{max}} chains': '{{count}} / {{max}} 个分组链',
  groups: '个分组',
  Close: '关闭',
} as const

async function createI18n() {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'zhCN',
    resources: { zhCN: { translation: translations } },
  })
  return i18n
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return createI18n().then((i18n) =>
    render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <GroupChainsDialog open onOpenChange={() => undefined} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  )
}

beforeEach(() => {
  mocks.getGroupChains.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 1,
          user_id: 7,
          name: 'test',
          groups: [
            '【Claude】【非自营】CC纯享分组（可外接，蒸馏）',
            '【Claude】【自营】仅限Claude code使用，禁止外接【不允许破限】',
          ],
          token_count: 2,
          created_time: 0,
          updated_time: 0,
        },
      ],
      total: 1,
      limit: 10,
    },
  })
  mocks.getUserGroups.mockResolvedValue({
    success: true,
    data: { default: { desc: '默认分组', ratio: 1 } },
  })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('GroupChainsDialog layout and translation', () => {
  test('uses the wide responsive dialog and keeps long chain details readable in Chinese', async () => {
    await renderDialog()

    const dialog = await screen.findByRole('dialog', { name: '分组链' })
    expect(dialog).toHaveClass('w-[calc(100%-2rem)]', '!max-w-4xl')
    expect(dialog).toHaveTextContent(
      '创建可复用的回退链。系统先尝试第一个分组，再按顺序尝试其余分组。'
    )
    await screen.findByText('test')
    expect(dialog).toHaveTextContent(
      '【Claude】【自营】仅限Claude code使用，禁止外接【不允许破限】'
    )
    expect(dialog).toHaveTextContent('2 个分组')
  })
})
