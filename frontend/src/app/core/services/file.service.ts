import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { DriveFile, IntegrityResult } from '../models/drive.models';

/** Rad sa datotekama: otpremanje (1.5), nova verzija (1.5), preimenovanje (1.6), integritet (2.18). */
@Injectable({ providedIn: 'root' })
export class FileService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${environment.filesApiUrl}/files`;

  /** folderId null => koren ("Moj disk") */
  upload(file: File, folderId: string | null): Observable<DriveFile> {
    const form = new FormData();
    form.append('file', file, file.name);
    if (folderId) form.append('folderId', folderId);
    return this.http.post<DriveFile>(this.baseUrl, form);
  }

  uploadNewVersion(fileId: string, file: File): Observable<DriveFile> {
    const form = new FormData();
    form.append('file', file, file.name);
    return this.http.put<DriveFile>(`${this.baseUrl}/${encodeURIComponent(fileId)}`, form);
  }

  rename(fileId: string, name: string): Observable<DriveFile> {
    return this.http.patch<DriveFile>(`${this.baseUrl}/${encodeURIComponent(fileId)}`, { name });
  }

  verify(fileId: string): Observable<IntegrityResult> {
    return this.http.get<IntegrityResult>(`${this.baseUrl}/${encodeURIComponent(fileId)}/verify`);
  }
}

/**
 * Klijentska provera pre slanja (2.18 trazi validaciju i na klijentu i na
 * serveru). Ovo je samo za brzu povratnu informaciju korisniku - pravu,
 * obavezujucu proveru tipa po sadrzaju radi server.
 */
export const MAX_UPLOAD_BYTES = 50 * 1024 * 1024;
const BLOCKED_EXTENSIONS = ['html', 'htm', 'svg', 'xml', 'exe', 'bat', 'cmd', 'sh', 'js', 'msi', 'dll'];

export function validateUpload(file: File): string | null {
  if (file.size > MAX_UPLOAD_BYTES) return 'Datoteka je veća od 50 MB.';
  const ext = file.name.split('.').pop()?.toLowerCase() ?? '';
  if (BLOCKED_EXTENSIONS.includes(ext)) return `Tip datoteke .${ext} nije dozvoljen.`;
  return null;
}
