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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  FileText,
  Folder,
  FolderPlus,
  Lock,
  Pencil,
  Shield,
  Trash2,
  Upload,
  Eye,
  Save,
} from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'

function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 B'
  if (bytes < 0) return `-${formatBytes(-bytes, decimals)}`
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}

export type ManagedFileItem = {
  id: number
  path: string
  name: string
  is_dir: boolean
  size: number
  password?: string
  enable_captcha?: boolean
  created_at: number
  updated_at: number
}

export function SystemFilesSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [currentFolder, setCurrentFolder] = useState('')
  const [newFolderOpen, setNewFolderOpen] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')

  const [editorOpen, setEditorOpen] = useState(false)
  const [editorPath, setEditorPath] = useState('')
  const [editorContent, setEditorContent] = useState('')
  const [loadingContent, setLoadingContent] = useState(false)

  const [protOpen, setProtOpen] = useState(false)
  const [protPath, setProtPath] = useState('')
  const [protPassword, setProtPassword] = useState('')
  const [protCaptcha, setProtCaptcha] = useState(false)

  const [renameOpen, setRenameOpen] = useState(false)
  const [renameOldPath, setRenameOldPath] = useState('')
  const [renameNewName, setRenameNewName] = useState('')

  const filesQuery = useQuery({
    queryKey: ['admin-managed-files'],
    queryFn: async () => {
      const res = await api.get('/api/files/')
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Failed')
      }
      return res.data.data as ManagedFileItem[]
    },
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['admin-managed-files'] })

  const handleUpload = async (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    const targetPath = currentFolder ? `${currentFolder}/${file.name}` : file.name
    formData.append('path', targetPath)
    formData.append('is_dir', 'false')

    try {
      const res = await api.post('/api/files/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      if (res.data?.success) {
        toast.success(t('File uploaded'))
        invalidate()
      } else {
        toast.error(res.data?.message || t('Upload failed'))
      }
    } catch {
      toast.error(t('Upload failed'))
    }
  }

  const handleCreateFolder = async () => {
    if (!newFolderName.trim()) return
    const targetPath = currentFolder ? `${currentFolder}/${newFolderName.trim()}` : newFolderName.trim()
    const formData = new FormData()
    formData.append('path', targetPath)
    formData.append('is_dir', 'true')

    try {
      const res = await api.post('/api/files/upload', formData)
      if (res.data?.success) {
        toast.success(t('Folder created'))
        setNewFolderOpen(false)
        setNewFolderName('')
        invalidate()
      } else {
        toast.error(res.data?.message || t('Failed to create folder'))
      }
    } catch {
      toast.error(t('Failed to create folder'))
    }
  }

  const handleOpenEdit = async (path: string) => {
    setEditorPath(path)
    setLoadingContent(true)
    setEditorOpen(true)
    try {
      const res = await api.get(`/api/files/content?path=${encodeURIComponent(path)}`)
      if (res.data?.success) {
        setEditorContent(res.data.data)
      } else {
        toast.error(res.data?.message || t('Failed to load file content'))
      }
    } catch {
      toast.error(t('Failed to load file content'))
    } finally {
      setLoadingContent(false)
    }
  }

  const handleSaveContent = async () => {
    try {
      const res = await api.post('/api/files/content', {
        path: editorPath,
        content: editorContent,
      })
      if (res.data?.success) {
        toast.success(t('File saved'))
        setEditorOpen(false)
        invalidate()
      } else {
        toast.error(res.data?.message || t('Failed to save file'))
      }
    } catch {
      toast.error(t('Failed to save file'))
    }
  }

  const handleOpenProt = (item: ManagedFileItem) => {
    setProtPath(item.path)
    setProtPassword(item.password || '')
    setProtCaptcha(Boolean(item.enable_captcha))
    setProtOpen(true)
  }

  const handleSaveProt = async () => {
    try {
      const res = await api.post('/api/files/settings', {
        path: protPath,
        password: protPassword,
        enable_captcha: protCaptcha,
      })
      if (res.data?.success) {
        toast.success(t('Protection settings saved'))
        setProtOpen(false)
        invalidate()
      } else {
        toast.error(res.data?.message || t('Failed to update protection'))
      }
    } catch {
      toast.error(t('Failed to update protection'))
    }
  }

  const handleDelete = async (path: string) => {
    if (!window.confirm(t('Delete this file/folder?'))) return
    try {
      const res = await api.delete(`/api/files/?path=${encodeURIComponent(path)}`)
      if (res.data?.success) {
        toast.success(t('Deleted'))
        invalidate()
      } else {
        toast.error(res.data?.message || t('Delete failed'))
      }
    } catch {
      toast.error(t('Delete failed'))
    }
  }

  const handleRename = async () => {
    if (!renameNewName.trim()) return
    const dir = renameOldPath.includes('/') ? renameOldPath.substring(0, renameOldPath.lastIndexOf('/')) : ''
    const newPath = dir ? `${dir}/${renameNewName.trim()}` : renameNewName.trim()

    try {
      const res = await api.post('/api/files/rename', {
        old_path: renameOldPath,
        new_path: newPath,
      })
      if (res.data?.success) {
        toast.success(t('Renamed'))
        setRenameOpen(false)
        invalidate()
      } else {
        toast.error(res.data?.message || t('Rename failed'))
      }
    } catch {
      toast.error(t('Rename failed'))
    }
  }

  const allItems = filesQuery.data ?? []
  const isCurrentFolderReadOnly = currentFolder === 'uploads' || currentFolder.startsWith('uploads/')

  // Filter items in current folder
  const currentItems = allItems.filter((item) => {
    if (!currentFolder) {
      return !item.path.includes('/')
    }
    if (!item.path.startsWith(`${currentFolder}/`)) return false
    const sub = item.path.substring(currentFolder.length + 1)
    return !sub.includes('/')
  })

  return (
    <SettingsSection title={t('System Files Manager')}>
      <div className='space-y-4'>
        <div className='flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2 font-mono text-sm'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => setCurrentFolder('')}
            >
              /
            </Button>
            {currentFolder.split('/').filter(Boolean).map((part, idx, arr) => {
              const subPath = arr.slice(0, idx + 1).join('/')
              return (
                <div key={subPath} className='flex items-center gap-1'>
                  <span>/</span>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={() => setCurrentFolder(subPath)}
                  >
                    {part}
                  </Button>
                </div>
              )
            })}
            {isCurrentFolderReadOnly && (
              <span className='ml-2 rounded bg-amber-500/10 px-2 py-0.5 text-xs text-amber-600 dark:text-amber-400'>
                {t('Read-Only')}
              </span>
            )}
          </div>

          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={isCurrentFolderReadOnly}
              onClick={() => setNewFolderOpen(true)}
            >
              <FolderPlus className='me-1 h-4 w-4' />
              {t('New Folder')}
            </Button>
            <Button
              type='button'
              size='sm'
              disabled={isCurrentFolderReadOnly}
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload className='me-1 h-4 w-4' />
              {t('Upload File')}
            </Button>
          </div>
        </div>

        <Card>
          <CardContent className='p-0'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('Path / Link')}</TableHead>
                  <TableHead>{t('Size')}</TableHead>
                  <TableHead>{t('Protection')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {currentItems.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className='text-muted-foreground text-center py-6'>
                      {t('Folder is empty.')}
                    </TableCell>
                  </TableRow>
                )}
                {currentItems.map((item) => (
                  <TableRow key={item.path}>
                    <TableCell className='font-medium'>
                      <div className='flex items-center gap-2'>
                        {item.is_dir ? (
                          <Folder className='h-4 w-4 text-blue-500' />
                        ) : (
                          <FileText className='h-4 w-4 text-zinc-500' />
                        )}
                        {item.is_dir ? (
                          <button
                            type='button'
                            className='hover:underline font-semibold text-left'
                            onClick={() => setCurrentFolder(item.path)}
                          >
                            {item.name}
                          </button>
                        ) : (
                          <span>{item.name}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <a
                        href={`/${item.path}`}
                        target='_blank'
                        rel='noreferrer'
                        className='text-blue-600 hover:underline dark:text-blue-400'
                      >
                        /{item.path}
                      </a>
                    </TableCell>
                    <TableCell className='text-xs tabular-nums'>
                      {item.is_dir ? '-' : formatBytes(item.size)}
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-1.5 text-xs'>
                        {item.password && (
                          <span className='inline-flex items-center gap-1 rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-600 dark:text-amber-400'>
                            <Lock className='h-3 w-3' />
                            {t('Password')}
                          </span>
                        )}
                        {item.enable_captcha && (
                          <span className='inline-flex items-center gap-1 rounded bg-indigo-500/10 px-1.5 py-0.5 text-indigo-600 dark:text-indigo-400'>
                            <Shield className='h-3 w-3' />
                            {t('Captcha')}
                          </span>
                        )}
                        {!item.password && !item.enable_captcha && (
                          <span className='text-muted-foreground'>{t('Public')}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        {!item.is_dir && (
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            onClick={() => void handleOpenEdit(item.path)}
                            title={item.path.startsWith('uploads') ? t('View Content') : t('Edit / Preview Content')}
                          >
                            <Eye className='h-4 w-4' />
                          </Button>
                        )}
                        {!item.path.startsWith('uploads') && (
                          <>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              onClick={() => handleOpenProt(item)}
                              title={t('Security & Password')}
                            >
                              <Lock className='h-4 w-4' />
                            </Button>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              onClick={() => {
                                setRenameOldPath(item.path)
                                setRenameNewName(item.name)
                                setRenameOpen(true)
                              }}
                              title={t('Rename')}
                            >
                              <Pencil className='h-4 w-4' />
                            </Button>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              onClick={() => void handleDelete(item.path)}
                              title={t('Delete')}
                            >
                              <Trash2 className='h-4 w-4' />
                            </Button>
                          </>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      <input
        type='file'
        ref={fileInputRef}
        className='hidden'
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) void handleUpload(file)
          e.target.value = ''
        }}
      />

      {/* Create Folder Dialog */}
      <Dialog open={newFolderOpen} onOpenChange={setNewFolderOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Create Folder')}</DialogTitle>
          </DialogHeader>
          <div className='space-y-3 py-2'>
            <Label>{t('Folder Name')}</Label>
            <Input
              value={newFolderName}
              placeholder='documents'
              onChange={(e) => setNewFolderName(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setNewFolderOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={() => void handleCreateFolder()}>
              {t('Create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Editor & Preview Dialog */}
      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className='max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Edit File')} - /{editorPath}</DialogTitle>
          </DialogHeader>
          <div className='space-y-3 py-2'>
            {loadingContent ? (
              <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
            ) : (
              <Textarea
                rows={16}
                value={editorContent}
                onChange={(e) => setEditorContent(e.target.value)}
                className='font-mono text-xs'
              />
            )}
          </div>
          <DialogFooter className='flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between'>
            <Button
              type='button'
              variant='outline'
              className='w-full sm:w-auto'
              onClick={() => window.open(`/${editorPath}`, '_blank')}
            >
              <Eye className='me-1 h-4 w-4' />
              {t('Preview in new tab')}
            </Button>
            <div className='flex w-full items-center justify-end gap-2 sm:w-auto'>
              <Button variant='outline' className='flex-1 sm:flex-none' onClick={() => setEditorOpen(false)}>
                {t('Cancel')}
              </Button>
              <Button className='flex-1 sm:flex-none' onClick={() => void handleSaveContent()}>
                <Save className='me-1 h-4 w-4' />
                {t('Save File')}
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Protection Settings Dialog */}
      <Dialog open={protOpen} onOpenChange={setProtOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Protection & Password')} - /{protPath}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4 py-2'>
            <div className='space-y-2'>
              <Label>{t('Password Protection')}</Label>
              <Input
                type='password'
                value={protPassword}
                placeholder={t('Leave empty for public access')}
                onChange={(e) => setProtPassword(e.target.value)}
              />
            </div>
            <div className='flex items-center justify-between'>
              <div>
                <Label>{t('Enable Captcha Challenge')}</Label>
                <p className='text-muted-foreground text-xs'>
                  {t('Require captcha verification before serving this file or folder.')}
                </p>
              </div>
              <Switch
                checked={protCaptcha}
                onCheckedChange={setProtCaptcha}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setProtOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={() => void handleSaveProt()}>
              {t('Save Settings')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename Dialog */}
      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Rename')} - {renameOldPath}</DialogTitle>
          </DialogHeader>
          <div className='space-y-3 py-2'>
            <Label>{t('New Name')}</Label>
            <Input
              value={renameNewName}
              onChange={(e) => setRenameNewName(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRenameOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={() => void handleRename()}>
              {t('Rename')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
