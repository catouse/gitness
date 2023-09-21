/*
 * Copyright 2023 Harness, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useEffect, useMemo, useState } from 'react'
import { stringSubstitute } from '@harnessio/uicore'
import { Color } from '@harnessio/design-system'
import type { GitInfoProps } from 'utils/GitUtils'
import { useStrings } from 'framework/strings'
import { useListStatusCheckResults } from 'services/code'
import { ExecutionState } from 'components/ExecutionStatus/ExecutionStatus'

export function usePRChecksDecision({
  repoMetadata,
  pullRequestMetadata
}: Partial<Pick<GitInfoProps, 'repoMetadata' | 'pullRequestMetadata'>>) {
  const { data, error, refetch } = useListStatusCheckResults({
    repo_ref: `${repoMetadata?.path as string}/+`,
    commit_sha: pullRequestMetadata?.source_sha as string,
    lazy: !repoMetadata?.path || !pullRequestMetadata?.source_sha
  })
  const [count, setCount] = useState(DEFAULT_COUNTS)
  const { getString } = useStrings()
  const [color, setColor] = useState<Color>(Color.GREEN_500)
  const [background, setBackground] = useState<Color>(Color.GREEN_50)
  const [message, setMessage] = useState('')
  const [complete, setComplete] = useState(true)
  const status = useMemo(() => {
    let _status: ExecutionState | undefined
    const _count = { ...DEFAULT_COUNTS }
    const total = data?.length

    if (total) {
      for (const check of data) {
        switch (check.status) {
          case ExecutionState.ERROR:
          case ExecutionState.FAILURE:
          case ExecutionState.RUNNING:
          case ExecutionState.PENDING:
          case ExecutionState.SUCCESS:
            _count[check.status]++
            setCount({ ..._count })
            break
          default:
            console.error('Unrecognized PR check status', check) // eslint-disable-line no-console
            break
        }
      }

      if (_count.error) {
        _status = ExecutionState.ERROR
        setColor(Color.RED_900)
        setBackground(Color.RED_50)
        setMessage(stringSubstitute(getString('prChecks.error'), { count: _count.error, total }) as string)
      } else if (_count.failure) {
        _status = ExecutionState.FAILURE
        setColor(Color.RED_900)
        setBackground(Color.RED_50)
        setMessage(stringSubstitute(getString('prChecks.failure'), { count: _count.failure, total }) as string)
      } else if (_count.killed) {
        _status = ExecutionState.KILLED
        setColor(Color.RED_900)
        setBackground(Color.RED_50)
        setMessage(stringSubstitute(getString('prChecks.killed'), { count: _count.killed, total }) as string)
      } else if (_count.running) {
        _status = ExecutionState.RUNNING
        setColor(Color.ORANGE_900)
        setBackground(Color.ORANGE_100)
        setMessage(stringSubstitute(getString('prChecks.running'), { count: _count.running, total }) as string)
      } else if (_count.pending) {
        _status = ExecutionState.PENDING
        setColor(Color.GREY_600)
        setBackground(Color.GREY_100)
        setMessage(stringSubstitute(getString('prChecks.pending'), { count: _count.pending, total }) as string)
      } else if (_count.skipped) {
        _status = ExecutionState.SKIPPED
        setColor(Color.GREY_600)
        setBackground(Color.GREY_100)
        setMessage(stringSubstitute(getString('prChecks.skipped'), { count: _count.skipped, total }) as string)
      } else if (_count.success) {
        _status = ExecutionState.SUCCESS
        setColor(Color.GREEN_800)
        setBackground(Color.GREEN_50)
        setMessage(stringSubstitute(getString('prChecks.success'), { count: _count.success, total }) as string)
      }

      setComplete(!_count.pending && !_count.running)
    } else {
      setComplete(false)
    }

    return _status
  }, [data]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const pollingFn = () => {
      if (repoMetadata?.path && pullRequestMetadata?.source_sha && !complete) {
        refetch().then(() => {
          interval = window.setTimeout(pollingFn, POLLING_INTERVAL)
        })
      }
    }
    let interval = window.setTimeout(pollingFn, POLLING_INTERVAL)
    return () => window.clearTimeout(interval)
  }, [repoMetadata?.path, pullRequestMetadata?.source_sha, complete]) // eslint-disable-line react-hooks/exhaustive-deps

  return {
    overallStatus: status,
    count,
    error,
    data,
    color,
    background,
    message
  }
}

export type PRChecksDecisionResult = ReturnType<typeof usePRChecksDecision>

const POLLING_INTERVAL = 10000

const DEFAULT_COUNTS = {
  error: 0,
  failure: 0,
  pending: 0,
  running: 0,
  success: 0,
  skipped: 0,
  killed: 0
}
