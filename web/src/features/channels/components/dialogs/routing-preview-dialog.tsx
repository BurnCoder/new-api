/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { Loader2, Route } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getRoutingPreview } from '../../api'
import type { RoutingPreview } from '../../types'

type RoutingPreviewDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type PreviewParams = {
  group: string
  model: string
  request_path?: string
  responses_websocket?: boolean
}

export function RoutingPreviewDialog({
  open,
  onOpenChange,
}: RoutingPreviewDialogProps) {
  const { t } = useTranslation()
  const [group, setGroup] = useState('default')
  const [model, setModel] = useState('')
  const [requestPath, setRequestPath] = useState('')
  const [responsesWebsocket, setResponsesWebsocket] = useState(false)
  const [submitted, setSubmitted] = useState<PreviewParams | null>(null)

  const query = useQuery({
    queryKey: ['routing-preview', submitted],
    queryFn: async () => {
      if (!submitted) throw new Error('Preview parameters are missing')
      return requireServerSuccess(await getRoutingPreview(submitted)).data
    },
    enabled: open && submitted !== null,
    retry: false,
    meta: { errorToast: false },
  })

  const preview = query.data as RoutingPreview | undefined
  const handlePreview = () => {
    const next: PreviewParams = {
      group: group.trim(),
      model: model.trim(),
    }
    if (requestPath.trim()) next.request_path = requestPath.trim()
    if (responsesWebsocket) next.responses_websocket = true
    setSubmitted(next)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) setSubmitted(null)
    onOpenChange(nextOpen)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Route Preview')}
      description={t(
        'Inspect priority tiers and weighted channel candidates without calling an upstream provider.'
      )}
      contentClassName='sm:max-w-3xl'
      footer={
        <>
          <Button variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Close')}
          </Button>
          <Button
            onClick={handlePreview}
            disabled={!group.trim() || !model.trim() || query.isFetching}
          >
            {query.isFetching && <Loader2 className='animate-spin' />}
            {t('Preview Route')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='route-preview-group'>{t('Group')}</Label>
            <Input
              id='route-preview-group'
              value={group}
              onChange={(event) => setGroup(event.target.value)}
              placeholder='default'
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='route-preview-model'>{t('Model')}</Label>
            <Input
              id='route-preview-model'
              value={model}
              onChange={(event) => setModel(event.target.value)}
              placeholder='gpt-4o'
            />
          </div>
        </div>

        <div className='space-y-1.5'>
          <Label htmlFor='route-preview-path'>{t('Request path (optional)')}</Label>
          <Input
            id='route-preview-path'
            value={requestPath}
            onChange={(event) => setRequestPath(event.target.value)}
            placeholder='/v1/chat/completions'
          />
        </div>

        <div className='flex items-center justify-between rounded-lg border p-3'>
          <Label htmlFor='route-preview-websocket'>
            {t('Responses WebSocket only')}
          </Label>
          <Switch
            id='route-preview-websocket'
            checked={responsesWebsocket}
            onCheckedChange={setResponsesWebsocket}
          />
        </div>

        {query.error && (
          <Alert variant='destructive'>
            <AlertDescription>
              {query.error instanceof Error
                ? query.error.message
                : t('Failed to preview route')}
            </AlertDescription>
          </Alert>
        )}

        {preview && <RoutingPreviewResult preview={preview} />}
      </div>
    </Dialog>
  )
}

function RoutingPreviewResult({ preview }: { preview: RoutingPreview }) {
  const { t } = useTranslation()
  const percent = new Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 1,
  })

  return (
    <div className='space-y-3'>
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        <Route className='size-4' />
        <span>
          {t('Strategy')}: {preview.strategy} · {t('Source')}: {preview.source}
        </span>
      </div>
      {preview.unavailable_reason ? (
        <Alert>
          <AlertDescription>
            {t('No available channel')}: {preview.unavailable_reason}
          </AlertDescription>
        </Alert>
      ) : (
        preview.tiers.map((tier) => (
          <div key={tier.priority} className='rounded-lg border'>
            <div className='bg-muted/40 flex items-center justify-between border-b px-3 py-2 text-sm font-medium'>
              <span>
                {t('Priority')} {tier.priority}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Effective weight')}: {tier.total_effective_weight}
              </span>
            </div>
            <div className='divide-y'>
              {tier.channels.map((channel) => (
                <div
                  key={channel.channel_id}
                  className='grid grid-cols-[1fr_auto_auto] items-center gap-3 px-3 py-2 text-sm'
                >
                  <span className='truncate'>
                    #{channel.channel_id} {channel.channel_name || t('Unnamed channel')}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {t('Weight')} {channel.weight} → {channel.effective_weight}
                  </span>
                  <span className='font-medium'>
                    {percent.format(channel.probability ?? 0)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        ))
      )}
      {preview.excluded && preview.excluded.length > 0 && (
        <div className='text-muted-foreground text-xs'>
          {t('Excluded candidates')}: {preview.excluded.length}
        </div>
      )}
      <div className='text-muted-foreground text-xs'>
        {t('Effective weight uses weight + 10; priority selects the failover tier.')}
      </div>
    </div>
  )
}
