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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getAdminBannedNodes, unbanAdminNode } from '../../byog/api'
import type { ByogNodeBan } from '../../byog/types'

export function ByogBannedNodesTab() {
  const { t } = useTranslation()
  const [bannedNodes, setBannedNodes] = useState<ByogNodeBan[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedBan, setSelectedBan] = useState<ByogNodeBan | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [unbanningId, setUnbanningId] = useState<number | null>(null)

  useEffect(() => {
    loadBannedNodes()
  }, [])

  async function loadBannedNodes() {
    try {
      setLoading(true)
      const data = await getAdminBannedNodes()
      setBannedNodes(data || [])
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.load_banned_failed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleUnban(id: number) {
    if (!window.confirm(t('byog.admin.confirm_unban'))) return
    try {
      setUnbanningId(id)
      await unbanAdminNode(id)
      toast.success(t('byog.admin.unban_success'))
      if (detailsOpen) setDetailsOpen(false)
      loadBannedNodes()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.unban_failed'))
    } finally {
      setUnbanningId(null)
    }
  }

  function handleOpenDetails(ban: ByogNodeBan) {
    setSelectedBan(ban)
    setDetailsOpen(true)
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle>
                {t('byog.admin.banned_nodes_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.banned_nodes_desc')}
              </CardDescription>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={loadBannedNodes}
            >
              {t('byog.refresh')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className='text-muted-foreground flex h-32 items-center justify-center'>
              {t('byog.loading')}
            </div>
          ) : bannedNodes.length === 0 ? (
            <div className='text-muted-foreground py-12 text-center'>
              {t('byog.admin.no_banned_nodes')}
            </div>
          ) : (
            <div className='overflow-x-auto rounded-md border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('byog.admin.node_id')}</TableHead>
                    <TableHead>{t('byog.admin.user_id')}</TableHead>
                    <TableHead>{t('byog.admin.violation_code')}</TableHead>
                    <TableHead>{t('byog.admin.ban_reason')}</TableHead>
                    <TableHead>{t('byog.admin.banned_at')}</TableHead>
                    <TableHead className='text-right'>
                      {t('byog.admin.actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {bannedNodes.map((ban) => (
                    <TableRow key={ban.id}>
                      <TableCell className='font-mono text-xs'>
                        {ban.node_id ? `${ban.node_id.slice(0, 14)}...` : 'N/A'}
                      </TableCell>
                      <TableCell>#{ban.user_id}</TableCell>
                      <TableCell>
                        <Badge
                          variant='destructive'
                          className='font-mono text-[11px]'
                        >
                          {ban.violation_code}
                        </Badge>
                      </TableCell>
                      <TableCell
                        className='max-w-xs truncate text-xs'
                        title={ban.ban_reason}
                      >
                        {ban.ban_reason}
                      </TableCell>
                      <TableCell className='text-muted-foreground text-xs'>
                        {new Date(ban.banned_at * 1000).toLocaleString()}
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex items-center justify-end gap-1.5'>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 px-2 text-xs'
                            onClick={() => handleOpenDetails(ban)}
                          >
                            {t('byog.admin.evidence')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 px-2 text-xs text-emerald-500 hover:bg-emerald-500/10 hover:text-emerald-600'
                            disabled={unbanningId === ban.id}
                            onClick={() => handleUnban(ban.id)}
                          >
                            {t('byog.admin.unban')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Forensic Evidence Modal */}
      <Dialog open={detailsOpen} onOpenChange={setDetailsOpen}>
        <DialogContent className='max-h-[85vh] max-w-2xl overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {t('byog.admin.forensic_evidence_title')}
            </DialogTitle>
            <DialogDescription className='font-mono text-xs'>
              Node: {selectedBan?.node_id} • HWID: {selectedBan?.hwid}
            </DialogDescription>
          </DialogHeader>

          {selectedBan && (
            <div className='space-y-4 py-2'>
              <div className='bg-destructive/5 space-y-2 rounded-xl border p-4'>
                <div className='flex items-center justify-between'>
                  <span className='text-destructive text-sm font-semibold'>
                    {t('byog.admin.violation_code')}
                  </span>
                  <Badge variant='destructive'>
                    {selectedBan.violation_code}
                  </Badge>
                </div>
                <div className='text-muted-foreground text-xs'>
                  {selectedBan.ban_reason}
                </div>
              </div>

              {selectedBan.test_prompt && (
                <div className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-semibold uppercase'>
                    {t('byog.admin.canary_prompt_sent')}
                  </div>
                  <div className='bg-muted/20 rounded-lg border p-3 font-mono text-xs'>
                    {selectedBan.test_prompt}
                  </div>
                </div>
              )}

              <div className='grid grid-cols-2 gap-4'>
                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.expected_logprob')}
                  </div>
                  <div className='font-mono font-medium text-emerald-500'>
                    {selectedBan.expected_logprob?.toFixed(4)} (±
                    {selectedBan.tolerance})
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.actual_logprob_produced')}
                  </div>
                  <div className='text-destructive font-mono font-medium'>
                    {selectedBan.actual_logprob?.toFixed(4)}
                  </div>
                </div>
              </div>

              {selectedBan.detailed_logs && (
                <div className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-semibold uppercase'>
                    {t('byog.admin.detailed_logs')}
                  </div>
                  <div className='max-h-48 overflow-y-auto rounded-lg border bg-black/80 p-3 font-mono text-[11px] whitespace-pre-wrap text-emerald-400'>
                    {selectedBan.detailed_logs}
                  </div>
                </div>
              )}
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setDetailsOpen(false)}>
              {t('byog.close')}
            </Button>
            {selectedBan && (
              <Button
                variant='destructive'
                onClick={() => handleUnban(selectedBan.id)}
              >
                {t('byog.admin.unban_node')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
