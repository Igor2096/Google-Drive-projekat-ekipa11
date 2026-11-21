import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { Folder, FolderContents } from '../models/drive.models';

@Injectable({ providedIn: 'root' })
export class FolderService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = `${environment.filesApiUrl}/folders`;

  /** null => korenski folder ("Moj disk") */
  getContents(folderId: string | null): Observable<FolderContents> {
    const id = folderId ?? 'root';
    return this.http.get<FolderContents>(`${this.baseUrl}/${encodeURIComponent(id)}/contents`);
  }

  create(name: string, parentId: string | null): Observable<Folder> {
    return this.http.post<Folder>(this.baseUrl, { name, parentId: parentId ?? '' });
  }

  rename(folderId: string, name: string): Observable<Folder> {
    return this.http.patch<Folder>(`${this.baseUrl}/${encodeURIComponent(folderId)}`, { name });
  }
}
