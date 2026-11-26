
import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';

export interface QuotaInfo {
  storage_quota: number;
  used_storage: number;
  available_storage: number;
}

export interface QuotaRequest {
  id: string;
  user_id: string;
  requested_quota: number;
  status: 'pending' | 'approved' | 'rejected';
  created_at: string;
  updated_at: string;
}

@Injectable({
  providedIn: 'root',
})
export class QuotaService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = environment.usersApiUrl;

  getMyQuota(): Observable<QuotaInfo> {
    return this.http.get<QuotaInfo>(
      `${this.baseUrl}/users/me/quota`,
    );
  }

  createRequest(requestedQuota: number): Observable<QuotaRequest> {
    return this.http.post<QuotaRequest>(
      `${this.baseUrl}/quota-requests`,
      {
        requested_quota: requestedQuota,
      },
    );
  }
}
