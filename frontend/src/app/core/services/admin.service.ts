
import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';

export interface AdminUser {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  role: string;
  storage_quota: number;
  used_storage: number;
  available_storage: number;
}

export interface AdminQuotaRequest {
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
export class AdminService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = environment.usersApiUrl;

  getUsers(): Observable<AdminUser[]> {
    return this.http.get<AdminUser[]>(`${this.baseUrl}/admin/users`);
  }

  getQuotaRequests(): Observable<AdminQuotaRequest[]> {
    return this.http.get<AdminQuotaRequest[]>(
      `${this.baseUrl}/admin/quota-requests`
    );
  }

  updateQuotaRequest(
    id: string,
    status: 'approved' | 'rejected'
  ): Observable<AdminQuotaRequest> {
    return this.http.patch<AdminQuotaRequest>(
      `${this.baseUrl}/admin/quota-requests/${id}`,
      { status }
    );
  }
}
