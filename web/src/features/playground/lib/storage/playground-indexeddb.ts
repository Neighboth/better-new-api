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

/**
 * IndexedDB storage for playground image attachments.
 * Prevents LocalStorage QuotaExceededError by offloading base64 image data to IndexedDB.
 */

const DB_NAME = 'better-playground-media'
const STORE_NAME = 'images'
const DB_VERSION = 1

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB is not available in this browser'))
      return
    }
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME)
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('failed to open IndexedDB'))
  })
}

export async function putPlaygroundImages(images: Record<string, string>): Promise<void> {
  if (typeof indexedDB === 'undefined') return
  const db = await openDatabase()
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite')
      const store = tx.objectStore(STORE_NAME)
      for (const [id, url] of Object.entries(images)) {
        store.put(url, id)
      }
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error ?? new Error('failed to store images'))
      tx.onabort = () => reject(tx.error ?? new Error('failed to store images'))
    })
  } finally {
    db.close()
  }
}

export async function getPlaygroundImages(): Promise<Record<string, string>> {
  if (typeof indexedDB === 'undefined') return {}
  const db = await openDatabase()
  try {
    return await new Promise<Record<string, string>>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readonly')
      const store = tx.objectStore(STORE_NAME)
      const result: Record<string, string> = {}

      if ('getAllKeys' in store && 'getAll' in store) {
        const keysReq = store.getAllKeys()
        const valsReq = store.getAll()
        tx.oncomplete = () => {
          const keys = (keysReq.result as string[]) || []
          const vals = (valsReq.result as string[]) || []
          for (let i = 0; i < keys.length; i++) {
            result[keys[i]] = vals[i]
          }
          resolve(result)
        }
        tx.onerror = () => reject(tx.error ?? new Error('failed to read images'))
      } else {
        const cursorReq = store.openCursor()
        cursorReq.onsuccess = (e) => {
          const cursor = (e.target as IDBRequest<IDBCursorWithValue>).result
          if (cursor) {
            result[cursor.key as string] = cursor.value as string
            cursor.continue()
          } else {
            resolve(result)
          }
        }
        cursorReq.onerror = () => reject(cursorReq.error ?? new Error('failed to read images'))
      }
    })
  } finally {
    db.close()
  }
}

export async function clearPlaygroundImages(): Promise<void> {
  if (typeof indexedDB === 'undefined') return
  const db = await openDatabase()
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite')
      tx.objectStore(STORE_NAME).clear()
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error ?? new Error('failed to clear images'))
    })
  } finally {
    db.close()
  }
}
