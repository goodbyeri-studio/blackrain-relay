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

For commercial licensing, please contact support@quantumnous.com
*/
import { Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

type ChannelModelRatiosEditorProps = {
  models: string[]
  value: Record<string, number>
  onChange: (value: Record<string, number>) => void
  disabled?: boolean
}

const MAX_MODEL_RATIO = 1_000_000

export function ChannelModelRatiosEditor(props: ChannelModelRatiosEditorProps) {
  const { t } = useTranslation()
  const [selectedModelValue, setSelectedModelValue] = useState(
    props.models[0] || ''
  )
  const [ratioInput, setRatioInput] = useState('')
  const [error, setError] = useState('')
  const selectedModel = props.models.includes(selectedModelValue)
    ? selectedModelValue
    : props.models[0] || ''
  const selectedRatio = props.value[selectedModel]

  const configuredRatios = useMemo(
    () =>
      Object.entries(props.value).filter(([model]) =>
        props.models.includes(model)
      ),
    [props.models, props.value]
  )

  useEffect(() => {
    setRatioInput(selectedRatio === undefined ? '' : String(selectedRatio))
    setError('')
  }, [selectedModel, selectedRatio])

  const handleSave = () => {
    const ratio = Number(ratioInput)
    if (!ratioInput.trim() || !Number.isFinite(ratio)) {
      setError(t('Please enter a valid ratio'))
      return
    }
    if (ratio < 0 || ratio > MAX_MODEL_RATIO) {
      setError(t('Ratio must be between 0 and 1000000'))
      return
    }
    setError('')
    props.onChange({ ...props.value, [selectedModel]: ratio })
  }

  const handleRemove = (model: string) => {
    const next = { ...props.value }
    delete next[model]
    props.onChange(next)
    if (model === selectedModel) {
      setRatioInput('')
    }
  }

  return (
    <div className='border-border/60 rounded-lg border p-4'>
      <FieldSet>
        <FieldLegend variant='label'>{t('Channel Model Ratios')}</FieldLegend>
        <FieldDescription>
          {t(
            'Set a channel-specific model ratio. Models without an override use the global ratio.'
          )}
        </FieldDescription>

        {props.models.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('Select models above before configuring ratios.')}
          </p>
        ) : (
          <>
            <FieldGroup className='gap-3 sm:flex-row sm:items-end'>
              <Field className='min-w-0 flex-1'>
                <FieldLabel htmlFor='channel-model-ratio-model'>
                  {t('Model')}
                </FieldLabel>
                <Select
                  value={selectedModel}
                  onValueChange={(value) =>
                    value && setSelectedModelValue(value)
                  }
                  disabled={props.disabled}
                >
                  <SelectTrigger
                    id='channel-model-ratio-model'
                    className='w-full'
                  >
                    <SelectValue placeholder={t('Select a model')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {props.models.map((model) => (
                        <SelectItem key={model} value={model}>
                          {model}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field className='sm:w-36' data-invalid={Boolean(error)}>
                <FieldLabel htmlFor='channel-model-ratio-value'>
                  {t('Ratio')}
                </FieldLabel>
                <Input
                  id='channel-model-ratio-value'
                  type='number'
                  min={0}
                  max={MAX_MODEL_RATIO}
                  step='any'
                  value={ratioInput}
                  onChange={(event) => setRatioInput(event.target.value)}
                  placeholder='1'
                  disabled={props.disabled}
                  aria-invalid={Boolean(error)}
                  aria-describedby={
                    error ? 'channel-model-ratio-error' : undefined
                  }
                />
                <FieldError id='channel-model-ratio-error'>{error}</FieldError>
              </Field>
              <Button
                type='button'
                variant='outline'
                onClick={handleSave}
                disabled={props.disabled || !selectedModel}
              >
                {t('Save ratio')}
              </Button>
            </FieldGroup>

            {configuredRatios.length > 0 && (
              <div className='flex flex-col gap-2'>
                <p className='text-muted-foreground text-xs font-medium'>
                  {t('Configured overrides')}
                </p>
                <div className='divide-border/60 rounded-md border'>
                  {configuredRatios.map(([model, ratio]) => (
                    <div
                      key={model}
                      className='flex items-center justify-between gap-3 border-b px-3 py-2 text-sm last:border-b-0'
                    >
                      <span className='min-w-0 truncate font-mono'>
                        {model}
                      </span>
                      <div className='flex shrink-0 items-center gap-2'>
                        <span className='text-muted-foreground tabular-nums'>
                          {ratio}
                        </span>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type='button'
                                variant='ghost'
                                size='icon-sm'
                                onClick={() => handleRemove(model)}
                                disabled={props.disabled}
                                aria-label={t('Remove ratio override')}
                              />
                            }
                          >
                            <HugeiconsIcon
                              icon={Delete02Icon}
                              strokeWidth={2}
                              aria-hidden='true'
                            />
                          </TooltipTrigger>
                          <TooltipContent>
                            {t('Remove ratio override')}
                          </TooltipContent>
                        </Tooltip>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </FieldSet>
    </div>
  )
}
