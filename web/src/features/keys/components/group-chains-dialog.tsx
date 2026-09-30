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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Edit2, Link2, Plus, Trash2, X } from 'lucide-react'
import { Reorder } from 'motion/react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { AutoGroupOrderItem } from '@/components/auto-group-order-item'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getUserGroups } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import {
  createGroupChain,
  deleteGroupChain,
  getGroupChains,
  updateGroupChain,
} from '../api'
import type { GroupChain } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { GroupRatioBadge } from './auto-group-visuals'

const MAX_GROUPS_PER_CHAIN = 20

type GroupChainsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function GroupChainsDialog({
  open,
  onOpenChange,
}: GroupChainsDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [name, setName] = useState('')
  const [selectedGroups, setSelectedGroups] = useState<string[]>([])
  const [isSaving, setIsSaving] = useState(false)
  const [deletingChain, setDeletingChain] = useState<GroupChain | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const chainsQuery = useQuery({
    queryKey: ['group-chains'],
    queryFn: async () => requireServerSuccess(await getGroupChains()),
    enabled: open,
  })
  const groupsQuery = useQuery({
    queryKey: ['user-groups'],
    queryFn: async () => requireServerSuccess(await getUserGroups()),
    enabled: open,
    staleTime: 60_000,
  })

  const chains = chainsQuery.data?.data?.items ?? []
  const groupOptions = useMemo<ApiKeyGroupOption[]>(
    () =>
      Object.entries(groupsQuery.data?.data || {})
        .filter(([value]) => value !== 'auto')
        .map(([value, info]) => ({
          value,
          label: value,
          desc: info.desc || value,
          ratio: info.ratio,
        })),
    [groupsQuery.data]
  )
  const candidates = useMemo(
    () =>
      groupOptions.filter((option) => !selectedGroups.includes(option.value)),
    [groupOptions, selectedGroups]
  )

  const handleOpenChange = (value: boolean) => {
    if (!value) {
      setEditorOpen(false)
      setEditingId(null)
      setName('')
      setSelectedGroups([])
      setDeletingChain(null)
    }
    onOpenChange(value)
  }

  const startCreate = () => {
    setEditingId(null)
    setName('')
    setSelectedGroups([])
    setEditorOpen(true)
  }

  const startEdit = (chain: GroupChain) => {
    setEditingId(chain.id)
    setName(chain.name)
    setSelectedGroups([...chain.groups])
    setEditorOpen(true)
  }

  const resetEditor = () => {
    setEditorOpen(false)
    setEditingId(null)
    setName('')
    setSelectedGroups([])
  }

  const closeEditor = () => {
    if (isSaving) return
    resetEditor()
  }

  const handleSave = async () => {
    const trimmedName = name.trim()
    if (!trimmedName) {
      toast.error(t('Please enter a chain name'))
      return
    }
    if (selectedGroups.length === 0) {
      toast.error(t('Select at least one group'))
      return
    }
    if (selectedGroups.length > MAX_GROUPS_PER_CHAIN) {
      toast.error(
        t('A group chain can contain at most {{max}} groups', {
          max: MAX_GROUPS_PER_CHAIN,
        })
      )
      return
    }

    setIsSaving(true)
    try {
      const payload = { name: trimmedName, groups: selectedGroups }
      const result = editingId
        ? await updateGroupChain(editingId, payload)
        : await createGroupChain(payload)
      if (!result.success) {
        handleServerError(result, t('Failed to save group chain'))
        return
      }
      toast.success(
        t(editingId ? 'Group chain updated' : 'Group chain created')
      )
      await queryClient.invalidateQueries({ queryKey: ['group-chains'] })
      await queryClient.invalidateQueries({ queryKey: ['token-auto-groups'] })
      resetEditor()
    } catch (error) {
      handleServerError(error, t('Failed to save group chain'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deletingChain) return
    setIsDeleting(true)
    try {
      const result = await deleteGroupChain(deletingChain.id)
      if (!result.success) {
        handleServerError(result, t('Failed to delete group chain'))
        return
      }
      toast.success(t('Group chain deleted'))
      await queryClient.invalidateQueries({ queryKey: ['group-chains'] })
      await queryClient.invalidateQueries({ queryKey: ['token-auto-groups'] })
      if (editingId === deletingChain.id) resetEditor()
      setDeletingChain(null)
    } catch (error) {
      handleServerError(error, t('Failed to delete group chain'))
    } finally {
      setIsDeleting(false)
    }
  }

  const moveGroup = (index: number, direction: 'up' | 'down') => {
    const target = direction === 'up' ? index - 1 : index + 1
    if (target < 0 || target >= selectedGroups.length) return
    setSelectedGroups((groups) => {
      const next = [...groups]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Link2 className='size-4' />
              {t('Group chains')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'Create reusable fallback chains. The first group is tried first, then the remaining groups are tried in order.'
              )}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-3'>
            <div className='flex items-center justify-between gap-3'>
              <p className='text-muted-foreground text-xs'>
                {t('{{count}} / {{max}} chains', {
                  count: chains.length,
                  max: chainsQuery.data?.data?.limit ?? 10,
                })}
              </p>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={startCreate}
                disabled={
                  editorOpen ||
                  chains.length >= (chainsQuery.data?.data?.limit ?? 10)
                }
              >
                <Plus className='size-4' />
                {t('New chain')}
              </Button>
            </div>

            {chainsQuery.isLoading && (
              <p className='text-muted-foreground py-4 text-center text-sm'>
                {t('Loading...')}
              </p>
            )}
            {chainsQuery.isError && (
              <p className='text-destructive py-4 text-sm'>
                {t('Failed to load group chains')}
              </p>
            )}
            {!chainsQuery.isLoading &&
              !chainsQuery.isError &&
              chains.length === 0 && (
                <div className='rounded-lg border border-dashed p-6 text-center'>
                  <p className='font-medium'>{t('No group chains yet')}</p>
                  <p className='text-muted-foreground mt-1 text-sm'>
                    {t(
                      'Create a chain to reuse the same fallback order across API keys.'
                    )}
                  </p>
                </div>
              )}
            {chains.map((chain) => (
              <div
                key={chain.id}
                className='bg-muted/20 flex min-w-0 items-start gap-3 rounded-lg border p-3'
              >
                <div className='min-w-0 flex-1'>
                  <div className='flex items-center gap-2'>
                    <span className='truncate font-medium'>{chain.name}</span>
                    <span className='text-muted-foreground shrink-0 text-xs'>
                      {chain.groups.length} {t('groups')}
                    </span>
                    {chain.token_count > 0 && (
                      <span className='text-muted-foreground shrink-0 text-xs'>
                        · {chain.token_count} {t('API keys')}
                      </span>
                    )}
                  </div>
                  <p
                    className='text-muted-foreground mt-1 truncate text-xs'
                    title={chain.groups.join(' → ')}
                  >
                    {chain.groups.join(' → ')}
                  </p>
                </div>
                <div className='flex shrink-0 gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    aria-label={t('Edit {{name}}', { name: chain.name })}
                    onClick={() => startEdit(chain)}
                  >
                    <Edit2 className='size-4' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    aria-label={t('Delete {{name}}', { name: chain.name })}
                    onClick={() => setDeletingChain(chain)}
                  >
                    <Trash2 className='text-destructive size-4' />
                  </Button>
                </div>
              </div>
            ))}

            {editorOpen && (
              <div className='space-y-4 rounded-lg border p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <h3 className='font-medium'>
                    {t(editingId ? 'Edit group chain' : 'New group chain')}
                  </h3>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    onClick={closeEditor}
                    disabled={isSaving}
                    aria-label={t('Close')}
                  >
                    <X className='size-4' />
                  </Button>
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='group-chain-name'>{t('Chain name')}</Label>
                  <Input
                    id='group-chain-name'
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder={t('e.g. Primary fallback')}
                    maxLength={100}
                    disabled={isSaving}
                  />
                </div>
                <div className='space-y-2'>
                  <div className='flex items-center justify-between gap-3'>
                    <Label>{t('Groups in order')}</Label>
                    <span className='text-muted-foreground text-xs'>
                      {selectedGroups.length} / {MAX_GROUPS_PER_CHAIN}
                    </span>
                  </div>
                  <ApiKeyGroupCombobox
                    options={candidates}
                    onValueChange={(group) =>
                      setSelectedGroups((current) => [...current, group])
                    }
                    placeholder={
                      candidates.length === 0
                        ? t('No more groups available')
                        : t('Add a group')
                    }
                    disabled={
                      isSaving ||
                      selectedGroups.length >= MAX_GROUPS_PER_CHAIN ||
                      candidates.length === 0
                    }
                  />
                  {selectedGroups.length > 0 ? (
                    <Reorder.Group
                      axis='y'
                      values={selectedGroups}
                      onReorder={setSelectedGroups}
                      className='flex flex-col gap-2'
                    >
                      {selectedGroups.map((group, index) => {
                        const option = groupOptions.find(
                          (item) => item.value === group
                        )
                        return (
                          <AutoGroupOrderItem
                            key={group}
                            group={group}
                            index={index}
                            count={selectedGroups.length}
                            onMove={moveGroup}
                            onRemove={(value) =>
                              setSelectedGroups((current) =>
                                current.filter((item) => item !== value)
                              )
                            }
                            leading={<GroupRatioBadge ratio={option?.ratio} />}
                          />
                        )
                      })}
                    </Reorder.Group>
                  ) : (
                    <p className='text-muted-foreground rounded-lg border border-dashed p-3 text-sm'>
                      {t(
                        'Add at least one group, then arrange the fallback order.'
                      )}
                    </p>
                  )}
                </div>
                <div className='flex justify-end gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={closeEditor}
                    disabled={isSaving}
                  >
                    {t('Cancel')}
                  </Button>
                  <Button
                    type='button'
                    onClick={handleSave}
                    disabled={isSaving}
                  >
                    {isSaving ? t('Saving...') : t('Save')}
                  </Button>
                </div>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
            >
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deletingChain !== null}
        onOpenChange={(value) =>
          !value && !isDeleting && setDeletingChain(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete group chain?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This chain is used by {{count}} API keys. Deleting it will move those keys to automatic group selection.',
                { count: deletingChain?.token_count ?? 0 }
              )}{' '}
              <span className='font-medium'>{deletingChain?.name}</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={isDeleting}
              onClick={handleDelete}
            >
              {isDeleting ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
